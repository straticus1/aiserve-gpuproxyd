package api

import (
	"encoding/json"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/loadbalancer"
)

type LoadBalancerHandler struct {
	lbService *loadbalancer.LoadBalancerService
}

func NewLoadBalancerHandler(lbService *loadbalancer.LoadBalancerService) *LoadBalancerHandler {
	return &LoadBalancerHandler{lbService: lbService}
}

func (h *LoadBalancerHandler) GetLoads(w http.ResponseWriter, r *http.Request) {
	loads := h.lbService.GetAllLoads()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"strategy": h.lbService.GetStrategy(),
		"loads":    loads,
		"count":    len(loads),
	})
}

func (h *LoadBalancerHandler) GetInstanceLoad(w http.ResponseWriter, r *http.Request) {
	instanceID := r.URL.Query().Get("instance_id")
	if instanceID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "instance_id required"})
		return
	}

	load := h.lbService.GetInstanceLoad(instanceID)
	if load == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Instance not found"})
		return
	}

	respondJSON(w, http.StatusOK, load)
}

func (h *LoadBalancerHandler) SetStrategy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Strategy string `json:"strategy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	validStrategies := map[string]bool{
		"round_robin":           true,
		"equal_weighted":        true,
		"weighted_round_robin":  true,
		"least_connections":     true,
		"least_response_time":   true,
	}

	if !validStrategies[req.Strategy] {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid strategy"})
		return
	}

	h.lbService.SetStrategy(loadbalancer.Strategy(req.Strategy))

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"strategy": req.Strategy,
		"message":  "Load balancing strategy updated",
	})
}

func (h *LoadBalancerHandler) GetStrategy(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"strategy": h.lbService.GetStrategy(),
	})
}
