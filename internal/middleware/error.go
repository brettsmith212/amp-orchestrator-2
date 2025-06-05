package middleware

import (
	"log"
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/pkg/apierr"
	"github.com/brettsmith212/amp-orchestrator-2/pkg/response"
)

// ErrorHandler is a handler function that can return an error
type ErrorHandler func(w http.ResponseWriter, r *http.Request) error

// Error wraps a handler that returns an error and converts it to an HTTP response
func Error(handler ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := handler(w, r)
		if err == nil {
			return
		}

		// Log the error for debugging
		log.Printf("API Error: %v", err)

		// Check if it's an APIError
		if apiErr, ok := err.(*apierr.APIError); ok {
			response.Error(w, apiErr.StatusCode, apiErr.Message)
			return
		}

		// Generic error - return 500
		response.Error(w, http.StatusInternalServerError, "Internal server error")
	}
}

// Recovery middleware recovers from panics and converts them to 500 errors
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				response.Error(w, http.StatusInternalServerError, "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
