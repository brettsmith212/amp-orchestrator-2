package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

// TaskHandler handles task-related API requests
type TaskHandler struct {
	manager *worker.Manager
	hub     *hub.Hub
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(manager *worker.Manager, h *hub.Hub) *TaskHandler {
	return &TaskHandler{
		manager: manager,
		hub:     h,
	}
}

// broadcastTaskUpdate sends a task-update event over WebSocket
func (h *TaskHandler) broadcastTaskUpdate(task TaskDTO) {
	if h.hub == nil {
		return
	}

	event := TaskUpdateEvent{
		Type: "task-update",
		Data: task,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		// Log error but don't fail the request
		return
	}

	h.hub.Broadcast(eventJSON)
}

// broadcastTaskAfterStop gets the task and broadcasts its updated status
func (h *TaskHandler) broadcastTaskAfterStop(taskID string) {
	// Get the updated worker status
	workers, err := h.manager.ListWorkers()
	if err != nil {
		return
	}

	// Find the worker and broadcast its updated status
	for _, worker := range workers {
		if worker.ID == taskID {
			task := TaskDTO{
				ID:       worker.ID,
				ThreadID: worker.ThreadID,
				Status:   worker.Status,
				Started:  worker.Started,
				LogFile:  worker.LogFile,
			}
			h.broadcastTaskUpdate(task)
			break
		}
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

// StartTask creates and starts a new task
func (h *TaskHandler) StartTask(w http.ResponseWriter, r *http.Request) {
	var req StartTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Start the worker
	if err := h.manager.StartWorker(req.Message); err != nil {
		http.Error(w, "Failed to start task", http.StatusInternalServerError)
		return
	}

	// Get the latest workers to find the one we just created
	workers, err := h.manager.ListWorkers()
	if err != nil {
		http.Error(w, "Failed to retrieve created task", http.StatusInternalServerError)
		return
	}

	// Find the most recently started worker (the one we just created)
	var latestWorker *worker.Worker
	for _, w := range workers {
		if latestWorker == nil || w.Started.After(latestWorker.Started) {
			latestWorker = w
		}
	}

	if latestWorker == nil {
		http.Error(w, "Failed to find created task", http.StatusInternalServerError)
		return
	}

	// Convert to DTO and return
	task := TaskDTO{
		ID:       latestWorker.ID,
		ThreadID: latestWorker.ThreadID,
		Status:   latestWorker.Status,
		Started:  latestWorker.Started,
		LogFile:  latestWorker.LogFile,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	// Broadcast task update event
	h.broadcastTaskUpdate(task)
}

// StopTask stops a running task
func (h *TaskHandler) StopTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	err := h.manager.StopWorker(taskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "not running") {
			http.Error(w, "Task is not running", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to stop task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	// Broadcast task update after stopping
	h.broadcastTaskAfterStop(taskID)
}

// ContinueTask sends a message to a running task
func (h *TaskHandler) ContinueTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	var req StartTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	err := h.manager.ContinueWorker(taskID, req.Message)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "not running") {
			http.Error(w, "Task is not running", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to continue task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
