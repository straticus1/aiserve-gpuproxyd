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
	GuardRails   GuardRailsConfig
	Logging      LoggingConfig
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

type GuardRailsConfig struct {
	Enabled         bool
	Max5MinRate     float64
	Max15MinRate    float64
	Max30MinRate    float64
	Max60MinRate    float64
	Max90MinRate    float64
	Max120MinRate   float64
	Max240MinRate   float64
	Max300MinRate   float64
	Max360MinRate   float64
	Max400MinRate   float64
	Max460MinRate   float64
	Max520MinRate   float64
	Max640MinRate   float64
	Max700MinRate   float64
	Max1440MinRate  float64
	Max48HRate      float64
	Max72HRate      float64
}

type LoggingConfig struct {
	SyslogEnabled  bool
	SyslogNetwork  string
	SyslogAddress  string
	SyslogTag      string
	SyslogFacility string
	LogFile        string
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
		GuardRails: GuardRailsConfig{
			Enabled:         getEnvAsBool("GUARDRAILS_ENABLED", false),
			Max5MinRate:     getEnvAsFloat("GUARDRAILS_MAX_5MIN_RATE", 0),
			Max15MinRate:    getEnvAsFloat("GUARDRAILS_MAX_15MIN_RATE", 0),
			Max30MinRate:    getEnvAsFloat("GUARDRAILS_MAX_30MIN_RATE", 0),
			Max60MinRate:    getEnvAsFloat("GUARDRAILS_MAX_60MIN_RATE", 0),
			Max90MinRate:    getEnvAsFloat("GUARDRAILS_MAX_90MIN_RATE", 0),
			Max120MinRate:   getEnvAsFloat("GUARDRAILS_MAX_120MIN_RATE", 0),
			Max240MinRate:   getEnvAsFloat("GUARDRAILS_MAX_240MIN_RATE", 0),
			Max300MinRate:   getEnvAsFloat("GUARDRAILS_MAX_300MIN_RATE", 0),
			Max360MinRate:   getEnvAsFloat("GUARDRAILS_MAX_360MIN_RATE", 0),
			Max400MinRate:   getEnvAsFloat("GUARDRAILS_MAX_400MIN_RATE", 0),
			Max460MinRate:   getEnvAsFloat("GUARDRAILS_MAX_460MIN_RATE", 0),
			Max520MinRate:   getEnvAsFloat("GUARDRAILS_MAX_520MIN_RATE", 0),
			Max640MinRate:   getEnvAsFloat("GUARDRAILS_MAX_640MIN_RATE", 0),
			Max700MinRate:   getEnvAsFloat("GUARDRAILS_MAX_700MIN_RATE", 0),
			Max1440MinRate:  getEnvAsFloat("GUARDRAILS_MAX_1440MIN_RATE", 0),
			Max48HRate:      getEnvAsFloat("GUARDRAILS_MAX_48H_RATE", 0),
			Max72HRate:      getEnvAsFloat("GUARDRAILS_MAX_72H_RATE", 0),
		},
		Logging: LoggingConfig{
			SyslogEnabled:  getEnvAsBool("SYSLOG_ENABLED", false),
			SyslogNetwork:  getEnv("SYSLOG_NETWORK", ""),
			SyslogAddress:  getEnv("SYSLOG_ADDRESS", ""),
			SyslogTag:      getEnv("SYSLOG_TAG", "aiserve-gpuproxy"),
			SyslogFacility: getEnv("SYSLOG_FACILITY", "LOG_LOCAL0"),
			LogFile:        getEnv("LOG_FILE", ""),
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

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}
	var value float64
	fmt.Sscanf(valueStr, "%f", &value)
	return value
}
