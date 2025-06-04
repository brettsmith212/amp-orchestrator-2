package hub

import (
	"testing"
	"time"
)

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock clients with just the send channel (no WebSocket connection)
	client1 := &Client{
		hub:  hub,
		conn: nil, // We don't need the connection for this test
		send: make(chan []byte, 256),
	}

	client2 := &Client{
		hub:  hub,
		conn: nil, // We don't need the connection for this test
		send: make(chan []byte, 256),
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
		hub:  hub,
		conn: nil, // We don't need the connection for this test
		send: make(chan []byte, 256),
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

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	const numClients = 5
	clients := make([]*Client, numClients)

	// Create and register multiple clients
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:  hub,
			conn: nil,
			send: make(chan []byte, 256),
		}
		hub.Register(clients[i])
	}

	time.Sleep(10 * time.Millisecond)

	// Broadcast a message
	testMessage := []byte("broadcast to all")
	hub.Broadcast(testMessage)

	time.Sleep(10 * time.Millisecond)

	// Verify all clients received the message
	for i, client := range clients {
		select {
		case msg := <-client.send:
			if string(msg) != string(testMessage) {
				t.Errorf("Client %d received wrong message: got %s, want %s", i, string(msg), string(testMessage))
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Client %d did not receive broadcast message", i)
		}
	}
}
