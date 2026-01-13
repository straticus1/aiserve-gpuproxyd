package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mu        sync.RWMutex
}

func NewWebSocketHandler() *WebSocketHandler {
	h := &WebSocketHandler{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte, 256),
	}
	go h.handleBroadcasts()
	return h
}

func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			h.handleMessage(conn, message)
		}
	}
}

func (h *WebSocketHandler) handleMessage(conn *websocket.Conn, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		h.sendError(conn, "Invalid JSON")
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		h.sendError(conn, "Missing message type")
		return
	}

	switch msgType {
	case "ping":
		h.sendResponse(conn, map[string]interface{}{
			"type": "pong",
		})
	case "subscribe":
		h.sendResponse(conn, map[string]interface{}{
			"type":    "subscribed",
			"message": "Successfully subscribed to GPU stream",
		})
	default:
		h.sendError(conn, "Unknown message type")
	}
}

func (h *WebSocketHandler) handleBroadcasts() {
	for message := range h.broadcast {
		h.mu.RLock()
		for client := range h.clients {
			if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.RUnlock()
	}
}

func (h *WebSocketHandler) sendResponse(conn *websocket.Conn, data interface{}) {
	response, err := json.Marshal(data)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, response); err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

func (h *WebSocketHandler) sendError(conn *websocket.Conn, message string) {
	h.sendResponse(conn, map[string]interface{}{
		"type":  "error",
		"error": message,
	})
}

func (h *WebSocketHandler) Broadcast(data interface{}) {
	message, err := json.Marshal(data)
	if err != nil {
		log.Printf("Broadcast marshal error: %v", err)
		return
	}
	h.broadcast <- message
}
