package api

import (
	"io"
	"log"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/mcp"
)

type MCPHandler struct {
	mcpServer *mcp.MCPServer
}

func NewMCPHandler(mcpServer *mcp.MCPServer) *MCPHandler {
	return &MCPHandler{mcpServer: mcpServer}
}

// HandleMCP processes MCP protocol requests
func (h *MCPHandler) HandleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Failed to read request body",
		})
		return
	}
	defer r.Body.Close()

	log.Printf("MCP Request: %s", string(body))

	respBody, err := h.mcpServer.HandleRequest(r.Context(), body)
	if err != nil {
		log.Printf("MCP Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleSSE handles Server-Sent Events for MCP streaming
func (h *MCPHandler) HandleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Streaming not supported",
		})
		return
	}

	// Send initial connection established message
	w.Write([]byte("event: connected\n"))
	w.Write([]byte("data: {\"status\":\"connected\"}\n\n"))
	flusher.Flush()

	// Keep connection alive
	<-r.Context().Done()
}

// HandleWebSocket handles WebSocket connections for MCP
func (h *MCPHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket support for bidirectional MCP communication
	respondJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "WebSocket support coming soon",
	})
}
