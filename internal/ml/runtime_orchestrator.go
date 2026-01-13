package ml

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aiserve/gpuproxy/internal/models"
)

// RuntimeOrchestrator manages all ML runtimes and routes requests
// Intelligently selects the best runtime for each model format
type RuntimeOrchestrator struct {
	mu sync.RWMutex

	// ML Runtimes
	golearnRuntime *GoLearnRuntime
	gomlxRuntime   *GoMLXRuntime
	sklearnRuntime *SklearnRuntime
	onnxRuntime    *ONNXRuntime

	// Runtime routing map
	runtimeMap map[models.ModelFormat]string  // format -> runtime_type

	// Statistics
	totalInferences map[string]int64  // runtime_type -> count
	startTime       time.Time
}

// NewRuntimeOrchestrator creates a new runtime orchestrator
func NewRuntimeOrchestrator(gpuEnabled bool, pythonBridgeURL string) *RuntimeOrchestrator {
	onnxRuntime := NewONNXRuntime(gpuEnabled, 0)
	if err := onnxRuntime.InitializeLibrary(); err != nil {
		// Log error but continue - ONNX may not be available
		fmt.Printf("Warning: Failed to initialize ONNX Runtime: %v\n", err)
	}

	return &RuntimeOrchestrator{
		golearnRuntime: NewGoLearnRuntime(),
		gomlxRuntime:   NewGoMLXRuntime(gpuEnabled, 0),
		sklearnRuntime: NewSklearnRuntime(pythonBridgeURL),
		onnxRuntime:    onnxRuntime,
		runtimeMap:     buildRuntimeMap(),
		totalInferences: make(map[string]int64),
		startTime:      time.Now(),
	}
}

// buildRuntimeMap creates the format -> runtime mapping
func buildRuntimeMap() map[models.ModelFormat]string {
	return map[models.ModelFormat]string{
		// Native Go runtimes (fastest, no external dependencies)
		models.FormatGoLearn: "golearn",
		models.FormatGoMLX:   "gomlx",
		models.FormatGoNum:   "golearn",  // Can handle basic numerical models

		// Python ML runtimes (via bridge)
		models.FormatPickle:  "sklearn",
		models.FormatJobLib:  "sklearn",

		// Other runtimes (would be handled by existing inference service)
		models.FormatONNX:       "onnx",
		models.FormatPyTorch:    "pytorch",
		models.FormatTensorFlow: "tensorflow",
		models.FormatKeras:      "tensorflow",
		models.FormatTensorRT:   "triton",
		models.FormatPMML:       "pmml",
		models.FormatCoreML:     "coreml",
		models.FormatTFLite:     "tflite",
	}
}

// LoadModel loads a model into the appropriate runtime
func (o *RuntimeOrchestrator) LoadModel(ctx context.Context, modelID string, format models.ModelFormat, filePath string, useGPU bool) error {
	runtime, err := o.selectRuntime(format)
	if err != nil {
		return fmt.Errorf("failed to select runtime: %w", err)
	}

	switch runtime {
	case "golearn":
		return o.golearnRuntime.LoadModel(ctx, modelID, filePath)

	case "gomlx":
		return o.gomlxRuntime.LoadModel(ctx, modelID, filePath, useGPU)

	case "sklearn":
		return o.sklearnRuntime.LoadModel(ctx, modelID, filePath)

	case "onnx":
		return o.onnxRuntime.LoadModel(ctx, modelID, filePath, useGPU)

	default:
		return fmt.Errorf("runtime not implemented: %s", runtime)
	}
}

// Predict performs inference using the appropriate runtime
func (o *RuntimeOrchestrator) Predict(ctx context.Context, modelID string, format models.ModelFormat, input map[string]interface{}) (map[string]interface{}, error) {
	runtime, err := o.selectRuntime(format)
	if err != nil {
		return nil, fmt.Errorf("failed to select runtime: %w", err)
	}

	// Track inference
	o.mu.Lock()
	o.totalInferences[runtime]++
	o.mu.Unlock()

	var result map[string]interface{}

	switch runtime {
	case "golearn":
		result, err = o.golearnRuntime.Predict(ctx, modelID, input)

	case "gomlx":
		result, err = o.gomlxRuntime.Predict(ctx, modelID, input)

	case "sklearn":
		result, err = o.sklearnRuntime.Predict(ctx, modelID, input)

	case "onnx":
		result, err = o.onnxRuntime.Predict(ctx, modelID, input)

	default:
		return nil, fmt.Errorf("runtime not implemented: %s", runtime)
	}

	if err != nil {
		return nil, err
	}

	// Add runtime metadata
	result["runtime"] = runtime
	result["model_id"] = modelID

	return result, nil
}

// UnloadModel removes a model from its runtime
func (o *RuntimeOrchestrator) UnloadModel(ctx context.Context, modelID string, format models.ModelFormat) error {
	runtime, err := o.selectRuntime(format)
	if err != nil {
		return fmt.Errorf("failed to select runtime: %w", err)
	}

	switch runtime {
	case "golearn":
		return o.golearnRuntime.UnloadModel(ctx, modelID)

	case "gomlx":
		return o.gomlxRuntime.UnloadModel(ctx, modelID)

	case "sklearn":
		return o.sklearnRuntime.UnloadModel(ctx, modelID)

	case "onnx":
		return o.onnxRuntime.UnloadModel(ctx, modelID)

	default:
		return fmt.Errorf("runtime not implemented: %s", runtime)
	}
}

// selectRuntime chooses the appropriate runtime for a model format
func (o *RuntimeOrchestrator) selectRuntime(format models.ModelFormat) (string, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	runtime, exists := o.runtimeMap[format]
	if !exists {
		return "", fmt.Errorf("no runtime available for format: %s", format)
	}

	return runtime, nil
}

// GetRuntimeForFormat returns the runtime type for a given format
func (o *RuntimeOrchestrator) GetRuntimeForFormat(format models.ModelFormat) string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return o.runtimeMap[format]
}

// GetStats returns comprehensive statistics across all runtimes
func (o *RuntimeOrchestrator) GetStats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	uptime := time.Since(o.startTime)

	stats := map[string]interface{}{
		"uptime_seconds": uptime.Seconds(),
		"runtimes": map[string]interface{}{
			"golearn": o.golearnRuntime.GetStats(),
			"gomlx":   o.gomlxRuntime.GetStats(),
			"sklearn": o.sklearnRuntime.GetStats(),
			"onnx":    o.onnxRuntime.GetStats(),
		},
		"total_inferences_by_runtime": o.totalInferences,
	}

	// Calculate total inferences
	totalInferences := int64(0)
	for _, count := range o.totalInferences {
		totalInferences += count
	}
	stats["total_inferences"] = totalInferences

	// Calculate inferences per second
	if uptime.Seconds() > 0 {
		stats["inferences_per_second"] = float64(totalInferences) / uptime.Seconds()
	}

	return stats
}

// HealthCheck checks health of all runtimes
func (o *RuntimeOrchestrator) HealthCheck(ctx context.Context) map[string]bool {
	health := map[string]bool{
		"golearn": true,  // Native Go, always healthy
		"gomlx":   true,  // Native Go, always healthy
		"sklearn": o.sklearnRuntime.HealthCheck(ctx) == nil,
		"onnx":    o.onnxRuntime.HealthCheck(ctx) == nil,
	}

	return health
}

// ListAllModels returns all loaded models across all runtimes
func (o *RuntimeOrchestrator) ListAllModels() map[string]interface{} {
	return map[string]interface{}{
		"golearn": o.golearnRuntime.ListModels(),
		"gomlx":   o.gomlxRuntime.ListModels(),
		"sklearn": o.sklearnRuntime.ListModels(),
		"onnx":    o.onnxRuntime.ListModels(),
	}
}

// GetCapabilities returns what each runtime can do
func (o *RuntimeOrchestrator) GetCapabilities() map[string]interface{} {
	return map[string]interface{}{
		"runtimes": []map[string]interface{}{
			{
				"name":          "golearn",
				"language":      "go",
				"external_deps": false,
				"gpu_support":   false,
				"algorithms": []string{
					"k-NN", "Decision Trees", "Random Forests",
					"Naive Bayes", "Linear Regression", "Logistic Regression",
				},
				"formats": []string{"golearn", "gonum"},
				"ports":   "3000-5000",
				"latency": "50-100 microseconds",
			},
			{
				"name":          "gomlx",
				"language":      "go",
				"external_deps": false,
				"gpu_support":   true,
				"algorithms": []string{
					"Transformers", "CNNs", "RNNs", "MLPs",
					"Attention Models", "Custom Architectures",
				},
				"formats": []string{"gomlx"},
				"ports":   "5001-8000",
				"latency": "1-5 milliseconds",
			},
			{
				"name":          "sklearn",
				"language":      "python",
				"external_deps": true,
				"gpu_support":   false,
				"algorithms": []string{
					"All scikit-learn", "XGBoost", "LightGBM",
					"CatBoost", "Ensemble Methods",
				},
				"formats": []string{"pickle", "joblib"},
				"ports":   "8001-11000",
				"latency": "5-20 milliseconds",
			},
			{
				"name":          "onnx",
				"language":      "c++",
				"external_deps": true,
				"gpu_support":   true,
				"algorithms": []string{
					"All ONNX-compatible models",
					"PyTorch exported models",
					"TensorFlow exported models",
					"scikit-learn exported models",
				},
				"formats": []string{"onnx"},
				"ports":   "11001-14000",
				"latency": "1-10 milliseconds",
			},
		},
		"total_supported_formats": 13,
		"port_range":              "3000-15000",
		"max_concurrent_models":   12000,
	}
}
