package main

import (
	"log"
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/internal/api"
)

func main() {
	router := api.NewRouter()
	
	log.Println("Starting ampd server on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
