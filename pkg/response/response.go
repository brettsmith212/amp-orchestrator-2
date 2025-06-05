package response

import (
	"encoding/json"
	"net/http"
)

// JSON sends a JSON response with the given status code and payload
func JSON(w http.ResponseWriter, statusCode int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if payload == nil {
		return nil
	}
	
	return json.NewEncoder(w).Encode(payload)
}

// OK sends a 200 OK response with JSON payload
func OK(w http.ResponseWriter, payload interface{}) error {
	return JSON(w, http.StatusOK, payload)
}

// Created sends a 201 Created response with JSON payload
func Created(w http.ResponseWriter, payload interface{}) error {
	return JSON(w, http.StatusCreated, payload)
}

// Accepted sends a 202 Accepted response with optional JSON payload
func Accepted(w http.ResponseWriter, payload interface{}) error {
	return JSON(w, http.StatusAccepted, payload)
}

// NoContent sends a 204 No Content response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Text sends a plain text response with the given status code
func Text(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}

// Error sends an error response with plain text
func Error(w http.ResponseWriter, statusCode int, message string) {
	Text(w, statusCode, message)
}
