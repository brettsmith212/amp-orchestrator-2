package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/apierr"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/query"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/response"
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
			Status:   string(worker.Status),
			Started:  worker.Started,
			LogFile:  worker.LogFile,
			}
			h.broadcastTaskUpdate(task)
			break
		}
	}
}

// BroadcastLogEvent sends a log event over WebSocket
func (h *TaskHandler) BroadcastLogEvent(logLine worker.LogLine) {
	if h.hub == nil {
		return
	}

	event := LogEvent{
		Type: "log",
		Data: LogData{
			WorkerID:  logLine.WorkerID,
			Timestamp: logLine.Timestamp,
			Content:   logLine.Content,
		},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		// Log error but don't fail
		return
	}

	h.hub.Broadcast(eventJSON)
}

// ListTasks returns tasks with optional filtering, sorting, and pagination
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) error {
	// Parse query parameters
	taskQuery, err := query.ParseTaskQuery(r.URL.Query())
	if err != nil {
		return err
	}

	// Get filtered and sorted workers
	workers, err := h.manager.ListWorkersWithFilter(
		taskQuery.Status,
		taskQuery.StartedBefore,
		taskQuery.StartedAfter,
		taskQuery.SortBy,
		taskQuery.SortOrder,
	)
	if err != nil {
		return apierr.WrapInternal(err, "Failed to list tasks")
	}

	// Apply cursor-based pagination
	var paginatedWorkers []*worker.Worker
	var startIndex int

	if taskQuery.Cursor != "" {
		cursorTime, cursorID, err := query.ParseCursor(taskQuery.Cursor)
		if err != nil {
			return err
		}

		// Find the starting position based on cursor
		for i, w := range workers {
			if w.Started.Equal(cursorTime) && w.ID == cursorID {
				startIndex = i + 1
				break
			} else if (taskQuery.SortOrder == "desc" && w.Started.Before(cursorTime)) ||
				(taskQuery.SortOrder == "asc" && w.Started.After(cursorTime)) {
				startIndex = i
				break
			}
		}
	}

	// Get the page of workers
	endIndex := startIndex + taskQuery.Limit
	if endIndex > len(workers) {
		endIndex = len(workers)
	}
	paginatedWorkers = workers[startIndex:endIndex]

	// Convert workers to DTOs
	tasks := make([]TaskDTO, len(paginatedWorkers))
	for i, worker := range paginatedWorkers {
		tasks[i] = TaskDTO{
			ID:       worker.ID,
			ThreadID: worker.ThreadID,
			Status:   string(worker.Status),
			Started:  worker.Started,
			LogFile:  worker.LogFile,
		}
	}

	// Prepare response
	resp := PaginatedTasksResponse{
		Tasks:   tasks,
		HasMore: endIndex < len(workers),
		Total:   len(workers),
	}

	// Generate next cursor if there are more results
	if resp.HasMore && len(paginatedWorkers) > 0 {
		lastTask := paginatedWorkers[len(paginatedWorkers)-1]
		resp.NextCursor = query.GenerateCursor(lastTask.ID, lastTask.Started)
	}

	return response.OK(w, resp)
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
		Status:   string(latestWorker.Status),
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

// InterruptTask interrupts a running task with SIGINT
func (h *TaskHandler) InterruptTask(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "id")
	
	if err := h.manager.InterruptWorker(workerID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "cannot interrupt") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Failed to interrupt task", http.StatusInternalServerError)
		return
	}

	// Broadcast the task update after interrupting
	h.broadcastTaskAfterStop(workerID)

	w.WriteHeader(http.StatusAccepted)
}

// AbortTask forcefully terminates a task with SIGKILL
func (h *TaskHandler) AbortTask(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "id")
	
	if err := h.manager.AbortWorker(workerID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "cannot abort") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Failed to abort task", http.StatusInternalServerError)
		return
	}

	// Broadcast the task update after aborting
	h.broadcastTaskAfterStop(workerID)

	w.WriteHeader(http.StatusAccepted)
}

// RetryTask restarts a task with a new message
func (h *TaskHandler) RetryTask(w http.ResponseWriter, r *http.Request) {
	workerID := chi.URLParam(r, "id")
	
	var req struct {
		Message string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	
	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}
	
	if err := h.manager.RetryWorker(workerID, req.Message); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "cannot retry") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Failed to retry task", http.StatusInternalServerError)
		return
	}

	// Broadcast the task update after retrying
	h.broadcastTaskAfterStop(workerID)

	w.WriteHeader(http.StatusAccepted)
}
