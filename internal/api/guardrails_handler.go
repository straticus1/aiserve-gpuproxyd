package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
)

type GuardRailsHandler struct {
	guardRails *middleware.GuardRails
}

func NewGuardRailsHandler(gr *middleware.GuardRails) *GuardRailsHandler {
	return &GuardRailsHandler{guardRails: gr}
}

func (h *GuardRailsHandler) GetSpendingInfo(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	ctx := context.Background()
	info, err := h.guardRails.GetSpendingInfo(ctx, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to get spending info",
		})
		return
	}

	respondJSON(w, http.StatusOK, info)
}

func (h *GuardRailsHandler) RecordSpending(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	if req.Amount <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Amount must be positive"})
		return
	}

	ctx := context.Background()
	if err := h.guardRails.RecordSpending(ctx, userID, req.Amount); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to record spending",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Spending recorded successfully"})
}

func (h *GuardRailsHandler) CheckSpending(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		EstimatedCost float64 `json:"estimated_cost"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	ctx := context.Background()
	info, err := h.guardRails.CheckSpending(ctx, userID, req.EstimatedCost)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to check spending",
		})
		return
	}

	if len(info.Violations) > 0 {
		respondJSON(w, http.StatusPaymentRequired, map[string]interface{}{
			"allowed":    false,
			"violations": info.Violations,
			"spent":      info.WindowSpent,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"allowed": true,
		"spent":   info.WindowSpent,
	})
}

func (h *GuardRailsHandler) ResetSpending(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req struct {
		WindowName string `json:"window_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	if req.WindowName == "" {
		req.WindowName = "all"
	}

	ctx := context.Background()
	if err := h.guardRails.ResetSpending(ctx, userID, req.WindowName); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to reset spending",
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Spending reset successfully",
	})
}
