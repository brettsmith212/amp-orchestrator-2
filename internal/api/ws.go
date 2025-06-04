package api

import (
	"net/http"

	"github.com/brettsmith212/amp-orchestrator-2/internal/hub"
)

// WSHandler handles WebSocket connections
type WSHandler struct {
	hub *hub.Hub
}

// NewWSHandler creates a new WebSocket handler
func NewWSHandler(h *hub.Hub) *WSHandler {
	return &WSHandler{
		hub: h,
	}
}

// ServeWS handles WebSocket upgrade requests
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeWS(w, r)
}
