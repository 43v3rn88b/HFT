package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// We define an Upgrader to upgrade standard HTTP requests to WebSockets
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// In production, configure this to check the origin (e.g., your Next.js domain)
	// For local development, we allow all origins.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSManager tracks connected clients to broadcast data to them
type WSManager struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewWSManager() *WSManager {
	return &WSManager{
		clients: make(map[*websocket.Conn]bool),
	}
}

// HandleConnection is the HTTP handler that upgrades the request
func (ws *WSManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ Failed to upgrade to WebSocket: %v\n", err)
		return
	}

	// Register the new client
	ws.mu.Lock()
	ws.clients[conn] = true
	ws.mu.Unlock()

	log.Printf("🔌 New client connected! Total clients: %d", len(ws.clients))

	// Clean up when the client disconnects
	defer func() {
		ws.mu.Lock()
		delete(ws.clients, conn)
		ws.mu.Unlock()
		conn.Close()
		log.Println("🔌 Client disconnected.")
	}()

	// Keep the connection alive and listen for any messages from the client
	// (Even if we only push data, we must read to detect disconnects)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			ws.mu.Lock()
			delete(ws.clients, conn)
			ws.mu.Unlock()
			conn.Close()
			break
		}
	}
}

// StartBroadcasting runs in the background, pushing the latest order book to everyone
func (ws *WSManager) StartBroadcasting(bookManager *OrderBookManager) {
	ticker := time.NewTicker(100 * time.Millisecond) // Broadcast at 10Hz
	defer ticker.Stop()

	for {
		<-ticker.C // Wait for the next tick

		// Grab the absolute latest snapshot from memory
		snapshot, exists := bookManager.GetLatest("AAPL")
		if !exists {
			continue 
		}

		// 1. Lock ONLY to copy the active clients into a temporary slice
		ws.mu.Lock()
		activeClients := make([]*websocket.Conn, 0, len(ws.clients))
		for client := range ws.clients {
			activeClients = append(activeClients, client)
		}
		ws.mu.Unlock() // 🔓 Unlock immediately!

		// 2. Do the slow network writing outside of the lock
		for _, client := range activeClients {
			err := client.WriteJSON(snapshot)
			if err != nil {
				log.Printf("🔌 Closing dead connection: %v", err)
				
				// 3. If a client is dead, quickly lock again just to remove them
				ws.mu.Lock()
				client.Close()
				delete(ws.clients, client)
				ws.mu.Unlock()
			}
		}
	}
}