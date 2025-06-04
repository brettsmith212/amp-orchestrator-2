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
