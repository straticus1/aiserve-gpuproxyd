package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPMiddleware provides HTTP caching middleware
type HTTPMiddleware struct {
	cache      *MultiLayerCache
	keyBuilder KeyBuilder
	ttl        time.Duration
}

// KeyBuilder generates cache keys from HTTP requests
type KeyBuilder func(*http.Request) string

// NewHTTPMiddleware creates HTTP caching middleware
func NewHTTPMiddleware(cache *MultiLayerCache, ttl time.Duration, keyBuilder KeyBuilder) *HTTPMiddleware {
	if keyBuilder == nil {
		keyBuilder = DefaultKeyBuilder
	}

	return &HTTPMiddleware{
		cache:      cache,
		keyBuilder: keyBuilder,
		ttl:        ttl,
	}
}

// Handler wraps an HTTP handler with caching
func (m *HTTPMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// Generate cache key
		key := m.keyBuilder(r)

		// Try to get from cache
		var cached CachedResponse
		err := m.cache.Get(r.Context(), key, &cached)
		if err == nil {
			// Cache hit - write cached response
			m.writeCachedResponse(w, &cached)
			w.Header().Set("X-Cache", "HIT")
			return
		}

		// Cache miss - execute handler and cache response
		w.Header().Set("X-Cache", "MISS")
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(recorder, r)

		// Cache successful responses
		if recorder.statusCode >= 200 && recorder.statusCode < 300 {
			cached := CachedResponse{
				StatusCode: recorder.statusCode,
				Headers:    recorder.Header().Clone(),
				Body:       recorder.body.Bytes(),
				CachedAt:   time.Now(),
			}

			_ = m.cache.Set(r.Context(), key, cached)
		}
	})
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CachedAt   time.Time
}

// writeCachedResponse writes a cached response to the response writer
func (m *HTTPMiddleware) writeCachedResponse(w http.ResponseWriter, cached *CachedResponse) {
	// Copy headers
	for key, values := range cached.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add cache metadata
	w.Header().Set("X-Cache-Date", cached.CachedAt.Format(time.RFC3339))

	// Write status code
	w.WriteHeader(cached.StatusCode)

	// Write body
	_, _ = w.Write(cached.Body)
}

// responseRecorder captures HTTP responses for caching
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// DefaultKeyBuilder generates cache keys from request URL and query params
func DefaultKeyBuilder(r *http.Request) string {
	return fmt.Sprintf("http:%s:%s", r.Method, r.URL.String())
}

// UserAwareKeyBuilder includes user ID in cache key
func UserAwareKeyBuilder(getUserID func(*http.Request) string) KeyBuilder {
	return func(r *http.Request) string {
		userID := getUserID(r)
		return fmt.Sprintf("http:%s:%s:user:%s", r.Method, r.URL.String(), userID)
	}
}

// HashKeyBuilder creates a hash-based cache key (shorter keys)
func HashKeyBuilder(r *http.Request) string {
	hash := sha256.New()
	hash.Write([]byte(r.Method))
	hash.Write([]byte(r.URL.String()))
	return "http:" + hex.EncodeToString(hash.Sum(nil))[:16]
}

// InvalidateHandler provides HTTP endpoint to invalidate cache
func (m *HTTPMiddleware) InvalidateHandler(pattern string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read pattern from request body if not provided
		if pattern == "" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}
			pattern = string(body)
		}

		// Invalidate cache
		if err := m.cache.InvalidatePattern(r.Context(), pattern); err != nil {
			http.Error(w, fmt.Sprintf("Failed to invalidate: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// CacheableQuery wraps a database query with caching
func CacheableQuery[T any](
	ctx context.Context,
	cache *MultiLayerCache,
	key string,
	queryFn func() (T, error),
) (T, error) {
	var result T

	err := cache.GetOrSet(ctx, key, &result, func() (interface{}, error) {
		return queryFn()
	})

	return result, err
}

// CacheableList caches a list of items with individual item caching
func CacheableList[T any](
	ctx context.Context,
	cache *MultiLayerCache,
	listKey string,
	itemKeyFn func(T) string,
	queryFn func() ([]T, error),
) ([]T, error) {
	var result []T

	err := cache.GetOrSet(ctx, listKey, &result, func() (interface{}, error) {
		items, err := queryFn()
		if err != nil {
			return nil, err
		}

		// Cache individual items
		for _, item := range items {
			itemKey := itemKeyFn(item)
			_ = cache.Set(ctx, itemKey, item)
		}

		return items, nil
	})

	return result, err
}

// InvalidateOnWrite invalidates cache after a write operation
func InvalidateOnWrite(
	ctx context.Context,
	cache *MultiLayerCache,
	patterns []string,
	writeFn func() error,
) error {
	// Execute write
	if err := writeFn(); err != nil {
		return err
	}

	// Invalidate related cache entries
	for _, pattern := range patterns {
		_ = cache.InvalidatePattern(ctx, pattern)
	}

	return nil
}

// CacheKeyGen provides common cache key generation patterns
type CacheKeyGen struct {
	prefix string
}

// NewCacheKeyGen creates a new cache key generator
func NewCacheKeyGen(prefix string) *CacheKeyGen {
	return &CacheKeyGen{prefix: prefix}
}

// User generates a user-specific cache key
func (g *CacheKeyGen) User(userID, resource string) string {
	return fmt.Sprintf("%s:user:%s:%s", g.prefix, userID, resource)
}

// Resource generates a resource-specific cache key
func (g *CacheKeyGen) Resource(resourceType, resourceID string) string {
	return fmt.Sprintf("%s:resource:%s:%s", g.prefix, resourceType, resourceID)
}

// List generates a list cache key
func (g *CacheKeyGen) List(resourceType string, filters ...string) string {
	if len(filters) == 0 {
		return fmt.Sprintf("%s:list:%s", g.prefix, resourceType)
	}
	return fmt.Sprintf("%s:list:%s:%s", g.prefix, resourceType, strings.Join(filters, ":"))
}

// Query generates a query cache key with parameters
func (g *CacheKeyGen) Query(queryName string, params map[string]string) string {
	// Sort params for consistent keys
	var parts []string
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("%s:query:%s:%s", g.prefix, queryName, strings.Join(parts, "&"))
}

// Timestamped generates a cache key with timestamp bucket (for time-based caching)
func (g *CacheKeyGen) Timestamped(resource string, bucket time.Duration) string {
	now := time.Now()
	bucketTime := now.Truncate(bucket).Unix()
	return fmt.Sprintf("%s:ts:%s:%d", g.prefix, resource, bucketTime)
}
