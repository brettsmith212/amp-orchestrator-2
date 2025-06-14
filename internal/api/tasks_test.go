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

func TestListTasks_EmptyManager(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	err := handler.ListTasks(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response PaginatedTasksResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Empty(t, response.Tasks)
	assert.Equal(t, 0, response.Total)
	assert.False(t, response.HasMore)
}

func TestListTasks_WithWorkers(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)

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

	err = handler.ListTasks(w, req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response PaginatedTasksResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Len(t, response.Tasks, 2)
	assert.Equal(t, 2, response.Total)
	assert.False(t, response.HasMore)

	// Sort tasks by ID for consistent testing
	tasks := response.Tasks
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

func TestStartTask_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)
	
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.StartTask(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid JSON request body")
}

func TestStartTask_EmptyMessage(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)
	
	reqBody := `{"message":""}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.StartTask(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Message is required")
}

func TestStartTask_MissingMessage(t *testing.T) {
	tempDir := t.TempDir()
	manager := worker.NewManager(tempDir)
	h := hub.NewHub()
	handler := NewTaskHandler(manager, h)
	
	reqBody := `{}`
	req := httptest.NewRequest("POST", "/api/tasks", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	handler.StartTask(w, req)
	
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Message is required")
}

func TestInterruptTask(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run() // Start the hub in a goroutine
	handler := NewTaskHandler(manager, h)

// Create a test worker
testWorkers := map[string]*worker.Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999, // Use fake PID that doesn't exist
LogFile:  filepath.Join(tempDir, "test.log"),
Started:  time.Now(),
Status:   worker.StatusRunning,
},
}

err := manager.SaveWorkersForTest(testWorkers, filepath.Join(tempDir, "workers.json"))
require.NoError(t, err)

req := httptest.NewRequest("POST", "/api/tasks/test-worker/interrupt", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
  Keys:   []string{"id"},
			Values: []string{"test-worker"},
 },
	}))
	w := httptest.NewRecorder()

	handler.InterruptTask(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestInterruptTask_NotFound(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run() // Start the hub in a goroutine
	handler := NewTaskHandler(manager, h)

req := httptest.NewRequest("POST", "/api/tasks/nonexistent/interrupt", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
  Keys:   []string{"id"},
			Values: []string{"nonexistent"},
 },
}))
	w := httptest.NewRecorder()

	handler.InterruptTask(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Task not found")
}

func TestAbortTask(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run() // Start the hub in a goroutine
	handler := NewTaskHandler(manager, h)

testWorkers := map[string]*worker.Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999, // Use fake PID that doesn't exist
LogFile:  filepath.Join(tempDir, "test.log"),
Started:  time.Now(),
Status:   worker.StatusRunning,
},
}

err := manager.SaveWorkersForTest(testWorkers, filepath.Join(tempDir, "workers.json"))
require.NoError(t, err)

req := httptest.NewRequest("POST", "/api/tasks/test-worker/abort", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
  Keys:   []string{"id"},
			Values: []string{"test-worker"},
 },
	}))
	w := httptest.NewRecorder()

	handler.AbortTask(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestPatchTask(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run() // Start the hub in a goroutine
handler := NewTaskHandler(manager, h)

// Create a test worker
testWorkers := map[string]*worker.Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999,
LogFile:  filepath.Join(tempDir, "test.log"),
Started:  time.Now(),
Status:   worker.StatusRunning,
},
}

err := manager.SaveWorkersForTest(testWorkers, filepath.Join(tempDir, "workers.json"))
require.NoError(t, err)

reqBody := `{"title": "Updated Task", "description": "New description", "priority": "high", "tags": ["urgent", "bug"]}`
req := httptest.NewRequest("PATCH", "/api/tasks/test-worker", strings.NewReader(reqBody))
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"test-worker"},
},
}))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handler.PatchTask(w, req)

assert.Equal(t, http.StatusOK, w.Code)
}

func TestPatchTask_NotFound(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run()
handler := NewTaskHandler(manager, h)

reqBody := `{"title": "Updated Task"}`
req := httptest.NewRequest("PATCH", "/api/tasks/nonexistent", strings.NewReader(reqBody))
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"nonexistent"},
},
}))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()

handler.PatchTask(w, req)

assert.Equal(t, http.StatusNotFound, w.Code)
assert.Contains(t, w.Body.String(), "Task not found")
}

func TestDeleteTask(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run()
handler := NewTaskHandler(manager, h)

// Create a test worker
testWorkers := map[string]*worker.Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999,
LogFile:  filepath.Join(tempDir, "test.log"),
Started:  time.Now(),
Status:   worker.StatusStopped,
},
}

err := manager.SaveWorkersForTest(testWorkers, filepath.Join(tempDir, "workers.json"))
require.NoError(t, err)

req := httptest.NewRequest("DELETE", "/api/tasks/test-worker", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"test-worker"},
},
}))
w := httptest.NewRecorder()

handler.DeleteTask(w, req)

assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteTask_NotFound(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run()
handler := NewTaskHandler(manager, h)

req := httptest.NewRequest("DELETE", "/api/tasks/nonexistent", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"nonexistent"},
},
}))
w := httptest.NewRecorder()

handler.DeleteTask(w, req)

assert.Equal(t, http.StatusNotFound, w.Code)
assert.Contains(t, w.Body.String(), "Task not found")
}

func TestGitStubEndpoints(t *testing.T) {
tempDir := t.TempDir()
manager := worker.NewManager(tempDir)
h := hub.NewHub()
go h.Run()
handler := NewTaskHandler(manager, h)

// Create a test worker
testWorkers := map[string]*worker.Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999,
LogFile:  filepath.Join(tempDir, "test.log"),
Started:  time.Now(),
Status:   worker.StatusCompleted,
},
}

err := manager.SaveWorkersForTest(testWorkers, filepath.Join(tempDir, "workers.json"))
require.NoError(t, err)

// Test merge endpoint
req := httptest.NewRequest("POST", "/api/tasks/test-worker/merge", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"test-worker"},
},
}))
w := httptest.NewRecorder()

handler.MergeTask(w, req)

assert.Equal(t, http.StatusAccepted, w.Code)
assert.Contains(t, w.Body.String(), "TODO: Git merge operation not yet implemented")

// Test delete-branch endpoint
req = httptest.NewRequest("POST", "/api/tasks/test-worker/delete-branch", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"test-worker"},
},
}))
w = httptest.NewRecorder()

handler.DeleteBranchTask(w, req)

assert.Equal(t, http.StatusAccepted, w.Code)
assert.Contains(t, w.Body.String(), "TODO: Git branch deletion not yet implemented")

// Test create-pr endpoint
req = httptest.NewRequest("POST", "/api/tasks/test-worker/create-pr", nil)
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
URLParams: chi.RouteParams{
Keys:   []string{"id"},
Values: []string{"test-worker"},
},
}))
w = httptest.NewRecorder()

handler.CreatePRTask(w, req)

assert.Equal(t, http.StatusAccepted, w.Code)
assert.Contains(t, w.Body.String(), "TODO: Create pull request operation not yet implemented")
}
