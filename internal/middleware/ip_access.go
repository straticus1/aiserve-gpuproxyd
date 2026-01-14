package middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetUser is already defined in auth.go in the same package
// We can reference it directly since we're in the middleware package

// IPAccessControl middleware enforces IP allowlist/denylist per user
type IPAccessControl struct {
	db *pgxpool.Pool
}

// NewIPAccessControl creates a new IP access control middleware
func NewIPAccessControl(db *pgxpool.Pool) *IPAccessControl {
	return &IPAccessControl{
		db: db,
	}
}

// Middleware returns the HTTP middleware handler
func (iac *IPAccessControl) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user from context (set by auth middleware)
			user := GetUser(r.Context())
			if user == nil {
				// No user authenticated - skip IP filtering
				next.ServeHTTP(w, r)
				return
			}

			userIDStr := user.ID.String()

			// Get client IP
			clientIP := extractClientIP(r)
			if clientIP == "" {
				http.Error(w, "Unable to determine client IP", http.StatusBadRequest)
				return
			}

			// Check IP access
			result, err := iac.CheckAccess(r.Context(), userIDStr, clientIP)
			if err != nil {
				log.Printf("IP access check error for user %s from %s: %v", userIDStr, clientIP, err)
				http.Error(w, "IP access check failed", http.StatusInternalServerError)
				return
			}

			// Log access attempt if audit logging is enabled
			go iac.logAccess(userIDStr, clientIP, result, r)

			if !result.Allowed {
				log.Printf("IP access denied for user %s from %s: %s", userIDStr, clientIP, result.Reason)
				http.Error(w, fmt.Sprintf("Access denied: %s", result.Reason), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CheckAccess checks if an IP is allowed to access the API for a user
func (iac *IPAccessControl) CheckAccess(ctx context.Context, userID, ipAddress string) (*models.IPAccessCheckResult, error) {
	// Get user's IP access configuration
	config, err := iac.getConfig(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP access config: %w", err)
	}

	// If disabled, allow all
	if config == nil || config.Mode == models.IPAccessModeDisabled {
		return &models.IPAccessCheckResult{
			Allowed:   true,
			Reason:    "IP access control disabled",
			MatchType: "disabled",
		}, nil
	}

	// Check denylist first (highest priority)
	if config.DenylistEnabled || config.Mode == models.IPAccessModeDenylist || config.Mode == models.IPAccessModeStrict {
		denied, reason, err := iac.isInDenylist(ctx, userID, ipAddress)
		if err != nil {
			return nil, fmt.Errorf("denylist check failed: %w", err)
		}
		if denied {
			return &models.IPAccessCheckResult{
				Allowed:   false,
				Reason:    reason,
				MatchType: "denylist",
			}, nil
		}
	}

	// Check allowlist (if enabled)
	if config.AllowlistEnabled || config.Mode == models.IPAccessModeAllowlist || config.Mode == models.IPAccessModeStrict {
		allowed, reason, err := iac.isInAllowlist(ctx, userID, ipAddress)
		if err != nil {
			return nil, fmt.Errorf("allowlist check failed: %w", err)
		}
		if !allowed {
			if config.BlockOnNoMatch {
				return &models.IPAccessCheckResult{
					Allowed:   false,
					Reason:    reason,
					MatchType: "allowlist",
				}, nil
			}
		} else {
			return &models.IPAccessCheckResult{
				Allowed:   true,
				Reason:    reason,
				MatchType: "allowlist",
			}, nil
		}
	}

	// Default: allow if denylist only and not in denylist
	return &models.IPAccessCheckResult{
		Allowed:   true,
		Reason:    "IP not in denylist",
		MatchType: "none",
	}, nil
}

// getConfig retrieves the IP access configuration for a user
func (iac *IPAccessControl) getConfig(ctx context.Context, userID string) (*models.IPAccessConfig, error) {
	query := `
		SELECT id, user_id, mode, allowlist_enabled, denylist_enabled,
		       block_on_no_match, audit_log_enabled, created_at, updated_at
		FROM ip_access_config
		WHERE user_id = $1
	`

	var config models.IPAccessConfig
	err := iac.db.QueryRow(ctx, query, userID).Scan(
		&config.ID, &config.UserID, &config.Mode, &config.AllowlistEnabled,
		&config.DenylistEnabled, &config.BlockOnNoMatch, &config.AuditLogEnabled,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			// No config means disabled
			return nil, nil
		}
		return nil, err
	}

	return &config, nil
}

// isInAllowlist checks if an IP is in the user's allowlist
func (iac *IPAccessControl) isInAllowlist(ctx context.Context, userID, ipAddress string) (bool, string, error) {
	// Check exact IP match first
	query := `
		SELECT COUNT(*) FROM ip_allowlist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
	`
	var count int
	err := iac.db.QueryRow(ctx, query, userID, ipAddress).Scan(&count)
	if err != nil {
		return false, "", err
	}
	if count > 0 {
		return true, fmt.Sprintf("IP %s in allowlist", ipAddress), nil
	}

	// Check CIDR range match
	query = `
		SELECT ip_range FROM ip_allowlist
		WHERE user_id = $1 AND ip_range IS NOT NULL AND is_active = TRUE
	`
	rows, err := iac.db.Query(ctx, query, userID)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false, fmt.Sprintf("Invalid IP address: %s", ipAddress), nil
	}

	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			continue
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		if network.Contains(ip) {
			return true, fmt.Sprintf("IP %s matches CIDR range %s in allowlist", ipAddress, cidr), nil
		}
	}

	return false, fmt.Sprintf("IP %s not in allowlist", ipAddress), nil
}

// isInDenylist checks if an IP is in the user's denylist
func (iac *IPAccessControl) isInDenylist(ctx context.Context, userID, ipAddress string) (bool, string, error) {
	// Check exact IP match first (with expiration check)
	query := `
		SELECT COUNT(*) FROM ip_denylist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
		  AND (expires_at IS NULL OR expires_at > NOW())
	`
	var count int
	err := iac.db.QueryRow(ctx, query, userID, ipAddress).Scan(&count)
	if err != nil {
		return false, "", err
	}
	if count > 0 {
		return true, fmt.Sprintf("IP %s is blocked (in denylist)", ipAddress), nil
	}

	// Check CIDR range match
	query = `
		SELECT ip_range, reason FROM ip_denylist
		WHERE user_id = $1 AND ip_range IS NOT NULL AND is_active = TRUE
		  AND (expires_at IS NULL OR expires_at > NOW())
	`
	rows, err := iac.db.Query(ctx, query, userID)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false, "", nil
	}

	for rows.Next() {
		var cidr string
		var reason *string
		if err := rows.Scan(&cidr, &reason); err != nil {
			continue
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		if network.Contains(ip) {
			reasonStr := "in denylist CIDR range " + cidr
			if reason != nil && *reason != "" {
				reasonStr = *reason
			}
			return true, fmt.Sprintf("IP %s blocked: %s", ipAddress, reasonStr), nil
		}
	}

	return false, "", nil
}

// logAccess logs an IP access attempt to the audit log
func (iac *IPAccessControl) logAccess(userID, ipAddress string, result *models.IPAccessCheckResult, r *http.Request) {
	// Check if audit logging is enabled
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := iac.getConfig(ctx, userID)
	if err != nil || config == nil || !config.AuditLogEnabled {
		return
	}

	action := "check"
	resultStr := "allowed"
	if !result.Allowed {
		resultStr = "blocked"
	}

	query := `
		INSERT INTO ip_access_log (id, user_id, ip_address, action, result, reason, endpoint, method, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = iac.db.Exec(ctx, query,
		uuid.New().String(),
		userID,
		ipAddress,
		action,
		resultStr,
		result.Reason,
		r.URL.Path,
		r.Method,
		r.UserAgent(),
		time.Now(),
	)
	if err != nil {
		log.Printf("Failed to log IP access: %v", err)
	}
}

// extractClientIP extracts the real client IP from the request
// Handles X-Forwarded-For, X-Real-IP headers for proxy/load balancer scenarios
func extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (most common)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// We want the first one (the real client)
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (nginx, cloudflare)
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	// Check CF-Connecting-IP (Cloudflare)
	cfIP := r.Header.Get("CF-Connecting-IP")
	if cfIP != "" {
		return strings.TrimSpace(cfIP)
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
