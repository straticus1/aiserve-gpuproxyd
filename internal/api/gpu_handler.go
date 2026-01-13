package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/loadbalancer"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/gorilla/mux"
)

type GPUHandler struct {
	gpuService      *gpu.Service
	protocolHandler *gpu.ProtocolHandler
	lbService       *loadbalancer.LoadBalancerService
}

func NewGPUHandler(gpuService *gpu.Service, protocolHandler *gpu.ProtocolHandler, lbService *loadbalancer.LoadBalancerService) *GPUHandler {
	return &GPUHandler{
		gpuService:      gpuService,
		protocolHandler: protocolHandler,
		lbService:       lbService,
	}
}

func (h *GPUHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	providerParam := r.URL.Query().Get("provider")
	if providerParam == "" {
		providerParam = "all"
	}

	provider := gpu.Provider(providerParam)

	instances, err := h.gpuService.ListInstances(r.Context(), provider)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	filters := make(map[string]interface{})
	if minVRAM := r.URL.Query().Get("min_vram"); minVRAM != "" {
		var vram int
		if _, err := json.Marshal(minVRAM); err == nil {
			filters["min_vram"] = vram
		}
	}
	if maxPrice := r.URL.Query().Get("max_price"); maxPrice != "" {
		var price float64
		if _, err := json.Marshal(maxPrice); err == nil {
			filters["max_price"] = price
		}
	}
	if gpuModel := r.URL.Query().Get("gpu_model"); gpuModel != "" {
		filters["gpu_model"] = gpuModel
	}
	if location := r.URL.Query().Get("location"); location != "" {
		filters["location"] = location
	}

	if len(filters) > 0 {
		instances = h.gpuService.FilterInstances(instances, filters)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
		"provider":  provider,
	})
}

func (h *GPUHandler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := gpu.Provider(vars["provider"])
	instanceID := vars["instanceId"]

	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		config = make(map[string]interface{})
	}

	contractID, err := h.gpuService.CreateInstance(r.Context(), provider, instanceID, config)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"contract_id": contractID,
		"provider":    provider,
		"instance_id": instanceID,
	})
}

func (h *GPUHandler) DestroyInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := gpu.Provider(vars["provider"])
	instanceID := vars["instanceId"]

	if err := h.gpuService.DestroyInstance(r.Context(), provider, instanceID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Instance destroyed successfully",
	})
}

func (h *GPUHandler) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	var proxyReq gpu.ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	resp, err := h.protocolHandler.ProxyRequest(r.Context(), &proxyReq)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *GPUHandler) BatchCreateInstances(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VastAICount int                    `json:"vastai_count"`
		IONetCount  int                    `json:"ionet_count"`
		Config      map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.VastAICount < 0 || req.VastAICount > 8 {
		req.VastAICount = 1
	}
	if req.IONetCount < 0 || req.IONetCount > 8 {
		req.IONetCount = 1
	}

	if req.VastAICount == 0 && req.IONetCount == 0 {
		req.VastAICount = 1
		req.IONetCount = 1
	}

	vastInstances := []string{}
	ionetInstances := []string{}
	errors := []string{}

	if req.VastAICount > 0 {
		instances, err := h.gpuService.ListInstances(r.Context(), gpu.ProviderVastAI)
		if err != nil {
			errors = append(errors, fmt.Sprintf("vast.ai list error: %v", err))
		} else {
			for i := 0; i < req.VastAICount && i < len(instances); i++ {
				instanceID := instances[i].ID[5:]
				contractID, err := h.gpuService.CreateInstance(r.Context(), gpu.ProviderVastAI, instanceID, req.Config)
				if err != nil {
					errors = append(errors, fmt.Sprintf("vast.ai create error: %v", err))
				} else {
					vastInstances = append(vastInstances, contractID)
				}
			}
		}
	}

	if req.IONetCount > 0 {
		instances, err := h.gpuService.ListInstances(r.Context(), gpu.ProviderIONet)
		if err != nil {
			errors = append(errors, fmt.Sprintf("io.net list error: %v", err))
		} else {
			for i := 0; i < req.IONetCount && i < len(instances); i++ {
				instanceID := instances[i].ID[6:]
				contractID, err := h.gpuService.CreateInstance(r.Context(), gpu.ProviderIONet, instanceID, req.Config)
				if err != nil {
					errors = append(errors, fmt.Sprintf("io.net create error: %v", err))
				} else {
					ionetInstances = append(ionetInstances, contractID)
				}
			}
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"vastai_instances": vastInstances,
		"ionet_instances":  ionetInstances,
		"total_created":    len(vastInstances) + len(ionetInstances),
		"errors":           errors,
	})
}

func (h *GPUHandler) ReserveInstances(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count   int                    `json:"count"`
		Filters map[string]interface{} `json:"filters"`
		Config  map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Count < 1 || req.Count > 16 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Count must be between 1 and 16"})
		return
	}

	instances, err := h.gpuService.ListInstances(r.Context(), gpu.ProviderAll)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if len(req.Filters) > 0 {
		instances = h.gpuService.FilterInstances(instances, req.Filters)
	}

	if len(instances) < req.Count {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Not enough instances available. Requested: %d, Available: %d", req.Count, len(instances)),
		})
		return
	}

	reserved := []map[string]interface{}{}
	errors := []string{}

	for i := 0; i < req.Count && i < len(instances); i++ {
		var selected *models.GPUInstance

		if h.lbService != nil {
			selected, err = h.lbService.SelectInstance(r.Context(), instances[i:])
			if err != nil {
				selected = &instances[i]
			}
		} else {
			selected = &instances[i]
		}

		provider := gpu.Provider(selected.Provider)
		instanceID := selected.ID
		if provider == gpu.ProviderVastAI && len(instanceID) > 5 {
			instanceID = instanceID[5:]
		} else if provider == gpu.ProviderIONet && len(instanceID) > 6 {
			instanceID = instanceID[6:]
		}

		contractID, err := h.gpuService.CreateInstance(r.Context(), provider, instanceID, req.Config)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", selected.ID, err))
		} else {
			if h.lbService != nil {
				h.lbService.TrackConnection(selected.ID)
			}

			reserved = append(reserved, map[string]interface{}{
				"instance_id": selected.ID,
				"contract_id": contractID,
				"provider":    provider,
				"gpu_model":   selected.GPUName,
				"vram":        selected.VRAM,
				"price":       selected.PricePerHour,
			})
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"reserved": reserved,
		"count":    len(reserved),
		"requested": req.Count,
		"errors":   errors,
	})
}

