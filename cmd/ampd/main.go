package main

import (
	"log"
	"net/http"

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
	
	router := api.NewRouter(manager, h)
	
	addr := ":" + cfg.Port
	log.Printf("Starting ampd server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
