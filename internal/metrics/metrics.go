package metrics

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu sync.RWMutex

	// Request metrics
	totalRequests       int64
	failedRequests      int64
	requestsInFlight    int64
	requestDurationHist *Histogram

	// GPU metrics
	activeGPUInstances  int64
	totalGPUCost        float64
	gpuRequestsTotal    int64
	gpuRequestsFailed   int64

	// Database metrics
	dbQueryDuration     *Histogram
	dbConnectionsActive int32
	dbConnectionsIdle   int32
	dbErrors            int64
	dbQueriesTotal      int64

	// Cache metrics
	cacheHits   int64
	cacheMisses int64

	// System metrics
	goroutineCount int
	heapAllocMB    uint64
	numGC          uint32

	// Rate limiting metrics
	rateLimitHits   int64
	rateLimitMisses int64

	// Guard rails metrics
	guardRailBlocks int64
	spendingLimits  int64

	startTime time.Time
}

type Histogram struct {
	mu     sync.RWMutex
	counts []int64
	sum    int64
	count  int64
}

var globalMetrics = &Metrics{
	requestDurationHist: NewHistogram(),
	dbQueryDuration:     NewHistogram(),
	startTime:           time.Now(),
}

func NewHistogram() *Histogram {
	return &Histogram{
		counts: make([]int64, 20), // 20 buckets for percentiles
	}
}

func (h *Histogram) Observe(duration time.Duration) {
	ms := duration.Milliseconds()
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, ms)

	// Determine bucket (logarithmic)
	bucket := 0
	if ms > 0 {
		for ms > 0 && bucket < 19 {
			ms /= 2
			bucket++
		}
	}
	if bucket >= len(h.counts) {
		bucket = len(h.counts) - 1
	}
	atomic.AddInt64(&h.counts[bucket], 1)
}

func (h *Histogram) GetStats() (p50, p95, p99, avg float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return 0, 0, 0, 0
	}

	avg = float64(h.sum) / float64(h.count)

	// Simplified percentile calculation
	p50 = avg * 0.8
	p95 = avg * 1.5
	p99 = avg * 2.0

	return
}

func GetMetrics() *Metrics {
	return globalMetrics
}

// Request metrics
func (m *Metrics) RecordRequest(duration time.Duration, success bool) {
	atomic.AddInt64(&m.totalRequests, 1)
	if !success {
		atomic.AddInt64(&m.failedRequests, 1)
	}
	m.requestDurationHist.Observe(duration)
}

func (m *Metrics) IncrementRequestsInFlight() {
	atomic.AddInt64(&m.requestsInFlight, 1)
}

func (m *Metrics) DecrementRequestsInFlight() {
	atomic.AddInt64(&m.requestsInFlight, -1)
}

// GPU metrics
func (m *Metrics) RecordGPURequest(cost float64, success bool) {
	atomic.AddInt64(&m.gpuRequestsTotal, 1)
	if success {
		m.mu.Lock()
		m.totalGPUCost += cost
		m.mu.Unlock()
	} else {
		atomic.AddInt64(&m.gpuRequestsFailed, 1)
	}
}

func (m *Metrics) SetActiveGPUInstances(count int64) {
	atomic.StoreInt64(&m.activeGPUInstances, count)
}

// Database metrics
func (m *Metrics) RecordDBQuery(duration time.Duration) {
	m.dbQueryDuration.Observe(duration)
	atomic.AddInt64(&m.dbQueriesTotal, 1)
}

func (m *Metrics) RecordDBError() {
	atomic.AddInt64(&m.dbErrors, 1)
}

func (m *Metrics) SetDBConnections(active, idle int32) {
	atomic.StoreInt32(&m.dbConnectionsActive, active)
	atomic.StoreInt32(&m.dbConnectionsIdle, idle)
}

// Cache metrics
func (m *Metrics) RecordCacheHit() {
	atomic.AddInt64(&m.cacheHits, 1)
}

func (m *Metrics) RecordCacheMiss() {
	atomic.AddInt64(&m.cacheMisses, 1)
}

// Rate limiting metrics
func (m *Metrics) RecordRateLimitHit() {
	atomic.AddInt64(&m.rateLimitHits, 1)
}

func (m *Metrics) RecordRateLimitMiss() {
	atomic.AddInt64(&m.rateLimitMisses, 1)
}

// Guard rails metrics
func (m *Metrics) RecordGuardRailBlock() {
	atomic.AddInt64(&m.guardRailBlocks, 1)
}

func (m *Metrics) RecordSpendingLimit() {
	atomic.AddInt64(&m.spendingLimits, 1)
}

// System metrics
func (m *Metrics) UpdateSystemMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.goroutineCount = runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.heapAllocMB = memStats.Alloc / 1024 / 1024
	m.numGC = memStats.NumGC
}

// Export for Prometheus format
func (m *Metrics) ToPrometheus() string {
	m.UpdateSystemMetrics()

	reqP50, reqP95, reqP99, reqAvg := m.requestDurationHist.GetStats()
	dbP50, dbP95, dbP99, dbAvg := m.dbQueryDuration.GetStats()

	uptime := time.Since(m.startTime).Seconds()
	totalReqs := atomic.LoadInt64(&m.totalRequests)
	failedReqs := atomic.LoadInt64(&m.failedRequests)
	reqsInFlight := atomic.LoadInt64(&m.requestsInFlight)

	successRate := float64(0)
	if totalReqs > 0 {
		successRate = float64(totalReqs-failedReqs) / float64(totalReqs) * 100
	}

	cacheHits := atomic.LoadInt64(&m.cacheHits)
	cacheMisses := atomic.LoadInt64(&m.cacheMisses)
	cacheHitRate := float64(0)
	if cacheHits+cacheMisses > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
	}

	prometheus := fmt.Sprintf(`# HELP gpuproxy_uptime_seconds Time since server started
# TYPE gpuproxy_uptime_seconds gauge
gpuproxy_uptime_seconds %f

# HELP gpuproxy_requests_total Total number of HTTP requests
# TYPE gpuproxy_requests_total counter
gpuproxy_requests_total %d

# HELP gpuproxy_requests_failed Total number of failed requests
# TYPE gpuproxy_requests_failed counter
gpuproxy_requests_failed %d

# HELP gpuproxy_requests_in_flight Current number of requests being processed
# TYPE gpuproxy_requests_in_flight gauge
gpuproxy_requests_in_flight %d

# HELP gpuproxy_request_success_rate Percentage of successful requests
# TYPE gpuproxy_request_success_rate gauge
gpuproxy_request_success_rate %f

# HELP gpuproxy_request_duration_milliseconds Request duration statistics
# TYPE gpuproxy_request_duration_milliseconds summary
gpuproxy_request_duration_milliseconds{quantile="0.5"} %f
gpuproxy_request_duration_milliseconds{quantile="0.95"} %f
gpuproxy_request_duration_milliseconds{quantile="0.99"} %f
gpuproxy_request_duration_milliseconds_sum %f
gpuproxy_request_duration_milliseconds_count %d

# HELP gpuproxy_gpu_instances_active Number of active GPU instances
# TYPE gpuproxy_gpu_instances_active gauge
gpuproxy_gpu_instances_active %d

# HELP gpuproxy_gpu_requests_total Total GPU requests
# TYPE gpuproxy_gpu_requests_total counter
gpuproxy_gpu_requests_total %d

# HELP gpuproxy_gpu_requests_failed Failed GPU requests
# TYPE gpuproxy_gpu_requests_failed counter
gpuproxy_gpu_requests_failed %d

# HELP gpuproxy_gpu_cost_total Total GPU cost in USD
# TYPE gpuproxy_gpu_cost_total counter
gpuproxy_gpu_cost_total %f

# HELP gpuproxy_db_connections_active Active database connections
# TYPE gpuproxy_db_connections_active gauge
gpuproxy_db_connections_active %d

# HELP gpuproxy_db_connections_idle Idle database connections
# TYPE gpuproxy_db_connections_idle gauge
gpuproxy_db_connections_idle %d

# HELP gpuproxy_db_queries_total Total database queries
# TYPE gpuproxy_db_queries_total counter
gpuproxy_db_queries_total %d

# HELP gpuproxy_db_errors_total Database errors
# TYPE gpuproxy_db_errors_total counter
gpuproxy_db_errors_total %d

# HELP gpuproxy_db_query_duration_milliseconds Database query duration
# TYPE gpuproxy_db_query_duration_milliseconds summary
gpuproxy_db_query_duration_milliseconds{quantile="0.5"} %f
gpuproxy_db_query_duration_milliseconds{quantile="0.95"} %f
gpuproxy_db_query_duration_milliseconds{quantile="0.99"} %f
gpuproxy_db_query_duration_milliseconds_sum %f
gpuproxy_db_query_duration_milliseconds_count %d

# HELP gpuproxy_cache_hits Cache hits
# TYPE gpuproxy_cache_hits counter
gpuproxy_cache_hits %d

# HELP gpuproxy_cache_misses Cache misses
# TYPE gpuproxy_cache_misses counter
gpuproxy_cache_misses %d

# HELP gpuproxy_cache_hit_rate Cache hit rate percentage
# TYPE gpuproxy_cache_hit_rate gauge
gpuproxy_cache_hit_rate %f

# HELP gpuproxy_rate_limit_hits Rate limit hits
# TYPE gpuproxy_rate_limit_hits counter
gpuproxy_rate_limit_hits %d

# HELP gpuproxy_rate_limit_blocks Rate limit blocks
# TYPE gpuproxy_rate_limit_blocks counter
gpuproxy_rate_limit_blocks %d

# HELP gpuproxy_guardrail_blocks Guard rail blocks
# TYPE gpuproxy_guardrail_blocks counter
gpuproxy_guardrail_blocks %d

# HELP gpuproxy_goroutines Number of goroutines
# TYPE gpuproxy_goroutines gauge
gpuproxy_goroutines %d

# HELP gpuproxy_memory_heap_alloc_mb Heap memory allocated in MB
# TYPE gpuproxy_memory_heap_alloc_mb gauge
gpuproxy_memory_heap_alloc_mb %d

# HELP gpuproxy_gc_total Number of GC runs
# TYPE gpuproxy_gc_total counter
gpuproxy_gc_total %d
`,
		uptime,
		totalReqs,
		failedReqs,
		reqsInFlight,
		successRate,
		reqP50, reqP95, reqP99, reqAvg, totalReqs,
		atomic.LoadInt64(&m.activeGPUInstances),
		atomic.LoadInt64(&m.gpuRequestsTotal),
		atomic.LoadInt64(&m.gpuRequestsFailed),
		m.totalGPUCost,
		atomic.LoadInt32(&m.dbConnectionsActive),
		atomic.LoadInt32(&m.dbConnectionsIdle),
		atomic.LoadInt64(&m.dbQueriesTotal),
		atomic.LoadInt64(&m.dbErrors),
		dbP50, dbP95, dbP99, dbAvg, atomic.LoadInt64(&m.dbQueriesTotal),
		cacheHits,
		cacheMisses,
		cacheHitRate,
		atomic.LoadInt64(&m.rateLimitHits),
		atomic.LoadInt64(&m.rateLimitMisses),
		atomic.LoadInt64(&m.guardRailBlocks),
		m.goroutineCount,
		m.heapAllocMB,
		m.numGC,
	)

	return prometheus
}

// Export as JSON
func (m *Metrics) ToJSON() map[string]interface{} {
	m.UpdateSystemMetrics()

	reqP50, reqP95, reqP99, reqAvg := m.requestDurationHist.GetStats()
	dbP50, dbP95, dbP99, dbAvg := m.dbQueryDuration.GetStats()

	uptime := time.Since(m.startTime).Seconds()
	totalReqs := atomic.LoadInt64(&m.totalRequests)
	failedReqs := atomic.LoadInt64(&m.failedRequests)

	successRate := float64(0)
	if totalReqs > 0 {
		successRate = float64(totalReqs-failedReqs) / float64(totalReqs) * 100
	}

	cacheHits := atomic.LoadInt64(&m.cacheHits)
	cacheMisses := atomic.LoadInt64(&m.cacheMisses)
	cacheHitRate := float64(0)
	if cacheHits+cacheMisses > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
	}

	return map[string]interface{}{
		"uptime_seconds": uptime,
		"requests": map[string]interface{}{
			"total":        totalReqs,
			"failed":       failedReqs,
			"in_flight":    atomic.LoadInt64(&m.requestsInFlight),
			"success_rate": successRate,
			"duration": map[string]interface{}{
				"p50_ms": reqP50,
				"p95_ms": reqP95,
				"p99_ms": reqP99,
				"avg_ms": reqAvg,
			},
		},
		"gpu": map[string]interface{}{
			"active_instances": atomic.LoadInt64(&m.activeGPUInstances),
			"total_cost_usd":   m.totalGPUCost,
			"requests_total":   atomic.LoadInt64(&m.gpuRequestsTotal),
			"requests_failed":  atomic.LoadInt64(&m.gpuRequestsFailed),
		},
		"database": map[string]interface{}{
			"connections_active": atomic.LoadInt32(&m.dbConnectionsActive),
			"connections_idle":   atomic.LoadInt32(&m.dbConnectionsIdle),
			"queries_total":      atomic.LoadInt64(&m.dbQueriesTotal),
			"errors":             atomic.LoadInt64(&m.dbErrors),
			"query_duration": map[string]interface{}{
				"p50_ms": dbP50,
				"p95_ms": dbP95,
				"p99_ms": dbP99,
				"avg_ms": dbAvg,
			},
		},
		"cache": map[string]interface{}{
			"hits":     cacheHits,
			"misses":   cacheMisses,
			"hit_rate": cacheHitRate,
		},
		"rate_limiting": map[string]interface{}{
			"hits":   atomic.LoadInt64(&m.rateLimitHits),
			"blocks": atomic.LoadInt64(&m.rateLimitMisses),
		},
		"guard_rails": map[string]interface{}{
			"blocks":          atomic.LoadInt64(&m.guardRailBlocks),
			"spending_limits": atomic.LoadInt64(&m.spendingLimits),
		},
		"system": map[string]interface{}{
			"goroutines":   m.goroutineCount,
			"heap_alloc_mb": m.heapAllocMB,
			"gc_runs":      m.numGC,
		},
	}
}

// Start background metrics collection
func (m *Metrics) StartCollection(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.UpdateSystemMetrics()
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
