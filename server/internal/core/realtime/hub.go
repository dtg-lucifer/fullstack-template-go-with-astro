// Package realtime provides a WebSocket hub that manages connected clients
// and broadcasts domain events to all of them.
package realtime

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/utils"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages to them.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]struct{}
	upgrader websocket.Upgrader
	logger   *pkg.Logger
}

// NewHub creates a Hub and wires it to the domain event bus.
// origins is the list of allowed WebSocket origins from config.yaml.
func NewHub(bus *events.Bus, origins []string, readBuf, writeBuf int, logger *pkg.Logger) *Hub {
	h := &Hub{
		clients: make(map[*client]struct{}),
		logger:  logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  readBuf,
			WriteBufferSize: writeBuf,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				for _, o := range origins {
					if o == origin {
						return true
					}
				}
				return false
			},
		},
	}

	bus.OnUserRegistered(func(p events.UserRegisteredPayload) {
		h.Broadcast("auth:user-registered", p)
	})
	bus.OnJobEnqueued(func(p events.JobEnqueuedPayload) {
		h.Broadcast("queue:job-enqueued", p)
	})

	return h
}

// ServeHTTP upgrades the HTTP connection to WebSocket and registers the client.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("[WS] Upgrade failed", "error", err)
		return
	}

	c := &client{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.register(c)

	h.logger.Info("[WS] Client connected",
		"remote", conn.RemoteAddr().String(),
		"request_id", w.Header().Get("X-Request-ID"),
	)

	c.writeJSON("system:hello", utils.M{
		"message": "Connected to realtime server",
		"remote":  conn.RemoteAddr().String(),
	})

	go c.writePump()
	go c.readPump()
}

// Broadcast sends an event frame to every connected client.
func (h *Hub) Broadcast(event string, payload any) {
	frame, err := marshalFrame(event, payload)
	if err != nil {
		h.logger.Error("[WS] Failed to marshal broadcast frame", "event", event, "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- frame:
		default:
			h.logger.Warn("[WS] Dropping message for slow client", "remote", c.conn.RemoteAddr())
		}
	}
}

// ConnectedClients returns the number of currently connected WebSocket clients.
func (h *Hub) ConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
	h.logger.Info("[WS] Client disconnected", "remote", c.conn.RemoteAddr().String())
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// readPump drains incoming messages. It must run to detect disconnections
// and honour pings from the browser.
func (c *client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				c.hub.logger.Warn("[WS] Unexpected close", "error", err)
			}
			break
		}
	}
}

// writePump drains the send channel and writes frames to the connection.
func (c *client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			c.hub.logger.Warn("[WS] Write error", "error", err)
			return
		}
	}
}

func (c *client) writeJSON(event string, payload any) {
	frame, err := marshalFrame(event, payload)
	if err != nil {
		return
	}
	select {
	case c.send <- frame:
	default:
	}
}

type wsFrame struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
}

func marshalFrame(event string, payload any) ([]byte, error) {
	return json.Marshal(wsFrame{Event: event, Payload: payload})
}
