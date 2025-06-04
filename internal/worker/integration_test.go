package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerLogIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir)

	var receivedLogs []LogLine
	manager.SetLogCallback(func(line LogLine) {
		receivedLogs = append(receivedLogs, line)
	})

	// Create a test script that outputs some lines
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	script := `#!/bin/bash
echo "Starting test"
sleep 0.1
echo "Middle line"
sleep 0.1
echo "Ending test"
`
	err := os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	// Override the amp binary path to use our test script
	manager.ampBinaryPath = "bash"

	// Start a "worker" using our test script instead of amp
	// We'll use a direct approach by manually creating a worker and log file
	workerID := "test-worker-123"
	logFile := filepath.Join(tmpDir, "worker-"+workerID+".log")

	// Start log tailer
	tailer := NewLogTailer(logFile, workerID, manager.onLogLine)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = tailer.Start(ctx)
	require.NoError(t, err)
	defer tailer.Stop()

	// Simulate worker output by writing to log file
	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	// Write log lines with delays to test real-time tailing
	lines := []string{"Starting test", "Middle line", "Ending test"}
	for _, line := range lines {
		_, err = file.WriteString(line + "\n")
		require.NoError(t, err)
		file.Sync()
		time.Sleep(50 * time.Millisecond) // Give tailer time to read
	}

	// Wait for all log lines to be processed
	assert.Eventually(t, func() bool {
		return len(receivedLogs) == 3
	}, 2*time.Second, 50*time.Millisecond)

	// Verify received logs
	require.Len(t, receivedLogs, 3)
	assert.Equal(t, workerID, receivedLogs[0].WorkerID)
	assert.Equal(t, "Starting test", receivedLogs[0].Content)
	assert.Equal(t, "Middle line", receivedLogs[1].Content)
	assert.Equal(t, "Ending test", receivedLogs[2].Content)

	// Verify timestamps are reasonable
	for _, log := range receivedLogs {
		assert.WithinDuration(t, time.Now(), log.Timestamp, time.Minute)
	}
}
