package worker

import "time"

// WorkerStatus defines the possible states for a worker
type WorkerStatus string

const (
	StatusRunning     WorkerStatus = "running"
	StatusStopped     WorkerStatus = "stopped"
	StatusInterrupted WorkerStatus = "interrupted"
	StatusAborted     WorkerStatus = "aborted"
	StatusFailed      WorkerStatus = "failed"
	StatusCompleted   WorkerStatus = "completed"
)

type Worker struct {
	ID          string       `json:"id"`
	ThreadID    string       `json:"thread_id"`
	PID         int          `json:"pid"`
	LogFile     string       `json:"log_file"`     // Stdout/stderr log file
	AmpLogFile  string       `json:"amp_log_file"` // Amp internal log file
	Started     time.Time    `json:"started"`
	Status      WorkerStatus `json:"status"`
	Title       string       `json:"title,omitempty"`       // User-friendly task name
	Description string       `json:"description,omitempty"` // Task description
	Tags        []string     `json:"tags,omitempty"`        // Task tags/labels
	Priority    string       `json:"priority,omitempty"`    // Task priority (low, medium, high)
}

// AllowedTransitions defines valid state transitions for workers
var AllowedTransitions = map[WorkerStatus][]WorkerStatus{
	StatusRunning: {
		StatusStopped,     // Normal stop
		StatusInterrupted, // User interruption
		StatusAborted,     // Force kill
		StatusCompleted,   // Natural completion
		StatusFailed,      // Process failure
	},
	StatusStopped: {
		StatusRunning, // Continue/retry
		StatusAborted, // Force kill any remaining processes
	},
	StatusInterrupted: {
		StatusRunning, // Resume/continue
		StatusAborted, // Force kill
	},
	StatusAborted: {
		StatusRunning, // Retry with new process
	},
	StatusFailed: {
		StatusRunning, // Retry
	},
	StatusCompleted: {
		StatusRunning, // Retry/restart
	},
}

// CanTransition checks if a status transition is allowed
func CanTransition(from, to WorkerStatus) bool {
	allowed, exists := AllowedTransitions[from]
	if !exists {
		return false
	}
	
	for _, status := range allowed {
		if status == to {
			return true
		}
	}
	return false
}
