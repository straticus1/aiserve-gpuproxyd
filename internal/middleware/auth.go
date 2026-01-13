package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserContextKey    contextKey = "user"
	RequestIDKey      contextKey = "request_id"
)

type AuthMiddleware struct {
	authService *auth.Service
	jwtSecret   string
}

func NewAuthMiddleware(authService *auth.Service, jwtSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		jwtSecret:   jwtSecret,
	}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			user, err := m.authService.ValidateAPIKey(r.Context(), apiKey)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid API key"})
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing authorization"})
			return
		}

		bearerToken := strings.TrimPrefix(authHeader, "Bearer ")
		if bearerToken == authHeader {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid authorization format"})
			return
		}

		claims, err := auth.ValidateToken(bearerToken, m.jwtSecret)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			return
		}

		user := &models.User{
			ID:      claims.UserID,
			Email:   claims.Email,
			IsAdmin: claims.IsAdmin,
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil || !user.IsAdmin {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func GetUser(ctx context.Context) *models.User {
	user, ok := ctx.Value(UserContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

func GetUserID(ctx context.Context) uuid.UUID {
	user := GetUser(ctx)
	if user == nil {
		return uuid.Nil
	}
	return user.ID
}
