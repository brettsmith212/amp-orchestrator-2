package worker

import "time"

type Worker struct {
	ID       string    `json:"id"`
	ThreadID string    `json:"thread_id"`
	PID      int       `json:"pid"`
	LogFile  string    `json:"log_file"`
	Started  time.Time `json:"started"`
	Status   string    `json:"status"` // "running", "stopped"
}
