package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestLogHandler_GetTaskLogs(t *testing.T) {
	tmpDir := t.TempDir()
	manager := worker.NewManager(tmpDir)
	handler := NewLogHandler(manager)

	// Create a test worker and log file
	workerID := "test-worker-123"
	logFile := filepath.Join(tmpDir, fmt.Sprintf("worker-%s.log", workerID))
	
	// Create log file with test content
	logContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	err := os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)

	// Create and save a worker
	testWorker := &worker.Worker{
		ID:       workerID,
		ThreadID: "T-123",
		PID:      12345,
		LogFile:  logFile,
		Started:  time.Now(),
		Status:   "running",
	}
	
	// Save worker to manager's state
	workers := map[string]*worker.Worker{workerID: testWorker}
	stateFile := filepath.Join(tmpDir, "workers.json")
	manager.SaveWorkersForTest(workers, stateFile) // We'll need to add this method

	// Test getting full log
	t.Run("full log", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{workerID},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, logContent, w.Body.String())
	})

	// Test tail parameter
	t.Run("tail 2 lines", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs?tail=2", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{workerID},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		expected := "Line 4\nLine 5\n"
		assert.Equal(t, expected, w.Body.String())
	})

	// Test tail parameter with more lines than available
	t.Run("tail more than available", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs?tail=10", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{workerID},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, logContent, w.Body.String())
	})

	// Test task not found
	t.Run("task not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/nonexistent/logs", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{"nonexistent"},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), "Task not found")
	})

	// Test invalid tail parameter
	t.Run("invalid tail parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs?tail=invalid", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{workerID},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid tail parameter")
	})

	// Test negative tail parameter
	t.Run("negative tail parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs?tail=-5", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{"id"},
				Values: []string{workerID},
			},
		}))
		
		w := httptest.NewRecorder()
		handler.GetTaskLogs(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid tail parameter")
	})
}

func TestLogHandler_EmptyLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager := worker.NewManager(tmpDir)
	handler := NewLogHandler(manager)

	// Create a test worker with empty log file
	workerID := "empty-worker"
	logFile := filepath.Join(tmpDir, fmt.Sprintf("worker-%s.log", workerID))
	
	// Create empty log file
	err := os.WriteFile(logFile, []byte(""), 0644)
	require.NoError(t, err)

	// Create and save a worker
	testWorker := &worker.Worker{
		ID:       workerID,
		ThreadID: "T-456",
		PID:      12346,
		LogFile:  logFile,
		Started:  time.Now(),
		Status:   "running",
	}
	
	workers := map[string]*worker.Worker{workerID: testWorker}
	stateFile := filepath.Join(tmpDir, "workers.json")
	manager.SaveWorkersForTest(workers, stateFile)

	req := httptest.NewRequest("GET", "/api/tasks/"+workerID+"/logs", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"id"},
			Values: []string{workerID},
		},
	}))
	
	w := httptest.NewRecorder()
	handler.GetTaskLogs(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "", w.Body.String())
}

func TestReadLastLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.log")

	tests := []struct {
		name     string
		content  string
		n        int
		expected []string
	}{
		{
			name:     "simple case",
			content:  "line1\nline2\nline3\n",
			n:        2,
			expected: []string{"line2", "line3"},
		},
		{
			name:     "more lines requested than available",
			content:  "line1\nline2\n",
			n:        5,
			expected: []string{"line1", "line2"},
		},
		{
			name:     "single line",
			content:  "single line",
			n:        1,
			expected: []string{"single line"},
		},
		{
			name:     "empty file", 
			content:  "",
			n:        3,
			expected: []string{},
		},
		{
			name:     "zero lines requested",
			content:  "line1\nline2\n",
			n:        0,
			expected: []string{},
		},
		{
			name:     "lines with empty line",
			content:  "line1\n\nline3\n",
			n:        3,
			expected: []string{"line1", "", "line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			file, err := os.Open(testFile)
			require.NoError(t, err)
			defer file.Close()

			lines, err := readLastLines(file, tt.n)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, lines)
		})
	}
}
