package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"fmt"

	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IPAccessHandler handles IP access control API endpoints
type IPAccessHandler struct {
	db *pgxpool.Pool
}

// NewIPAccessHandler creates a new IP access handler
func NewIPAccessHandler(db *pgxpool.Pool) *IPAccessHandler {
	return &IPAccessHandler{db: db}
}

// getUserIDAndEmail extracts user ID and email from request context
func (h *IPAccessHandler) getUserIDAndEmail(r *http.Request) (string, string, error) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return "", "", fmt.Errorf("unauthorized")
	}
	return user.ID.String(), user.Email, nil
}

// GetConfig returns the IP access configuration for the authenticated user
func (h *IPAccessHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := user.ID.String()

	query := `
		SELECT id, user_id, mode, allowlist_enabled, denylist_enabled,
		       block_on_no_match, audit_log_enabled, created_at, updated_at
		FROM ip_access_config
		WHERE user_id = $1
	`

	var config models.IPAccessConfig
	err := h.db.QueryRow(r.Context(), query, userID).Scan(
		&config.ID, &config.UserID, &config.Mode, &config.AllowlistEnabled,
		&config.DenylistEnabled, &config.BlockOnNoMatch, &config.AuditLogEnabled,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			// Return default config
			json.NewEncoder(w).Encode(map[string]interface{}{
				"mode":               "disabled",
				"allowlist_enabled":  false,
				"denylist_enabled":   true,
				"block_on_no_match":  false,
				"audit_log_enabled":  true,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// UpdateConfig updates the IP access configuration for the authenticated user
func (h *IPAccessHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := user.ID.String()

	var req struct {
		Mode             models.IPAccessMode `json:"mode"`
		AllowlistEnabled bool                `json:"allowlist_enabled"`
		DenylistEnabled  bool                `json:"denylist_enabled"`
		BlockOnNoMatch   bool                `json:"block_on_no_match"`
		AuditLogEnabled  bool                `json:"audit_log_enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate mode
	validModes := map[models.IPAccessMode]bool{
		models.IPAccessModeDisabled:  true,
		models.IPAccessModeAllowlist: true,
		models.IPAccessModeDenylist:  true,
		models.IPAccessModeStrict:    true,
	}
	if !validModes[req.Mode] {
		http.Error(w, "Invalid mode", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO ip_access_config (id, user_id, mode, allowlist_enabled, denylist_enabled,
		                              block_on_no_match, audit_log_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE SET
			mode = EXCLUDED.mode,
			allowlist_enabled = EXCLUDED.allowlist_enabled,
			denylist_enabled = EXCLUDED.denylist_enabled,
			block_on_no_match = EXCLUDED.block_on_no_match,
			audit_log_enabled = EXCLUDED.audit_log_enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, mode, allowlist_enabled, denylist_enabled,
		          block_on_no_match, audit_log_enabled, created_at, updated_at
	`

	var config models.IPAccessConfig
	err := h.db.QueryRow(r.Context(), query,
		uuid.New().String(), userID, req.Mode, req.AllowlistEnabled, req.DenylistEnabled,
		req.BlockOnNoMatch, req.AuditLogEnabled, time.Now(), time.Now(),
	).Scan(
		&config.ID, &config.UserID, &config.Mode, &config.AllowlistEnabled,
		&config.DenylistEnabled, &config.BlockOnNoMatch, &config.AuditLogEnabled,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// ListAllowlist returns all allowlist entries for the authenticated user
func (h *IPAccessHandler) ListAllowlist(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	query := `
		SELECT id, user_id, ip_address, ip_range, description, is_active,
		       created_at, updated_at, created_by
		FROM ip_allowlist
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(r.Context(), query, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []models.IPAllowlistEntry
	for rows.Next() {
		var entry models.IPAllowlistEntry
		err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.IPAddress, &entry.IPRange,
			&entry.Description, &entry.IsActive, &entry.CreatedAt,
			&entry.UpdatedAt, &entry.CreatedBy,
		)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// AddAllowlist adds an IP to the allowlist
func (h *IPAccessHandler) AddAllowlist(w http.ResponseWriter, r *http.Request) {
	userID, email, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	var req struct {
		IPAddress   string  `json:"ip_address"`
		IPRange     *string `json:"ip_range,omitempty"`
		Description string  `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.IPAddress == "" {
		http.Error(w, "ip_address is required", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO ip_allowlist (id, user_id, ip_address, ip_range, description, is_active,
		                          created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8)
		ON CONFLICT (user_id, ip_address) DO UPDATE SET
			ip_range = EXCLUDED.ip_range,
			description = EXCLUDED.description,
			is_active = TRUE,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, ip_address, ip_range, description, is_active,
		          created_at, updated_at, created_by
	`

	createdBy := "API:" + email
	var entry models.IPAllowlistEntry
	err := h.db.QueryRow(r.Context(), query,
		uuid.New().String(), userID, req.IPAddress, req.IPRange, req.Description,
		time.Now(), time.Now(), createdBy,
	).Scan(
		&entry.ID, &entry.UserID, &entry.IPAddress, &entry.IPRange,
		&entry.Description, &entry.IsActive, &entry.CreatedAt,
		&entry.UpdatedAt, &entry.CreatedBy,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// RemoveAllowlist removes an IP from the allowlist
func (h *IPAccessHandler) RemoveAllowlist(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }
	vars := mux.Vars(r)
	entryID := vars["id"]

	query := `DELETE FROM ip_allowlist WHERE id = $1 AND user_id = $2`
	result, err := h.db.Exec(r.Context(), query, entryID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListDenylist returns all denylist entries for the authenticated user
func (h *IPAccessHandler) ListDenylist(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	query := `
		SELECT id, user_id, ip_address, ip_range, reason, is_active,
		       expires_at, created_at, updated_at, created_by
		FROM ip_denylist
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(r.Context(), query, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []models.IPDenylistEntry
	for rows.Next() {
		var entry models.IPDenylistEntry
		err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.IPAddress, &entry.IPRange,
			&entry.Reason, &entry.IsActive, &entry.ExpiresAt,
			&entry.CreatedAt, &entry.UpdatedAt, &entry.CreatedBy,
		)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// AddDenylist adds an IP to the denylist
func (h *IPAccessHandler) AddDenylist(w http.ResponseWriter, r *http.Request) {
	userID, email, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	var req struct {
		IPAddress string     `json:"ip_address"`
		IPRange   *string    `json:"ip_range,omitempty"`
		Reason    string     `json:"reason,omitempty"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.IPAddress == "" {
		http.Error(w, "ip_address is required", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO ip_denylist (id, user_id, ip_address, ip_range, reason, is_active,
		                         expires_at, created_at, updated_at, created_by)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9)
		ON CONFLICT (user_id, ip_address) DO UPDATE SET
			ip_range = EXCLUDED.ip_range,
			reason = EXCLUDED.reason,
			is_active = TRUE,
			expires_at = EXCLUDED.expires_at,
			updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, ip_address, ip_range, reason, is_active,
		          expires_at, created_at, updated_at, created_by
	`

	createdBy := "API:" + email
	var entry models.IPDenylistEntry
	err := h.db.QueryRow(r.Context(), query,
		uuid.New().String(), userID, req.IPAddress, req.IPRange, req.Reason,
		req.ExpiresAt, time.Now(), time.Now(), createdBy,
	).Scan(
		&entry.ID, &entry.UserID, &entry.IPAddress, &entry.IPRange,
		&entry.Reason, &entry.IsActive, &entry.ExpiresAt,
		&entry.CreatedAt, &entry.UpdatedAt, &entry.CreatedBy,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

// RemoveDenylist removes an IP from the denylist
func (h *IPAccessHandler) RemoveDenylist(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }
	vars := mux.Vars(r)
	entryID := vars["id"]

	query := `DELETE FROM ip_denylist WHERE id = $1 AND user_id = $2`
	result, err := h.db.Exec(r.Context(), query, entryID, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAccessLog returns the IP access log for the authenticated user
func (h *IPAccessHandler) GetAccessLog(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	// Parse query parameters for pagination
	limit := 100
	offset := 0

	query := `
		SELECT id, user_id, ip_address, action, result, reason, endpoint, method,
		       user_agent, created_at
		FROM ip_access_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := h.db.Query(r.Context(), query, userID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []models.IPAccessLogEntry
	for rows.Next() {
		var log models.IPAccessLogEntry
		err := rows.Scan(
			&log.ID, &log.UserID, &log.IPAddress, &log.Action, &log.Result,
			&log.Reason, &log.Endpoint, &log.Method, &log.UserAgent, &log.CreatedAt,
		)
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// CheckIP allows users to test if an IP would be allowed or blocked
func (h *IPAccessHandler) CheckIP(w http.ResponseWriter, r *http.Request) {
	userID, _, err := h.getUserIDAndEmail(r); if err != nil { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

	var req struct {
		IPAddress string `json:"ip_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.IPAddress == "" {
		http.Error(w, "ip_address is required", http.StatusBadRequest)
		return
	}

	// Import middleware to use CheckAccess
	// We'll create a helper function in the handler
	result, err := h.checkAccess(r.Context(), userID, req.IPAddress)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// checkAccess is a helper that duplicates the middleware logic for testing
func (h *IPAccessHandler) checkAccess(ctx context.Context, userID, ipAddress string) (*models.IPAccessCheckResult, error) {
	// This is a simplified version - in production, you'd want to reuse the middleware logic
	// For now, just check denylist and allowlist

	// Check if IP is in denylist
	denyQuery := `
		SELECT COUNT(*) FROM ip_denylist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
		  AND (expires_at IS NULL OR expires_at > NOW())
	`
	var denyCount int
	err := h.db.QueryRow(ctx, denyQuery, userID, ipAddress).Scan(&denyCount)
	if err != nil {
		return nil, err
	}
	if denyCount > 0 {
		return &models.IPAccessCheckResult{
			Allowed:   false,
			Reason:    "IP in denylist",
			MatchType: "denylist",
		}, nil
	}

	// Check if IP is in allowlist
	allowQuery := `
		SELECT COUNT(*) FROM ip_allowlist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
	`
	var allowCount int
	err = h.db.QueryRow(ctx, allowQuery, userID, ipAddress).Scan(&allowCount)
	if err != nil {
		return nil, err
	}
	if allowCount > 0 {
		return &models.IPAccessCheckResult{
			Allowed:   true,
			Reason:    "IP in allowlist",
			MatchType: "allowlist",
		}, nil
	}

	return &models.IPAccessCheckResult{
		Allowed:   true,
		Reason:    "No restrictions",
		MatchType: "none",
	}, nil
}
