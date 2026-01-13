package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/aiserve/gpuproxy/internal/logging"
	"github.com/aiserve/gpuproxy/internal/metrics"
)

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Preferred-Payment, X-Timelimit, X-Billing")
		w.Header().Set("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Increment requests in flight
		m := metrics.GetMetrics()
		m.IncrementRequestsInFlight()
		defer m.DecrementRequestsInFlight()

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		success := wrapped.statusCode >= 200 && wrapped.statusCode < 400

		// Record metrics
		m.RecordRequest(duration, success)

		// Structured logging
		requestID := GetRequestID(r.Context())
		userID := GetUserID(r.Context())

		fields := map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": wrapped.statusCode,
			"duration":    duration,
			"remote_addr": r.RemoteAddr,
		}

		if requestID != "" {
			fields["request_id"] = requestID
		}

		if userID.String() != "00000000-0000-0000-0000-000000000000" {
			fields["user_id"] = userID.String()
		}

		if wrapped.statusCode >= 400 {
			logging.Error("Request failed", fields)
		} else {
			logging.Info("Request completed", fields)
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(r.Context())
				stackTrace := string(debug.Stack())

				fields := map[string]interface{}{
					"method":      r.Method,
					"path":        r.URL.Path,
					"error":       err,
					"stack_trace": stackTrace,
				}

				if requestID != "" {
					fields["request_id"] = requestID
				}

				logging.Error("Panic recovered", fields)
				log.Printf("panic: %v\n%s", err, stackTrace)

				respondJSON(w, http.StatusInternalServerError, map[string]string{
					"error":      "Internal server error",
					"request_id": requestID,
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
