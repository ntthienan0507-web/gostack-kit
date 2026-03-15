package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	maxMessageSize = 4096
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	writeWait      = 10 * time.Second
)

// ClientMessage pairs an incoming message with the client that sent it.
type ClientMessage struct {
	Client  *Client
	Message Message
}

// Client represents a single WebSocket connection.
type Client struct {
	ID       string
	conn     *websocket.Conn
	hub      *Hub
	send     chan []byte
	rooms    map[string]bool
	metadata map[string]any
	mu       sync.RWMutex
	logger   *zap.Logger
}

// newClient creates a new Client.
func newClient(id string, conn *websocket.Conn, hub *Hub, logger *zap.Logger) *Client {
	return &Client{
		ID:       id,
		conn:     conn,
		hub:      hub,
		send:     make(chan []byte, 256),
		rooms:    make(map[string]bool),
		metadata: make(map[string]any),
		logger:   logger.With(zap.String("client_id", id)),
	}
}

// Send marshals the message and queues it for delivery to the client.
func (c *Client) Send(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("failed to marshal message", zap.Error(err))
		return
	}
	select {
	case c.send <- data:
	default:
		c.logger.Warn("send buffer full, dropping message")
	}
}

// SetMeta stores a metadata value for the client.
func (c *Client) SetMeta(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMeta retrieves a metadata value for the client.
func (c *Client) GetMeta(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.metadata[key]
	return v, ok
}

// readPump reads messages from the WebSocket connection and dispatches them to the hub.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("unexpected close", zap.Error(err))
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn("invalid message format", zap.Error(err))
			c.Send(NewMessage(TypeError, map[string]string{"error": "invalid message format"}))
			continue
		}

		c.hub.incoming <- ClientMessage{
			Client:  c,
			Message: msg,
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
