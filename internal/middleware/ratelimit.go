package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/aiserve/gpuproxy/internal/database"
)

type RateLimiter struct {
	redis *database.RedisClient
}

func NewRateLimiter(redis *database.RedisClient) *RateLimiter {
	return &RateLimiter{redis: redis}
}

func (rl *RateLimiter) Limit(requestsPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID.String() == "00000000-0000-0000-0000-000000000000" {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
				return
			}

			key := fmt.Sprintf("ratelimit:%s:%d", userID.String(), time.Now().Unix()/60)
			ctx := context.Background()

			count, err := rl.redis.Increment(ctx, key)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Rate limit check failed"})
				return
			}

			if count == 1 {
				rl.redis.Expire(ctx, key, 60*time.Second)
			}

			if count > int64(requestsPerMinute) {
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(requestsPerMinute))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+60, 10))
				respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Rate limit exceeded"})
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(requestsPerMinute))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(int64(requestsPerMinute)-count, 10))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Unix()+60, 10))

			next.ServeHTTP(w, r)
		})
	}
}
