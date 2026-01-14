package resilience

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"syscall"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	JitterFactor   float64
}

// DefaultRetryConfig provides sensible defaults
var DefaultRetryConfig = RetryConfig{
	MaxRetries:     3,
	InitialBackoff: 100 * time.Millisecond,
	MaxBackoff:     10 * time.Second,
	Multiplier:     2.0,
	JitterFactor:   0.5,
}

// RetryFunc is a function that can be retried
type RetryFunc func() error

// RetryFuncWithResult is a function that returns a result and can be retried
type RetryFuncWithResult[T any] func() (T, error)

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryFunc) error {
	_, err := RetryWithResult(ctx, config, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// RetryWithResult executes a function with exponential backoff retry logic and returns a result
func RetryWithResult[T any](ctx context.Context, config RetryConfig, fn RetryFuncWithResult[T]) (T, error) {
	var zero T
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Execute function
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry on non-retryable errors
		if !IsRetryable(err) {
			return zero, fmt.Errorf("non-retryable error: %w", err)
		}

		// Don't sleep after last attempt
		if attempt < config.MaxRetries {
			backoff := calculateBackoff(config, attempt)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
	}

	return zero, fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}

// calculateBackoff calculates the backoff duration with jitter
func calculateBackoff(config RetryConfig, attempt int) time.Duration {
	// Calculate base backoff with exponential growth
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter to prevent thundering herd
	jitter := backoff * config.JitterFactor * (rand.Float64()*2 - 1) // Random between -jitter and +jitter
	backoff += jitter

	// Ensure minimum of 0
	if backoff < 0 {
		backoff = 0
	}

	return time.Duration(backoff)
}

// IsRetryable determines if an error should trigger a retry
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Connection errors are retryable
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	// HTTP status codes
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		// Retry on 5xx errors and 429 (rate limit)
		return httpErr.StatusCode >= 500 || httpErr.StatusCode == http.StatusTooManyRequests
	}

	// Circuit breaker open is retryable (with backoff, circuit may close)
	if errors.Is(err, ErrCircuitOpen) {
		return true
	}

	// Default: don't retry
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// RetryWithCircuitBreaker combines retry logic with circuit breaker
func RetryWithCircuitBreaker[T any](
	ctx context.Context,
	cb *CircuitBreaker,
	service string,
	config RetryConfig,
	fn RetryFuncWithResult[T],
) (T, error) {
	return RetryWithResult(ctx, config, func() (T, error) {
		result, err := cb.ExecuteContext(ctx, service, func() (interface{}, error) {
			return fn()
		})

		if err != nil {
			var zero T
			return zero, err
		}

		return result.(T), nil
	})
}

// RetryCondition allows custom retry logic based on result
type RetryCondition[T any] func(T, error) bool

// RetryWithCondition retries based on a custom condition
func RetryWithCondition[T any](
	ctx context.Context,
	config RetryConfig,
	fn RetryFuncWithResult[T],
	condition RetryCondition[T],
) (T, error) {
	var zero T
	var lastErr error
	var lastResult T

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		result, err := fn()
		lastResult = result
		lastErr = err

		// Check custom condition
		if !condition(result, err) {
			return result, err
		}

		if attempt < config.MaxRetries {
			backoff := calculateBackoff(config, attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}
	}

	return lastResult, fmt.Errorf("max retries exceeded: %w", lastErr)
}
