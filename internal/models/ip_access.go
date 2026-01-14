package models

import (
	"time"
)

// IPAccessMode defines the IP access control mode
type IPAccessMode string

const (
	IPAccessModeDisabled  IPAccessMode = "disabled"  // No IP filtering
	IPAccessModeAllowlist IPAccessMode = "allowlist" // Only allowed IPs can access
	IPAccessModeDenylist  IPAccessMode = "denylist"  // All IPs except denied ones can access
	IPAccessModeStrict    IPAccessMode = "strict"    // Both allowlist and denylist enforced
)

// IPAllowlistEntry represents an allowed IP address or range for a user
type IPAllowlistEntry struct {
	ID          string     `json:"id" db:"id"`
	UserID      string     `json:"user_id" db:"user_id"`
	IPAddress   string     `json:"ip_address" db:"ip_address"`
	IPRange     *string    `json:"ip_range,omitempty" db:"ip_range"` // CIDR notation (e.g., "192.168.1.0/24")
	Description string     `json:"description,omitempty" db:"description"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy   *string    `json:"created_by,omitempty" db:"created_by"` // CLI, API, admin email
}

// IPDenylistEntry represents a blocked IP address or range for a user
type IPDenylistEntry struct {
	ID         string     `json:"id" db:"id"`
	UserID     string     `json:"user_id" db:"user_id"`
	IPAddress  string     `json:"ip_address" db:"ip_address"`
	IPRange    *string    `json:"ip_range,omitempty" db:"ip_range"` // CIDR notation
	Reason     string     `json:"reason,omitempty" db:"reason"`
	IsActive   bool       `json:"is_active" db:"is_active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty" db:"expires_at"` // Auto-expire blocked IPs
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy  *string    `json:"created_by,omitempty" db:"created_by"`
}

// IPAccessConfig stores the IP access control configuration per user
type IPAccessConfig struct {
	ID                string       `json:"id" db:"id"`
	UserID            string       `json:"user_id" db:"user_id"`
	Mode              IPAccessMode `json:"mode" db:"mode"`
	AllowlistEnabled  bool         `json:"allowlist_enabled" db:"allowlist_enabled"`
	DenylistEnabled   bool         `json:"denylist_enabled" db:"denylist_enabled"`
	BlockOnNoMatch    bool         `json:"block_on_no_match" db:"block_on_no_match"` // Block if IP not in allowlist
	AuditLogEnabled   bool         `json:"audit_log_enabled" db:"audit_log_enabled"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// IPAccessLogEntry records IP access attempts for audit
type IPAccessLogEntry struct {
	ID         string    `json:"id" db:"id"`
	UserID     *string   `json:"user_id,omitempty" db:"user_id"`
	IPAddress  string    `json:"ip_address" db:"ip_address"`
	Action     string    `json:"action" db:"action"`       // "allow", "deny", "check"
	Result     string    `json:"result" db:"result"`       // "allowed", "blocked", "error"
	Reason     string    `json:"reason,omitempty" db:"reason"`
	Endpoint   string    `json:"endpoint,omitempty" db:"endpoint"`
	Method     string    `json:"method,omitempty" db:"method"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// IPAccessCheckResult represents the result of an IP access check
type IPAccessCheckResult struct {
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason"`
	MatchType string `json:"match_type"` // "allowlist", "denylist", "none", "disabled"
}
