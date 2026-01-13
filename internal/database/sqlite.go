package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDB struct {
	db *sql.DB
}

func NewSQLiteDB(dbPath string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open SQLite database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("unable to ping SQLite database: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

func (db *SQLiteDB) Close() error {
	return db.db.Close()
}

func (db *SQLiteDB) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := db.db.ExecContext(ctx, query, args...)
	return err
}

func (db *SQLiteDB) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &sqliteRow{row: db.db.QueryRowContext(ctx, query, args...)}
}

func (db *SQLiteDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &sqliteRows{rows: rows}, nil
}

func (db *SQLiteDB) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			is_admin INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			key_hash TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			last_used_at TIMESTAMP,
			expires_at TIMESTAMP,
			is_active INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token TEXT UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,

		`CREATE TABLE IF NOT EXISTS usage_quotas (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			max_gpu_hours REAL DEFAULT 100.00,
			used_gpu_hours REAL DEFAULT 0.00,
			max_requests INTEGER DEFAULT 10000,
			used_requests INTEGER DEFAULT 0,
			reset_at TIMESTAMP NOT NULL,
			reset_interval TEXT DEFAULT 'monthly',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_usage_quotas_user_id ON usage_quotas(user_id)`,

		`CREATE TABLE IF NOT EXISTS credits (
			id TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			ip TEXT,
			date TEXT NOT NULL,
			time TEXT NOT NULL,
			duration INTEGER DEFAULT 0,
			credits_remaining REAL DEFAULT 0,
			credits_total REAL DEFAULT 0,
			credits_overage REAL DEFAULT 0,
			credits_cap REAL DEFAULT 0,
			credits_auto_renew INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE INDEX IF NOT EXISTS idx_credits_client_id ON credits(client_id)`,
		`CREATE INDEX IF NOT EXISTS idx_credits_date ON credits(date)`,

		`CREATE TABLE IF NOT EXISTS gpu_usage (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			instance_id TEXT NOT NULL,
			gpu_model TEXT NOT NULL,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			duration REAL DEFAULT 0,
			cost REAL DEFAULT 0,
			billing_id TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_user_id ON gpu_usage(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_provider ON gpu_usage(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_gpu_usage_start_time ON gpu_usage(start_time)`,

		`CREATE TABLE IF NOT EXISTS billing_transactions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			amount REAL NOT NULL,
			currency TEXT DEFAULT 'USD',
			status TEXT NOT NULL,
			payment_method TEXT NOT NULL,
			payment_provider TEXT NOT NULL,
			external_id TEXT,
			metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_user_id ON billing_transactions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_status ON billing_transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_billing_transactions_external_id ON billing_transactions(external_id)`,
	}

	ctx := context.Background()
	for _, query := range queries {
		if err := db.Exec(ctx, query); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

type sqliteRow struct {
	row *sql.Row
}

func (r *sqliteRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

type sqliteRows struct {
	rows *sql.Rows
}

func (r *sqliteRows) Next() bool {
	return r.rows.Next()
}

func (r *sqliteRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *sqliteRows) Close() {
	r.rows.Close()
}
