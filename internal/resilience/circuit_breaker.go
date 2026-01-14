package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreaker provides circuit breaker functionality for services
type CircuitBreaker struct {
	breakers map[string]*gobreaker.CircuitBreaker
	mu       sync.RWMutex
	settings Settings
}

// Settings defines circuit breaker configuration
type Settings struct {
	MaxRequests       uint32        // Max requests allowed in half-open state
	Interval          time.Duration // Period for collecting stats
	Timeout           time.Duration // Time before transitioning from open to half-open
	FailureThreshold  float64       // Failure ratio to trip (0.0-1.0)
	MinRequests       uint32        // Minimum requests before checking failure ratio
	OnStateChange     func(name string, from gobreaker.State, to gobreaker.State)
}

var (
	// ErrCircuitOpen is returned when circuit is open
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// DefaultSettings provides sensible defaults
	DefaultSettings = Settings{
		MaxRequests:      3,
		Interval:         60 * time.Second,
		Timeout:          30 * time.Second,
		FailureThreshold: 0.6,
		MinRequests:      10,
	}
)

// NewCircuitBreaker creates a new circuit breaker manager
func NewCircuitBreaker(settings Settings) *CircuitBreaker {
	if settings.MaxRequests == 0 {
		settings = DefaultSettings
	}

	return &CircuitBreaker{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		settings: settings,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(service string, fn func() (interface{}, error)) (interface{}, error) {
	breaker := cb.getOrCreateBreaker(service)

	result, err := breaker.Execute(fn)
	if err == gobreaker.ErrOpenState {
		return nil, ErrCircuitOpen
	}

	return result, err
}

// ExecuteContext runs a function with circuit breaker protection and context
func (cb *CircuitBreaker) ExecuteContext(ctx context.Context, service string, fn func() (interface{}, error)) (interface{}, error) {
	// Check context before execution
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return cb.Execute(service, fn)
}

// GetState returns the current state of a circuit breaker
func (cb *CircuitBreaker) GetState(service string) gobreaker.State {
	cb.mu.RLock()
	breaker, exists := cb.breakers[service]
	cb.mu.RUnlock()

	if !exists {
		return gobreaker.StateClosed
	}

	return breaker.State()
}

// GetCounts returns the current counts for a circuit breaker
func (cb *CircuitBreaker) GetCounts(service string) gobreaker.Counts {
	cb.mu.RLock()
	breaker, exists := cb.breakers[service]
	cb.mu.RUnlock()

	if !exists {
		return gobreaker.Counts{}
	}

	return breaker.Counts()
}

// Reset resets a circuit breaker to closed state by removing it
// The next request will create a new breaker in closed state
func (cb *CircuitBreaker) Reset(service string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.breakers, service)
}

// getOrCreateBreaker gets or creates a circuit breaker for a service
func (cb *CircuitBreaker) getOrCreateBreaker(service string) *gobreaker.CircuitBreaker {
	cb.mu.RLock()
	breaker, exists := cb.breakers[service]
	cb.mu.RUnlock()

	if exists {
		return breaker
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := cb.breakers[service]; exists {
		return breaker
	}

	// Create new breaker
	breaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        service,
		MaxRequests: cb.settings.MaxRequests,
		Interval:    cb.settings.Interval,
		Timeout:     cb.settings.Timeout,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < cb.settings.MinRequests {
				return false
			}

			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRatio >= cb.settings.FailureThreshold
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if cb.settings.OnStateChange != nil {
				cb.settings.OnStateChange(name, from, to)
			}
			// Default logging
			fmt.Printf("Circuit breaker '%s' changed from %s to %s\n", name, from, to)
		},
	})

	cb.breakers[service] = breaker
	return breaker
}

// ListBreakers returns a list of all circuit breaker names
func (cb *CircuitBreaker) ListBreakers() []string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	names := make([]string, 0, len(cb.breakers))
	for name := range cb.breakers {
		names = append(names, name)
	}
	return names
}

// GetStats returns statistics for all circuit breakers
func (cb *CircuitBreaker) GetStats() map[string]BreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	stats := make(map[string]BreakerStats)
	for name, breaker := range cb.breakers {
		counts := breaker.Counts()
		stats[name] = BreakerStats{
			State:           breaker.State().String(),
			Requests:        counts.Requests,
			TotalSuccesses:  counts.TotalSuccesses,
			TotalFailures:   counts.TotalFailures,
			ConsecutiveSuccesses: counts.ConsecutiveSuccesses,
			ConsecutiveFailures:  counts.ConsecutiveFailures,
		}
	}
	return stats
}

// BreakerStats represents circuit breaker statistics
type BreakerStats struct {
	State                string
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}
