package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_StartWorker(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy script that simulates amp behavior
	scriptPath := filepath.Join(tmpDir, "dummy-amp")
	script := `#!/bin/bash
if [ "$1" = "threads" ] && [ "$2" = "new" ]; then
	echo "T-test-thread-123"
elif [ "$1" = "threads" ] && [ "$2" = "continue" ]; then
	echo "Message received: $(cat)"
	sleep 1
fi
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	// Create manager with custom amp binary path
	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath

	// Test starting a worker
	err = manager.StartWorker("test message")
	assert.NoError(t, err)

	// Give the worker a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify worker was saved
	workers, err := manager.ListWorkers()
	require.NoError(t, err)
	assert.Len(t, workers, 1)

	worker := workers[0]
	assert.Equal(t, StatusRunning, worker.Status)
	assert.Equal(t, "T-test-thread-123", worker.ThreadID)
	assert.NotEmpty(t, worker.ID)
	assert.Greater(t, worker.PID, 0)
}

func TestManager_StartWorker_ThreadCreationFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a script that fails thread creation
	scriptPath := filepath.Join(tmpDir, "failing-amp")
	script := `#!/bin/bash
exit 1
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath

	// Test starting a worker should fail
	err = manager.StartWorker("test message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create thread")
}

func TestManager_ListWorkers_EmptyState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	workers, err := manager.ListWorkers()
	assert.NoError(t, err)
	assert.Empty(t, workers)
}

func TestManager_StopWorker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a long-running dummy script
	scriptPath := filepath.Join(tmpDir, "dummy-amp")
	script := `#!/bin/bash
if [ "$1" = "threads" ] && [ "$2" = "new" ]; then
	echo "T-test-thread-123"
elif [ "$1" = "threads" ] && [ "$2" = "continue" ]; then
	echo "Message received: $(cat)"
	sleep 10  # Run for a while so we can stop it
fi
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath

	// Start a worker
	err = manager.StartWorker("test message")
	require.NoError(t, err)

	// Get the worker ID
	workers, err := manager.ListWorkers()
	require.NoError(t, err)
	require.Len(t, workers, 1)
	workerID := workers[0].ID

	// Stop the worker
	err = manager.StopWorker(workerID)
	assert.NoError(t, err)

	// Verify worker status is updated
	workers, err = manager.ListWorkers()
	require.NoError(t, err)
	require.Len(t, workers, 1)
	assert.Equal(t, StatusStopped, workers[0].Status)
}

func TestManager_StopWorker_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	err = manager.StopWorker("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "worker nonexistent not found")
}

func TestManager_createThread(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy script that outputs a thread ID
	scriptPath := filepath.Join(tmpDir, "dummy-amp")
	script := `#!/bin/bash
if [ "$1" = "threads" ] && [ "$2" = "new" ]; then
	echo "T-test-thread-123"
fi
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath

	threadID, err := manager.createThread()
	assert.NoError(t, err)
	assert.Equal(t, "T-test-thread-123", threadID)
}

func TestManager_createThread_InvalidFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a script that outputs invalid thread ID format
	scriptPath := filepath.Join(tmpDir, "dummy-amp")
	script := `#!/bin/bash
echo "invalid-thread-id"
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath

	_, err = manager.createThread()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected thread ID format")
}

// Test helper to create a command context that times out
func createTestCommand(ctx context.Context, script string) *exec.Cmd {
	return exec.CommandContext(ctx, "bash", "-c", script)
}

func TestManager_InterruptWorker(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)
	
	// Create a test worker directly in the state file
	testWorkers := map[string]*Worker{
		"test-worker": {
			ID:       "test-worker",
			ThreadID: "T-test-123",
			PID:      999999, // Use fake PID that doesn't exist
			LogFile:  filepath.Join(tmpDir, "test.log"),
			Started:  time.Now(),
			Status:   StatusRunning,
		},
	}
	
	err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
	require.NoError(t, err)
	
	// Test interrupt - expect error since PID doesn't exist, but state should still update
	err = manager.InterruptWorker("test-worker")
	// Don't require no error since fake PID causes signal failure
	
	// Verify status changed even though signal failed
	workers, err := manager.loadWorkers()
	require.NoError(t, err)
	assert.Equal(t, StatusInterrupted, workers["test-worker"].Status)
}

func TestManager_InterruptWorker_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)
	
	err = manager.InterruptWorker("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_InterruptWorker_InvalidTransition(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)
	
	// Create a test worker in stopped state
	testWorkers := map[string]*Worker{
		"test-worker": {
			ID:       "test-worker",
			ThreadID: "T-test-123",
			PID:      12345,
			LogFile:  filepath.Join(tmpDir, "test.log"),
			Started:  time.Now(),
			Status:   StatusCompleted, // Cannot interrupt completed worker
		},
	}
	
	err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
	require.NoError(t, err)
	
	err = manager.InterruptWorker("test-worker")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot interrupt")
}

func TestManager_AbortWorker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)
	
	testWorkers := map[string]*Worker{
		"test-worker": {
			ID:       "test-worker",
			ThreadID: "T-test-123",
			PID:      999999, // Use fake PID that doesn't exist
			LogFile:  filepath.Join(tmpDir, "test.log"),
			Started:  time.Now(),
			Status:   StatusRunning,
		},
	}
	
	err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
	require.NoError(t, err)
	
	err = manager.AbortWorker("test-worker")
	// Don't require no error since fake PID causes signal failure
	
	workers, err := manager.loadWorkers()
	require.NoError(t, err)
	assert.Equal(t, StatusAborted, workers["test-worker"].Status)
}

func TestManager_RetryWorker(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy script that simulates amp behavior
	scriptPath := filepath.Join(tmpDir, "dummy-amp")
	script := `#!/bin/bash
if [ "$1" = "threads" ] && [ "$2" = "continue" ]; then
	echo "Retry message: $(cat)"
	exit 0
fi
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	manager := NewManager(tmpDir)
	manager.ampBinaryPath = scriptPath
	
	// Create a stopped worker that can be retried
	testWorkers := map[string]*Worker{
		"test-worker": {
			ID:       "test-worker",
			ThreadID: "T-test-123",
			PID:      12345, // Old PID
			LogFile:  filepath.Join(tmpDir, "test.log"),
			Started:  time.Now(),
			Status:   StatusStopped,
		},
	}
	
	err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
	require.NoError(t, err)
	
	// Create log file
	_, err = os.Create(filepath.Join(tmpDir, "test.log"))
	require.NoError(t, err)
	
	err = manager.RetryWorker("test-worker", "retry message")
	require.NoError(t, err)
	
	workers, err := manager.loadWorkers()
	require.NoError(t, err)
	
	worker := workers["test-worker"]
	assert.Equal(t, StatusRunning, worker.Status)
	assert.NotEqual(t, 12345, worker.PID) // PID should have changed
}

func TestManager_RetryWorker_InvalidTransition(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worker-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)
	
	// Create a worker in an invalid state for retry (doesn't exist in our state machine)
	testWorkers := map[string]*Worker{
		"test-worker": {
			ID:       "test-worker",
			ThreadID: "T-test-123",
			PID:      12345,
			LogFile:  filepath.Join(tmpDir, "test.log"),
			Started:  time.Now(),
			Status:   WorkerStatus("invalid"), // Invalid status
		},
	}
	
	err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
	require.NoError(t, err)
	
	err = manager.RetryWorker("test-worker", "retry message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot retry")
}

func TestManager_UpdateWorkerMetadata(t *testing.T) {
tmpDir, err := os.MkdirTemp("", "worker-test-*")
require.NoError(t, err)
defer os.RemoveAll(tmpDir)

manager := NewManager(tmpDir)

// Create a test worker
testWorkers := map[string]*Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      12345,
LogFile:  filepath.Join(tmpDir, "test.log"),
Started:  time.Now(),
Status:   StatusRunning,
},
}

err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
require.NoError(t, err)

// Update metadata
title := "Updated Task"
description := "New description"
priority := "high"
tags := []string{"urgent", "bug"}

err = manager.UpdateWorkerMetadata("test-worker", &title, &description, &priority, tags)
require.NoError(t, err)

// Verify updates
workers, err := manager.loadWorkers()
require.NoError(t, err)

worker := workers["test-worker"]
assert.Equal(t, "Updated Task", worker.Title)
assert.Equal(t, "New description", worker.Description)
assert.Equal(t, "high", worker.Priority)
assert.Equal(t, []string{"urgent", "bug"}, worker.Tags)
}

func TestManager_UpdateWorkerMetadata_NotFound(t *testing.T) {
tmpDir, err := os.MkdirTemp("", "worker-test-*")
require.NoError(t, err)
defer os.RemoveAll(tmpDir)

manager := NewManager(tmpDir)

title := "Updated Task"
err = manager.UpdateWorkerMetadata("nonexistent", &title, nil, nil, nil)
assert.Error(t, err)
assert.Contains(t, err.Error(), "not found")
}

func TestManager_DeleteWorker(t *testing.T) {
tmpDir, err := os.MkdirTemp("", "worker-test-*")
require.NoError(t, err)
defer os.RemoveAll(tmpDir)

manager := NewManager(tmpDir)

// Create test log file
logFile := filepath.Join(tmpDir, "test.log")
_, err = os.Create(logFile)
require.NoError(t, err)

// Create a test worker
testWorkers := map[string]*Worker{
"test-worker": {
ID:       "test-worker",
ThreadID: "T-test-123",
PID:      999999, // Fake PID
LogFile:  logFile,
Started:  time.Now(),
Status:   StatusStopped,
},
}

err = manager.SaveWorkersForTest(testWorkers, filepath.Join(tmpDir, "workers.json"))
require.NoError(t, err)

// Delete worker
err = manager.DeleteWorker("test-worker")
require.NoError(t, err)

// Verify worker is deleted
workers, err := manager.loadWorkers()
require.NoError(t, err)
_, exists := workers["test-worker"]
assert.False(t, exists)

// Verify log file is cleaned up
_, err = os.Stat(logFile)
assert.True(t, os.IsNotExist(err))
}

func TestManager_DeleteWorker_NotFound(t *testing.T) {
tmpDir, err := os.MkdirTemp("", "worker-test-*")
require.NoError(t, err)
defer os.RemoveAll(tmpDir)

manager := NewManager(tmpDir)

err = manager.DeleteWorker("nonexistent")
assert.Error(t, err)
assert.Contains(t, err.Error(), "not found")
}

func TestManagerThreadMessages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "manager_thread_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)
	workerID := "test-worker-123"

	t.Run("AppendThreadMessage", func(t *testing.T) {
		// Test basic message appending
		err := manager.AppendThreadMessage(workerID, MessageTypeUser, "Hello", nil)
		assert.NoError(t, err)

		// Test with metadata
		metadata := map[string]interface{}{"source": "api"}
		err = manager.AppendThreadMessage(workerID, MessageTypeAssistant, "Hello back!", metadata)
		assert.NoError(t, err)
	})

	t.Run("GetThreadMessages", func(t *testing.T) {
		messages, err := manager.GetThreadMessages(workerID, 0, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 2)

		// Check first message
		assert.Equal(t, MessageTypeUser, messages[0].Type)
		assert.Equal(t, "Hello", messages[0].Content)
		assert.Nil(t, messages[0].Metadata)

		// Check second message
		assert.Equal(t, MessageTypeAssistant, messages[1].Type)
		assert.Equal(t, "Hello back!", messages[1].Content)
		assert.Equal(t, "api", messages[1].Metadata["source"])
	})

	t.Run("GetThreadMessagesWithPagination", func(t *testing.T) {
		// Test with limit
		messages, err := manager.GetThreadMessages(workerID, 1, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Hello", messages[0].Content)

		// Test with offset
		messages, err = manager.GetThreadMessages(workerID, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Hello back!", messages[0].Content)
	})

	t.Run("CountThreadMessages", func(t *testing.T) {
		count, err := manager.CountThreadMessages(workerID)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("ThreadMessageCallback", func(t *testing.T) {
		callbackCalled := false
		var receivedWorkerID string
		var receivedMessage ThreadMessage

		manager.SetThreadMessageCallback(func(wID string, msg ThreadMessage) {
			callbackCalled = true
			receivedWorkerID = wID
			receivedMessage = msg
		})

		err := manager.AppendThreadMessage("callback-test", MessageTypeSystem, "System message", nil)
		assert.NoError(t, err)

		assert.True(t, callbackCalled)
		assert.Equal(t, "callback-test", receivedWorkerID)
		assert.Equal(t, MessageTypeSystem, receivedMessage.Type)
		assert.Equal(t, "System message", receivedMessage.Content)
		assert.NotEmpty(t, receivedMessage.ID)
		assert.False(t, receivedMessage.Timestamp.IsZero())
	})

	t.Run("NonExistentWorker", func(t *testing.T) {
		messages, err := manager.GetThreadMessages("non-existent", 0, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 0)

		count, err := manager.CountThreadMessages("non-existent")
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
