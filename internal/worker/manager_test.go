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
	assert.Equal(t, "running", worker.Status)
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
	assert.Equal(t, "stopped", workers[0].Status)
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
