package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	logDir        string
	stateFile     string
	ampBinaryPath string
	onWorkerExit  func(workerID string) // Callback when worker exits
	onLogLine     func(LogLine)         // Callback for log lines
	tailers       map[string]*LogTailer // Active log tailers by worker ID
	tailersMu     sync.RWMutex          // Protects tailers map
}

func NewManager(logDir string) *Manager {
	if logDir == "" {
		logDir = "./logs"
	}

	// Ensure log directory exists
	os.MkdirAll(logDir, 0755)

	return &Manager{
		logDir:        logDir,
		stateFile:     filepath.Join(logDir, "workers.json"),
		ampBinaryPath: "amp", // Assume amp is in PATH
		onWorkerExit:  nil,   // Will be set via SetExitCallback
		onLogLine:     nil,   // Will be set via SetLogCallback
		tailers:       make(map[string]*LogTailer),
	}
}

// SetExitCallback sets the callback function to be called when a worker exits
func (m *Manager) SetExitCallback(callback func(workerID string)) {
	m.onWorkerExit = callback
}

// SetLogCallback sets the callback function to be called for each log line
func (m *Manager) SetLogCallback(callback func(LogLine)) {
	m.onLogLine = callback
}

func (m *Manager) StartWorker(message string) error {
	// Create new thread
	threadID, err := m.createThread()
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}

	// Generate worker ID
	workerID := uuid.New().String()[:8]

	// Setup log file
	logFile := filepath.Join(m.logDir, fmt.Sprintf("worker-%s.log", workerID))

	// Create the command to pipe message to amp
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"echo %q | %s threads continue %s",
		message, m.ampBinaryPath, threadID,
	))

	// Set the process group ID so we can kill the entire group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Capture both stdout and stderr to the log file
	logFileHandle, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle

	// Start the process
	if err := cmd.Start(); err != nil {
		logFileHandle.Close()
		return fmt.Errorf("failed to start worker: %w", err)
	}

	worker := &Worker{
		ID:       workerID,
		ThreadID: threadID,
		PID:      cmd.Process.Pid,
		LogFile:  logFile,
		Started:  time.Now(),
		Status:   "running",
	}

	// Save worker state
	if err := m.saveWorker(worker); err != nil {
		// Kill the process if we can't save state
		cmd.Process.Kill()
		logFileHandle.Close()
		return fmt.Errorf("failed to save worker state: %w", err)
	}

	// Start log tailer if callback is set
	if m.onLogLine != nil {
		tailer := NewLogTailer(logFile, worker.ID, m.onLogLine)
		if err := tailer.Start(context.Background()); err == nil {
			m.tailersMu.Lock()
			m.tailers[worker.ID] = tailer
			m.tailersMu.Unlock()
		}
	}

	// Monitor the process in the background
	m.MonitorWorkerExit(worker.ID, cmd, func(workerID string) {
		// Stop log tailer when worker exits
		m.stopLogTailer(workerID)
		
		// Call the exit callback if set
		if m.onWorkerExit != nil {
			m.onWorkerExit(workerID)
		}
	})

	// Close log file after starting monitoring
	go func() {
		defer logFileHandle.Close()
		cmd.Wait()
	}()

	return nil
}

func (m *Manager) StopWorker(workerID string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	if worker.Status != "running" {
		return fmt.Errorf("worker %s is not running", workerID)
	}

	// Kill the process group to ensure we kill both bash and amp processes
	// First try to kill the entire process group
	if err := syscall.Kill(-worker.PID, syscall.SIGTERM); err != nil {
		// If process group kill fails, try individual process
		process, findErr := os.FindProcess(worker.PID)
		if findErr != nil {
			return fmt.Errorf("failed to find process %d: %w", worker.PID, findErr)
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			// Try SIGKILL if SIGTERM fails
			if killErr := process.Kill(); killErr != nil {
				return fmt.Errorf("failed to kill process %d: %w", worker.PID, killErr)
			}
		}
	}

	// Also try to kill any remaining amp processes for this thread
	m.killAmpProcesses(worker.ThreadID)

	// Stop log tailer
	m.stopLogTailer(workerID)

	// Update worker status
	worker.Status = "stopped"
	workers[workerID] = worker

	if err := m.saveWorkers(workers); err != nil {
		return fmt.Errorf("failed to update worker state: %w", err)
	}

	return nil
}

func (m *Manager) ContinueWorker(workerID, message string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Check if process is actually running
	if worker.Status == "running" && !m.checkProcessStatus(worker) {
		worker.Status = "stopped"
		workers[workerID] = worker
		m.saveWorkers(workers)
	}

	if worker.Status != "running" {
		return fmt.Errorf("worker %s is not running", workerID)
	}

	// Send message to the thread and append output to existing log file
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"echo %q | %s threads continue %s",
		message, m.ampBinaryPath, worker.ThreadID,
	))

	// Append to existing log file
	logFile, err := os.OpenFile(worker.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to continue worker: %w", err)
	}

	return nil
}

func (m *Manager) ListWorkers() ([]*Worker, error) {
	workers, err := m.loadWorkers()
	if err != nil {
		return nil, err
	}

	// Update status for all workers by checking actual process status
	updated := false
	for id, worker := range workers {
		if worker.Status == "running" && !m.checkProcessStatus(worker) {
			worker.Status = "stopped"
			workers[id] = worker
			updated = true
		}
	}

	// Save updated statuses if any changed
	if updated {
		m.saveWorkers(workers)
	}

	// Convert map to slice
	result := make([]*Worker, 0, len(workers))
	for _, worker := range workers {
		result = append(result, worker)
	}

	return result, nil
}

func (m *Manager) createThread() (string, error) {
	cmd := exec.Command(m.ampBinaryPath, "threads", "new")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to create thread: %w", err)
	}

	threadID := strings.TrimSpace(string(output))
	if !strings.HasPrefix(threadID, "T-") {
		return "", fmt.Errorf("unexpected thread ID format: %s", threadID)
	}

	return threadID, nil
}

func (m *Manager) loadWorkers() (map[string]*Worker, error) {
	workers := make(map[string]*Worker)

	file, err := os.Open(m.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return workers, nil // Return empty map if file doesn't exist
		}
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return workers, nil
	}

	if err := json.Unmarshal(data, &workers); err != nil {
		return nil, err
	}

	return workers, nil
}

func (m *Manager) saveWorkers(workers map[string]*Worker) error {
	data, err := json.MarshalIndent(workers, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.stateFile, data, 0644)
}

func (m *Manager) saveWorker(worker *Worker) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	workers[worker.ID] = worker
	return m.saveWorkers(workers)
}



func (m *Manager) checkProcessStatus(worker *Worker) bool {
	process, err := os.FindProcess(worker.PID)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (m *Manager) killAmpProcesses(threadID string) {
	// Use pkill to find and kill any amp processes for this thread
	cmd := exec.Command("pkill", "-f", fmt.Sprintf("amp threads continue %s", threadID))
	cmd.Run() // Ignore errors since the process might already be dead
}

// stopLogTailer stops the log tailer for a worker
func (m *Manager) stopLogTailer(workerID string) {
	m.tailersMu.Lock()
	defer m.tailersMu.Unlock()
	
	if tailer, exists := m.tailers[workerID]; exists {
		tailer.Stop()
		delete(m.tailers, workerID)
	}
}
