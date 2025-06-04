package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestLogEndpoint_Integration(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	
	// Create manager and task handler
	manager := worker.NewManager(tmpDir)
	h := hub.NewHub()
	taskHandler := NewTaskHandler(manager, h)
	
	// Create router with log endpoint
	router := NewRouter(taskHandler, h)
	
	// Create a test worker with log content
	workerID := "integration-worker"
	logFile := filepath.Join(tmpDir, fmt.Sprintf("worker-%s.log", workerID))
	logContent := "Log line 1\nLog line 2\nLog line 3\n"
	err := os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)
	
	// Save worker to state
	testWorker := &worker.Worker{
		ID:       workerID,
		ThreadID: "T-integration",
		PID:      12345,
		LogFile:  logFile,
		Started:  time.Now(),
		Status:   "running",
	}
	
	workers := map[string]*worker.Worker{workerID: testWorker}
	stateFile := filepath.Join(tmpDir, "workers.json")
	err = manager.SaveWorkersForTest(workers, stateFile)
	require.NoError(t, err)
	
	// Test full log endpoint
	t.Run("full log via router", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, logContent, w.Body.String())
	})
	
	// Test tail parameter via router
	t.Run("tail parameter via router", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs?tail=2", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		expected := "Log line 2\nLog line 3\n"
		assert.Equal(t, expected, w.Body.String())
	})
	
	// Test nonexistent task
	t.Run("nonexistent task via router", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/nonexistent/logs", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
