package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/redis/go-redis/v9"
)

// MultiLayerCache implements a 3-tier caching strategy:
// 1. Local in-memory cache (bigcache) - < 1ms, 100MB
// 2. Redis distributed cache - 1-5ms
// 3. Source of truth (database/API)
type MultiLayerCache struct {
	local  *bigcache.BigCache
	redis  *redis.Client
	config Config
	stats  *Stats
}

// Config defines cache behavior
type Config struct {
	// Local cache settings
	LocalEnabled    bool
	LocalSizeMB     int
	LocalTTL        time.Duration
	LocalEviction   time.Duration

	// Redis cache settings
	RedisEnabled bool
	RedisTTL     time.Duration

	// Cache key prefix
	KeyPrefix string
}

// Stats tracks cache performance metrics
type Stats struct {
	LocalHits    int64
	LocalMisses  int64
	RedisHits    int64
	RedisMisses  int64
	SourceHits   int64
	TotalRequests int64
}

// NewMultiLayerCache creates a new multi-layer cache
func NewMultiLayerCache(redis *redis.Client, config Config) (*MultiLayerCache, error) {
	cache := &MultiLayerCache{
		redis:  redis,
		config: config,
		stats:  &Stats{},
	}

	// Initialize local cache if enabled
	if config.LocalEnabled {
		localConfig := bigcache.Config{
			Shards:             1024,                  // Number of cache shards
			LifeWindow:         config.LocalTTL,       // TTL for entries
			CleanWindow:        config.LocalEviction,  // Cleanup interval
			MaxEntriesInWindow: 1000 * 10 * 60,        // Max entries
			MaxEntrySize:       500,                   // Max entry size in bytes
			HardMaxCacheSize:   config.LocalSizeMB,    // Max cache size in MB
			Verbose:            false,
		}

		local, err := bigcache.New(context.Background(), localConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create local cache: %w", err)
		}
		cache.local = local
	}

	return cache, nil
}

// Get retrieves a value from cache (local → Redis → source)
func (c *MultiLayerCache) Get(ctx context.Context, key string, dest interface{}) error {
	c.stats.TotalRequests++

	fullKey := c.config.KeyPrefix + key

	// Try local cache first
	if c.config.LocalEnabled && c.local != nil {
		data, err := c.local.Get(fullKey)
		if err == nil {
			c.stats.LocalHits++
			return json.Unmarshal(data, dest)
		}
		c.stats.LocalMisses++
	}

	// Try Redis cache
	if c.config.RedisEnabled && c.redis != nil {
		data, err := c.redis.Get(ctx, fullKey).Bytes()
		if err == nil {
			c.stats.RedisHits++

			// Store in local cache for next time
			if c.config.LocalEnabled && c.local != nil {
				_ = c.local.Set(fullKey, data)
			}

			return json.Unmarshal(data, dest)
		}
		c.stats.RedisMisses++
	}

	// Cache miss - caller should fetch from source
	return ErrCacheMiss
}

// Set stores a value in all cache layers
func (c *MultiLayerCache) Set(ctx context.Context, key string, value interface{}) error {
	fullKey := c.config.KeyPrefix + key

	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Store in local cache
	if c.config.LocalEnabled && c.local != nil {
		if err := c.local.Set(fullKey, data); err != nil {
			// Log but don't fail on local cache error
			_ = err
		}
	}

	// Store in Redis cache
	if c.config.RedisEnabled && c.redis != nil {
		if err := c.redis.Set(ctx, fullKey, data, c.config.RedisTTL).Err(); err != nil {
			return fmt.Errorf("failed to set Redis cache: %w", err)
		}
	}

	return nil
}

// Delete removes a value from all cache layers
func (c *MultiLayerCache) Delete(ctx context.Context, key string) error {
	fullKey := c.config.KeyPrefix + key

	// Delete from local cache
	if c.config.LocalEnabled && c.local != nil {
		_ = c.local.Delete(fullKey)
	}

	// Delete from Redis cache
	if c.config.RedisEnabled && c.redis != nil {
		if err := c.redis.Del(ctx, fullKey).Err(); err != nil {
			return fmt.Errorf("failed to delete from Redis: %w", err)
		}
	}

	return nil
}

// GetOrSet retrieves from cache or executes function and stores result
func (c *MultiLayerCache) GetOrSet(
	ctx context.Context,
	key string,
	dest interface{},
	fn func() (interface{}, error),
) error {
	// Try to get from cache
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	if err != ErrCacheMiss {
		return err
	}

	// Cache miss - fetch from source
	c.stats.SourceHits++
	value, err := fn()
	if err != nil {
		return err
	}

	// Store in cache for next time
	if err := c.Set(ctx, key, value); err != nil {
		// Log but don't fail on cache set error
		_ = err
	}

	// Copy value to dest
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// InvalidatePattern removes all keys matching a pattern
func (c *MultiLayerCache) InvalidatePattern(ctx context.Context, pattern string) error {
	fullPattern := c.config.KeyPrefix + pattern

	// For local cache, we need to iterate and delete matching keys
	// bigcache doesn't support pattern matching, so we skip local invalidation
	// for patterns. Individual key deletes will work fine.

	// For Redis, use SCAN to find and delete matching keys
	if c.config.RedisEnabled && c.redis != nil {
		iter := c.redis.Scan(ctx, 0, fullPattern, 0).Iterator()
		for iter.Next(ctx) {
			if err := c.redis.Del(ctx, iter.Val()).Err(); err != nil {
				return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
			}
		}
		if err := iter.Err(); err != nil {
			return fmt.Errorf("scan error: %w", err)
		}
	}

	return nil
}

// Flush clears all cache layers
func (c *MultiLayerCache) Flush(ctx context.Context) error {
	// Flush local cache
	if c.config.LocalEnabled && c.local != nil {
		if err := c.local.Reset(); err != nil {
			return fmt.Errorf("failed to flush local cache: %w", err)
		}
	}

	// Flush Redis cache (only keys with our prefix)
	if c.config.RedisEnabled && c.redis != nil {
		iter := c.redis.Scan(ctx, 0, c.config.KeyPrefix+"*", 0).Iterator()
		for iter.Next(ctx) {
			if err := c.redis.Del(ctx, iter.Val()).Err(); err != nil {
				return fmt.Errorf("failed to delete key %s: %w", iter.Val(), err)
			}
		}
		if err := iter.Err(); err != nil {
			return fmt.Errorf("scan error: %w", err)
		}
	}

	return nil
}

// Stats returns cache performance statistics
func (c *MultiLayerCache) Stats() Stats {
	return *c.stats
}

// HitRate returns the overall cache hit rate
func (c *MultiLayerCache) HitRate() float64 {
	if c.stats.TotalRequests == 0 {
		return 0.0
	}

	hits := c.stats.LocalHits + c.stats.RedisHits
	return float64(hits) / float64(c.stats.TotalRequests)
}

// LocalHitRate returns the local cache hit rate
func (c *MultiLayerCache) LocalHitRate() float64 {
	if c.stats.TotalRequests == 0 {
		return 0.0
	}
	return float64(c.stats.LocalHits) / float64(c.stats.TotalRequests)
}

// Close releases cache resources
func (c *MultiLayerCache) Close() error {
	if c.local != nil {
		if err := c.local.Close(); err != nil {
			return fmt.Errorf("failed to close local cache: %w", err)
		}
	}
	return nil
}

// DefaultConfig returns production-ready defaults
func DefaultConfig(keyPrefix string) Config {
	return Config{
		LocalEnabled:  true,
		LocalSizeMB:   100,                  // 100MB local cache
		LocalTTL:      5 * time.Minute,      // 5 min TTL
		LocalEviction: 1 * time.Minute,      // Clean every minute
		RedisEnabled:  true,
		RedisTTL:      30 * time.Minute,     // 30 min TTL
		KeyPrefix:     keyPrefix + ":",
	}
}

// ErrCacheMiss indicates the key was not found in any cache layer
var ErrCacheMiss = fmt.Errorf("cache miss")
