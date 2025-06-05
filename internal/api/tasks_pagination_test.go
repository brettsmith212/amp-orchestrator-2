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

	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
	"github.com/brettsmith212/amp-orchestrator-2/internal/middleware"
	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestListTasks_Pagination(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	// Create test workers with different timestamps
	now := time.Now()
	mockWorkers := map[string]*worker.Worker{
		"worker1": {
			ID:       "worker1",
			ThreadID: "T-1",
			PID:      1001,
			LogFile:  filepath.Join(tempDir, "worker-worker1.log"),
			Started:  now.Add(-3 * time.Hour),
			Status:   "running",
		},
		"worker2": {
			ID:       "worker2",
			ThreadID: "T-2",
			PID:      1002,
			LogFile:  filepath.Join(tempDir, "worker-worker2.log"),
			Started:  now.Add(-2 * time.Hour),
			Status:   "stopped",
		},
		"worker3": {
			ID:       "worker3",
			ThreadID: "T-3",
			PID:      1003,
			LogFile:  filepath.Join(tempDir, "worker-worker3.log"),
			Started:  now.Add(-1 * time.Hour),
			Status:   "running",
		},
	}

	stateFile := filepath.Join(tempDir, "workers.json")
	err := manager.SaveWorkersForTest(mockWorkers, stateFile)
	require.NoError(t, err)

	t.Run("default pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 3)
		assert.False(t, response.HasMore)
		assert.Equal(t, 3, response.Total)
		assert.Empty(t, response.NextCursor)

		// Verify sorting (default: by started desc)
		assert.Equal(t, "worker3", response.Tasks[0].ID) // Most recent
		assert.Equal(t, "worker2", response.Tasks[1].ID)
		assert.Equal(t, "worker1", response.Tasks[2].ID) // Oldest
	})

	t.Run("limit pagination", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?limit=2", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 2)
		assert.True(t, response.HasMore)
		assert.Equal(t, 3, response.Total)
		assert.NotEmpty(t, response.NextCursor)

		// Should get first 2 tasks (most recent)
		assert.Equal(t, "worker3", response.Tasks[0].ID)
		assert.Equal(t, "worker2", response.Tasks[1].ID)
	})

	t.Run("cursor pagination", func(t *testing.T) {
		// First request to get cursor
		req1 := httptest.NewRequest("GET", "/api/tasks?limit=1", nil)
		w1 := httptest.NewRecorder()

		err := handler.ListTasks(w1, req1)
		require.NoError(t, err)

		var response1 PaginatedTasksResponse
		err = json.Unmarshal(w1.Body.Bytes(), &response1)
		require.NoError(t, err)

		assert.Len(t, response1.Tasks, 1)
		assert.True(t, response1.HasMore)
		assert.NotEmpty(t, response1.NextCursor)

		// Second request using cursor
		req2 := httptest.NewRequest("GET", "/api/tasks?limit=1&cursor="+response1.NextCursor, nil)
		w2 := httptest.NewRecorder()

		err = handler.ListTasks(w2, req2)
		require.NoError(t, err)

		var response2 PaginatedTasksResponse
		err = json.Unmarshal(w2.Body.Bytes(), &response2)
		require.NoError(t, err)

		assert.Len(t, response2.Tasks, 1)
		assert.True(t, response2.HasMore)

		// Should get the next task
		assert.Equal(t, "worker2", response2.Tasks[0].ID)
		assert.NotEqual(t, response1.Tasks[0].ID, response2.Tasks[0].ID)
	})
}

func TestListTasks_Filtering(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	now := time.Now()
	// Use PIDs that represent the current process to avoid status changes
	currentPID := os.Getpid() // Use current process PID for testing
	mockWorkers := map[string]*worker.Worker{
		"running1": {
			ID:       "running1",
			ThreadID: "T-1",
			PID:      currentPID, // This should remain "running"
			LogFile:  filepath.Join(tempDir, "worker-running1.log"),
			Started:  now.Add(-1 * time.Hour),
			Status:   "running",
		},
		"stopped1": {
			ID:       "stopped1",
			ThreadID: "T-2",
			PID:      99999, // This will be marked as "stopped"
			LogFile:  filepath.Join(tempDir, "worker-stopped1.log"),
			Started:  now.Add(-2 * time.Hour),
			Status:   "stopped",
		},
		"running2": {
			ID:       "running2",
			ThreadID: "T-3",
			PID:      currentPID, // This should remain "running"
			LogFile:  filepath.Join(tempDir, "worker-running2.log"),
			Started:  now.Add(-3 * time.Hour),
			Status:   "running",
		},
	}

	stateFile := filepath.Join(tempDir, "workers.json")
	err := manager.SaveWorkersForTest(mockWorkers, stateFile)
	require.NoError(t, err)

	t.Run("filter by status running", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?status=running", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 2)
		assert.Equal(t, 2, response.Total)
		for _, task := range response.Tasks {
			assert.Equal(t, "running", task.Status)
		}
	})

	t.Run("filter by status stopped", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?status=stopped", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 1)
		assert.Equal(t, 1, response.Total)
		assert.Equal(t, "stopped", response.Tasks[0].Status)
		assert.Equal(t, "stopped1", response.Tasks[0].ID)
	})

	t.Run("filter by multiple statuses", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?status=running,stopped", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 3)
		assert.Equal(t, 3, response.Total)
	})
}

func TestListTasks_Sorting(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	now := time.Now()
	currentPID := os.Getpid()
	mockWorkers := map[string]*worker.Worker{
		"worker_a": {
			ID:       "worker_a",
			ThreadID: "T-1",
			PID:      currentPID,
			Started:  now.Add(-1 * time.Hour),
			Status:   "running",
		},
		"worker_c": {
			ID:       "worker_c",
			ThreadID: "T-2",
			PID:      99999, // This will be marked as "stopped"
			Started:  now.Add(-2 * time.Hour),
			Status:   "stopped",
		},
		"worker_b": {
			ID:       "worker_b",
			ThreadID: "T-3",
			PID:      currentPID,
			Started:  now.Add(-3 * time.Hour),
			Status:   "running",
		},
	}

	stateFile := filepath.Join(tempDir, "workers.json")
	err := manager.SaveWorkersForTest(mockWorkers, stateFile)
	require.NoError(t, err)

	t.Run("sort by id asc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?sort_by=id&sort_order=asc", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Tasks, 3)
		assert.Equal(t, "worker_a", response.Tasks[0].ID)
		assert.Equal(t, "worker_b", response.Tasks[1].ID)
		assert.Equal(t, "worker_c", response.Tasks[2].ID)
	})

	t.Run("sort by status desc", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?sort_by=status&sort_order=desc", nil)
		w := httptest.NewRecorder()

		err := handler.ListTasks(w, req)
		require.NoError(t, err)

		var response PaginatedTasksResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// "stopped" should come before "running" in desc order
		assert.Equal(t, "stopped", response.Tasks[0].Status)
		assert.Equal(t, "running", response.Tasks[1].Status)
		assert.Equal(t, "running", response.Tasks[2].Status)
	})
}

func TestListTasks_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	// Create wrapped handler with error middleware
	wrappedHandler := middleware.Error(handler.ListTasks)

	t.Run("invalid limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?limit=invalid", nil)
		w := httptest.NewRecorder()

		wrappedHandler(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid limit parameter")
	})

	t.Run("invalid status", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?status=invalid", nil)
		w := httptest.NewRecorder()

		wrappedHandler(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid status filter")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks?cursor=invalid", nil)
		w := httptest.NewRecorder()

		wrappedHandler(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid cursor")
	})
}
