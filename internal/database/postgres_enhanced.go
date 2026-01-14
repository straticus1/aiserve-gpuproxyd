package database

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnhancedPostgresDB provides read replica support and advanced pooling
type EnhancedPostgresDB struct {
	writePool  *pgxpool.Pool
	readPools  []*pgxpool.Pool
	readIndex  atomic.Uint32
	config     EnhancedConfig
}

// EnhancedConfig extends DatabaseConfig with replica support
type EnhancedConfig struct {
	// Primary database
	PrimaryURL string

	// Read replicas
	ReadReplicaURLs []string

	// Connection pool settings
	MaxConns           int32
	MinConns           int32
	MaxConnLifetime    time.Duration
	MaxConnIdleTime    time.Duration
	HealthCheckPeriod  time.Duration

	// Advanced settings
	PreparedStatementCache int
	StatementTimeout       time.Duration
	LockTimeout            time.Duration
}

// NewEnhancedPostgresDB creates a new database instance with replica support
func NewEnhancedPostgresDB(config EnhancedConfig) (*EnhancedPostgresDB, error) {
	// Create write pool (primary)
	poolConfig, err := pgxpool.ParseConfig(config.PrimaryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse primary URL: %w", err)
	}

	configurePool(poolConfig, config)

	writePool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create write pool: %w", err)
	}

	db := &EnhancedPostgresDB{
		writePool: writePool,
		readPools: make([]*pgxpool.Pool, 0, len(config.ReadReplicaURLs)),
		config:    config,
	}

	// Create read pools (replicas)
	for _, replicaURL := range config.ReadReplicaURLs {
		replicaPoolConfig, err := pgxpool.ParseConfig(replicaURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse replica URL: %w", err)
		}

		configurePool(replicaPoolConfig, config)

		replicaPool, err := pgxpool.NewWithConfig(context.Background(), replicaPoolConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create replica pool: %w", err)
		}

		db.readPools = append(db.readPools, replicaPool)
	}

	// If no replicas, use primary for reads too
	if len(db.readPools) == 0 {
		db.readPools = append(db.readPools, writePool)
	}

	return db, nil
}

// configurePool applies configuration to a pool
func configurePool(poolConfig *pgxpool.Config, config EnhancedConfig) {
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnLifetime
	poolConfig.MaxConnIdleTime = config.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheckPeriod

	// Set connection timeouts
	if config.StatementTimeout > 0 {
		poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprintf("%dms", config.StatementTimeout.Milliseconds())
	}
	if config.LockTimeout > 0 {
		poolConfig.ConnConfig.RuntimeParams["lock_timeout"] = fmt.Sprintf("%dms", config.LockTimeout.Milliseconds())
	}

	// Enable prepared statement cache
	if config.PreparedStatementCache > 0 {
		poolConfig.ConnConfig.DefaultQueryExecMode = 0 // Default mode uses prepared statements
	}
}

// Write returns the write pool (primary database)
func (db *EnhancedPostgresDB) Write() *pgxpool.Pool {
	return db.writePool
}

// Read returns a read pool (replica or primary)
// Uses round-robin load balancing across replicas
func (db *EnhancedPostgresDB) Read() *pgxpool.Pool {
	if len(db.readPools) == 1 {
		return db.readPools[0]
	}

	// Round-robin across replicas
	idx := db.readIndex.Add(1) % uint32(len(db.readPools))
	return db.readPools[idx]
}

// ReadWithFallback tries to read from replica, falls back to primary if replica fails
func (db *EnhancedPostgresDB) ReadWithFallback(ctx context.Context, fn func(*pgxpool.Pool) error) error {
	// Try replica first
	err := fn(db.Read())
	if err == nil {
		return nil
	}

	// If replica fails and we have separate write pool, try primary
	if len(db.readPools) > 1 || db.readPools[0] != db.writePool {
		return fn(db.writePool)
	}

	return err
}

// Stats returns statistics for all pools
func (db *EnhancedPostgresDB) Stats() PoolStats {
	stats := PoolStats{
		Write:    getPoolStat(db.writePool),
		Replicas: make([]PoolStat, len(db.readPools)),
	}

	for i, pool := range db.readPools {
		if pool != db.writePool {
			stats.Replicas[i] = getPoolStat(pool)
		}
	}

	return stats
}

// getPoolStat extracts statistics from a pool
func getPoolStat(pool *pgxpool.Pool) PoolStat {
	stat := pool.Stat()
	return PoolStat{
		AcquireCount:         stat.AcquireCount(),
		AcquiredConns:        stat.AcquiredConns(),
		CanceledAcquireCount: stat.CanceledAcquireCount(),
		ConstructingConns:    stat.ConstructingConns(),
		EmptyAcquireCount:    stat.EmptyAcquireCount(),
		IdleConns:            stat.IdleConns(),
		MaxConns:             stat.MaxConns(),
		TotalConns:           stat.TotalConns(),
	}
}

// PoolStats represents statistics for all database pools
type PoolStats struct {
	Write    PoolStat
	Replicas []PoolStat
}

// PoolStat represents statistics for a single pool
type PoolStat struct {
	AcquireCount         int64
	AcquiredConns        int32
	CanceledAcquireCount int64
	ConstructingConns    int32
	EmptyAcquireCount    int64
	IdleConns            int32
	MaxConns             int32
	TotalConns           int32
}

// Close closes all database connections
func (db *EnhancedPostgresDB) Close() {
	if db.writePool != nil {
		db.writePool.Close()
	}

	for _, pool := range db.readPools {
		if pool != db.writePool {
			pool.Close()
		}
	}
}

// HealthCheck verifies all pools are healthy
func (db *EnhancedPostgresDB) HealthCheck(ctx context.Context) error {
	// Check write pool
	if err := db.writePool.Ping(ctx); err != nil {
		return fmt.Errorf("write pool unhealthy: %w", err)
	}

	// Check read pools
	for i, pool := range db.readPools {
		if pool == db.writePool {
			continue
		}
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("read pool %d unhealthy: %w", i, err)
		}
	}

	return nil
}

// DefaultEnhancedConfig returns production-ready defaults
func DefaultEnhancedConfig(primaryURL string, replicaURLs []string) EnhancedConfig {
	return EnhancedConfig{
		PrimaryURL:             primaryURL,
		ReadReplicaURLs:        replicaURLs,
		MaxConns:               100,  // 4x default
		MinConns:               25,   // 5x default
		MaxConnLifetime:        30 * time.Minute,
		MaxConnIdleTime:        10 * time.Minute,
		HealthCheckPeriod:      1 * time.Minute,
		PreparedStatementCache: 1000,
		StatementTimeout:       30 * time.Second,
		LockTimeout:            10 * time.Second,
	}
}
