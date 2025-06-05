package api

import "time"

// TaskDTO represents a task for API responses
type TaskDTO struct {
	ID       string    `json:"id"`
	ThreadID string    `json:"thread_id"`
	Status   string    `json:"status"`
	Started  time.Time `json:"started"`
	LogFile  string    `json:"log_file"`
}

// StartTaskRequest represents the request body for starting a task
type StartTaskRequest struct {
	Message string `json:"message"`
}

// WebSocketEvent represents events sent over WebSocket
type WebSocketEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// TaskUpdateEvent represents a task update event
type TaskUpdateEvent struct {
	Type string  `json:"type"` // "task-update"
	Data TaskDTO `json:"data"`
}

// LogEvent represents a log line event
type LogEvent struct {
	Type string `json:"type"` // "log"
	Data LogData `json:"data"`
}

// LogData represents log line data
type LogData struct {
	WorkerID  string    `json:"worker_id"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// PaginatedTasksResponse represents a paginated response for tasks
type PaginatedTasksResponse struct {
	Tasks      []TaskDTO `json:"tasks"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
	Total      int       `json:"total"`
}
