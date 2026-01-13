package database

import (
	"context"
	"fmt"
	"time"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDB struct {
	Pool *pgxpool.Pool
}

func NewPostgresDB(cfg config.DatabaseConfig) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode, cfg.MaxConns, cfg.MinConns,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &PostgresDB{Pool: pool}, nil
}

func (db *PostgresDB) Close() error {
	db.Pool.Close()
	return nil
}

func (db *PostgresDB) Migrate() error {
	ctx := context.Background()

	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			is_admin BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			key_hash VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			last_used_at TIMESTAMP,
			expires_at TIMESTAMP,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(512) UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ip_address VARCHAR(45),
			user_agent TEXT
		)`,

		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,

		`CREATE TABLE IF NOT EXISTS usage_quotas (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			max_gpu_hours DECIMAL(10,2) DEFAULT 100.00,
			used_gpu_hours DECIMAL(10,2) DEFAULT 0.00,
			max_requests BIGINT DEFAULT 10000,
			used_requests BIGINT DEFAULT 0,
			reset_at TIMESTAMP NOT NULL,
			reset_interval VARCHAR(50) DEFAULT 'monthly',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_usage_quotas_user_id ON usage_quotas(user_id)`,

		`CREATE TABLE IF NOT EXISTS gpu_usage (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider VARCHAR(50) NOT NULL,
			instance_id VARCHAR(255) NOT NULL,
			gpu_model VARCHAR(255) NOT NULL,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			duration DECIMAL(10,4) DEFAULT 0,
			cost DECIMAL(10,4) DEFAULT 0,
			billing_id UUID,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_user_id ON gpu_usage(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_provider ON gpu_usage(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_start_time ON gpu_usage(start_time)`,

		`CREATE TABLE IF NOT EXISTS billing_transactions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(10,2) NOT NULL,
			currency VARCHAR(10) DEFAULT 'USD',
			status VARCHAR(50) NOT NULL,
			payment_method VARCHAR(50) NOT NULL,
			payment_provider VARCHAR(50) NOT NULL,
			external_id VARCHAR(255),
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_user_id ON billing_transactions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_status ON billing_transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_external_id ON billing_transactions(external_id)`,

		`CREATE TABLE IF NOT EXISTS credits (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			client_id VARCHAR(255) NOT NULL,
			ip VARCHAR(45),
			date VARCHAR(10) NOT NULL,
			time VARCHAR(8) NOT NULL,
			duration INTEGER DEFAULT 0,
			credits_remaining DECIMAL(10,2) DEFAULT 0,
			credits_total DECIMAL(10,2) DEFAULT 0,
			credits_overage DECIMAL(10,2) DEFAULT 0,
			credits_cap DECIMAL(10,2) DEFAULT 0,
			credits_auto_renew BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_credits_client_id ON credits(client_id)`,
		`CREATE INDEX IF NOT EXISTS idx_credits_date ON credits(date)`,
	}

	for _, query := range queries {
		if _, err := db.Pool.Exec(ctx, query); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
