package worker

import (
	"testing"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     WorkerStatus
		to       WorkerStatus
		expected bool
	}{
		// Valid transitions from running
		{"running to stopped", StatusRunning, StatusStopped, true},
		{"running to interrupted", StatusRunning, StatusInterrupted, true},
		{"running to aborted", StatusRunning, StatusAborted, true},
		{"running to completed", StatusRunning, StatusCompleted, true},
		{"running to failed", StatusRunning, StatusFailed, true},
		
		// Invalid transition from running
		{"running to running", StatusRunning, StatusRunning, false},
		
		// Valid transitions from stopped
		{"stopped to running", StatusStopped, StatusRunning, true},
		{"stopped to aborted", StatusStopped, StatusAborted, true},
		
		// Invalid transitions from stopped
		{"stopped to interrupted", StatusStopped, StatusInterrupted, false},
		{"stopped to completed", StatusStopped, StatusCompleted, false},
		{"stopped to failed", StatusStopped, StatusFailed, false},
		
		// Valid transitions from interrupted
		{"interrupted to running", StatusInterrupted, StatusRunning, true},
		{"interrupted to aborted", StatusInterrupted, StatusAborted, true},
		
		// Invalid transitions from interrupted
		{"interrupted to stopped", StatusInterrupted, StatusStopped, false},
		{"interrupted to completed", StatusInterrupted, StatusCompleted, false},
		
		// Valid transitions from aborted
		{"aborted to running", StatusAborted, StatusRunning, true},
		
		// Invalid transitions from aborted
		{"aborted to stopped", StatusAborted, StatusStopped, false},
		{"aborted to interrupted", StatusAborted, StatusInterrupted, false},
		
		// Valid transitions from failed
		{"failed to running", StatusFailed, StatusRunning, true},
		
		// Invalid transitions from failed
		{"failed to stopped", StatusFailed, StatusStopped, false},
		{"failed to aborted", StatusFailed, StatusAborted, false},
		
		// Valid transitions from completed
		{"completed to running", StatusCompleted, StatusRunning, true},
		
		// Invalid transitions from completed
		{"completed to stopped", StatusCompleted, StatusStopped, false},
		{"completed to interrupted", StatusCompleted, StatusInterrupted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v, expected %v", 
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestCanTransitionInvalidStatus(t *testing.T) {
	// Test with an invalid status that doesn't exist in AllowedTransitions
	invalidStatus := WorkerStatus("invalid")
	result := CanTransition(invalidStatus, StatusRunning)
	
	if result {
		t.Errorf("CanTransition with invalid status should return false, got true")
	}
}
