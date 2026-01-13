package database

import (
	"context"
	"fmt"

	"github.com/aiserve/gpuproxy/internal/config"
)

type DB interface {
	Exec(ctx context.Context, query string, args ...interface{}) error
	QueryRow(ctx context.Context, query string, args ...interface{}) Row
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	Close() error
	Migrate() error
}

type Row interface {
	Scan(dest ...interface{}) error
}

type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close()
}

func NewDatabase(dbType string, cfg config.DatabaseConfig) (interface{}, error) {
	switch dbType {
	case "postgres", "postgresql":
		return NewPostgresDB(cfg)
	case "sqlite":
		return NewSQLiteDB(cfg.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

type postgresRow struct {
	row interface {
		Scan(dest ...interface{}) error
	}
}

func (r *postgresRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

type postgresRows struct {
	rows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Close()
	}
}

func (r *postgresRows) Next() bool {
	return r.rows.Next()
}

func (r *postgresRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *postgresRows) Close() {
	r.rows.Close()
}

type PostgresDBWrapper struct {
	*PostgresDB
}

func (db *PostgresDBWrapper) Exec(ctx context.Context, query string, args ...interface{}) error {
	_, err := db.Pool.Exec(ctx, query, args...)
	return err
}

func (db *PostgresDBWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return &postgresRow{row: db.Pool.QueryRow(ctx, query, args...)}
}

func (db *PostgresDBWrapper) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &postgresRows{rows: rows}, nil
}

func (db *PostgresDBWrapper) Close() error {
	db.Pool.Close()
	return nil
}

func NewPostgresDBI(cfg config.DatabaseConfig) (DB, error) {
	pg, err := NewPostgresDB(cfg)
	if err != nil {
		return nil, err
	}
	return &PostgresDBWrapper{PostgresDB: pg}, nil
}

func (db *PostgresDB) ExecCompat(ctx context.Context, query string, args ...interface{}) error {
	_, err := db.Pool.Exec(ctx, query, args...)
	return err
}

func (db *PostgresDB) QueryRowCompat(ctx context.Context, query string, args ...interface{}) Row {
	return &postgresRow{row: db.Pool.QueryRow(ctx, query, args...)}
}

func (db *PostgresDB) QueryCompat(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &postgresRows{rows: rows}, nil
}
