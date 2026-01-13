package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/metrics"
)

type ObservabilityHandler struct {
	db    *database.PostgresDB
	redis *database.RedisClient
}

func NewObservabilityHandler(db *database.PostgresDB, redis *database.RedisClient) *ObservabilityHandler {
	return &ObservabilityHandler{
		db:    db,
		redis: redis,
	}
}

// HandleMetrics returns Prometheus-formatted metrics
func (h *ObservabilityHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	m := metrics.GetMetrics()
	prometheusText := m.ToPrometheus()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, prometheusText)
}

// HandleStats returns JSON-formatted statistics
func (h *ObservabilityHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	m := metrics.GetMetrics()
	stats := m.ToJSON()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

// HandlePolling returns real-time system status for polling
func (h *ObservabilityHandler) HandlePolling(w http.ResponseWriter, r *http.Request) {
	m := metrics.GetMetrics()

	// Quick snapshot for polling
	polling := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "ok",
		"metrics": map[string]interface{}{
			"requests_in_flight": m.ToJSON()["requests"].(map[string]interface{})["in_flight"],
			"active_gpu_instances": m.ToJSON()["gpu"].(map[string]interface{})["active_instances"],
			"db_connections_active": m.ToJSON()["database"].(map[string]interface{})["connections_active"],
			"cache_hit_rate": m.ToJSON()["cache"].(map[string]interface{})["hit_rate"],
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(polling)
}

// HandleMonitor returns comprehensive monitoring data
func (h *ObservabilityHandler) HandleMonitor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	m := metrics.GetMetrics()

	// Perform health checks
	dbHealth := h.checkDatabaseHealth(ctx)
	redisHealth := h.checkRedisHealth(ctx)

	monitor := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "ok",
		"health": map[string]interface{}{
			"database": dbHealth,
			"redis":    redisHealth,
		},
		"metrics": m.ToJSON(),
	}

	// Set overall status based on health checks
	if !dbHealth["healthy"].(bool) || !redisHealth["healthy"].(bool) {
		monitor["status"] = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(monitor)
}

// HandleHealth returns comprehensive health check
func (h *ObservabilityHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbHealth := h.checkDatabaseHealth(ctx)
	redisHealth := h.checkRedisHealth(ctx)

	allHealthy := dbHealth["healthy"].(bool) && redisHealth["healthy"].(bool)

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks": map[string]interface{}{
			"database": dbHealth,
			"redis":    redisHealth,
		},
	}

	if !allHealthy {
		health["status"] = "unhealthy"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (h *ObservabilityHandler) checkDatabaseHealth(ctx context.Context) map[string]interface{} {
	start := time.Now()

	err := h.db.Pool.Ping(ctx)
	duration := time.Since(start).Milliseconds()

	stats := h.db.Pool.Stat()

	health := map[string]interface{}{
		"healthy":           err == nil,
		"response_time_ms":  duration,
		"total_connections": stats.TotalConns(),
		"idle_connections":  stats.IdleConns(),
		"max_connections":   stats.MaxConns(),
	}

	if err != nil {
		health["error"] = err.Error()
	}

	return health
}

func (h *ObservabilityHandler) checkRedisHealth(ctx context.Context) map[string]interface{} {
	start := time.Now()

	err := h.redis.Client.Ping(ctx).Err()
	duration := time.Since(start).Milliseconds()

	poolStats := h.redis.Client.PoolStats()

	health := map[string]interface{}{
		"healthy":          err == nil,
		"response_time_ms": duration,
		"total_connections": poolStats.TotalConns,
		"idle_connections":  poolStats.IdleConns,
		"stale_connections": poolStats.StaleConns,
	}

	if err != nil {
		health["error"] = err.Error()
	}

	return health
}
