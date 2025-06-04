package main

import (
	"log"
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/internal/api"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/config"
)

func main() {
	cfg := config.Load()
	router := api.NewRouter()
	
	addr := ":" + cfg.Port
	log.Printf("Starting ampd server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
