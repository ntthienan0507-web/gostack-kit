package ws

import (
	"encoding/json"
	"sync"

	"go.uber.org/zap"
)

// MessageHandler is a function that handles a specific message type from a client.
type MessageHandler func(client *Client, msg Message)

// Hub maintains the set of active clients, rooms, and message handlers.
type Hub struct {
	clients    map[string]*Client
	rooms      map[string]map[*Client]bool
	handlers   map[string]MessageHandler
	register   chan *Client
	unregister chan *Client
	incoming   chan ClientMessage
	mu         sync.RWMutex
	logger     *zap.Logger
}

// NewHub creates a new Hub and starts its event loop.
func NewHub(logger *zap.Logger) *Hub {
	h := &Hub{
		clients:    make(map[string]*Client),
		rooms:      make(map[string]map[*Client]bool),
		handlers:   make(map[string]MessageHandler),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan ClientMessage, 256),
		logger:     logger.Named("ws.hub"),
	}
	go h.run()
	return h
}

// Handle registers a handler for a specific message type.
func (h *Hub) Handle(msgType string, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[msgType] = handler
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal broadcast message", zap.Error(err))
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
			h.logger.Warn("broadcast: send buffer full", zap.String("client_id", client.ID))
		}
	}
}

// BroadcastToRoom sends a message to all clients in a specific room.
func (h *Hub) BroadcastToRoom(room string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal room broadcast message", zap.Error(err))
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if members, ok := h.rooms[room]; ok {
		for client := range members {
			select {
			case client.send <- data:
			default:
				h.logger.Warn("room broadcast: send buffer full", zap.String("client_id", client.ID))
			}
		}
	}
}

// BroadcastToRoomExcept sends a message to all clients in a room except the specified client.
func (h *Hub) BroadcastToRoomExcept(room string, msg Message, excludeID string) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal room broadcast message", zap.Error(err))
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if members, ok := h.rooms[room]; ok {
		for client := range members {
			if client.ID == excludeID {
				continue
			}
			select {
			case client.send <- data:
			default:
				h.logger.Warn("room broadcast: send buffer full", zap.String("client_id", client.ID))
			}
		}
	}
}

// SendTo sends a message to a specific client by ID.
func (h *Hub) SendTo(clientID string, msg Message) {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()
	if ok {
		client.Send(msg)
	}
}

// JoinRoom adds a client to a room.
func (h *Hub) JoinRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
	client.rooms[room] = true
}

// LeaveRoom removes a client from a room.
func (h *Hub) LeaveRoom(client *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if members, ok := h.rooms[room]; ok {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
	delete(client.rooms, room)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RoomCount returns the number of active rooms.
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// run is the main event loop for the hub.
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			h.logger.Info("client registered", zap.String("client_id", client.ID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.send)
				// Remove client from all rooms.
				for room := range client.rooms {
					if members, ok := h.rooms[room]; ok {
						delete(members, client)
						if len(members) == 0 {
							delete(h.rooms, room)
						}
					}
				}
			}
			h.mu.Unlock()
			h.logger.Info("client unregistered", zap.String("client_id", client.ID))

		case cm := <-h.incoming:
			h.mu.RLock()
			handler, ok := h.handlers[cm.Message.Type]
			h.mu.RUnlock()
			if ok {
				go func(handler MessageHandler, client *Client, msg Message) {
					defer func() {
						if r := recover(); r != nil {
							h.logger.Error("panic in message handler",
								zap.String("type", msg.Type),
								zap.Any("recover", r),
							)
						}
					}()
					handler(client, msg)
				}(handler, cm.Client, cm.Message)
			} else {
				h.logger.Warn("no handler for message type", zap.String("type", cm.Message.Type))
			}
		}
	}
}
