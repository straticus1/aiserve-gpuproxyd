package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// SklearnRuntime handles scikit-learn model inference via Python bridge
// Uses gRPC or HTTP to communicate with Python inference server
type SklearnRuntime struct {
	mu sync.RWMutex

	models map[string]*SklearnModel  // model_id -> loaded model

	// Python bridge configuration
	bridgeURL    string  // URL of Python inference server
	bridgeClient *http.Client
	bridgeHealthy bool
}

// SklearnModel represents a scikit-learn model
type SklearnModel struct {
	ID             string
	Name           string
	FilePath       string
	Algorithm      string  // "RandomForest", "SVM", "LogisticRegression", etc.
	Framework      string  // "sklearn", "xgboost", "lightgbm"
	Loaded         bool
	InferenceCount int64
	AvgLatencyMs   float64

	// Python process info
	pythonPID  int
	pythonPort int
}

// NewSklearnRuntime creates a new scikit-learn runtime
func NewSklearnRuntime(bridgeURL string) *SklearnRuntime {
	if bridgeURL == "" {
		bridgeURL = "http://localhost:9000"  // Default Python bridge
	}

	return &SklearnRuntime{
		models:    make(map[string]*SklearnModel),
		bridgeURL: bridgeURL,
		bridgeClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// LoadModel loads a scikit-learn model via Python bridge
func (r *SklearnRuntime) LoadModel(ctx context.Context, modelID, filePath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if model already loaded
	if _, exists := r.models[modelID]; exists {
		return fmt.Errorf("model already loaded: %s", modelID)
	}

	// Read model file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read model file: %w", err)
	}

	// Detect framework (sklearn, xgboost, lightgbm)
	framework := detectPythonFramework(filePath)

	// Send load request to Python bridge
	loadReq := map[string]interface{}{
		"model_id":   modelID,
		"model_data": data,
		"framework":  framework,
	}

	reqBody, err := json.Marshal(loadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := r.bridgeClient.Post(
		fmt.Sprintf("%s/load", r.bridgeURL),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return fmt.Errorf("failed to load model via bridge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bridge returned error: %s", string(body))
	}

	// Parse response
	var loadResp struct {
		ModelID   string `json:"model_id"`
		Algorithm string `json:"algorithm"`
		Port      int    `json:"port"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&loadResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Create model instance
	model := &SklearnModel{
		ID:         modelID,
		FilePath:   filePath,
		Algorithm:  loadResp.Algorithm,
		Framework:  framework,
		Loaded:     true,
		pythonPort: loadResp.Port,
	}

	r.models[modelID] = model

	return nil
}

// Predict performs inference with a scikit-learn model
func (r *SklearnRuntime) Predict(ctx context.Context, modelID string, input map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	model, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not loaded: %s", modelID)
	}

	if !model.Loaded {
		return nil, fmt.Errorf("model not ready: %s", modelID)
	}

	start := time.Now()

	// Send predict request to Python bridge
	predictReq := map[string]interface{}{
		"model_id": modelID,
		"input":    input,
	}

	reqBody, err := json.Marshal(predictReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := r.bridgeClient.Post(
		fmt.Sprintf("%s/predict", r.bridgeURL),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to predict via bridge: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bridge returned error: %s", string(body))
	}

	// Parse response
	var prediction map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		return nil, fmt.Errorf("failed to parse prediction: %w", err)
	}

	// Update metrics
	latency := time.Since(start).Milliseconds()
	r.mu.Lock()
	model.InferenceCount++
	model.AvgLatencyMs = (model.AvgLatencyMs*float64(model.InferenceCount-1) + float64(latency)) / float64(model.InferenceCount)
	r.mu.Unlock()

	// Add metadata
	prediction["algorithm"] = model.Algorithm
	prediction["framework"] = model.Framework
	prediction["latency_ms"] = latency

	return prediction, nil
}

// UnloadModel removes a model from Python bridge
func (r *SklearnRuntime) UnloadModel(ctx context.Context, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("model not found: %s", modelID)
	}

	// Send unload request to Python bridge
	unloadReq := map[string]interface{}{
		"model_id": modelID,
	}

	reqBody, _ := json.Marshal(unloadReq)

	resp, err := r.bridgeClient.Post(
		fmt.Sprintf("%s/unload", r.bridgeURL),
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return fmt.Errorf("failed to unload model via bridge: %w", err)
	}
	defer resp.Body.Close()

	delete(r.models, modelID)
	return nil
}

// ListModels returns all loaded models
func (r *SklearnRuntime) ListModels() []*SklearnModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*SklearnModel, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}

	return models
}

// GetModelInfo returns information about a loaded model
func (r *SklearnRuntime) GetModelInfo(modelID string) (*SklearnModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// GetStats returns runtime statistics
func (r *SklearnRuntime) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalInferences := int64(0)
	avgLatency := 0.0
	for _, model := range r.models {
		totalInferences += model.InferenceCount
		avgLatency += model.AvgLatencyMs
	}

	if len(r.models) > 0 {
		avgLatency /= float64(len(r.models))
	}

	return map[string]interface{}{
		"runtime":          "sklearn-bridge",
		"loaded_models":    len(r.models),
		"total_inferences": totalInferences,
		"avg_latency_ms":   avgLatency,
		"bridge_url":       r.bridgeURL,
		"bridge_healthy":   r.bridgeHealthy,
		"language":         "python",
	}
}

// HealthCheck checks if Python bridge is healthy
func (r *SklearnRuntime) HealthCheck(ctx context.Context) error {
	resp, err := r.bridgeClient.Get(fmt.Sprintf("%s/health", r.bridgeURL))
	if err != nil {
		r.bridgeHealthy = false
		return fmt.Errorf("bridge unhealthy: %w", err)
	}
	defer resp.Body.Close()

	r.bridgeHealthy = (resp.StatusCode == http.StatusOK)

	if !r.bridgeHealthy {
		return fmt.Errorf("bridge returned status: %d", resp.StatusCode)
	}

	return nil
}

// detectPythonFramework detects the Python ML framework from file extension
func detectPythonFramework(filePath string) string {
	switch {
	case contains(filePath, "xgboost") || contains(filePath, ".xgb"):
		return "xgboost"
	case contains(filePath, "lightgbm") || contains(filePath, ".lgb"):
		return "lightgbm"
	case contains(filePath, ".pkl") || contains(filePath, ".pickle"):
		return "sklearn"
	case contains(filePath, ".joblib"):
		return "sklearn"
	default:
		return "sklearn"
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[len(s)-len(substr):] == substr)
}
