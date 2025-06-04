package main

import (
	"log"
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/internal/api"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/config"
)

func main() {
	cfg := config.Load()
	
	// Initialize worker manager
	manager := worker.NewManager(cfg.LogDir)
	
	router := api.NewRouter(manager)
	
	addr := ":" + cfg.Port
	log.Printf("Starting ampd server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
