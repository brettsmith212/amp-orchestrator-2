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
	onThreadMsg   func(workerID string, message ThreadMessage) // Callback for thread messages
	tailers       map[string]*LogTailer // Active log tailers by worker ID
	tailersMu     sync.RWMutex          // Protects tailers map
	threadStorage *ThreadStorage        // Thread message storage
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
		onThreadMsg:   nil,   // Will be set via SetThreadMessageCallback
		tailers:       make(map[string]*LogTailer),
		threadStorage: NewThreadStorage(filepath.Join(logDir, "threads")),
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

// SetThreadMessageCallback sets the callback function to be called for thread messages
func (m *Manager) SetThreadMessageCallback(callback func(workerID string, message ThreadMessage)) {
	m.onThreadMsg = callback
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
		Status:   StatusRunning,
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

	if worker.Status != StatusRunning {
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
	worker.Status = StatusStopped
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
	if worker.Status == StatusRunning && !m.checkProcessStatus(worker) {
		worker.Status = StatusStopped
		workers[workerID] = worker
		m.saveWorkers(workers)
	}

	if worker.Status != StatusRunning {
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

// InterruptWorker interrupts a running worker with SIGINT
func (m *Manager) InterruptWorker(workerID string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	if !CanTransition(worker.Status, StatusInterrupted) {
		return fmt.Errorf("cannot interrupt worker %s with status %s", workerID, worker.Status)
	}

	// Send SIGINT to the process group
	if err := syscall.Kill(-worker.PID, syscall.SIGINT); err != nil {
		// If process group kill fails, try individual process
		process, findErr := os.FindProcess(worker.PID)
		if findErr == nil {
			// Try to signal individual process, but don't fail if it doesn't work
			process.Signal(syscall.SIGINT)
		}
		// Continue even if signaling fails - the process might already be dead
	}

	// Update worker status
	worker.Status = StatusInterrupted
	workers[workerID] = worker

	if err := m.saveWorkers(workers); err != nil {
		return fmt.Errorf("failed to update worker state: %w", err)
	}

	return nil
}

// AbortWorker forcefully terminates a worker with SIGKILL
func (m *Manager) AbortWorker(workerID string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	if !CanTransition(worker.Status, StatusAborted) {
		return fmt.Errorf("cannot abort worker %s with status %s", workerID, worker.Status)
	}

	// Force kill the process group
	if err := syscall.Kill(-worker.PID, syscall.SIGKILL); err != nil {
		// If process group kill fails, try individual process
		process, findErr := os.FindProcess(worker.PID)
		if findErr == nil {
			// Try to kill individual process, but don't fail if it doesn't work
			process.Kill()
		}
		// Continue even if killing fails - the process might already be dead
	}

	// Kill any remaining amp processes for this thread
	m.killAmpProcesses(worker.ThreadID)

	// Stop log tailer
	m.stopLogTailer(workerID)

	// Update worker status
	worker.Status = StatusAborted
	workers[workerID] = worker

	if err := m.saveWorkers(workers); err != nil {
		return fmt.Errorf("failed to update worker state: %w", err)
	}

	return nil
}

// RetryWorker starts a new worker instance for the same thread
func (m *Manager) RetryWorker(workerID, message string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	if !CanTransition(worker.Status, StatusRunning) {
		return fmt.Errorf("cannot retry worker %s with status %s", workerID, worker.Status)
	}

	// Ensure any old processes are cleaned up
	if worker.Status == StatusRunning {
		m.killAmpProcesses(worker.ThreadID)
	}

	// Create the command to send message to the existing thread
	cmd := exec.Command("bash", "-c", fmt.Sprintf(
		"echo %q | %s threads continue %s",
		message, m.ampBinaryPath, worker.ThreadID,
	))

	// Set the process group ID so we can kill the entire group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Append to existing log file
	logFile, err := os.OpenFile(worker.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start the process
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to retry worker: %w", err)
	}

	// Update worker with new PID and status
	worker.PID = cmd.Process.Pid
	worker.Status = StatusRunning
	workers[workerID] = worker

	// Save worker state
	if err := m.saveWorkers(workers); err != nil {
		// Kill the process if we can't save state
		cmd.Process.Kill()
		logFile.Close()
		return fmt.Errorf("failed to save worker state: %w", err)
	}

	// Start log tailer if callback is set
	if m.onLogLine != nil {
		tailer := NewLogTailer(worker.LogFile, worker.ID, m.onLogLine)
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
		defer logFile.Close()
		cmd.Wait()
	}()

	return nil
}

// UpdateWorkerMetadata updates the metadata fields of a worker
func (m *Manager) UpdateWorkerMetadata(workerID string, title, description, priority *string, tags []string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// Update fields if provided
	if title != nil {
		worker.Title = *title
	}
	if description != nil {
		worker.Description = *description
	}
	if priority != nil {
		worker.Priority = *priority
	}
	if tags != nil {
		worker.Tags = tags
	}

	// Save updated worker
	workers[workerID] = worker
	return m.saveWorkers(workers)
}

// DeleteWorker removes a worker from the system
func (m *Manager) DeleteWorker(workerID string) error {
	workers, err := m.loadWorkers()
	if err != nil {
		return err
	}

	worker, exists := workers[workerID]
	if !exists {
		return fmt.Errorf("worker %s not found", workerID)
	}

	// If worker is running, stop it first
	if worker.Status == StatusRunning {
		// Kill the process if it's still running
		if err := syscall.Kill(-worker.PID, syscall.SIGTERM); err != nil {
			// Try individual process if group kill fails
			if process, findErr := os.FindProcess(worker.PID); findErr == nil {
				process.Kill()
			}
		}
		
		// Kill any remaining amp processes
		m.killAmpProcesses(worker.ThreadID)
		
		// Stop log tailer
		m.stopLogTailer(workerID)
	}

	// Remove from workers map
	delete(workers, workerID)
	
	// Clean up log file if it exists
	if worker.LogFile != "" {
		os.Remove(worker.LogFile)
	}

	return m.saveWorkers(workers)
}

func (m *Manager) ListWorkers() ([]*Worker, error) {
	workers, err := m.loadWorkers()
	if err != nil {
		return nil, err
	}

	// Update status for all workers by checking actual process status
	updated := false
	for id, worker := range workers {
		if worker.Status == StatusRunning && !m.checkProcessStatus(worker) {
			worker.Status = StatusStopped
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

// ListWorkersWithFilter returns workers with filtering and sorting options
func (m *Manager) ListWorkersWithFilter(statusFilter []string, startedBefore, startedAfter *time.Time, sortBy, sortOrder string) ([]*Worker, error) {
	allWorkers, err := m.ListWorkers()
	if err != nil {
		return nil, err
	}

	// Apply status filter
	var filtered []*Worker
	if len(statusFilter) > 0 {
		statusSet := make(map[string]bool)
		for _, status := range statusFilter {
			statusSet[status] = true
		}
		
		for _, worker := range allWorkers {
			if statusSet[string(worker.Status)] {
				filtered = append(filtered, worker)
			}
		}
	} else {
		filtered = allWorkers
	}

	// Apply time filters
	if startedBefore != nil || startedAfter != nil {
		var timeFiltered []*Worker
		for _, worker := range filtered {
			if startedBefore != nil && worker.Started.After(*startedBefore) {
				continue
			}
			if startedAfter != nil && worker.Started.Before(*startedAfter) {
				continue
			}
			timeFiltered = append(timeFiltered, worker)
		}
		filtered = timeFiltered
	}

	// Sort workers
	m.sortWorkers(filtered, sortBy, sortOrder)

	return filtered, nil
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

// SaveWorkersForTest is a test helper to save workers to a specific state file
func (m *Manager) SaveWorkersForTest(workers map[string]*Worker, stateFile string) error {
	originalStateFile := m.stateFile
	m.stateFile = stateFile
	defer func() { m.stateFile = originalStateFile }()
	
	return m.saveWorkers(workers)
}

// AppendThreadMessage appends a message to the thread and optionally broadcasts it
func (m *Manager) AppendThreadMessage(workerID string, messageType MessageType, content string, metadata map[string]interface{}) error {
	message := ThreadMessage{
		ID:        uuid.New().String(),
		Type:      messageType,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	// Store the message
	if err := m.threadStorage.AppendMessage(workerID, message); err != nil {
		return fmt.Errorf("failed to store thread message: %w", err)
	}

	// Broadcast the message if callback is set
	if m.onThreadMsg != nil {
		m.onThreadMsg(workerID, message)
	}

	return nil
}

// GetThreadMessages retrieves thread messages for a worker with pagination
func (m *Manager) GetThreadMessages(workerID string, limit, offset int) ([]ThreadMessage, error) {
	return m.threadStorage.ReadMessages(workerID, limit, offset)
}

// CountThreadMessages returns the total number of messages in a thread
func (m *Manager) CountThreadMessages(workerID string) (int, error) {
	return m.threadStorage.CountMessages(workerID)
}

// sortWorkers sorts a slice of workers based on the given criteria
func (m *Manager) sortWorkers(workers []*Worker, sortBy, sortOrder string) {
	if len(workers) <= 1 {
		return
	}

	// Use a custom sort function
	for i := 0; i < len(workers)-1; i++ {
		for j := i + 1; j < len(workers); j++ {
			var shouldSwap bool
			
			switch sortBy {
			case "id":
				if sortOrder == "asc" {
					shouldSwap = workers[i].ID > workers[j].ID
				} else {
					shouldSwap = workers[i].ID < workers[j].ID
				}
			case "status":
				if sortOrder == "asc" {
					shouldSwap = workers[i].Status > workers[j].Status
				} else {
					shouldSwap = workers[i].Status < workers[j].Status
				}
			case "started":
				fallthrough
			default:
				if sortOrder == "asc" {
					shouldSwap = workers[i].Started.After(workers[j].Started)
				} else {
					shouldSwap = workers[i].Started.Before(workers[j].Started)
				}
			}
			
			if shouldSwap {
				workers[i], workers[j] = workers[j], workers[i]
			}
		}
	}
}
