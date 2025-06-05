package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	errormw "github.com/brettsmith212/amp-orchestrator-2/internal/middleware"
)

func NewRouter(taskHandler *TaskHandler, h *hub.Hub) *chi.Mux {
	r := chi.NewRouter()
	
	// Add basic middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// Health check endpoint
	r.Get("/healthz", HealthHandler)
	
	// Create log handler using the same manager from task handler
	logHandler := NewLogHandler(taskHandler.manager)
	
	// WebSocket handler
	wsHandler := NewWSHandler(h)
	
	r.Route("/api", func(r chi.Router) {
		r.Get("/tasks", errormw.Error(taskHandler.ListTasks))
		r.Post("/tasks", taskHandler.StartTask)
		r.Patch("/tasks/{id}", taskHandler.PatchTask)
		r.Delete("/tasks/{id}", taskHandler.DeleteTask)
		r.Post("/tasks/{id}/stop", taskHandler.StopTask)
		r.Post("/tasks/{id}/continue", taskHandler.ContinueTask)
		r.Post("/tasks/{id}/interrupt", taskHandler.InterruptTask)
		r.Post("/tasks/{id}/abort", taskHandler.AbortTask)
		r.Post("/tasks/{id}/retry", taskHandler.RetryTask)
		r.Post("/tasks/{id}/merge", taskHandler.MergeTask)
		r.Post("/tasks/{id}/delete-branch", taskHandler.DeleteBranchTask)
		r.Post("/tasks/{id}/create-pr", taskHandler.CreatePRTask)
		r.Get("/tasks/{id}/logs", logHandler.GetTaskLogs)
		r.Get("/tasks/{id}/thread", GetTaskThread(taskHandler.manager))
		r.Get("/ws", wsHandler.ServeWS)
	})
	
	return r
}
