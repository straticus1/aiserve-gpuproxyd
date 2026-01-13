package models

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelFormat represents supported model formats
type ModelFormat string

const (
	// Python/External ML Frameworks
	FormatPickle     ModelFormat = "pickle"      // Scikit-learn, PyTorch state_dict
	FormatONNX       ModelFormat = "onnx"        // ONNX Runtime
	FormatTensorFlow ModelFormat = "tensorflow"  // TensorFlow SavedModel
	FormatPMML       ModelFormat = "pmml"        // Predictive Model Markup Language
	FormatKeras      ModelFormat = "keras"       // Keras H5
	FormatJobLib     ModelFormat = "joblib"      // Scikit-learn JobLib
	FormatPyTorch    ModelFormat = "pytorch"     // PyTorch .pt/.pth
	FormatTensorRT   ModelFormat = "tensorrt"    // NVIDIA TensorRT
	FormatCoreML     ModelFormat = "coreml"      // Apple Core ML
	FormatTFLite     ModelFormat = "tflite"      // TensorFlow Lite

	// Native Go ML Frameworks
	FormatGoLearn    ModelFormat = "golearn"     // GoLearn (Classical ML in Go)
	FormatGoMLX      ModelFormat = "gomlx"       // GoMLX (Deep Learning in Go)
	FormatGoNum      ModelFormat = "gonum"       // Gonum (Numerical computing)
)

// ServedModel represents a user-uploaded model being served
type ServedModel struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Format       ModelFormat            `json:"format"`
	FilePath     string                 `json:"file_path"`
	Version      string                 `json:"version"`
	Framework    string                 `json:"framework"`     // pytorch, tensorflow, sklearn, etc.
	Runtime      string                 `json:"runtime"`       // triton, torchserve, tfserving, onnxruntime
	GPURequired  bool                   `json:"gpu_required"`
	GPUType      string                 `json:"gpu_type"`      // Preferred GPU (H100, A100, etc.)
	MinVRAM      int                    `json:"min_vram"`      // Minimum VRAM in GB
	Metadata     map[string]interface{} `json:"metadata"`
	Endpoint     string                 `json:"endpoint"`      // /serve/models/{id}/predict
	Status       string                 `json:"status"`        // loading, ready, error
	Replicas     int                    `json:"replicas"`      // Number of instances
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	UserID       string                 `json:"user_id"`

	// Performance metrics
	TotalRequests   int64   `json:"total_requests"`
	AverageLatency  float64 `json:"average_latency_ms"`
	ErrorRate       float64 `json:"error_rate"`
}

// ModelRegistry manages served models
type ModelRegistry struct {
	mu          sync.RWMutex
	models      map[string]*ServedModel  // model_id -> model
	userModels  map[string][]string      // user_id -> []model_id
	endpoints   map[string]string        // endpoint -> model_id
	storageRoot string                   // Root directory for model storage
}

var globalRegistry *ModelRegistry
var registryOnce sync.Once

// GetModelRegistry returns the global model registry
func GetModelRegistry() *ModelRegistry {
	registryOnce.Do(func() {
		globalRegistry = &ModelRegistry{
			models:      make(map[string]*ServedModel),
			userModels:  make(map[string][]string),
			endpoints:   make(map[string]string),
			storageRoot: os.Getenv("MODEL_STORAGE_PATH"),
		}
		if globalRegistry.storageRoot == "" {
			globalRegistry.storageRoot = "/app/models"
		}
	})
	return globalRegistry
}

// SetStorageRoot sets the storage root directory for the model registry
func (r *ModelRegistry) SetStorageRoot(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.storageRoot = path
}

// GetStorageRoot returns the storage root directory
func (r *ModelRegistry) GetStorageRoot() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storageRoot
}

// RegisterModel registers a new model for serving
func (r *ModelRegistry) RegisterModel(model *ServedModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate model format
	if !isValidFormat(model.Format) {
		return fmt.Errorf("unsupported model format: %s", model.Format)
	}

	// Check if model file exists
	if _, err := os.Stat(model.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", model.FilePath)
	}

	// Generate endpoint
	model.Endpoint = fmt.Sprintf("/serve/models/%s/predict", model.ID)
	model.Status = "loading"
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()

	// Store model
	r.models[model.ID] = model
	r.endpoints[model.Endpoint] = model.ID

	// Track user's models
	r.userModels[model.UserID] = append(r.userModels[model.UserID], model.ID)

	// Start model loading in background
	go r.loadModel(model.ID)

	return nil
}

// GetModel retrieves a model by ID
func (r *ModelRegistry) GetModel(modelID string) (*ServedModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// ListUserModels returns all models for a user
func (r *ModelRegistry) ListUserModels(userID string) []*ServedModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modelIDs := r.userModels[userID]
	models := make([]*ServedModel, 0, len(modelIDs))

	for _, id := range modelIDs {
		if model, exists := r.models[id]; exists {
			models = append(models, model)
		}
	}

	return models
}

// DeleteModel removes a model from the registry
func (r *ModelRegistry) DeleteModel(modelID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("model not found: %s", modelID)
	}

	// Verify ownership
	if model.UserID != userID {
		return fmt.Errorf("unauthorized: model belongs to different user")
	}

	// Remove from maps
	delete(r.models, modelID)
	delete(r.endpoints, model.Endpoint)

	// Remove from user's model list
	userModelIDs := r.userModels[userID]
	for i, id := range userModelIDs {
		if id == modelID {
			r.userModels[userID] = append(userModelIDs[:i], userModelIDs[i+1:]...)
			break
		}
	}

	// Clean up model files (optional - may want to keep for backup)
	// os.Remove(model.FilePath)

	return nil
}

// UpdateModelStatus updates the status of a model
func (r *ModelRegistry) UpdateModelStatus(modelID, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if model, exists := r.models[modelID]; exists {
		model.Status = status
		model.UpdatedAt = time.Now()
	}
}

// RecordInference records metrics for a model inference
func (r *ModelRegistry) RecordInference(modelID string, latencyMs float64, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return
	}

	model.TotalRequests++

	// Update average latency (exponential moving average)
	alpha := 0.1
	if model.TotalRequests == 1 {
		model.AverageLatency = latencyMs
	} else {
		model.AverageLatency = alpha*latencyMs + (1-alpha)*model.AverageLatency
	}

	// Update error rate
	if !success {
		totalErrors := model.ErrorRate * float64(model.TotalRequests-1)
		model.ErrorRate = (totalErrors + 1) / float64(model.TotalRequests)
	} else {
		totalErrors := model.ErrorRate * float64(model.TotalRequests-1)
		model.ErrorRate = totalErrors / float64(model.TotalRequests)
	}

	model.UpdatedAt = time.Now()
}

// loadModel loads a model into the appropriate runtime
func (r *ModelRegistry) loadModel(modelID string) {
	model, err := r.GetModel(modelID)
	if err != nil {
		return
	}

	// Determine runtime based on format
	runtime := determineRuntime(model.Format)
	model.Runtime = runtime

	// TODO: Actual model loading logic would go here
	// This would integrate with:
	// - NVIDIA Triton Inference Server
	// - TorchServe
	// - TensorFlow Serving
	// - ONNX Runtime
	// - Custom Python runtime

	// Simulate loading delay
	time.Sleep(2 * time.Second)

	// Mark as ready
	r.UpdateModelStatus(modelID, "ready")
}

// isValidFormat checks if a model format is supported
func isValidFormat(format ModelFormat) bool {
	validFormats := []ModelFormat{
		// Python/External Formats (10)
		FormatPickle,
		FormatONNX,
		FormatTensorFlow,
		FormatPMML,
		FormatKeras,
		FormatJobLib,
		FormatPyTorch,
		FormatTensorRT,
		FormatCoreML,
		FormatTFLite,

		// Native Go Formats (3)
		FormatGoLearn,
		FormatGoMLX,
		FormatGoNum,
	}

	for _, f := range validFormats {
		if f == format {
			return true
		}
	}
	return false
}

// determineRuntime selects the best runtime for a model format
func determineRuntime(format ModelFormat) string {
	runtimeMap := map[ModelFormat]string{
		// Python/External Runtimes
		FormatONNX:       "onnxruntime",
		FormatTensorFlow: "tfserving",
		FormatPyTorch:    "torchserve",
		FormatTensorRT:   "triton",
		FormatKeras:      "tfserving",
		FormatPickle:     "sklearn-server",
		FormatJobLib:     "sklearn-server",
		FormatPMML:       "pmml-server",
		FormatCoreML:     "coreml-server",
		FormatTFLite:     "tflite-runtime",

		// Native Go Runtimes (fastest, no external dependencies)
		FormatGoLearn:    "golearn-native",
		FormatGoMLX:      "gomlx-gpu",
		FormatGoNum:      "gonum-native",
	}

	if runtime, exists := runtimeMap[format]; exists {
		return runtime
	}

	return "generic"
}

// DetectModelFormat attempts to detect the model format from file extension
func DetectModelFormat(filename string) ModelFormat {
	ext := strings.ToLower(filepath.Ext(filename))

	formatMap := map[string]ModelFormat{
		// Python/External Formats
		".pkl":        FormatPickle,
		".pickle":     FormatPickle,
		".onnx":       FormatONNX,
		".pb":         FormatTensorFlow,
		".pmml":       FormatPMML,
		".h5":         FormatKeras,
		".hdf5":       FormatKeras,
		".joblib":     FormatJobLib,
		".pt":         FormatPyTorch,
		".pth":        FormatPyTorch,
		".plan":       FormatTensorRT,
		".trt":        FormatTensorRT,
		".mlmodel":    FormatCoreML,
		".tflite":     FormatTFLite,

		// Native Go Formats
		".golearn":    FormatGoLearn,
		".gomlx":      FormatGoMLX,
		".gonum":      FormatGoNum,
	}

	if format, exists := formatMap[ext]; exists {
		return format
	}

	// Check for TensorFlow SavedModel directory structure
	if ext == "" {
		if _, err := os.Stat(filepath.Join(filename, "saved_model.pb")); err == nil {
			return FormatTensorFlow
		}
	}

	return ""
}

// ModelServeRequest represents an inference request
type ModelServeRequest struct {
	Inputs     map[string]interface{} `json:"inputs"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// ModelServeResponse represents an inference response
type ModelServeResponse struct {
	ModelID     string                 `json:"model_id"`
	Outputs     map[string]interface{} `json:"outputs"`
	LatencyMs   float64                `json:"latency_ms"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// InferenceService handles model inference
type InferenceService struct {
	registry *ModelRegistry
}

// NewInferenceService creates a new inference service
func NewInferenceService() *InferenceService {
	return &InferenceService{
		registry: GetModelRegistry(),
	}
}

// Predict performs inference on a model
func (s *InferenceService) Predict(ctx context.Context, modelID string, request *ModelServeRequest) (*ModelServeResponse, error) {
	start := time.Now()

	model, err := s.registry.GetModel(modelID)
	if err != nil {
		return nil, err
	}

	if model.Status != "ready" {
		return nil, fmt.Errorf("model not ready: status=%s", model.Status)
	}

	// TODO: Route to appropriate runtime and perform actual inference
	// This would integrate with Triton, TorchServe, etc.

	// Placeholder response
	response := &ModelServeResponse{
		ModelID:   modelID,
		Outputs:   map[string]interface{}{},
		LatencyMs: time.Since(start).Seconds() * 1000,
		Metadata: map[string]interface{}{
			"format":  model.Format,
			"runtime": model.Runtime,
			"version": model.Version,
		},
	}

	// Record metrics
	s.registry.RecordInference(modelID, response.LatencyMs, true)

	return response, nil
}
