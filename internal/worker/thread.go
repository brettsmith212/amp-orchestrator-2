package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MessageType represents the type of thread message
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeSystem    MessageType = "system"
	MessageTypeTool      MessageType = "tool"
)

// ThreadMessage represents a single message in a task's conversation thread
type ThreadMessage struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	Content   string      `json:"content"`
	Timestamp time.Time   `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ThreadStorage handles reading and writing thread messages to JSONL files
type ThreadStorage struct {
	baseDir string
}

// NewThreadStorage creates a new thread storage instance
func NewThreadStorage(baseDir string) *ThreadStorage {
	return &ThreadStorage{
		baseDir: baseDir,
	}
}

// getThreadFilePath returns the path to the thread file for a given task ID
func (ts *ThreadStorage) getThreadFilePath(taskID string) string {
	return filepath.Join(ts.baseDir, fmt.Sprintf("thread_%s.jsonl", taskID))
}

// AppendMessage appends a message to the thread file for the given task
func (ts *ThreadStorage) AppendMessage(taskID string, message ThreadMessage) error {
	filePath := ts.getThreadFilePath(taskID)
	
	// Ensure directory exists
	if err := os.MkdirAll(ts.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create thread directory: %w", err)
	}
	
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open thread file: %w", err)
	}
	defer file.Close()
	
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	if _, err := file.Write(append(messageJSON, '\n')); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	return nil
}

// ReadMessages reads messages from the thread file with optional pagination
func (ts *ThreadStorage) ReadMessages(taskID string, limit, offset int) ([]ThreadMessage, error) {
	filePath := ts.getThreadFilePath(taskID)
	
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ThreadMessage{}, nil
		}
		return nil, fmt.Errorf("failed to open thread file: %w", err)
	}
	defer file.Close()
	
	var messages []ThreadMessage
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		if offset > 0 && lineNum < offset {
			lineNum++
			continue
		}
		
		if limit > 0 && len(messages) >= limit {
			break
		}
		
		var message ThreadMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			// Skip malformed lines
			continue
		}
		
		messages = append(messages, message)
		lineNum++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read thread file: %w", err)
	}
	
	return messages, nil
}

// CountMessages returns the total number of messages in the thread
func (ts *ThreadStorage) CountMessages(taskID string) (int, error) {
	filePath := ts.getThreadFilePath(taskID)
	
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to open thread file: %w", err)
	}
	defer file.Close()
	
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	
	return count, nil
}
