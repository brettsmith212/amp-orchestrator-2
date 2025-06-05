package hub

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// Outbound message types (server -> client)
	MessageTypeTaskUpdate     MessageType = "task-update"
	MessageTypeLog            MessageType = "log"
	MessageTypeThreadMessage  MessageType = "thread_message"
	MessageTypePong           MessageType = "pong"
	MessageTypeHeartbeat      MessageType = "heartbeat"
	
	// Inbound message types (client -> server)
	MessageTypePing           MessageType = "ping"
	MessageTypeSubscribe      MessageType = "subscribe"
	MessageTypeUnsubscribe    MessageType = "unsubscribe"
)

// WebSocketMessage represents a structured WebSocket message
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
	ID        string          `json:"id,omitempty"`
}

// PingMessage represents a ping message from client
type PingMessage struct {
	ID        string    `json:"id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// PongMessage represents a pong response to ping
type PongMessage struct {
	ID        string    `json:"id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	PingID    string    `json:"ping_id,omitempty"`
}

// SubscribeMessage represents a subscription request
type SubscribeMessage struct {
	Types   []MessageType `json:"types"`
	TaskIDs []string      `json:"task_ids,omitempty"`
}

// HeartbeatMessage represents server heartbeat
type HeartbeatMessage struct {
	Timestamp time.Time `json:"timestamp"`
	ServerID  string    `json:"server_id,omitempty"`
}

// CreateMessage creates a WebSocket message with the given type and data
func CreateMessage(msgType MessageType, data interface{}) (*WebSocketMessage, error) {
	var rawData json.RawMessage
	var err error
	
	if data != nil {
		rawData, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}
	
	return &WebSocketMessage{
		Type:      msgType,
		Data:      rawData,
		Timestamp: time.Now(),
	}, nil
}

// ParseMessage parses a raw message into a WebSocketMessage
func ParseMessage(raw []byte) (*WebSocketMessage, error) {
	var msg WebSocketMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// MarshalMessage marshals a WebSocketMessage to JSON bytes
func MarshalMessage(msg *WebSocketMessage) ([]byte, error) {
	return json.Marshal(msg)
}
