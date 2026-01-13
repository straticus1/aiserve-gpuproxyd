package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
)

type GPUPreferencesHandler struct {
	db *database.PostgresDB
}

func NewGPUPreferencesHandler(db *database.PostgresDB) *GPUPreferencesHandler {
	return &GPUPreferencesHandler{db: db}
}

// GetPreferences returns user's current GPU preferences
func (h *GPUPreferencesHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	var pref models.UserGPUPreference
	err := h.db.Pool.QueryRow(
		context.Background(),
		`SELECT id, user_id, preferences_json, is_active, created_at, updated_at
		 FROM user_gpu_preferences
		 WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY created_at DESC
		 LIMIT 1`,
		user.ID,
	).Scan(&pref.ID, &pref.UserID, &pref.PreferencesJSON, &pref.IsActive, &pref.CreatedAt, &pref.UpdatedAt)

	if err != nil {
		// Return default preferences if none exist
		defaultPrefs := gpu.DefaultPreferences()
		defaultPrefs.UserID = user.ID.String()
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"preferences": defaultPrefs,
			"is_default":  true,
		})
		return
	}

	// Parse JSON preferences
	prefs, err := gpu.FromJSON(pref.PreferencesJSON)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to parse preferences")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":          pref.ID,
		"preferences": prefs,
		"is_default":  false,
		"created_at":  pref.CreatedAt,
		"updated_at":  pref.UpdatedAt,
	})
}

// SetPreferences creates or updates user's GPU preferences
func (h *GPUPreferencesHandler) SetPreferences(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	var prefs gpu.UserGPUPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Set user ID
	prefs.UserID = user.ID.String()

	// Validate preferences
	if err := prefs.ValidatePreferences(); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid preferences: %v", err))
		return
	}

	// Convert to JSON
	prefsJSON, err := prefs.ToJSON()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to serialize preferences")
		return
	}

	// Deactivate existing preferences
	_, err = h.db.Pool.Exec(
		context.Background(),
		`UPDATE user_gpu_preferences SET is_active = FALSE WHERE user_id = $1`,
		user.ID,
	)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update preferences")
		return
	}

	// Insert new preferences
	var prefID uuid.UUID
	err = h.db.Pool.QueryRow(
		context.Background(),
		`INSERT INTO user_gpu_preferences (user_id, preferences_json, is_active)
		 VALUES ($1, $2, TRUE)
		 RETURNING id`,
		user.ID,
		prefsJSON,
	).Scan(&prefID)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save preferences")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":          prefID,
		"message":     "preferences saved successfully",
		"preferences": prefs,
	})
}

// GetAvailableGPUs returns list of all available GPU groups
func (h *GPUPreferencesHandler) GetAvailableGPUs(w http.ResponseWriter, r *http.Request) {
	vendor := r.URL.Query().Get("vendor")
	tier := r.URL.Query().Get("tier")

	var gpus []gpu.GPUGroup

	if vendor != "" && tier != "" {
		gpus = gpu.GetGPUsByVendorAndTier(gpu.GPUVendor(vendor), gpu.GPUTier(tier))
	} else if vendor != "" {
		gpus = gpu.GetGPUsByVendor(gpu.GPUVendor(vendor))
	} else if tier != "" {
		gpus = gpu.GetGPUsByTier(gpu.GPUTier(tier))
	} else {
		// Return all GPUs
		for _, g := range gpu.GPUGroups {
			gpus = append(gpus, g)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"gpus":  gpus,
		"count": len(gpus),
	})
}

// GetGPUGroups returns GPU groups organized by vendor
func (h *GPUPreferencesHandler) GetGPUGroups(w http.ResponseWriter, r *http.Request) {
	groups := map[string][]gpu.GPUGroup{
		"NVIDIA": gpu.GetGPUsByVendor(gpu.VendorNVIDIA),
		"AMD":    gpu.GetGPUsByVendor(gpu.VendorAMD),
		"Intel":  gpu.GetGPUsByVendor(gpu.VendorIntel),
		"Apple":  gpu.GetGPUsByVendor(gpu.VendorApple),
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"groups": groups,
	})
}

// TestSelection tests GPU selection with current preferences
func (h *GPUPreferencesHandler) TestSelection(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(middleware.UserContextKey).(*models.User)

	// Get user preferences
	var pref models.UserGPUPreference
	err := h.db.Pool.QueryRow(
		context.Background(),
		`SELECT preferences_json FROM user_gpu_preferences
		 WHERE user_id = $1 AND is_active = TRUE
		 ORDER BY created_at DESC LIMIT 1`,
		user.ID,
	).Scan(&pref.PreferencesJSON)

	var prefs *gpu.UserGPUPreferences
	if err != nil {
		// Use default preferences
		prefs = gpu.DefaultPreferences()
	} else {
		prefs, err = gpu.FromJSON(pref.PreferencesJSON)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to parse preferences")
			return
		}
	}

	// Get all available GPUs
	var availableGPUs []gpu.GPUGroup
	for _, g := range gpu.GPUGroups {
		availableGPUs = append(availableGPUs, g)
	}

	// Test selection
	result, err := prefs.SelectGPU(availableGPUs)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("selection failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetExamplePreferences returns example preference configurations
func (h *GPUPreferencesHandler) GetExamplePreferences(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"examples": gpu.ExamplePreferences,
		"available_examples": []string{
			"cost_optimized",
			"performance_focused",
			"nvidia_only",
			"amd_preferred",
		},
	})
}

// ClassifyGPU classifies a GPU by name
func (h *GPUPreferencesHandler) ClassifyGPU(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "name parameter required")
		return
	}

	group, found := gpu.ClassifyGPU(name)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"gpu":         group,
		"found_exact": found,
		"name":        name,
	})
}
