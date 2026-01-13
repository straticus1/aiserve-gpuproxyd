package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/aiserve/gpuproxy/internal/models"
)

type BillingHandler struct {
	billingService *billing.Service
}

func NewBillingHandler(billingService *billing.Service) *BillingHandler {
	return &BillingHandler{billingService: billingService}
}

type CreatePaymentRequest struct {
	Amount            float64                  `json:"amount"`
	Currency          string                   `json:"currency"`
	Provider          billing.Provider         `json:"provider"`
	PaymentPreference models.PaymentPreference `json:"payment_preference"`
}

func (h *BillingHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	preferredPayment := r.Header.Get("X-Preferred-Payment")
	billingProvider := r.Header.Get("X-Billing")

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Amount <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid amount"})
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}

	if billingProvider != "" {
		req.Provider = billing.Provider(strings.ToLower(billingProvider))
	}

	if preferredPayment != "" {
		req.PaymentPreference = parsePaymentPreference(preferredPayment)
	}

	transaction, err := h.billingService.CreateTransaction(
		r.Context(),
		userID,
		req.Amount,
		req.Currency,
		req.Provider,
		req.PaymentPreference.Type,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.billingService.ProcessPayment(r.Context(), transaction, &req.PaymentPreference); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"transaction": transaction,
	})
}

func (h *BillingHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	transactions, err := h.billingService.GetTransactionsByUser(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	})
}

func parsePaymentPreference(header string) models.PaymentPreference {
	parts := strings.Split(header, ":")

	if len(parts) < 2 {
		return models.PaymentPreference{Type: "card"}
	}

	pref := models.PaymentPreference{
		Type: parts[0],
	}

	if pref.Type == "card" && len(parts) >= 4 {
		pref.CardNum = parts[1]
		pref.Expiry = parts[2]
		pref.CVV = parts[3]
	} else if pref.Type == "crypto" && len(parts) >= 3 {
		pref.Network = parts[1]
		pref.Wallet = parts[2]
	}

	return pref
}
