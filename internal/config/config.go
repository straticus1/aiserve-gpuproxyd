package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type SessionMode string

const (
	SessionModeSQL      SessionMode = "sql"
	SessionModeRedis    SessionMode = "redis"
	SessionModeBalanced SessionMode = "balanced"
)

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	Auth         AuthConfig
	Billing      BillingConfig
	GPU          GPUConfig
	LoadBalancer LoadBalancerConfig
}

type ServerConfig struct {
	Host         string
	Port         int
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
	MinConns int
}

type RedisConfig struct {
	Host        string
	Port        int
	Password    string
	DB          int
	SessionMode SessionMode
}

type AuthConfig struct {
	JWTSecret          string
	JWTExpiration      time.Duration
	RefreshExpiration  time.Duration
	APIKeyLength       int
}

type BillingConfig struct {
	StripeSecretKey     string
	StripeWebhookSecret string
	AfterDarkAPIURL     string
	AfterDarkAPIKey     string
	CryptoEnabled       bool
	CryptoNetworks      []string
}

type GPUConfig struct {
	VastAIAPIKey string
	IONetAPIKey  string
	Timeout      time.Duration
}

type LoadBalancerConfig struct {
	Strategy string
	Enabled  bool
}

func Load() (*Config, error) {
	godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnvAsInt("SERVER_PORT", 8080),
			Environment:  getEnv("ENVIRONMENT", "development"),
			ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvAsDuration("IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			Type:     getEnv("DB_TYPE", "postgres"),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "gpuproxy"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
			MaxConns: getEnvAsInt("DB_MAX_CONNS", 25),
			MinConns: getEnvAsInt("DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Host:        getEnv("REDIS_HOST", "localhost"),
			Port:        getEnvAsInt("REDIS_PORT", 6379),
			Password:    getEnv("REDIS_PASSWORD", ""),
			DB:          getEnvAsInt("REDIS_DB", 0),
			SessionMode: SessionMode(getEnv("SESSION_MODE", string(SessionModeBalanced))),
		},
		Auth: AuthConfig{
			JWTSecret:         getEnv("JWT_SECRET", "changeme"),
			JWTExpiration:     getEnvAsDuration("JWT_EXPIRATION", 24*time.Hour),
			RefreshExpiration: getEnvAsDuration("REFRESH_EXPIRATION", 7*24*time.Hour),
			APIKeyLength:      getEnvAsInt("API_KEY_LENGTH", 32),
		},
		Billing: BillingConfig{
			StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
			StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
			AfterDarkAPIURL:     getEnv("AFTERDARK_API_URL", "https://billing.afterdarksys.com"),
			AfterDarkAPIKey:     getEnv("AFTERDARK_API_KEY", ""),
			CryptoEnabled:       getEnvAsBool("CRYPTO_ENABLED", true),
			CryptoNetworks:      []string{"ethereum", "bitcoin", "polygon"},
		},
		GPU: GPUConfig{
			VastAIAPIKey: getEnv("VASTAI_API_KEY", ""),
			IONetAPIKey:  getEnv("IONET_API_KEY", ""),
			Timeout:      getEnvAsDuration("GPU_API_TIMEOUT", 30*time.Second),
		},
		LoadBalancer: LoadBalancerConfig{
			Strategy: getEnv("LB_STRATEGY", "round_robin"),
			Enabled:  getEnvAsBool("LB_ENABLED", true),
		},
	}

	return cfg, cfg.Validate()
}

func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "changeme" && c.Server.Environment == "production" {
		return fmt.Errorf("JWT_SECRET must be set in production")
	}

	if c.GPU.VastAIAPIKey == "" && c.GPU.IONetAPIKey == "" {
		return fmt.Errorf("at least one GPU provider API key must be configured")
	}

	validSessionModes := map[SessionMode]bool{
		SessionModeSQL:      true,
		SessionModeRedis:    true,
		SessionModeBalanced: true,
	}
	if !validSessionModes[c.Redis.SessionMode] {
		return fmt.Errorf("invalid session mode: %s", c.Redis.SessionMode)
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	var value int
	fmt.Sscanf(valueStr, "%d", &value)
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	return valueStr == "true" || valueStr == "1"
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return duration
}
