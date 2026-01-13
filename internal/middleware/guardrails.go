package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/google/uuid"
)

type GuardRails struct {
	redis  *database.RedisClient
	config *config.GuardRailsConfig
}

type TimeWindow struct {
	Duration time.Duration
	Limit    float64
	Name     string
}

type SpendingInfo struct {
	UserID      uuid.UUID              `json:"user_id"`
	TotalSpent  float64                `json:"total_spent"`
	WindowSpent map[string]float64     `json:"window_spent"`
	Violations  []string               `json:"violations,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

func NewGuardRails(redis *database.RedisClient, cfg *config.GuardRailsConfig) *GuardRails {
	return &GuardRails{
		redis:  redis,
		config: cfg,
	}
}

func (gr *GuardRails) getTimeWindows() []TimeWindow {
	windows := []TimeWindow{}

	if gr.config.Max5MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 5 * time.Minute, Limit: gr.config.Max5MinRate, Name: "5min"})
	}
	if gr.config.Max15MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 15 * time.Minute, Limit: gr.config.Max15MinRate, Name: "15min"})
	}
	if gr.config.Max30MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 30 * time.Minute, Limit: gr.config.Max30MinRate, Name: "30min"})
	}
	if gr.config.Max60MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 60 * time.Minute, Limit: gr.config.Max60MinRate, Name: "60min"})
	}
	if gr.config.Max90MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 90 * time.Minute, Limit: gr.config.Max90MinRate, Name: "90min"})
	}
	if gr.config.Max120MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 120 * time.Minute, Limit: gr.config.Max120MinRate, Name: "120min"})
	}
	if gr.config.Max240MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 240 * time.Minute, Limit: gr.config.Max240MinRate, Name: "240min"})
	}
	if gr.config.Max300MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 300 * time.Minute, Limit: gr.config.Max300MinRate, Name: "300min"})
	}
	if gr.config.Max360MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 360 * time.Minute, Limit: gr.config.Max360MinRate, Name: "360min"})
	}
	if gr.config.Max400MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 400 * time.Minute, Limit: gr.config.Max400MinRate, Name: "400min"})
	}
	if gr.config.Max460MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 460 * time.Minute, Limit: gr.config.Max460MinRate, Name: "460min"})
	}
	if gr.config.Max520MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 520 * time.Minute, Limit: gr.config.Max520MinRate, Name: "520min"})
	}
	if gr.config.Max640MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 640 * time.Minute, Limit: gr.config.Max640MinRate, Name: "640min"})
	}
	if gr.config.Max700MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 700 * time.Minute, Limit: gr.config.Max700MinRate, Name: "700min"})
	}
	if gr.config.Max1440MinRate > 0 {
		windows = append(windows, TimeWindow{Duration: 1440 * time.Minute, Limit: gr.config.Max1440MinRate, Name: "1440min"})
	}
	if gr.config.Max48HRate > 0 {
		windows = append(windows, TimeWindow{Duration: 48 * time.Hour, Limit: gr.config.Max48HRate, Name: "48h"})
	}
	if gr.config.Max72HRate > 0 {
		windows = append(windows, TimeWindow{Duration: 72 * time.Hour, Limit: gr.config.Max72HRate, Name: "72h"})
	}

	return windows
}

func (gr *GuardRails) CheckSpending(ctx context.Context, userID uuid.UUID, estimatedCost float64) (*SpendingInfo, error) {
	windows := gr.getTimeWindows()
	info := &SpendingInfo{
		UserID:      userID,
		WindowSpent: make(map[string]float64),
		Violations:  []string{},
		Timestamp:   time.Now(),
	}

	for _, window := range windows {
		key := fmt.Sprintf("guardrails:spending:%s:%s", userID.String(), window.Name)

		// Get current spending in this window
		spent, err := gr.getWindowSpending(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to get spending for window %s: %w", window.Name, err)
		}

		info.WindowSpent[window.Name] = spent

		// Check if adding the estimated cost would exceed the limit
		if spent+estimatedCost > window.Limit {
			info.Violations = append(info.Violations, fmt.Sprintf(
				"%s: $%.2f + $%.2f = $%.2f > $%.2f limit",
				window.Name, spent, estimatedCost, spent+estimatedCost, window.Limit,
			))
		}
	}

	return info, nil
}

func (gr *GuardRails) RecordSpending(ctx context.Context, userID uuid.UUID, amount float64) error {
	windows := gr.getTimeWindows()

	for _, window := range windows {
		key := fmt.Sprintf("guardrails:spending:%s:%s", userID.String(), window.Name)

		// Add the spending amount to the window
		if err := gr.addToWindowSpending(ctx, key, amount, window.Duration); err != nil {
			return fmt.Errorf("failed to record spending for window %s: %w", window.Name, err)
		}
	}

	return nil
}

func (gr *GuardRails) GetSpendingInfo(ctx context.Context, userID uuid.UUID) (*SpendingInfo, error) {
	return gr.CheckSpending(ctx, userID, 0)
}

func (gr *GuardRails) ResetSpending(ctx context.Context, userID uuid.UUID, windowName string) error {
	if windowName == "all" {
		windows := gr.getTimeWindows()
		for _, window := range windows {
			key := fmt.Sprintf("guardrails:spending:%s:%s", userID.String(), window.Name)
			if err := gr.redis.Delete(ctx, key); err != nil {
				return fmt.Errorf("failed to reset window %s: %w", window.Name, err)
			}
		}
		return nil
	}

	key := fmt.Sprintf("guardrails:spending:%s:%s", userID.String(), windowName)
	return gr.redis.Delete(ctx, key)
}

func (gr *GuardRails) getWindowSpending(ctx context.Context, key string) (float64, error) {
	val, err := gr.redis.Get(ctx, key)
	if err != nil {
		// Key doesn't exist, return 0
		return 0, nil
	}

	var spent float64
	if err := json.Unmarshal([]byte(val), &spent); err != nil {
		return 0, fmt.Errorf("failed to unmarshal spending data: %w", err)
	}

	return spent, nil
}

func (gr *GuardRails) addToWindowSpending(ctx context.Context, key string, amount float64, ttl time.Duration) error {
	// Get current spending
	current, err := gr.getWindowSpending(ctx, key)
	if err != nil {
		return err
	}

	// Add the new amount
	newTotal := current + amount

	// Store the updated value
	data, err := json.Marshal(newTotal)
	if err != nil {
		return fmt.Errorf("failed to marshal spending data: %w", err)
	}

	if err := gr.redis.Set(ctx, key, string(data), ttl); err != nil {
		return fmt.Errorf("failed to set spending data: %w", err)
	}

	return nil
}

// Middleware checks spending limits before allowing requests
func (gr *GuardRails) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !gr.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			userID := GetUserID(r.Context())
			if userID.String() == "00000000-0000-0000-0000-000000000000" {
				next.ServeHTTP(w, r)
				return
			}

			// For now, we do a basic check with a small estimated cost
			// The actual cost will be recorded after the request completes
			estimatedCost := 0.01 // Minimal check, actual tracking happens post-request

			ctx := r.Context()
			info, err := gr.CheckSpending(ctx, userID, estimatedCost)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "Failed to check spending limits",
				})
				return
			}

			if len(info.Violations) > 0 {
				w.Header().Set("X-GuardRails-Exceeded", "true")
				respondJSON(w, http.StatusPaymentRequired, map[string]interface{}{
					"error":      "Spending limit exceeded",
					"violations": info.Violations,
					"spent":      info.WindowSpent,
				})
				return
			}

			// Add spending info to response headers
			w.Header().Set("X-GuardRails-Enabled", "true")
			for window, spent := range info.WindowSpent {
				w.Header().Set(fmt.Sprintf("X-GuardRails-%s", window), fmt.Sprintf("%.2f", spent))
			}

			next.ServeHTTP(w, r)
		})
	}
}
