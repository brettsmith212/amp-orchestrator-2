package api

import (
	"encoding/json"
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

// TaskHandler handles task-related API requests
type TaskHandler struct {
	manager *worker.Manager
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(manager *worker.Manager) *TaskHandler {
	return &TaskHandler{
		manager: manager,
	}
}

// ListTasks returns all tasks as JSON
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	workers, err := h.manager.ListWorkers()
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}

	// Convert workers to DTOs
	tasks := make([]TaskDTO, len(workers))
	for i, worker := range workers {
		tasks[i] = TaskDTO{
			ID:       worker.ID,
			ThreadID: worker.ThreadID,
			Status:   worker.Status,
			Started:  worker.Started,
			LogFile:  worker.LogFile,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
