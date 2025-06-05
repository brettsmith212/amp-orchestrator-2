package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brettsmith212/amp-orchestrator-2/internal/worker"
)

func TestGetTaskThread(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "thread_api_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	manager := worker.NewManager(tempDir)
	handler := GetTaskThread(manager)

	// Add some test messages
	taskID := "test-task-123"
	err = manager.AppendThreadMessage(taskID, worker.MessageTypeUser, "Hello", nil)
	require.NoError(t, err)

	metadata := map[string]interface{}{"tool": "test_tool"}
	err = manager.AppendThreadMessage(taskID, worker.MessageTypeAssistant, "Hello back!", metadata)
	require.NoError(t, err)

	err = manager.AppendThreadMessage(taskID, worker.MessageTypeSystem, "System message", nil)
	require.NoError(t, err)

	setURLParam := func(req *http.Request, key, value string) *http.Request {
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
			URLParams: chi.RouteParams{
				Keys:   []string{key},
				Values: []string{value},
			},
		}))
	}

	t.Run("GetAllMessages", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Messages, 3)
		assert.Equal(t, 3, response.Total)
		assert.False(t, response.HasMore)

		// Check message order and content
		assert.Equal(t, "user", response.Messages[0].Type)
		assert.Equal(t, "Hello", response.Messages[0].Content)
		assert.Nil(t, response.Messages[0].Metadata)

		assert.Equal(t, "assistant", response.Messages[1].Type)
		assert.Equal(t, "Hello back!", response.Messages[1].Content)
		assert.Equal(t, "test_tool", response.Messages[1].Metadata["tool"])

		assert.Equal(t, "system", response.Messages[2].Type)
		assert.Equal(t, "System message", response.Messages[2].Content)
	})

	t.Run("GetMessagesWithLimit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread?limit=2", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Messages, 2)
		assert.Equal(t, 3, response.Total)
		assert.True(t, response.HasMore)

		// Should get first two messages
		assert.Equal(t, "Hello", response.Messages[0].Content)
		assert.Equal(t, "Hello back!", response.Messages[1].Content)
	})

	t.Run("GetMessagesWithOffset", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread?limit=1&offset=1", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Messages, 1)
		assert.Equal(t, 3, response.Total)
		assert.True(t, response.HasMore)

		// Should get second message
		assert.Equal(t, "Hello back!", response.Messages[0].Content)
	})

	t.Run("GetMessagesExceedsLimit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread?limit=200", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Should be capped at 100, but we only have 3 messages
		assert.Len(t, response.Messages, 3)
		assert.Equal(t, 3, response.Total)
		assert.False(t, response.HasMore)
	})

	t.Run("InvalidParameters", func(t *testing.T) {
		// Test invalid limit
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread?limit=invalid", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Should use default limit

		// Test invalid offset
		req = httptest.NewRequest("GET", "/api/tasks/test-task-123/thread?offset=invalid", nil)
		req = setURLParam(req, "id", taskID)

		w = httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // Should use default offset
	})

	t.Run("NonExistentTask", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/nonexistent/thread", nil)
		req = setURLParam(req, "id", "nonexistent")

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Len(t, response.Messages, 0)
		assert.Equal(t, 0, response.Total)
		assert.False(t, response.HasMore)
	})

	t.Run("MissingTaskID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks//thread", nil)
		// Don't set URL param

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("MessageTimestamps", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/test-task-123/thread", nil)
		req = setURLParam(req, "id", taskID)

		w := httptest.NewRecorder()
		handler(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response PaginatedThreadResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check that all messages have valid timestamps
		for i, msg := range response.Messages {
			assert.False(t, msg.Timestamp.IsZero(), "Message %d should have a valid timestamp", i)
			assert.NotEmpty(t, msg.ID, "Message %d should have an ID", i)
		}

		// Check timestamps are in order (first message should be earliest)
		if len(response.Messages) > 1 {
			for i := 1; i < len(response.Messages); i++ {
				assert.True(t, 
					response.Messages[i].Timestamp.After(response.Messages[i-1].Timestamp) ||
					response.Messages[i].Timestamp.Equal(response.Messages[i-1].Timestamp),
					"Messages should be ordered by timestamp")
			}
		}
	})
}
