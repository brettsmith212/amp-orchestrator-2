package hub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Client is a middleman between the websocket connection and the hub
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte
	
	// Client ID for tracking
	id string
	
	// Last heartbeat received/sent times
	lastHeartbeat time.Time
	lastPong      time.Time
	
	// Subscription preferences
	subscribedTypes map[MessageType]bool
	subscribedTasks map[string]bool
	
	// Mutex for thread-safe access to subscription state
	mu sync.RWMutex
	
	// Connection state
	connected bool
}

// readPump pumps messages from the websocket connection to the hub
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdateLastPong()
		return nil
	})

	for {
		_, rawMessage, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Process incoming message
		c.handleMessage(rawMessage)
	}
}

// writePump pumps messages from the hub to the websocket connection
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Client) handleMessage(rawMessage []byte) {
	msg, err := ParseMessage(rawMessage)
	if err != nil {
		log.Printf("Failed to parse message from client %s: %v", c.id, err)
		return
	}

	c.mu.Lock()
	c.lastHeartbeat = time.Now()
	c.mu.Unlock()

	switch msg.Type {
	case MessageTypePing:
		c.handlePing(msg)
	case MessageTypeSubscribe:
		c.handleSubscribe(msg)
	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(msg)
	default:
		log.Printf("Unknown message type from client %s: %s", c.id, msg.Type)
	}
}

// handlePing responds to ping messages with pong
func (c *Client) handlePing(msg *WebSocketMessage) {
	var pingData PingMessage
	if msg.Data != nil {
		if err := json.Unmarshal(msg.Data, &pingData); err != nil {
			log.Printf("Failed to parse ping data from client %s: %v", c.id, err)
			return
		}
	}

	// Create pong response
	pongData := PongMessage{
		ID:        pingData.ID,
		Timestamp: time.Now(),
		PingID:    pingData.ID,
	}

	pongMsg, err := CreateMessage(MessageTypePong, pongData)
	if err != nil {
		log.Printf("Failed to create pong message for client %s: %v", c.id, err)
		return
	}

	// Send pong response
	pongBytes, err := MarshalMessage(pongMsg)
	if err != nil {
		log.Printf("Failed to marshal pong message for client %s: %v", c.id, err)
		return
	}

	select {
	case c.send <- pongBytes:
	default:
		log.Printf("Failed to send pong to client %s: send channel full", c.id)
	}
}

// handleSubscribe processes subscription requests
func (c *Client) handleSubscribe(msg *WebSocketMessage) {
	var subData SubscribeMessage
	if err := json.Unmarshal(msg.Data, &subData); err != nil {
		log.Printf("Failed to parse subscribe data from client %s: %v", c.id, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Subscribe to message types
	for _, msgType := range subData.Types {
		c.subscribedTypes[msgType] = true
	}

	// Subscribe to specific task IDs
	for _, taskID := range subData.TaskIDs {
		c.subscribedTasks[taskID] = true
	}

	log.Printf("Client %s subscribed to types: %v, tasks: %v", c.id, subData.Types, subData.TaskIDs)
}

// handleUnsubscribe processes unsubscription requests
func (c *Client) handleUnsubscribe(msg *WebSocketMessage) {
	var subData SubscribeMessage
	if err := json.Unmarshal(msg.Data, &subData); err != nil {
		log.Printf("Failed to parse unsubscribe data from client %s: %v", c.id, err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Unsubscribe from message types
	for _, msgType := range subData.Types {
		delete(c.subscribedTypes, msgType)
	}

	// Unsubscribe from specific task IDs
	for _, taskID := range subData.TaskIDs {
		delete(c.subscribedTasks, taskID)
	}

	log.Printf("Client %s unsubscribed from types: %v, tasks: %v", c.id, subData.Types, subData.TaskIDs)
}

// ShouldReceiveMessage checks if client should receive a message based on subscriptions
func (c *Client) ShouldReceiveMessage(msgType MessageType, taskID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// If no subscriptions are set, receive all messages (default behavior)
	if len(c.subscribedTypes) == 0 && len(c.subscribedTasks) == 0 {
		return true
	}

	// Check message type subscription
	if c.subscribedTypes[msgType] {
		return true
	}

	// Check task ID subscription (if taskID is provided)
	if taskID != "" && c.subscribedTasks[taskID] {
		return true
	}

	return false
}

// IsConnected returns the connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SetConnected sets the connection status
func (c *Client) SetConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

// GetLastHeartbeat returns the last heartbeat time
func (c *Client) GetLastHeartbeat() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastHeartbeat
}

// UpdateLastPong updates the last pong received time
func (c *Client) UpdateLastPong() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPong = time.Now()
}
