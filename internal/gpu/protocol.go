package gpu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Protocol string

const (
	ProtocolHTTP              Protocol = "http"
	ProtocolHTTPS             Protocol = "https"
	ProtocolMCP               Protocol = "mcp"
	ProtocolOpenInference     Protocol = "openinference"
)

type ProxyRequest struct {
	Protocol       Protocol               `json:"protocol"`
	TargetURL      string                 `json:"target_url"`
	Method         string                 `json:"method"`
	Headers        map[string]string      `json:"headers"`
	Body           interface{}            `json:"body"`
	Timeout        time.Duration          `json:"timeout"`
	StreamResponse bool                   `json:"stream_response"`
}

type ProxyResponse struct {
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Duration   time.Duration          `json:"duration"`
	Error      string                 `json:"error,omitempty"`
}

type MCPRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type MCPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OpenInferenceRequest struct {
	Model       string                 `json:"model"`
	Prompt      string                 `json:"prompt,omitempty"`
	Messages    []interface{}          `json:"messages,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type OpenInferenceResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []interface{}          `json:"choices"`
	Usage   map[string]interface{} `json:"usage,omitempty"`
}

type ProtocolHandler struct {
	httpClient *http.Client
}

func NewProtocolHandler(timeout time.Duration) *ProtocolHandler {
	return &ProtocolHandler{
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (h *ProtocolHandler) ProxyRequest(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	start := time.Now()

	switch req.Protocol {
	case ProtocolHTTP, ProtocolHTTPS:
		return h.handleHTTPRequest(ctx, req)
	case ProtocolMCP:
		return h.handleMCPRequest(ctx, req)
	case ProtocolOpenInference:
		return h.handleOpenInferenceRequest(ctx, req)
	default:
		return &ProxyResponse{
			StatusCode: http.StatusBadRequest,
			Error:      fmt.Sprintf("unsupported protocol: %s", req.Protocol),
			Duration:   time.Since(start),
		}, nil
	}
}

func (h *ProtocolHandler) handleHTTPRequest(ctx context.Context, proxyReq *ProxyRequest) (*ProxyResponse, error) {
	start := time.Now()

	var bodyReader io.Reader
	if proxyReq.Body != nil {
		bodyBytes, err := json.Marshal(proxyReq.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, proxyReq.Method, proxyReq.TargetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range proxyReq.Headers {
		req.Header.Set(key, value)
	}

	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return &ProxyResponse{
			StatusCode: http.StatusBadGateway,
			Error:      err.Error(),
			Duration:   time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ProxyResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("failed to read response body: %v", err),
			Duration:   time.Since(start),
		}, nil
	}

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	var bodyJSON interface{}
	if err := json.Unmarshal(respBody, &bodyJSON); err != nil {
		bodyJSON = string(respBody)
	}

	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       bodyJSON,
		Duration:   time.Since(start),
	}, nil
}

func (h *ProtocolHandler) handleMCPRequest(ctx context.Context, proxyReq *ProxyRequest) (*ProxyResponse, error) {
	start := time.Now()

	mcpReq, ok := proxyReq.Body.(*MCPRequest)
	if !ok {
		return &ProxyResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid MCP request body",
			Duration:   time.Since(start),
		}, nil
	}

	bodyBytes, err := json.Marshal(mcpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", proxyReq.TargetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for key, value := range proxyReq.Headers {
		req.Header.Set(key, value)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return &ProxyResponse{
			StatusCode: http.StatusBadGateway,
			Error:      err.Error(),
			Duration:   time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return &ProxyResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("failed to decode MCP response: %v", err),
			Duration:   time.Since(start),
		}, nil
	}

	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Body:       mcpResp,
		Duration:   time.Since(start),
	}, nil
}

func (h *ProtocolHandler) handleOpenInferenceRequest(ctx context.Context, proxyReq *ProxyRequest) (*ProxyResponse, error) {
	start := time.Now()

	oiReq, ok := proxyReq.Body.(*OpenInferenceRequest)
	if !ok {
		return &ProxyResponse{
			StatusCode: http.StatusBadRequest,
			Error:      "invalid Open Inference request body",
			Duration:   time.Since(start),
		}, nil
	}

	bodyBytes, err := json.Marshal(oiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Open Inference request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", proxyReq.TargetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for key, value := range proxyReq.Headers {
		req.Header.Set(key, value)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return &ProxyResponse{
			StatusCode: http.StatusBadGateway,
			Error:      err.Error(),
			Duration:   time.Since(start),
		}, nil
	}
	defer resp.Body.Close()

	var oiResp OpenInferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&oiResp); err != nil {
		return &ProxyResponse{
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("failed to decode Open Inference response: %v", err),
			Duration:   time.Since(start),
		}, nil
	}

	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Body:       oiResp,
		Duration:   time.Since(start),
	}, nil
}
