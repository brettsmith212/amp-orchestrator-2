package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/brettsmith212/amp-orchestrator-2/internal/api"
	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/config"
)

func main() {
	cfg := config.Load()
	
	// Initialize worker manager
	manager := worker.NewManager(cfg.LogDir)
	
	// Initialize WebSocket hub
	h := hub.NewHub()
	go h.Run()
	
	// Create task handler to handle broadcasting
	taskHandler := api.NewTaskHandler(manager, h)
	
	// Set up log callback to broadcast log events
	manager.SetLogCallback(taskHandler.BroadcastLogEvent)
	
	// Set up thread message callback to broadcast thread message events
	manager.SetThreadMessageCallback(func(workerID string, message worker.ThreadMessage) {
		event := api.ThreadMessageEvent{
			Type: "thread_message",
			Data: api.ThreadMessageDTO{
				ID:        message.ID,
				Type:      string(message.Type),
				Content:   message.Content,
				Timestamp: message.Timestamp,
				Metadata:  message.Metadata,
			},
		}
		
		if eventJSON, err := json.Marshal(event); err == nil {
			h.Broadcast(eventJSON)
		}
	})
	
	// Set up worker exit callback to broadcast task updates
	manager.SetExitCallback(func(workerID string) {
		// Get the updated worker and broadcast its status
		workers, err := manager.ListWorkers()
		if err != nil {
			return
		}
		
		for _, w := range workers {
			if w.ID == workerID {
				taskDTO := struct {
					ID          string    `json:"id"`
					ThreadID    string    `json:"thread_id"`
					Status      string    `json:"status"`
					Started     time.Time `json:"started"`
					LogFile     string    `json:"log_file"`
					Title       string    `json:"title,omitempty"`
					Description string    `json:"description,omitempty"`
					Tags        []string  `json:"tags,omitempty"`
					Priority    string    `json:"priority,omitempty"`
				}{
					ID:          w.ID,
					ThreadID:    w.ThreadID,
					Status:      string(w.Status),
					Started:     w.Started,
					LogFile:     w.LogFile,
					Title:       w.Title,
					Description: w.Description,
					Tags:        w.Tags,
					Priority:    w.Priority,
				}
				
				event := struct {
					Type string      `json:"type"`
					Data interface{} `json:"data"`
				}{
					Type: "task-update",
					Data: taskDTO,
				}
				
				if eventJSON, err := json.Marshal(event); err == nil {
					h.Broadcast(eventJSON)
				}
				break
			}
		}
		
		// Process stopped workers to generate thread messages
		manager.ProcessStoppedWorkers()
	})
	
	router := api.NewRouter(taskHandler, h)
	
	addr := ":" + cfg.Port
	log.Printf("Starting ampd server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
