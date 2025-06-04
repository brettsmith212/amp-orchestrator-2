package api

import (
	"bufio"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

// LogHandler handles log-related API requests
type LogHandler struct {
	manager *worker.Manager
}

// NewLogHandler creates a new log handler
func NewLogHandler(manager *worker.Manager) *LogHandler {
	return &LogHandler{
		manager: manager,
	}
}

// GetTaskLogs serves the log file for a specific task
// Supports optional ?tail=n query parameter to limit number of lines
func (h *LogHandler) GetTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		http.Error(w, "Task ID is required", http.StatusBadRequest)
		return
	}

	// Find the worker to get log file path
	workers, err := h.manager.ListWorkers()
	if err != nil {
		http.Error(w, "Failed to list workers", http.StatusInternalServerError)
		return
	}

	var logFile string
	for _, worker := range workers {
		if worker.ID == taskID {
			logFile = worker.LogFile
			break
		}
	}

	if logFile == "" {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		http.Error(w, "Log file not found", http.StatusNotFound)
		return
	}

	// Parse tail parameter
	tailParam := r.URL.Query().Get("tail")
	var tailLines int
	if tailParam != "" {
		var err error
		tailLines, err = strconv.Atoi(tailParam)
		if err != nil || tailLines < 0 {
			http.Error(w, "Invalid tail parameter", http.StatusBadRequest)
			return
		}
	}

	// Set response headers
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Open log file
	file, err := os.Open(logFile)
	if err != nil {
		http.Error(w, "Failed to open log file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if tailLines > 0 {
		// Read last N lines
		lines, err := readLastLines(file, tailLines)
		if err != nil {
			http.Error(w, "Failed to read log file", http.StatusInternalServerError)
			return
		}

		for _, line := range lines {
			w.Write([]byte(line + "\n"))
		}
	} else {
		// Stream entire file
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			w.Write([]byte(scanner.Text() + "\n"))
		}

		if err := scanner.Err(); err != nil {
			// Log error but don't fail the response since we may have already sent data
			return
		}
	}
}

// readLastLines reads the last n lines from a file
func readLastLines(file *os.File, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}

	// Simple approach: read entire file and get last n lines
	// For very large files, this could be optimized, but it's sufficient for log files
	scanner := bufio.NewScanner(file)
	var allLines []string
	
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	// Return empty slice for empty file
	if len(allLines) == 0 {
		return []string{}, nil
	}
	
	// Return last n lines
	if len(allLines) <= n {
		return allLines, nil
	}
	
	return allLines[len(allLines)-n:], nil
}
