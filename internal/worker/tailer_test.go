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

func TestLogTailer_Basic(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Collect log lines
	var lines []LogLine
	callback := func(line LogLine) {
		lines = append(lines, line)
	}

	// Create tailer
	tailer := NewLogTailer(logFile, "test-worker", callback)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := tailer.Start(ctx)
	require.NoError(t, err)
	defer tailer.Stop()

	// Write to file
	file, err := os.Create(logFile)
	require.NoError(t, err)
	
	_, err = file.WriteString("line 1\n")
	require.NoError(t, err)
	file.Sync()

	// Wait for line to be read
	assert.Eventually(t, func() bool {
		return len(lines) == 1
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "test-worker", lines[0].WorkerID)
	assert.Equal(t, "line 1", lines[0].Content)

	// Write another line
	_, err = file.WriteString("line 2\n")
	require.NoError(t, err)
	file.Sync()

	// Wait for second line
	assert.Eventually(t, func() bool {
		return len(lines) == 2
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "line 2", lines[1].Content)

	file.Close()
}

func TestLogTailer_FileDoesNotExistInitially(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "delayed.log")

	var lines []LogLine
	callback := func(line LogLine) {
		lines = append(lines, line)
	}

	tailer := NewLogTailer(logFile, "delayed-worker", callback)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := tailer.Start(ctx)
	require.NoError(t, err)
	defer tailer.Stop()

	// Wait a bit to ensure tailer is running but no file exists
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, lines)

	// Create file and write
	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.WriteString("delayed line\n")
	require.NoError(t, err)
	file.Sync()

	// Wait for line to be read
	assert.Eventually(t, func() bool {
		return len(lines) == 1
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "delayed-worker", lines[0].WorkerID)
	assert.Equal(t, "delayed line", lines[0].Content)
}

func TestLogTailer_MultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "multi.log")

	var lines []LogLine
	callback := func(line LogLine) {
		lines = append(lines, line)
	}

	tailer := NewLogTailer(logFile, "multi-worker", callback)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := tailer.Start(ctx)
	require.NoError(t, err)
	defer tailer.Stop()

	// Write multiple lines at once
	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	content := "line 1\nline 2\nline 3\n"
	_, err = file.WriteString(content)
	require.NoError(t, err)
	file.Sync()

	// Wait for all lines to be read
	assert.Eventually(t, func() bool {
		return len(lines) == 3
	}, time.Second, 10*time.Millisecond)

	assert.Equal(t, "line 1", lines[0].Content)
	assert.Equal(t, "line 2", lines[1].Content)
	assert.Equal(t, "line 3", lines[2].Content)
}

func TestLogTailer_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "stop.log")

	var lines []LogLine
	callback := func(line LogLine) {
		lines = append(lines, line)
	}

	tailer := NewLogTailer(logFile, "stop-worker", callback)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := tailer.Start(ctx)
	require.NoError(t, err)

	// Create file and write
	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.WriteString("before stop\n")
	require.NoError(t, err)
	file.Sync()

	// Wait for line to be read
	assert.Eventually(t, func() bool {
		return len(lines) == 1
	}, time.Second, 10*time.Millisecond)

	// Stop tailer
	tailer.Stop()
	time.Sleep(50 * time.Millisecond)

	// Write more lines - should not be read
	_, err = file.WriteString("after stop\n")
	require.NoError(t, err)
	file.Sync()

	time.Sleep(50 * time.Millisecond)
	assert.Len(t, lines, 1)
	assert.Equal(t, "before stop", lines[0].Content)
}
