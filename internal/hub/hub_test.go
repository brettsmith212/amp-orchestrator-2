package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock clients with just the send channel (no WebSocket connection)
	client1 := &Client{
		hub:             hub,
		conn:            nil, // We don't need the connection for this test
		send:            make(chan []byte, 256),
		id:              "test-client-1",
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
		connected:       false,
	}

	client2 := &Client{
		hub:             hub,
		conn:            nil, // We don't need the connection for this test
		send:            make(chan []byte, 256),
		id:              "test-client-2",
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
		connected:       false,
	}

	// Register clients
	hub.Register(client1)
	hub.Register(client2)

	// Give some time for registration
	time.Sleep(10 * time.Millisecond)

	// Broadcast a message
	testMessage := []byte("test broadcast message")
	hub.Broadcast(testMessage)

	// Give some time for message delivery
	time.Sleep(10 * time.Millisecond)

	// Check that both clients received the message
	select {
	case msg := <-client1.send:
		if string(msg) != string(testMessage) {
			t.Errorf("Client1 received wrong message: got %s, want %s", string(msg), string(testMessage))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client1 did not receive broadcast message")
	}

	select {
	case msg := <-client2.send:
		if string(msg) != string(testMessage) {
			t.Errorf("Client2 received wrong message: got %s, want %s", string(msg), string(testMessage))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client2 did not receive broadcast message")
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{
		hub:             hub,
		conn:            nil, // We don't need the connection for this test
		send:            make(chan []byte, 256),
		id:              "test-client",
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
		connected:       false,
	}

	// Register client
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Check if client is registered (we can't directly access the map, so we test via broadcast)
	hub.Broadcast([]byte("test"))
	time.Sleep(10 * time.Millisecond)

	select {
	case <-client.send:
		// Good, client received the message
	case <-time.After(100 * time.Millisecond):
		t.Error("Registered client did not receive broadcast message")
	}

	// Unregister client
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	// Channel should be closed
	select {
	case _, ok := <-client.send:
		if ok {
			t.Error("Client send channel should be closed after unregistration")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client send channel was not closed after unregistration")
	}
}

func TestHubBasicBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Connect a client
	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the client time to register
	time.Sleep(50 * time.Millisecond)

	// Send a broadcast message
	testMessage := []byte("test message")
	hub.Broadcast(testMessage)

	// Read the message from the client
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, testMessage, message)
}

func TestHubMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect multiple clients
	var clients []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		clients = append(clients, conn)
	}

	// Defer closing all clients
	for _, client := range clients {
		defer client.Close()
	}

	// Give clients time to register
	time.Sleep(100 * time.Millisecond)

	// Send a broadcast message
	testMessage := []byte("multi-client test")
	hub.Broadcast(testMessage)

	// Verify all clients receive the message
	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(clientIndex int, c *websocket.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, message, err := c.ReadMessage()
			assert.NoError(t, err, "Client %d should receive message", clientIndex)
			assert.Equal(t, testMessage, message, "Client %d should receive correct message", clientIndex)
		}(i, client)
	}

	wg.Wait()
}

func TestHubPingPongHandling(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the client time to register
	time.Sleep(50 * time.Millisecond)

	// Send a ping message
	pingData := PingMessage{
		ID:        "test-ping-123",
		Timestamp: time.Now(),
	}

	pingMsg, err := CreateMessage(MessageTypePing, pingData)
	require.NoError(t, err)

	msgBytes, err := MarshalMessage(pingMsg)
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	require.NoError(t, err)

	// Read the pong response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, responseBytes, err := conn.ReadMessage()
	require.NoError(t, err)

	// Parse the response
	response, err := ParseMessage(responseBytes)
	require.NoError(t, err)
	assert.Equal(t, MessageTypePong, response.Type)

	// Verify pong data
	var pongData PongMessage
	err = json.Unmarshal(response.Data, &pongData)
	require.NoError(t, err)
	assert.Equal(t, pingData.ID, pongData.PingID)
	assert.Equal(t, pingData.ID, pongData.ID)
}

func TestHubSubscriptionHandling(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the client time to register
	time.Sleep(50 * time.Millisecond)

	// Send a subscription message
	subData := SubscribeMessage{
		Types:   []MessageType{MessageTypeLog},
		TaskIDs: []string{"task1", "task2"},
	}

	subMsg, err := CreateMessage(MessageTypeSubscribe, subData)
	require.NoError(t, err)

	msgBytes, err := MarshalMessage(subMsg)
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	require.NoError(t, err)

	// Wait a bit for subscription to be processed
	time.Sleep(100 * time.Millisecond)

	// The subscription is processed in the background
	// This test verifies the message is accepted without error
}

func TestHubInvalidMessage(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the client time to register
	time.Sleep(50 * time.Millisecond)

	// Send invalid JSON
	invalidJSON := []byte(`{"type": "ping", "invalid": }`)
	err = conn.WriteMessage(websocket.TextMessage, invalidJSON)
	require.NoError(t, err)

	// Wait a bit - the server should handle the error gracefully
	time.Sleep(100 * time.Millisecond)

	// Send a valid message to verify connection is still working
	pingData := PingMessage{
		ID:        "test-after-invalid",
		Timestamp: time.Now(),
	}

	pingMsg, err := CreateMessage(MessageTypePing, pingData)
	require.NoError(t, err)

	msgBytes, err := MarshalMessage(pingMsg)
	require.NoError(t, err)

	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	require.NoError(t, err)

	// Should still receive pong response
	conn.SetReadDeadline(time.Now().Add(time.Second))
	_, responseBytes, err := conn.ReadMessage()
	require.NoError(t, err)

	response, err := ParseMessage(responseBytes)
	require.NoError(t, err)
	assert.Equal(t, MessageTypePong, response.Type)
}

func TestClientSubscriptionLogic(t *testing.T) {
	client := &Client{
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
	}

	t.Run("NoSubscriptions", func(t *testing.T) {
		// Should receive all messages when no subscriptions are set
		assert.True(t, client.ShouldReceiveMessage(MessageTypeLog, "task1"))
		assert.True(t, client.ShouldReceiveMessage(MessageTypeTaskUpdate, "task2"))
	})

	t.Run("TypeSubscription", func(t *testing.T) {
		client.subscribedTypes[MessageTypeLog] = true

		// Should receive subscribed type
		assert.True(t, client.ShouldReceiveMessage(MessageTypeLog, "task1"))
		// Should not receive unsubscribed type
		assert.False(t, client.ShouldReceiveMessage(MessageTypeTaskUpdate, "task1"))
	})

	t.Run("TaskSubscription", func(t *testing.T) {
		client.subscribedTypes = make(map[MessageType]bool) // Clear type subscriptions
		client.subscribedTasks["task1"] = true

		// Should receive messages for subscribed task
		assert.True(t, client.ShouldReceiveMessage(MessageTypeLog, "task1"))
		// Should not receive messages for unsubscribed task
		assert.False(t, client.ShouldReceiveMessage(MessageTypeLog, "task2"))
	})

	t.Run("MixedSubscriptions", func(t *testing.T) {
		client.subscribedTypes[MessageTypeTaskUpdate] = true
		client.subscribedTasks["task2"] = true

		// Should receive subscribed type regardless of task
		assert.True(t, client.ShouldReceiveMessage(MessageTypeTaskUpdate, "task1"))
		assert.True(t, client.ShouldReceiveMessage(MessageTypeTaskUpdate, "task2"))

		// Should receive any message for subscribed task
		assert.True(t, client.ShouldReceiveMessage(MessageTypeLog, "task2"))
		
		// Should not receive unsubscribed type for unsubscribed task
		assert.False(t, client.ShouldReceiveMessage(MessageTypeLog, "task3"))
	})
}

func TestClientConnectionState(t *testing.T) {
	client := &Client{
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
	}

	// Test initial state
	assert.False(t, client.IsConnected())

	// Test setting connected
	client.SetConnected(true)
	assert.True(t, client.IsConnected())

	// Test setting disconnected
	client.SetConnected(false)
	assert.False(t, client.IsConnected())
}

func TestClientHeartbeatTracking(t *testing.T) {
	client := &Client{
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
	}

	// Initial state - zero time
	assert.True(t, client.GetLastHeartbeat().IsZero())

	// Update heartbeat
	now := time.Now()
	client.mu.Lock()
	client.lastHeartbeat = now
	client.mu.Unlock()

	assert.Equal(t, now.Unix(), client.GetLastHeartbeat().Unix())

	// Update pong
	client.UpdateLastPong()
	assert.False(t, client.lastPong.IsZero())
}
