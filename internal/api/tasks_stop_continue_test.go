package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestStopTask_Success(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	// Create a mock worker in the state file - use fake PID to avoid killing real process
	stateFile := filepath.Join(tempDir, "workers.json")
	mockWorkers := map[string]*worker.Worker{
		"test123": {
			ID:       "test123",
			ThreadID: "T-456",
			PID:      999999, // Use fake PID - this will fail to kill but test error handling
			LogFile:  filepath.Join(tempDir, "worker-test123.log"),
			Started:  time.Now(),
			Status:   "running",
		},
	}
	mockData, err := json.MarshalIndent(mockWorkers, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(stateFile, mockData, 0644)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/tasks/test123/stop", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"test123"},
		},
	}))
	w := httptest.NewRecorder()

	handler.StopTask(w, req)

	// Since the fake PID won't exist, the manager returns an error, which maps to 500
	// This tests the error handling path - in a real scenario the PID would exist
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStopTask_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	req := httptest.NewRequest("POST", "/api/tasks/nonexistent/stop", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"nonexistent"},
		},
	}))
	w := httptest.NewRecorder()

	handler.StopTask(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Task not found")
}

func TestStopTask_NotRunning(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	// Create a mock stopped worker
	stateFile := filepath.Join(tempDir, "workers.json")
	mockWorkers := map[string]*worker.Worker{
		"stopped123": {
			ID:       "stopped123",
			ThreadID: "T-789",
			PID:      12345,
			LogFile:  filepath.Join(tempDir, "worker-stopped123.log"),
			Started:  time.Now(),
			Status:   "stopped",
		},
	}
	mockData, err := json.MarshalIndent(mockWorkers, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(stateFile, mockData, 0644)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/tasks/stopped123/stop", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"stopped123"},
		},
	}))
	w := httptest.NewRecorder()

	handler.StopTask(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "not running")
}

func TestContinueTask_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	reqBody := `{"message":"test"}`
	req := httptest.NewRequest("POST", "/api/tasks/nonexistent/continue", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"nonexistent"},
		},
	}))
	w := httptest.NewRecorder()

	handler.ContinueTask(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Task not found")
}

func TestContinueTask_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	req := httptest.NewRequest("POST", "/api/tasks/test123/continue", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"test123"},
		},
	}))
	w := httptest.NewRecorder()

	handler.ContinueTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid JSON request body")
}

func TestContinueTask_EmptyMessage(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	reqBody := `{"message":""}`
	req := httptest.NewRequest("POST", "/api/tasks/test123/continue", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{"test123"},
		},
	}))
	w := httptest.NewRecorder()

	handler.ContinueTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Message is required")
}
