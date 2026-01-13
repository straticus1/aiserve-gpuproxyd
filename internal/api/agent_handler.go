package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/aiserve/gpuproxy/internal/a2a"
	"github.com/aiserve/gpuproxy/internal/acp"
	"github.com/aiserve/gpuproxy/internal/fipa"
	"github.com/aiserve/gpuproxy/internal/kqml"
	"github.com/aiserve/gpuproxy/internal/langchain"
)

type AgentHandler struct {
	a2aServer       *a2a.A2AServer
	acpServer       *acp.ACPServer
	fipaServer      *fipa.FIPAServer
	kqmlServer      *kqml.KQMLServer
	langchainServer *langchain.LangChainServer
}

func NewAgentHandler(a2aServer *a2a.A2AServer, acpServer *acp.ACPServer, fipaServer *fipa.FIPAServer, kqmlServer *kqml.KQMLServer, langchainServer *langchain.LangChainServer) *AgentHandler {
	return &AgentHandler{
		a2aServer:       a2aServer,
		acpServer:       acpServer,
		fipaServer:      fipaServer,
		kqmlServer:      kqmlServer,
		langchainServer: langchainServer,
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

// HandleFIPA processes FIPA ACL messages
func (h *AgentHandler) HandleFIPA(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("FIPA ACL Message: %s", string(body))

	respBody, err := h.fipaServer.HandleMessage(r.Context(), body)
	if err != nil {
		log.Printf("FIPA Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleKQML processes KQML messages
func (h *AgentHandler) HandleKQML(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("KQML Message: %s", string(body))

	respBody, err := h.kqmlServer.HandleMessage(r.Context(), body)
	if err != nil {
		log.Printf("KQML Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleLangChain processes LangChain agent protocol requests
func (h *AgentHandler) HandleLangChain(w http.ResponseWriter, r *http.Request) {
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

	log.Printf("LangChain Request: %s", string(body))

	respBody, err := h.langchainServer.HandleChain(r.Context(), body)
	if err != nil {
		log.Printf("LangChain Error: %v", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleLangChainTools returns available LangChain tools
func (h *AgentHandler) HandleLangChainTools(w http.ResponseWriter, r *http.Request) {
	tools := h.langchainServer.GetTools()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
	})
}

// HandleAgentDiscovery returns agent capabilities
func (h *AgentHandler) HandleAgentDiscovery(w http.ResponseWriter, r *http.Request) {
	info := h.a2aServer.GetAgentInfo()
	respondJSON(w, http.StatusOK, info)
}

// HandleUnifiedAgent auto-detects protocol and routes to appropriate handler
func (h *AgentHandler) HandleUnifiedAgent(w http.ResponseWriter, r *http.Request) {
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

	// Auto-detect protocol
	protocol := h.detectProtocol(body)
	log.Printf("Auto-detected protocol: %s", protocol)

	var respBody []byte

	switch protocol {
	case "MCP":
		// Route to MCP - would need to inject MCP handler here
		respondJSON(w, http.StatusOK, map[string]string{
			"detected": "MCP",
			"message":  "Use /api/v1/mcp endpoint",
		})
		return
	case "A2A":
		respBody, err = h.a2aServer.HandleRequest(r.Context(), body)
	case "ACP":
		respBody, err = h.acpServer.HandleMessage(r.Context(), body)
	case "FIPA":
		respBody, err = h.fipaServer.HandleMessage(r.Context(), body)
	case "KQML":
		respBody, err = h.kqmlServer.HandleMessage(r.Context(), body)
	case "LangChain":
		respBody, err = h.langchainServer.HandleChain(r.Context(), body)
	default:
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Unknown or unsupported protocol",
		})
		return
	}

	if err != nil {
		log.Printf("Protocol %s Error: %v", protocol, err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Internal server error",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// detectProtocol attempts to auto-detect the agent protocol from message structure
func (h *AgentHandler) detectProtocol(body []byte) string {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "Unknown"
	}

	// Check for MCP (JSON-RPC 2.0)
	if jsonrpc, ok := data["jsonrpc"].(string); ok && jsonrpc == "2.0" {
		return "MCP"
	}

	// Check for A2A
	if _, hasAction := data["action"]; hasAction {
		if _, hasFrom := data["from_agent"]; hasFrom {
			return "A2A"
		}
	}

	// Check for ACP
	if header, ok := data["header"].(map[string]interface{}); ok {
		if _, hasSender := header["sender"]; hasSender {
			if _, hasMessageType := header["message_type"]; hasMessageType {
				return "ACP"
			}
		}
	}

	// Check for FIPA ACL
	if performative, ok := data["performative"].(string); ok {
		if sender, ok := data["sender"].(map[string]interface{}); ok {
			if _, hasName := sender["name"]; hasName {
				// FIPA has specific performatives
				fipaPerformatives := []string{"inform", "request", "query-ref", "query-if", "propose", "cfp"}
				for _, fp := range fipaPerformatives {
					if strings.ToLower(performative) == fp {
						return "FIPA"
					}
				}
			}
		}
		// Could also be KQML
		return "KQML"
	}

	// Check for LangChain
	if input, ok := data["input"].(map[string]interface{}); ok {
		if _, hasTool := input["tool"]; hasTool {
			return "LangChain"
		}
		if _, hasAction := input["action"]; hasAction {
			return "LangChain"
		}
	}

	return "Unknown"
}
