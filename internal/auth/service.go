package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
)

type Service struct {
	db     *database.PostgresDB
	redis  *database.RedisClient
	config *config.AuthConfig
}

func NewService(db *database.PostgresDB, redis *database.RedisClient, cfg *config.AuthConfig) *Service {
	return &Service{
		db:     db,
		redis:  redis,
		config: cfg,
	}
}

func (s *Service) GetJWTSecret() string {
	return s.config.JWTSecret
}

func (s *Service) Register(ctx context.Context, email, password, name string) (*models.User, error) {
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	userID := uuid.New()
	now := time.Now()

	query := `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, name, is_admin, is_active, created_at, updated_at
	`

	var user models.User
	err = s.db.Pool.QueryRow(ctx, query, userID, email, hashedPassword, name, now, now).
		Scan(&user.ID, &user.Email, &user.Name, &user.IsAdmin, &user.IsActive, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := s.createDefaultQuota(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("failed to create default quota: %w", err)
	}

	return &user, nil
}

func (s *Service) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*TokenPair, *models.User, error) {
	query := `SELECT id, email, password_hash, name, is_admin, is_active FROM users WHERE email = $1`

	var user models.User
	var passwordHash string

	err := s.db.Pool.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Email, &passwordHash, &user.Name, &user.IsAdmin, &user.IsActive)

	if err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}

	if !user.IsActive {
		return nil, nil, fmt.Errorf("user account is inactive")
	}

	if err := VerifyPassword(passwordHash, password); err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}

	tokens, err := GenerateTokenPair(
		user.ID,
		user.Email,
		user.IsAdmin,
		s.config.JWTSecret,
		s.config.JWTExpiration,
		s.config.RefreshExpiration,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := s.saveSession(ctx, user.ID, tokens.RefreshToken, tokens.ExpiresAt.Add(s.config.RefreshExpiration), ipAddress, userAgent); err != nil {
		return nil, nil, err
	}

	return tokens, &user, nil
}

func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	// CRITICAL FIX: Hash the API key first, then query for that specific hash
	// This eliminates the N+1 query problem where we were fetching ALL keys
	// and comparing each one. Now it's a single indexed query.
	keyHash, err := HashAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Single query with hash comparison - uses index on key_hash column
	query := `
		SELECT u.id, u.email, u.name, u.is_admin, u.is_active
		FROM users u
		JOIN api_keys ak ON u.id = ak.user_id
		WHERE ak.key_hash = $1
		AND ak.is_active = true
		AND (ak.expires_at IS NULL OR ak.expires_at > NOW())
		LIMIT 1
	`

	// Set query timeout to prevent long-running queries
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user models.User
	err = s.db.Pool.QueryRow(queryCtx, query, keyHash).
		Scan(&user.ID, &user.Email, &user.Name, &user.IsAdmin, &user.IsActive)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, fmt.Errorf("invalid API key")
		}
		return nil, fmt.Errorf("failed to validate API key: %w", err)
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account is inactive")
	}

	// Update last_used_at asynchronously (don't block response)
	go s.updateAPIKeyLastUsed(user.ID, keyHash)

	return &user, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (string, error) {
	apiKey, err := GenerateAPIKey(s.config.APIKeyLength)
	if err != nil {
		return "", err
	}

	keyHash, err := HashAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	query := `
		INSERT INTO api_keys (user_id, key_hash, name, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err = s.db.Pool.Exec(ctx, query, userID, keyHash, name, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

func (s *Service) saveSession(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time, ipAddress, userAgent string) error {
	if s.redis.UseRedisForSessions() {
		sessionData := map[string]interface{}{
			"user_id":    userID.String(),
			"expires_at": expiresAt.Unix(),
			"ip_address": ipAddress,
			"user_agent": userAgent,
		}

		data, err := json.Marshal(sessionData)
		if err != nil {
			return err
		}

		if err := s.redis.Set(ctx, "session:"+token, data, time.Until(expiresAt)); err != nil {
			return fmt.Errorf("failed to save session to Redis: %w", err)
		}
	}

	if s.redis.UseSQLForSessions() {
		query := `
			INSERT INTO sessions (user_id, token, expires_at, ip_address, user_agent)
			VALUES ($1, $2, $3, $4, $5)
		`

		_, err := s.db.Pool.Exec(ctx, query, userID, token, expiresAt, ipAddress, userAgent)
		if err != nil {
			return fmt.Errorf("failed to save session to database: %w", err)
		}
	}

	return nil
}

func (s *Service) createDefaultQuota(ctx context.Context, userID uuid.UUID) error {
	resetAt := time.Now().AddDate(0, 1, 0)

	query := `
		INSERT INTO usage_quotas (user_id, reset_at)
		VALUES ($1, $2)
	`

	_, err := s.db.Pool.Exec(ctx, query, userID, resetAt)
	return err
}

func (s *Service) updateAPIKeyLastUsed(userID uuid.UUID, keyHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `UPDATE api_keys SET last_used_at = NOW() WHERE user_id = $1 AND key_hash = $2`
	s.db.Pool.Exec(ctx, query, userID, keyHash)
}
