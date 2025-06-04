package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
)

func NewRouter(taskHandler *TaskHandler, h *hub.Hub) *chi.Mux {
	r := chi.NewRouter()
	
	// Add basic middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// Health check endpoint
	r.Get("/healthz", HealthHandler)
	
	// WebSocket handler
	wsHandler := NewWSHandler(h)
	
	r.Route("/api", func(r chi.Router) {
		r.Get("/tasks", taskHandler.ListTasks)
		r.Post("/tasks", taskHandler.StartTask)
		r.Post("/tasks/{id}/stop", taskHandler.StopTask)
		r.Post("/tasks/{id}/continue", taskHandler.ContinueTask)
		r.Get("/ws", wsHandler.ServeWS)
	})
	
	return r
}
