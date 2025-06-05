package api

import "time"

// TaskDTO represents a task for API responses
type TaskDTO struct {
	ID          string    `json:"id"`
	ThreadID    string    `json:"thread_id"`
	Status      string    `json:"status"`
	Started     time.Time `json:"started"`
	LogFile     string    `json:"log_file"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Priority    string    `json:"priority,omitempty"`
}

// StartTaskRequest represents the request body for starting a task
type StartTaskRequest struct {
	Message string `json:"message"`
}

// PatchTaskRequest represents the request body for updating a task
type PatchTaskRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Priority    *string  `json:"priority,omitempty"`
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

// ThreadMessageDTO represents a thread message for API responses
type ThreadMessageDTO struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PaginatedThreadResponse represents a paginated response for thread messages
type PaginatedThreadResponse struct {
	Messages []ThreadMessageDTO `json:"messages"`
	HasMore  bool               `json:"has_more"`
	Total    int                `json:"total"`
}

// ThreadMessageEvent represents a thread message event over WebSocket
type ThreadMessageEvent struct {
	Type string            `json:"type"` // "thread_message"
	Data ThreadMessageDTO `json:"data"`
}
