package worker

import (
	"log"
	"os/exec"
)

// WatcherCallback is called when a worker process exits
type WatcherCallback func(workerID string, exitCode int)

// Watcher monitors worker processes and calls a callback when they exit
type Watcher struct {
	callback WatcherCallback
}

// NewWatcher creates a new worker watcher
func NewWatcher(callback WatcherCallback) *Watcher {
	return &Watcher{
		callback: callback,
	}
}

// WatchProcess monitors a process and calls the callback when it exits
func (w *Watcher) WatchProcess(workerID string, cmd *exec.Cmd) {
	go func() {
		// Wait for the process to complete
		err := cmd.Wait()
		
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				// Process was killed or other error
				exitCode = -1
			}
		}
		
		log.Printf("Worker %s exited with code %d", workerID, exitCode)
		
		// Call the callback if set
		if w.callback != nil {
			w.callback(workerID, exitCode)
		}
	}()
}

// MonitorWorkerExit is a convenience function to watch a process and update status
func (m *Manager) MonitorWorkerExit(workerID string, cmd *exec.Cmd, onExit func(workerID string)) {
	go func() {
		// Wait for the process to complete
		cmd.Wait()
		
		// Update worker status in the manager
		workers, err := m.loadWorkers()
		if err != nil {
			log.Printf("Failed to load workers after exit: %v", err)
			return
		}
		
		if worker, exists := workers[workerID]; exists {
			worker.Status = "stopped"
			if err := m.saveWorkers(workers); err != nil {
				log.Printf("Failed to save worker state after exit: %v", err)
				return
			}
			
			log.Printf("Worker %s marked as stopped", workerID)
			
			// Call the exit callback
			if onExit != nil {
				onExit(workerID)
			}
		}
	}()
}
