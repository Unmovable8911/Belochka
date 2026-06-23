package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Envelope is the wire format for all WebSocket messages.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// client represents a single WebSocket connection.
type client struct {
	conn *websocket.Conn
	send chan []byte
}

const maxClients = 10

// Hub manages WebSocket client connections and broadcasts messages.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	snapshot json.RawMessage

	register   chan *client
	unregister chan *client
	broadcast  chan []byte
}

// New creates a new Hub.
func New() *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, 256),
	}
}

// SetSnapshot stores the current snapshot data sent to newly connecting clients.
func (h *Hub) SetSnapshot(data json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshot = data
}

// getSnapshot returns the current snapshot data.
func (h *Hub) getSnapshot() json.RawMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot
}

// Run starts the hub event loop. It blocks until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()

			// Send snapshot to newly connected client.
			snap := h.getSnapshot()
			if snap != nil {
				env, err := json.Marshal(Envelope{Type: "snapshot", Data: snap})
				if err != nil {
					slog.Error("marshal snapshot", "error", err)
				} else {
					select {
					case c.send <- env:
					default:
					}
				}
			}

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				close(c.send)
				delete(h.clients, c)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Client buffer full; drop message.
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastMsg marshals an Envelope with the given type and data, then sends it
// to all connected clients.
func (h *Hub) BroadcastMsg(msgType string, data json.RawMessage) {
	env, err := json.Marshal(Envelope{Type: msgType, Data: data})
	if err != nil {
		slog.Error("marshal broadcast", "error", err)
		return
	}
	h.broadcast <- env
}

// ClientCount returns the current number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS handles WebSocket upgrade requests.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "error", err)
		return
	}

	// Check connection limit. We upgrade first so we can send a proper
	// WebSocket close frame with code 1013 (Try Again Later).
	h.mu.RLock()
	count := len(h.clients)
	h.mu.RUnlock()

	if count >= maxClients {
		msg := websocket.FormatCloseMessage(1013, "too many connections")
		conn.WriteMessage(websocket.CloseMessage, msg)
		conn.Close()
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- c

	go h.writePump(c)
	go h.readPump(c)
}

// writePump pumps messages from the send channel to the WebSocket connection.
func (h *Hub) writePump(c *client) {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readPump reads messages from the WebSocket connection.
// We don't expect client messages, but we need to read to detect disconnects.
func (h *Hub) readPump(c *client) {
	defer func() {
		h.unregister <- c
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
	}
}
