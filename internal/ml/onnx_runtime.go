package ml

import (
	"context"
	"fmt"
	"sync"
	"time"

	onnxruntime "github.com/yalue/onnxruntime_go"
)

// ONNXRuntime handles ONNX model inference
// Uses ONNX Runtime (Microsoft's high-performance inference engine)
type ONNXRuntime struct {
	mu sync.RWMutex

	models map[string]*ONNXModel // model_id -> loaded model

	// GPU management
	gpuEnabled  bool
	gpuDeviceID int
}

// ONNXModel represents a loaded ONNX model
type ONNXModel struct {
	ID             string
	Name           string
	FilePath       string
	UseGPU         bool
	Loaded         bool
	InferenceCount int64
	AvgLatencyMs   float64

	// ONNX Runtime session
	session *onnxruntime.DynamicAdvancedSession

	// Model metadata
	InputNames  []string
	OutputNames []string
}

// NewONNXRuntime creates a new ONNX runtime
func NewONNXRuntime(gpuEnabled bool, gpuDeviceID int) *ONNXRuntime {
	return &ONNXRuntime{
		models:      make(map[string]*ONNXModel),
		gpuEnabled:  gpuEnabled,
		gpuDeviceID: gpuDeviceID,
	}
}

// InitializeLibrary initializes the ONNX Runtime library
// Must be called once at startup
func (r *ONNXRuntime) InitializeLibrary() error {
	// Initialize ONNX Runtime
	err := onnxruntime.InitializeEnvironment()
	if err != nil {
		return fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	return nil
}

// DestroyLibrary cleans up ONNX Runtime resources
// Should be called at shutdown
func (r *ONNXRuntime) DestroyLibrary() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Unload all models first
	for modelID := range r.models {
		if err := r.unloadModelUnsafe(modelID); err != nil {
			// Log error but continue cleanup
			fmt.Printf("Warning: failed to unload model %s: %v\n", modelID, err)
		}
	}

	// Destroy ONNX Runtime environment
	return onnxruntime.DestroyEnvironment()
}

// LoadModel loads an ONNX model from disk
func (r *ONNXRuntime) LoadModel(ctx context.Context, modelID, filePath string, useGPU bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if model already loaded
	if _, exists := r.models[modelID]; exists {
		return fmt.Errorf("model already loaded: %s", modelID)
	}

	// Validate GPU availability
	if useGPU && !r.gpuEnabled {
		return fmt.Errorf("GPU requested but not available")
	}

	// Get input/output names from ONNX model file
	inputs, outputs, err := onnxruntime.GetInputOutputInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to get model info: %w", err)
	}

	inputNames := make([]string, len(inputs))
	for i, input := range inputs {
		inputNames[i] = input.Name
	}

	outputNames := make([]string, len(outputs))
	for i, output := range outputs {
		outputNames[i] = output.Name
	}

	// Create session options
	options, err := onnxruntime.NewSessionOptions()
	if err != nil {
		return fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	// Configure GPU if requested
	if useGPU && r.gpuEnabled {
		cudaOptions, err := onnxruntime.NewCUDAProviderOptions()
		if err != nil {
			return fmt.Errorf("failed to create CUDA options: %w", err)
		}
		defer cudaOptions.Destroy()

		err = options.AppendExecutionProviderCUDA(cudaOptions)
		if err != nil {
			return fmt.Errorf("failed to enable CUDA provider: %w", err)
		}
	}

	// Set graph optimization level (99 = all optimizations)
	err = options.SetGraphOptimizationLevel(99)
	if err != nil {
		return fmt.Errorf("failed to set optimization level: %w", err)
	}

	// Create ONNX session
	session, err := onnxruntime.NewDynamicAdvancedSession(filePath, inputNames, outputNames, options)
	if err != nil {
		return fmt.Errorf("failed to create ONNX session: %w", err)
	}

	// Create model instance
	model := &ONNXModel{
		ID:          modelID,
		Name:        modelID,
		FilePath:    filePath,
		UseGPU:      useGPU && r.gpuEnabled,
		Loaded:      true,
		session:     session,
		InputNames:  inputNames,
		OutputNames: outputNames,
	}

	r.models[modelID] = model

	return nil
}

// Predict performs inference with an ONNX model
func (r *ONNXRuntime) Predict(ctx context.Context, modelID string, input map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	r.mu.RLock()
	model, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not loaded: %s", modelID)
	}

	if !model.Loaded {
		return nil, fmt.Errorf("model not ready: %s", modelID)
	}

	// Convert input map to ONNX values
	inputValues := make([]onnxruntime.Value, len(model.InputNames))
	for i, inputName := range model.InputNames {
		data, exists := input[inputName]
		if !exists {
			return nil, fmt.Errorf("missing input: %s", inputName)
		}

		// Convert to float32 slice
		var floatData []float32
		switch v := data.(type) {
		case []float32:
			floatData = v
		case []float64:
			floatData = make([]float32, len(v))
			for j, val := range v {
				floatData[j] = float32(val)
			}
		case []interface{}:
			floatData = make([]float32, len(v))
			for j, val := range v {
				switch fval := val.(type) {
				case float64:
					floatData[j] = float32(fval)
				case float32:
					floatData[j] = fval
				case int:
					floatData[j] = float32(fval)
				default:
					return nil, fmt.Errorf("unsupported data type in input array")
				}
			}
		default:
			return nil, fmt.Errorf("unsupported input type for %s", inputName)
		}

		// Create tensor
		// Assume 1D input for simplicity - production code would need actual shapes
		shape := onnxruntime.NewShape(int64(len(floatData)))
		tensor, err := onnxruntime.NewTensor(shape, floatData)
		if err != nil {
			return nil, fmt.Errorf("failed to create tensor for %s: %w", inputName, err)
		}
		defer tensor.Destroy()

		inputValues[i] = tensor
	}

	// Prepare output placeholders
	outputValues := make([]onnxruntime.Value, len(model.OutputNames))
	for i := range model.OutputNames {
		// Create empty tensors for outputs
		emptyTensor, err := onnxruntime.NewEmptyTensor[float32](onnxruntime.NewShape(1))
		if err != nil {
			return nil, fmt.Errorf("failed to create output tensor: %w", err)
		}
		defer emptyTensor.Destroy()
		outputValues[i] = emptyTensor
	}

	// Run inference
	err := model.session.Run(inputValues, outputValues)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Clean up output values
	defer func() {
		for _, val := range outputValues {
			if val != nil {
				val.Destroy()
			}
		}
	}()

	// Convert output values to map
	outputs := make(map[string]interface{})
	for i, outputName := range model.OutputNames {
		if i >= len(outputValues) {
			continue
		}

		// Try to get as float32 tensor
		if tensor, ok := outputValues[i].(*onnxruntime.Tensor[float32]); ok {
			outputs[outputName] = tensor.GetData()
		} else {
			outputs[outputName] = fmt.Sprintf("unsupported output type at index %d", i)
		}
	}

	// Calculate latency
	latencyMs := time.Since(start).Seconds() * 1000

	// Update statistics
	r.mu.Lock()
	model.InferenceCount++
	if model.InferenceCount == 1 {
		model.AvgLatencyMs = latencyMs
	} else {
		// Exponential moving average
		alpha := 0.1
		model.AvgLatencyMs = alpha*latencyMs + (1-alpha)*model.AvgLatencyMs
	}
	r.mu.Unlock()

	// Add metadata
	outputs["model_id"] = modelID
	outputs["latency_ms"] = latencyMs
	outputs["used_gpu"] = model.UseGPU

	return outputs, nil
}

// UnloadModel removes a model from memory
func (r *ONNXRuntime) UnloadModel(ctx context.Context, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.unloadModelUnsafe(modelID)
}

// unloadModelUnsafe removes a model without locking (internal use)
func (r *ONNXRuntime) unloadModelUnsafe(modelID string) error {
	model, exists := r.models[modelID]
	if !exists {
		return fmt.Errorf("model not found: %s", modelID)
	}

	// Destroy ONNX session
	if model.session != nil {
		if err := model.session.Destroy(); err != nil {
			return fmt.Errorf("failed to destroy session: %w", err)
		}
	}

	delete(r.models, modelID)
	return nil
}

// ListModels returns all loaded models
func (r *ONNXRuntime) ListModels() []*ONNXModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*ONNXModel, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}

	return models
}

// GetModelInfo returns information about a loaded model
func (r *ONNXRuntime) GetModelInfo(modelID string) (*ONNXModel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// GetStats returns runtime statistics
func (r *ONNXRuntime) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	totalInferences := int64(0)
	gpuModels := 0
	avgLatency := 0.0

	for _, model := range r.models {
		totalInferences += model.InferenceCount
		avgLatency += model.AvgLatencyMs
		if model.UseGPU {
			gpuModels++
		}
	}

	if len(r.models) > 0 {
		avgLatency /= float64(len(r.models))
	}

	return map[string]interface{}{
		"runtime":            "onnxruntime",
		"loaded_models":      len(r.models),
		"gpu_models":         gpuModels,
		"total_inferences":   totalInferences,
		"avg_latency_ms":     avgLatency,
		"gpu_enabled":        r.gpuEnabled,
		"gpu_device_id":      r.gpuDeviceID,
		"backend":            "onnxruntime",
		"supports_cpu":       true,
		"supports_gpu":       r.gpuEnabled,
	}
}

// HealthCheck verifies the runtime is working
func (r *ONNXRuntime) HealthCheck(ctx context.Context) error {
	// ONNX Runtime is healthy if it's initialized
	if !onnxruntime.IsInitialized() {
		return fmt.Errorf("ONNX Runtime not initialized")
	}
	return nil
}
