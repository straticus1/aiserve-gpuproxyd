package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
)

type UserHandler struct {
	db *database.PostgresDB
}

func NewUserHandler(db *database.PostgresDB) *UserHandler {
	return &UserHandler{db: db}
}

type UserExport struct {
	User          UserInfo              `json:"user"`
	APIKeys       []APIKeyInfo          `json:"api_keys"`
	UsageQuota    *UsageQuotaInfo       `json:"usage_quota"`
	GPUUsage      []GPUUsageInfo        `json:"gpu_usage"`
	Transactions  []TransactionInfo     `json:"transactions"`
	Credits       []CreditInfo          `json:"credits"`
	ExportedAt    time.Time             `json:"exported_at"`
}

type UserInfo struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type APIKeyInfo struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
}

type UsageQuotaInfo struct {
	MaxGPUHours   float64   `json:"max_gpu_hours"`
	UsedGPUHours  float64   `json:"used_gpu_hours"`
	MaxRequests   int64     `json:"max_requests"`
	UsedRequests  int64     `json:"used_requests"`
	ResetAt       time.Time `json:"reset_at"`
	ResetInterval string    `json:"reset_interval"`
}

type GPUUsageInfo struct {
	Provider   string     `json:"provider"`
	InstanceID string     `json:"instance_id"`
	GPUModel   string     `json:"gpu_model"`
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Duration   float64    `json:"duration"`
	Cost       float64    `json:"cost"`
}

type TransactionInfo struct {
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	PaymentMethod   string    `json:"payment_method"`
	PaymentProvider string    `json:"payment_provider"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreditInfo struct {
	Date              string  `json:"date"`
	Time              string  `json:"time"`
	Duration          int     `json:"duration"`
	CreditsRemaining  float64 `json:"credits_remaining"`
	CreditsTotal      float64 `json:"credits_total"`
	CreditsOverage    float64 `json:"credits_overage"`
	CreditsCap        float64 `json:"credits_cap"`
	CreditsAutoRenew  bool    `json:"credits_auto_renew"`
}

func (h *UserHandler) ExportAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	ctx := r.Context()

	export := &UserExport{
		ExportedAt: time.Now(),
	}

	userQuery := `SELECT id, email, name, is_admin, is_active, created_at, updated_at FROM users WHERE id = $1`
	var user UserInfo
	if err := h.db.Pool.QueryRow(ctx, userQuery, userID).Scan(
		&user.ID, &user.Email, &user.Name, &user.IsAdmin, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch user"})
		return
	}
	export.User = user

	apiKeysQuery := `SELECT id, name, last_used_at, expires_at, is_active, created_at FROM api_keys WHERE user_id = $1`
	rows, err := h.db.Pool.Query(ctx, apiKeysQuery, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key APIKeyInfo
			rows.Scan(&key.ID, &key.Name, &key.LastUsedAt, &key.ExpiresAt, &key.IsActive, &key.CreatedAt)
			export.APIKeys = append(export.APIKeys, key)
		}
	}

	quotaQuery := `SELECT max_gpu_hours, used_gpu_hours, max_requests, used_requests, reset_at, reset_interval FROM usage_quotas WHERE user_id = $1`
	var quota UsageQuotaInfo
	if err := h.db.Pool.QueryRow(ctx, quotaQuery, userID).Scan(
		&quota.MaxGPUHours, &quota.UsedGPUHours, &quota.MaxRequests, &quota.UsedRequests, &quota.ResetAt, &quota.ResetInterval,
	); err == nil {
		export.UsageQuota = &quota
	}

	usageQuery := `SELECT provider, instance_id, gpu_model, start_time, end_time, duration, cost FROM gpu_usage WHERE user_id = $1 ORDER BY start_time DESC LIMIT 100`
	rows, err = h.db.Pool.Query(ctx, usageQuery, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var usage GPUUsageInfo
			rows.Scan(&usage.Provider, &usage.InstanceID, &usage.GPUModel, &usage.StartTime, &usage.EndTime, &usage.Duration, &usage.Cost)
			export.GPUUsage = append(export.GPUUsage, usage)
		}
	}

	txQuery := `SELECT amount, currency, status, payment_method, payment_provider, created_at FROM billing_transactions WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err = h.db.Pool.Query(ctx, txQuery, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var tx TransactionInfo
			rows.Scan(&tx.Amount, &tx.Currency, &tx.Status, &tx.PaymentMethod, &tx.PaymentProvider, &tx.CreatedAt)
			export.Transactions = append(export.Transactions, tx)
		}
	}

	creditsQuery := `SELECT date, time, duration, credits_remaining, credits_total, credits_overage, credits_cap, credits_auto_renew FROM credits WHERE client_id = $1 ORDER BY created_at DESC LIMIT 100`
	rows, err = h.db.Pool.Query(ctx, creditsQuery, userID.String())
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var credit CreditInfo
			var autoRenew int
			rows.Scan(&credit.Date, &credit.Time, &credit.Duration, &credit.CreditsRemaining, &credit.CreditsTotal, &credit.CreditsOverage, &credit.CreditsCap, &autoRenew)
			credit.CreditsAutoRenew = autoRenew == 1
			export.Credits = append(export.Credits, credit)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=account-export.json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(export)
}
