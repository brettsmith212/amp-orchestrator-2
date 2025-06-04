package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestListTasks_EmptyManager(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	handler := NewTaskHandler(manager)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	handler.ListTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var tasks []TaskDTO
	err := json.Unmarshal(w.Body.Bytes(), &tasks)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestListTasks_WithWorkers(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	handler := NewTaskHandler(manager)

	// Create mock state file with some workers
	stateFile := filepath.Join(tempDir, "workers.json")
	mockWorkers := map[string]*worker.Worker{
		"worker1": {
			ID:       "worker1",
			ThreadID: "T-123",
			PID:      os.Getpid(), // Use current process PID so it exists
			LogFile:  filepath.Join(tempDir, "worker-worker1.log"),
			Started:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			Status:   "running",
		},
		"worker2": {
			ID:       "worker2",
			ThreadID: "T-456",
			PID:      67890, // Use fake PID so it will be marked as stopped
			LogFile:  filepath.Join(tempDir, "worker-worker2.log"),
			Started:  time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC),
			Status:   "stopped",
		},
	}

	// Write mock state to file
	mockData, err := json.MarshalIndent(mockWorkers, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(stateFile, mockData, 0644)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	handler.ListTasks(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var tasks []TaskDTO
	err = json.Unmarshal(w.Body.Bytes(), &tasks)
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	// Sort tasks by ID for consistent testing
	if len(tasks) > 1 && tasks[0].ID > tasks[1].ID {
		tasks[0], tasks[1] = tasks[1], tasks[0]
	}

	// Check first worker
	assert.Equal(t, "worker1", tasks[0].ID)
	assert.Equal(t, "T-123", tasks[0].ThreadID)
	assert.Equal(t, "running", tasks[0].Status)
	assert.Equal(t, time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC), tasks[0].Started)

	// Check second worker
	assert.Equal(t, "worker2", tasks[1].ID)
	assert.Equal(t, "T-456", tasks[1].ThreadID)
	assert.Equal(t, "stopped", tasks[1].Status)
	assert.Equal(t, time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC), tasks[1].Started)
}
