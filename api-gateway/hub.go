package main

import (
	"log"
	"sync"
	"github.com/gorilla/websocket"
)

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients. We use a map as a set. The boolean is just a placeholder.
	// The sync.RWMutex ensures thread-safe access when clients connect/disconnect.
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex

	// Inbound messages from Redpanda to broadcast to clients.
	Broadcast chan []byte
}

func NewHub() *Hub {
	return &Hub{
		Broadcast: make(chan []byte),
		clients:   make(map[*websocket.Conn]bool),
	}
}

// Run listens on the Broadcast channel and fans out to all clients.
// This runs as a continuous goroutine.
func (h *Hub) Run() {
	for {
		// Wait for a new order book snapshot from Redpanda
		message := <-h.Broadcast

		h.mu.RLock()
		for client := range h.clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Client disconnected: %v", err)
				client.Close()
				// We'll let a separate cleanup function remove them from the map
				// to keep this broadcast loop as fast as possible.
			}
		}
		h.mu.RUnlock()
	}
}

// AddClient safely registers a new websocket connection
func (h *Hub) AddClient(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[conn] = true
}

// RemoveClient safely unregisters a websocket connection
func (h *Hub) RemoveClient(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		conn.Close()
	}
}