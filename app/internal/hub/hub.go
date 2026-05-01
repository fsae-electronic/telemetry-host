package hub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"telemetry-host/internal/serial"
)

// client wraps a WebSocket connection.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub fans out packets to all connected WebSocket clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Register adds a new WebSocket connection and starts its write pump.
func (h *Hub) Register(conn *websocket.Conn) {
	c := &client{conn: conn, send: make(chan []byte, 64)}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump(func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	})

	// Keep connection alive
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// Broadcast sends a packet to all connected clients as JSON.
func (h *Hub) Broadcast(p serial.Packet) {
	type wsMsg struct {
		Type string        `json:"type"`
		serial.Packet
	}
	msg, err := json.Marshal(wsMsg{Type: "packet", Packet: p})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

// BroadcastStatus sends a status update to all clients.
func (h *Hub) BroadcastStatus(status any) {
	type wsMsg struct {
		Type string `json:"type"`
		Data any    `json:"data"`
	}
	msg, err := json.Marshal(wsMsg{Type: "status", Data: status})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (c *client) writePump(onClose func()) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		onClose()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("ws write error: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
