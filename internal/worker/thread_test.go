package worker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreadStorage(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "thread_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	storage := NewThreadStorage(tempDir)
	taskID := "test-task-123"

	t.Run("AppendMessage", func(t *testing.T) {
		message := ThreadMessage{
			ID:        "msg-1",
			Type:      MessageTypeUser,
			Content:   "Hello, world!",
			Timestamp: time.Now(),
			Metadata:  map[string]interface{}{"source": "test"},
		}

		err := storage.AppendMessage(taskID, message)
		assert.NoError(t, err)

		// Verify file was created
		filePath := storage.getThreadFilePath(taskID)
		_, err = os.Stat(filePath)
		assert.NoError(t, err)
	})

	t.Run("ReadMessages", func(t *testing.T) {
		// Add another message
		message2 := ThreadMessage{
			ID:        "msg-2",
			Type:      MessageTypeAssistant,
			Content:   "Hello back!",
			Timestamp: time.Now(),
			Metadata:  map[string]interface{}{"tool": "test"},
		}

		err := storage.AppendMessage(taskID, message2)
		require.NoError(t, err)

		// Read all messages
		messages, err := storage.ReadMessages(taskID, 0, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 2)

		// Check first message
		assert.Equal(t, "msg-1", messages[0].ID)
		assert.Equal(t, MessageTypeUser, messages[0].Type)
		assert.Equal(t, "Hello, world!", messages[0].Content)
		assert.Equal(t, "test", messages[0].Metadata["source"])

		// Check second message
		assert.Equal(t, "msg-2", messages[1].ID)
		assert.Equal(t, MessageTypeAssistant, messages[1].Type)
		assert.Equal(t, "Hello back!", messages[1].Content)
		assert.Equal(t, "test", messages[1].Metadata["tool"])
	})

	t.Run("ReadMessagesWithPagination", func(t *testing.T) {
		// Test with limit
		messages, err := storage.ReadMessages(taskID, 1, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "msg-1", messages[0].ID)

		// Test with offset
		messages, err = storage.ReadMessages(taskID, 1, 1)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "msg-2", messages[0].ID)

		// Test with offset beyond available messages
		messages, err = storage.ReadMessages(taskID, 1, 5)
		assert.NoError(t, err)
		assert.Len(t, messages, 0)
	})

	t.Run("CountMessages", func(t *testing.T) {
		count, err := storage.CountMessages(taskID)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("NonExistentTask", func(t *testing.T) {
		// Reading from non-existent task should return empty slice
		messages, err := storage.ReadMessages("non-existent", 0, 0)
		assert.NoError(t, err)
		assert.Len(t, messages, 0)

		// Counting non-existent task should return 0
		count, err := storage.CountMessages("non-existent")
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("MalformedJSONLines", func(t *testing.T) {
		// Write malformed JSON to file
		malformedTaskID := "malformed-task"
		filePath := storage.getThreadFilePath(malformedTaskID)
		
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		require.NoError(t, err)
		
		err = os.WriteFile(filePath, []byte(`{"valid": "json"}
invalid json line
{"another": "valid"}
`), 0644)
		require.NoError(t, err)

		// Should skip malformed lines and return only valid ones
		messages, err := storage.ReadMessages(malformedTaskID, 0, 0)
		assert.NoError(t, err)
		// The valid JSON lines will create empty ThreadMessage structs, malformed line will be skipped
		assert.Len(t, messages, 2) // Two valid JSON objects (though with zero values)

		// Count should still work (counts all lines including malformed)
		count, err := storage.CountMessages(malformedTaskID)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})
}

func TestThreadMessage(t *testing.T) {
	t.Run("MessageTypes", func(t *testing.T) {
		assert.Equal(t, MessageType("user"), MessageTypeUser)
		assert.Equal(t, MessageType("assistant"), MessageTypeAssistant)
		assert.Equal(t, MessageType("system"), MessageTypeSystem)
		assert.Equal(t, MessageType("tool"), MessageTypeTool)
	})

	t.Run("MessageCreation", func(t *testing.T) {
		timestamp := time.Now()
		metadata := map[string]interface{}{
			"model":  "gpt-4",
			"tokens": 150,
		}

		message := ThreadMessage{
			ID:        "test-id",
			Type:      MessageTypeAssistant,
			Content:   "Test content",
			Timestamp: timestamp,
			Metadata:  metadata,
		}

		assert.Equal(t, "test-id", message.ID)
		assert.Equal(t, MessageTypeAssistant, message.Type)
		assert.Equal(t, "Test content", message.Content)
		assert.Equal(t, timestamp, message.Timestamp)
		assert.Equal(t, "gpt-4", message.Metadata["model"])
		assert.Equal(t, 150, message.Metadata["tokens"])
	})
}
