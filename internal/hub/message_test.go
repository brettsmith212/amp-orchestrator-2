package hub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateMessage(t *testing.T) {
	t.Run("CreateMessageWithData", func(t *testing.T) {
		data := PingMessage{
			ID:        "test-ping",
			Timestamp: time.Now(),
		}

		msg, err := CreateMessage(MessageTypePing, data)
		require.NoError(t, err)
		assert.Equal(t, MessageTypePing, msg.Type)
		assert.NotNil(t, msg.Data)
		assert.False(t, msg.Timestamp.IsZero())

		// Verify data can be unmarshaled
		var pingData PingMessage
		err = json.Unmarshal(msg.Data, &pingData)
		require.NoError(t, err)
		assert.Equal(t, data.ID, pingData.ID)
	})

	t.Run("CreateMessageWithoutData", func(t *testing.T) {
		msg, err := CreateMessage(MessageTypeHeartbeat, nil)
		require.NoError(t, err)
		assert.Equal(t, MessageTypeHeartbeat, msg.Type)
		assert.Nil(t, msg.Data)
		assert.False(t, msg.Timestamp.IsZero())
	})

	t.Run("CreateMessageWithInvalidData", func(t *testing.T) {
		// Channel cannot be marshaled to JSON
		invalidData := make(chan int)
		_, err := CreateMessage(MessageTypePing, invalidData)
		assert.Error(t, err)
	})
}

func TestParseMessage(t *testing.T) {
	t.Run("ParseValidMessage", func(t *testing.T) {
		original := &WebSocketMessage{
			Type:      MessageTypePong,
			Timestamp: time.Now(),
		}

		// Add some data
		pongData := PongMessage{
			ID:        "test-pong",
			Timestamp: time.Now(),
			PingID:    "test-ping",
		}
		dataBytes, err := json.Marshal(pongData)
		require.NoError(t, err)
		original.Data = dataBytes

		// Marshal to JSON
		msgBytes, err := json.Marshal(original)
		require.NoError(t, err)

		// Parse back
		parsed, err := ParseMessage(msgBytes)
		require.NoError(t, err)
		assert.Equal(t, original.Type, parsed.Type)
		assert.Equal(t, original.Data, parsed.Data)
	})

	t.Run("ParseInvalidMessage", func(t *testing.T) {
		invalidJSON := []byte(`{"type": "ping", "invalid": }`)
		_, err := ParseMessage(invalidJSON)
		assert.Error(t, err)
	})

	t.Run("ParseEmptyMessage", func(t *testing.T) {
		_, err := ParseMessage([]byte{})
		assert.Error(t, err)
	})
}

func TestMarshalMessage(t *testing.T) {
	t.Run("MarshalValidMessage", func(t *testing.T) {
		msg := &WebSocketMessage{
			Type:      MessageTypeTaskUpdate,
			Timestamp: time.Now(),
		}

		bytes, err := MarshalMessage(msg)
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "task-update")
	})

	t.Run("MarshalNilMessage", func(t *testing.T) {
		bytes, err := MarshalMessage(nil)
		require.NoError(t, err)
		assert.Equal(t, "null", string(bytes))
	})
}

func TestMessageTypes(t *testing.T) {
	t.Run("MessageTypeConstants", func(t *testing.T) {
		// Verify all message type constants are defined
		assert.Equal(t, MessageType("task-update"), MessageTypeTaskUpdate)
		assert.Equal(t, MessageType("log"), MessageTypeLog)
		assert.Equal(t, MessageType("thread_message"), MessageTypeThreadMessage)
		assert.Equal(t, MessageType("pong"), MessageTypePong)
		assert.Equal(t, MessageType("heartbeat"), MessageTypeHeartbeat)
		assert.Equal(t, MessageType("ping"), MessageTypePing)
		assert.Equal(t, MessageType("subscribe"), MessageTypeSubscribe)
		assert.Equal(t, MessageType("unsubscribe"), MessageTypeUnsubscribe)
	})
}

func TestMessageStructures(t *testing.T) {
	t.Run("PingMessage", func(t *testing.T) {
		ping := PingMessage{
			ID:        "test-id",
			Timestamp: time.Now(),
		}

		// Test JSON serialization
		bytes, err := json.Marshal(ping)
		require.NoError(t, err)

		var parsed PingMessage
		err = json.Unmarshal(bytes, &parsed)
		require.NoError(t, err)
		assert.Equal(t, ping.ID, parsed.ID)
	})

	t.Run("PongMessage", func(t *testing.T) {
		pong := PongMessage{
			ID:        "pong-id",
			Timestamp: time.Now(),
			PingID:    "ping-id",
		}

		// Test JSON serialization
		bytes, err := json.Marshal(pong)
		require.NoError(t, err)

		var parsed PongMessage
		err = json.Unmarshal(bytes, &parsed)
		require.NoError(t, err)
		assert.Equal(t, pong.ID, parsed.ID)
		assert.Equal(t, pong.PingID, parsed.PingID)
	})

	t.Run("SubscribeMessage", func(t *testing.T) {
		sub := SubscribeMessage{
			Types:   []MessageType{MessageTypeLog, MessageTypeTaskUpdate},
			TaskIDs: []string{"task1", "task2"},
		}

		// Test JSON serialization
		bytes, err := json.Marshal(sub)
		require.NoError(t, err)

		var parsed SubscribeMessage
		err = json.Unmarshal(bytes, &parsed)
		require.NoError(t, err)
		assert.Equal(t, sub.Types, parsed.Types)
		assert.Equal(t, sub.TaskIDs, parsed.TaskIDs)
	})

	t.Run("HeartbeatMessage", func(t *testing.T) {
		heartbeat := HeartbeatMessage{
			Timestamp: time.Now(),
			ServerID:  "test-server",
		}

		// Test JSON serialization
		bytes, err := json.Marshal(heartbeat)
		require.NoError(t, err)

		var parsed HeartbeatMessage
		err = json.Unmarshal(bytes, &parsed)
		require.NoError(t, err)
		assert.Equal(t, heartbeat.ServerID, parsed.ServerID)
	})
}
