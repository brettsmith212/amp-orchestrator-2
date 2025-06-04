package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("Starting ampd server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
