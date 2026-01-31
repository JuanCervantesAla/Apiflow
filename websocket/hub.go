package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

type ExecutionUpdate struct {
	Type        string      `json:"type"`
	ExecutionID string      `json:"executionId"`
	FlowID      string      `json:"flowId"`
	NodeID      string      `json:"nodeId,omitempty"`
	Status      string      `json:"status"`
	Message     string      `json:"message,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	Timestamp   string      `json:"timestamp"`
}

type Hub struct {
	clients map[string]map[*Client]bool

	broadcast chan ExecutionUpdate

	register   chan *Client
	unregister chan *Client

	mu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan ExecutionUpdate, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.userID] == nil {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			log.Printf("WebSocket: Cliente conectado (user: %s)", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("WebSocket: Cliente desconectado (user: %s)", client.userID)

		case update := <-h.broadcast:
			h.mu.RLock()
			for userID, clients := range h.clients {
				for client := range clients {
					select {
					case client.send <- update:
					default:
						close(client.send)
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.clients, userID)
						}
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastUpdate(update ExecutionUpdate) {
	h.broadcast <- update
}

func (h *Hub) BroadcastToUser(userID string, update ExecutionUpdate) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[userID]; ok {
		message, _ := json.Marshal(update)
		for client := range clients {
			select {
			case client.send <- update:
			default:
			}
		}
		log.Printf("WebSocket: Enviado a user %s: %s", userID, string(message))
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}
