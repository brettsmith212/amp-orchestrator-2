package hub

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Heartbeat timeout - disconnect clients that haven't been active
	heartbeatTimeout = 120 * time.Second
	
	// Heartbeat check interval
	heartbeatInterval = 30 * time.Second
	
	// Server heartbeat send interval
	serverHeartbeatInterval = 45 * time.Second
)

// Hub maintains the set of active clients and broadcasts messages to clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// WebSocket upgrader
	upgrader websocket.Upgrader
	
	// Mutex for thread-safe access to clients
	mu sync.RWMutex
	
	// Ticker for heartbeat checks
	heartbeatTicker *time.Ticker
	
	// Ticker for server heartbeat messages
	serverHeartbeatTicker *time.Ticker
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	hub := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin for now
				return true
			},
		},
		heartbeatTicker:       time.NewTicker(heartbeatInterval),
		serverHeartbeatTicker: time.NewTicker(serverHeartbeatInterval),
	}
	return hub
}

// Run starts the hub and handles client registration, unregistration, and broadcasting
func (h *Hub) Run() {
	defer h.heartbeatTicker.Stop()
	defer h.serverHeartbeatTicker.Stop()
	
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			client.SetConnected(true)
			log.Printf("Client registered: %s", client.id)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				client.SetConnected(false)
				log.Printf("Client unregistered: %s", client.id)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if client.IsConnected() {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(h.clients, client)
						client.SetConnected(false)
					}
				}
			}
			h.mu.RUnlock()
			
		case <-h.heartbeatTicker.C:
			h.checkHeartbeats()
			
		case <-h.serverHeartbeatTicker.C:
			h.sendServerHeartbeat()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// checkHeartbeats disconnects clients that have timed out
func (h *Hub) checkHeartbeats() {
	now := time.Now()
	var timeoutClients []*Client

	h.mu.RLock()
	for client := range h.clients {
		if client.IsConnected() {
			lastHeartbeat := client.GetLastHeartbeat()
			if !lastHeartbeat.IsZero() && now.Sub(lastHeartbeat) > heartbeatTimeout {
				timeoutClients = append(timeoutClients, client)
			}
		}
	}
	h.mu.RUnlock()

	// Disconnect timed out clients
	for _, client := range timeoutClients {
		log.Printf("Client %s timed out, disconnecting", client.id)
		h.Unregister(client)
		client.conn.Close()
	}
}

// sendServerHeartbeat sends heartbeat messages to all connected clients
func (h *Hub) sendServerHeartbeat() {
	heartbeatData := HeartbeatMessage{
		Timestamp: time.Now(),
		ServerID:  "amp-orchestrator",
	}

	heartbeatMsg, err := CreateMessage(MessageTypeHeartbeat, heartbeatData)
	if err != nil {
		log.Printf("Failed to create heartbeat message: %v", err)
		return
	}

	heartbeatBytes, err := MarshalMessage(heartbeatMsg)
	if err != nil {
		log.Printf("Failed to marshal heartbeat message: %v", err)
		return
	}

	h.Broadcast(heartbeatBytes)
}

// ServeWS handles websocket requests from clients
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		hub:             h,
		conn:            conn,
		send:            make(chan []byte, 256),
		id:              uuid.New().String()[:8], // Short client ID
		lastHeartbeat:   time.Now(),
		lastPong:        time.Now(),
		subscribedTypes: make(map[MessageType]bool),
		subscribedTasks: make(map[string]bool),
		connected:       false,
	}

	client.hub.Register(client)

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines
	go client.writePump()
	go client.readPump()
}
