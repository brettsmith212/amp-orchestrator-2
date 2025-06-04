package worker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LogLine represents a single log line with metadata
type LogLine struct {
	WorkerID  string    `json:"worker_id"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// LogCallback is called when a new log line is read
type LogCallback func(LogLine)

// LogTailer follows a log file and calls the callback for each new line
type LogTailer struct {
	filePath string
	callback LogCallback
	cancel   context.CancelFunc
}

// NewLogTailer creates a new log tailer for the given file
func NewLogTailer(filePath string, workerID string, callback LogCallback) *LogTailer {
	wrappedCallback := func(line LogLine) {
		line.WorkerID = workerID
		callback(line)
	}
	
	return &LogTailer{
		filePath: filePath,
		callback: wrappedCallback,
	}
}

// Start begins tailing the log file
func (t *LogTailer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	// Ensure the directory exists
	dir := filepath.Dir(t.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	go t.tailFile(ctx)
	return nil
}

// Stop stops the log tailer
func (t *LogTailer) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

// tailFile implements the actual file tailing logic
func (t *LogTailer) tailFile(ctx context.Context) {
	var file *os.File
	var scanner *bufio.Scanner
	var lastSize int64

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if file != nil {
				file.Close()
			}
			return
		case <-ticker.C:
			stat, err := os.Stat(t.filePath)
			if err != nil {
				// File doesn't exist yet, wait for it
				if file != nil {
					file.Close()
					file = nil
					scanner = nil
				}
				continue
			}

			// File exists now
			if file == nil {
				file, err = os.Open(t.filePath)
				if err != nil {
					continue
				}
				scanner = bufio.NewScanner(file)
				lastSize = 0
			}

			// Check if file was truncated or rotated
			if stat.Size() < lastSize {
				file.Close()
				file, err = os.Open(t.filePath)
				if err != nil {
					continue
				}
				scanner = bufio.NewScanner(file)
				lastSize = 0
			}

			// Seek to where we left off
			if lastSize > 0 {
				file.Seek(lastSize, io.SeekStart)
				scanner = bufio.NewScanner(file)
			}

			// Read new lines
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					t.callback(LogLine{
						Timestamp: time.Now(),
						Content:   line,
					})
				}
			}

			// Update position
			pos, _ := file.Seek(0, io.SeekCurrent)
			lastSize = pos
		}
	}
}
