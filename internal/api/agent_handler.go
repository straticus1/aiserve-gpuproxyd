package api

import (
	"io"
	"log"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/a2a"
	"github.com/aiserve/gpuproxy/internal/acp"
)

type AgentHandler struct {
	a2aServer *a2a.A2AServer
	acpServer *acp.ACPServer
}

func NewAgentHandler(a2aServer *a2a.A2AServer, acpServer *acp.ACPServer) *AgentHandler {
	return &AgentHandler{
		a2aServer: a2aServer,
		acpServer: acpServer,
	}
}

// HandleA2A processes Agent-to-Agent Protocol requests
func (h *AgentHandler) HandleA2A(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("A2A Request: %s", string(body))

	respBody, err := h.a2aServer.HandleRequest(r.Context(), body)
	if err != nil {
		log.Printf("A2A Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleACP processes Agent Communications Protocol messages
func (h *AgentHandler) HandleACP(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("ACP Message: %s", string(body))

	respBody, err := h.acpServer.HandleMessage(r.Context(), body)
	if err != nil {
		log.Printf("ACP Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleAgentDiscovery returns agent capabilities
func (h *AgentHandler) HandleAgentDiscovery(w http.ResponseWriter, r *http.Request) {
	info := h.a2aServer.GetAgentInfo()
	respondJSON(w, http.StatusOK, info)
}
