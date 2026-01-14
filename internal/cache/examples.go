package cache

import (
	"context"
	"time"
)

// Example: GPU Instance List Caching
// This example shows how to cache expensive GPU instance queries

type GPUInstance struct {
	ID       string
	Provider string
	Status   string
	Price    float64
}

// ExampleGPUInstanceCache demonstrates caching GPU instance lists
func ExampleGPUInstanceCache(ctx context.Context, cache *MultiLayerCache) {
	keyGen := NewCacheKeyGen("gpu")

	// Cache all available instances (30 second TTL for fresh data)
	instances, err := CacheableList(
		ctx,
		cache,
		keyGen.List("instances", "status=available"),
		func(instance GPUInstance) string {
			return keyGen.Resource("instance", instance.ID)
		},
		func() ([]GPUInstance, error) {
			// Expensive query to multiple GPU providers
			return fetchGPUInstancesFromProviders()
		},
	)

	if err != nil {
		// Handle error
		return
	}

	_ = instances
}

// Example: User Profile Caching
// Cache user profiles with automatic invalidation

type UserProfile struct {
	ID       string
	Email    string
	Plan     string
	Credits  float64
	MetaData map[string]interface{}
}

// ExampleUserProfileCache demonstrates user-specific caching
func ExampleUserProfileCache(ctx context.Context, cache *MultiLayerCache, userID string) {
	keyGen := NewCacheKeyGen("user")

	// Get user profile (cached for 5 minutes)
	var profile UserProfile
	err := cache.GetOrSet(
		ctx,
		keyGen.User(userID, "profile"),
		&profile,
		func() (interface{}, error) {
			return fetchUserProfileFromDB(userID)
		},
	)

	if err != nil {
		// Handle error
		return
	}

	_ = profile
}

// Example: Update User with Cache Invalidation
func ExampleUpdateUserWithInvalidation(ctx context.Context, cache *MultiLayerCache, userID string, updates map[string]interface{}) error {
	keyGen := NewCacheKeyGen("user")

	// Invalidate related cache entries after update
	return InvalidateOnWrite(
		ctx,
		cache,
		[]string{
			keyGen.User(userID, "*"),         // All user-specific keys
			keyGen.List("users", "*"),        // User list caches
		},
		func() error {
			return updateUserInDB(userID, updates)
		},
	)
}

// Example: API Response Caching
// Cache expensive API calls with circuit breaker integration

type APIResponse struct {
	Data      interface{}
	Timestamp time.Time
	Source    string
}

// ExampleAPIResponseCache demonstrates caching external API calls
func ExampleAPIResponseCache(ctx context.Context, cache *MultiLayerCache, endpoint string, params map[string]string) (*APIResponse, error) {
	keyGen := NewCacheKeyGen("api")
	cacheKey := keyGen.Query(endpoint, params)

	var response APIResponse
	err := cache.GetOrSet(
		ctx,
		cacheKey,
		&response,
		func() (interface{}, error) {
			// Expensive external API call
			return callExternalAPI(endpoint, params)
		},
	)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

// Example: Time-Bucketed Metrics Caching
// Cache metrics in time buckets (e.g., per-minute buckets)

type Metrics struct {
	RequestCount  int64
	ErrorCount    int64
	AvgLatency    float64
	Timestamp     time.Time
}

// ExampleTimeBucketedMetrics demonstrates time-based caching
func ExampleTimeBucketedMetrics(ctx context.Context, cache *MultiLayerCache) (*Metrics, error) {
	keyGen := NewCacheKeyGen("metrics")

	// Cache metrics in 1-minute buckets
	cacheKey := keyGen.Timestamped("requests", 1*time.Minute)

	var metrics Metrics
	err := cache.GetOrSet(
		ctx,
		cacheKey,
		&metrics,
		func() (interface{}, error) {
			return calculateMetricsFromDB()
		},
	)

	if err != nil {
		return nil, err
	}

	return &metrics, nil
}

// Example: HTTP Endpoint Caching with Middleware

/*
import (
	"net/http"
	"github.com/redis/go-redis/v9"
)

func SetupCachedRoutes(redisClient *redis.Client) http.Handler {
	// Create multi-layer cache
	cache, err := NewMultiLayerCache(redisClient, DefaultConfig("http"))
	if err != nil {
		panic(err)
	}

	// Create cache middleware
	cacheMiddleware := NewHTTPMiddleware(cache, 5*time.Minute, DefaultKeyBuilder)

	// Setup routes
	mux := http.NewServeMux()

	// Cache GET /api/gpu/instances
	mux.Handle("/api/gpu/instances", cacheMiddleware.Handler(
		http.HandlerFunc(handleGPUInstances),
	))

	// Cache GET /api/models
	mux.Handle("/api/models", cacheMiddleware.Handler(
		http.HandlerFunc(handleModels),
	))

	// Cache invalidation endpoint
	mux.HandleFunc("/api/cache/invalidate", cacheMiddleware.InvalidateHandler(""))

	return mux
}
*/

// Example: Full Integration with Service Layer

/*
type GPUService struct {
	cache  *MultiLayerCache
	db     *sql.DB
	keyGen *CacheKeyGen
}

func NewGPUService(cache *MultiLayerCache, db *sql.DB) *GPUService {
	return &GPUService{
		cache:  cache,
		db:     db,
		keyGen: NewCacheKeyGen("gpu"),
	}
}

func (s *GPUService) GetInstance(ctx context.Context, instanceID string) (*GPUInstance, error) {
	var instance GPUInstance

	err := s.cache.GetOrSet(
		ctx,
		s.keyGen.Resource("instance", instanceID),
		&instance,
		func() (interface{}, error) {
			return s.fetchInstanceFromDB(ctx, instanceID)
		},
	)

	if err != nil {
		return nil, err
	}

	return &instance, nil
}

func (s *GPUService) ListAvailableInstances(ctx context.Context, filters map[string]string) ([]GPUInstance, error) {
	filterStr := make([]string, 0, len(filters))
	for k, v := range filters {
		filterStr = append(filterStr, fmt.Sprintf("%s=%s", k, v))
	}

	return CacheableList(
		ctx,
		s.cache,
		s.keyGen.List("instances", filterStr...),
		func(instance GPUInstance) string {
			return s.keyGen.Resource("instance", instance.ID)
		},
		func() ([]GPUInstance, error) {
			return s.queryAvailableInstances(ctx, filters)
		},
	)
}

func (s *GPUService) CreateInstance(ctx context.Context, req *CreateInstanceRequest) (*GPUInstance, error) {
	// Create instance in DB
	instance, err := s.createInstanceInDB(ctx, req)
	if err != nil {
		return nil, err
	}

	// Invalidate list caches
	_ = s.cache.InvalidatePattern(ctx, s.keyGen.List("instances", "*"))

	// Cache the new instance
	_ = s.cache.Set(ctx, s.keyGen.Resource("instance", instance.ID), instance)

	return instance, nil
}

func (s *GPUService) DeleteInstance(ctx context.Context, instanceID string) error {
	return InvalidateOnWrite(
		ctx,
		s.cache,
		[]string{
			s.keyGen.Resource("instance", instanceID),
			s.keyGen.List("instances", "*"),
		},
		func() error {
			return s.deleteInstanceFromDB(ctx, instanceID)
		},
	)
}
*/

// Helper functions (mock implementations)

func fetchGPUInstancesFromProviders() ([]GPUInstance, error) {
	// Mock: Query VastAI, IO.net, etc.
	return []GPUInstance{
		{ID: "vast-1", Provider: "vastai", Status: "available", Price: 0.50},
		{ID: "io-1", Provider: "ionet", Status: "available", Price: 0.45},
	}, nil
}

func fetchUserProfileFromDB(userID string) (*UserProfile, error) {
	// Mock: Query database
	return &UserProfile{
		ID:    userID,
		Email: "user@example.com",
		Plan:  "pro",
		Credits: 100.0,
	}, nil
}

func updateUserInDB(userID string, updates map[string]interface{}) error {
	// Mock: Update database
	return nil
}

func callExternalAPI(endpoint string, params map[string]string) (*APIResponse, error) {
	// Mock: External API call
	return &APIResponse{
		Data:      "mock response",
		Timestamp: time.Now(),
		Source:    endpoint,
	}, nil
}

func calculateMetricsFromDB() (*Metrics, error) {
	// Mock: Calculate metrics
	return &Metrics{
		RequestCount: 1000,
		ErrorCount:   10,
		AvgLatency:   50.5,
		Timestamp:    time.Now(),
	}, nil
}
