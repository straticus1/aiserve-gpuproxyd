package database

// IP Access Control Migrations
// These tables store per-user IP allowlists and denylists for API/gRPC access control

var ipAccessControlMigrations = []string{
	// IP allowlist - whitelist approach (only these IPs can access)
	`CREATE TABLE IF NOT EXISTS ip_allowlist (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		ip_address VARCHAR(45) NOT NULL,
		ip_range CIDR,
		description TEXT,
		is_active BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_by VARCHAR(255),
		UNIQUE(user_id, ip_address)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_ip_allowlist_user_id ON ip_allowlist(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_allowlist_user_active ON ip_allowlist(user_id, is_active) WHERE is_active = TRUE`,
	`CREATE INDEX IF NOT EXISTS idx_ip_allowlist_ip ON ip_allowlist(ip_address)`,

	// IP denylist - blacklist approach (these IPs are explicitly blocked)
	`CREATE TABLE IF NOT EXISTS ip_denylist (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		ip_address VARCHAR(45) NOT NULL,
		ip_range CIDR,
		reason TEXT,
		is_active BOOLEAN DEFAULT TRUE,
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_by VARCHAR(255),
		UNIQUE(user_id, ip_address)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_ip_denylist_user_id ON ip_denylist(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_denylist_user_active ON ip_denylist(user_id, is_active) WHERE is_active = TRUE`,
	`CREATE INDEX IF NOT EXISTS idx_ip_denylist_ip ON ip_denylist(ip_address)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_denylist_expires ON ip_denylist(expires_at) WHERE expires_at IS NOT NULL`,

	// IP access configuration per user (global settings)
	`CREATE TABLE IF NOT EXISTS ip_access_config (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
		mode VARCHAR(20) DEFAULT 'disabled',
		allowlist_enabled BOOLEAN DEFAULT FALSE,
		denylist_enabled BOOLEAN DEFAULT TRUE,
		block_on_no_match BOOLEAN DEFAULT FALSE,
		audit_log_enabled BOOLEAN DEFAULT TRUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE INDEX IF NOT EXISTS idx_ip_access_config_user_id ON ip_access_config(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_access_config_mode ON ip_access_config(mode)`,

	// IP access audit log (track all access attempts)
	`CREATE TABLE IF NOT EXISTS ip_access_log (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID REFERENCES users(id) ON DELETE SET NULL,
		ip_address VARCHAR(45) NOT NULL,
		action VARCHAR(20) NOT NULL,
		result VARCHAR(20) NOT NULL,
		reason TEXT,
		endpoint TEXT,
		method VARCHAR(10),
		user_agent TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE INDEX IF NOT EXISTS idx_ip_access_log_user_id ON ip_access_log(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_access_log_ip ON ip_access_log(ip_address)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_access_log_created ON ip_access_log(created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_ip_access_log_result ON ip_access_log(result)`,

	// Performance optimization for hot path queries
	`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ip_allowlist_user_ip_active
		ON ip_allowlist(user_id, ip_address, is_active)
		WHERE is_active = TRUE`,

	`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ip_denylist_user_ip_active
		ON ip_denylist(user_id, ip_address, is_active)
		WHERE is_active = TRUE AND (expires_at IS NULL OR expires_at > NOW())`,

	// CIDR range search optimization (for network blocks)
	`CREATE INDEX IF NOT EXISTS idx_ip_allowlist_range ON ip_allowlist USING gist(ip_range inet_ops) WHERE ip_range IS NOT NULL AND is_active = TRUE`,
	`CREATE INDEX IF NOT EXISTS idx_ip_denylist_range ON ip_denylist USING gist(ip_range inet_ops) WHERE ip_range IS NOT NULL AND is_active = TRUE`,
}
