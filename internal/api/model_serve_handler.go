package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// ModelServeHandler handles model serving endpoints
type ModelServeHandler struct {
	registry         *models.ModelRegistry
	inferenceService *models.InferenceService
	uploadMaxSize    int64 // Maximum model upload size in bytes
}

func NewModelServeHandler() *ModelServeHandler {
	return &ModelServeHandler{
		registry:         models.GetModelRegistry(),
		inferenceService: models.NewInferenceService(),
		uploadMaxSize:    10 * 1024 * 1024 * 1024, // 10GB default
	}
}

// UploadModel handles model file upload
func (h *ModelServeHandler) UploadModel(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (limit to uploadMaxSize)
	if err := r.ParseMultipartForm(h.uploadMaxSize); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Failed to parse form: %v", err),
		})
		return
	}

	// Get file from form
	file, header, err := r.FormFile("model")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "No model file provided",
		})
		return
	}
	defer file.Close()

	// Get metadata from form
	modelName := r.FormValue("name")
	if modelName == "" {
		modelName = header.Filename
	}

	formatStr := r.FormValue("format")
	var format models.ModelFormat
	if formatStr != "" {
		format = models.ModelFormat(formatStr)
	} else {
		format = models.DetectModelFormat(header.Filename)
	}

	framework := r.FormValue("framework")
	version := r.FormValue("version")
	if version == "" {
		version = "1.0.0"
	}

	gpuRequired := r.FormValue("gpu_required") == "true"
	gpuType := r.FormValue("gpu_type")

	// Get user ID from context
	userID := middleware.GetUserID(r.Context())

	// Generate model ID
	modelID := uuid.New().String()

	// Create storage directory
	modelDir := filepath.Join(h.registry.GetStorageRoot(), userID.String(), modelID)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to create storage directory: %v", err),
		})
		return
	}

	// Save file
	modelPath := filepath.Join(modelDir, header.Filename)
	dst, err := os.Create(modelPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to save model: %v", err),
		})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Failed to write model: %v", err),
		})
		return
	}

	// Register model
	model := &models.ServedModel{
		ID:          modelID,
		Name:        modelName,
		Format:      format,
		FilePath:    modelPath,
		Version:     version,
		Framework:   framework,
		GPURequired: gpuRequired,
		GPUType:     gpuType,
		UserID:      userID.String(),
		Metadata: map[string]interface{}{
			"filename": header.Filename,
			"size":     header.Size,
		},
	}

	if err := h.registry.RegisterModel(model); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Failed to register model: %v", err),
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"model_id": modelID,
		"name":     modelName,
		"format":   format,
		"endpoint": model.Endpoint,
		"status":   model.Status,
		"message":  "Model uploaded successfully and loading",
	})
}

// ListModels lists all models for the authenticated user
func (h *ModelServeHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	models := h.registry.ListUserModels(userID.String())

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"models": models,
		"count":  len(models),
	})
}

// GetModel retrieves model details
func (h *ModelServeHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["model_id"]

	model, err := h.registry.GetModel(modelID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Model not found",
		})
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(r.Context())
	if model.UserID != userID.String() {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "Access denied",
		})
		return
	}

	respondJSON(w, http.StatusOK, model)
}

// DeleteModel removes a model
func (h *ModelServeHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["model_id"]

	userID := middleware.GetUserID(r.Context())

	if err := h.registry.DeleteModel(modelID, userID.String()); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Model deleted successfully",
	})
}

// PredictModel performs inference on a model
func (h *ModelServeHandler) PredictModel(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["model_id"]

	// Verify model ownership
	model, err := h.registry.GetModel(modelID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Model not found",
		})
		return
	}

	userID := middleware.GetUserID(r.Context())
	if model.UserID != userID.String() {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "Access denied",
		})
		return
	}

	// Parse request
	var request models.ModelServeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Perform inference
	start := time.Now()
	response, err := h.inferenceService.Predict(r.Context(), modelID, &request)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("Inference failed: %v", err),
		})
		return
	}

	// Add timing
	response.LatencyMs = time.Since(start).Seconds() * 1000

	respondJSON(w, http.StatusOK, response)
}

// GetModelMetrics returns metrics for a model
func (h *ModelServeHandler) GetModelMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	modelID := vars["model_id"]

	model, err := h.registry.GetModel(modelID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{
			"error": "Model not found",
		})
		return
	}

	userID := middleware.GetUserID(r.Context())
	if model.UserID != userID.String() {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": "Access denied",
		})
		return
	}

	metrics := map[string]interface{}{
		"model_id":         model.ID,
		"name":             model.Name,
		"total_requests":   model.TotalRequests,
		"average_latency":  model.AverageLatency,
		"error_rate":       model.ErrorRate,
		"status":           model.Status,
		"replicas":         model.Replicas,
		"created_at":       model.CreatedAt,
		"updated_at":       model.UpdatedAt,
	}

	respondJSON(w, http.StatusOK, metrics)
}

// SupportedFormats returns list of supported model formats
func (h *ModelServeHandler) SupportedFormats(w http.ResponseWriter, r *http.Request) {
	formats := []map[string]interface{}{
		{
			"format":      "pickle",
			"extensions":  []string{".pkl", ".pickle"},
			"framework":   "scikit-learn, PyTorch",
			"description": "Python pickle format for scikit-learn models or PyTorch state_dict",
		},
		{
			"format":      "onnx",
			"extensions":  []string{".onnx"},
			"framework":   "ONNX",
			"description": "Open Neural Network Exchange format (cross-framework)",
		},
		{
			"format":      "tensorflow",
			"extensions":  []string{".pb", "saved_model.pb"},
			"framework":   "TensorFlow",
			"description": "TensorFlow SavedModel format",
		},
		{
			"format":      "pmml",
			"extensions":  []string{".pmml"},
			"framework":   "Multiple",
			"description": "Predictive Model Markup Language (XML-based)",
		},
		{
			"format":      "keras",
			"extensions":  []string{".h5", ".hdf5"},
			"framework":   "Keras/TensorFlow",
			"description": "Keras HDF5 model format",
		},
		{
			"format":      "joblib",
			"extensions":  []string{".joblib"},
			"framework":   "scikit-learn",
			"description": "JobLib serialization for scikit-learn models",
		},
		{
			"format":      "pytorch",
			"extensions":  []string{".pt", ".pth"},
			"framework":   "PyTorch",
			"description": "PyTorch model checkpoint",
		},
		{
			"format":      "tensorrt",
			"extensions":  []string{".plan", ".trt"},
			"framework":   "NVIDIA TensorRT",
			"description": "Optimized models for NVIDIA GPUs",
		},
		{
			"format":      "coreml",
			"extensions":  []string{".mlmodel"},
			"framework":   "Apple Core ML",
			"description": "Apple Core ML model format",
		},
		{
			"format":      "tflite",
			"extensions":  []string{".tflite"},
			"framework":   "TensorFlow Lite",
			"description": "TensorFlow Lite for mobile/edge devices",
		},
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"formats": formats,
		"count":   len(formats),
	})
}
