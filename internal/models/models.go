package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Password  string    `json:"-" db:"password_hash"`
	Name      string    `json:"name" db:"name"`
	IsAdmin   bool      `json:"is_admin" db:"is_admin"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type APIKey struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	Key         string     `json:"key" db:"key_hash"`
	Name        string     `json:"name" db:"name"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

type Session struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
}

type UsageQuota struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	MaxGPUHours     float64   `json:"max_gpu_hours" db:"max_gpu_hours"`
	UsedGPUHours    float64   `json:"used_gpu_hours" db:"used_gpu_hours"`
	MaxRequests     int64     `json:"max_requests" db:"max_requests"`
	UsedRequests    int64     `json:"used_requests" db:"used_requests"`
	ResetAt         time.Time `json:"reset_at" db:"reset_at"`
	ResetInterval   string    `json:"reset_interval" db:"reset_interval"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type GPUUsage struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Provider    string    `json:"provider" db:"provider"`
	InstanceID  string    `json:"instance_id" db:"instance_id"`
	GPUModel    string    `json:"gpu_model" db:"gpu_model"`
	StartTime   time.Time `json:"start_time" db:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty" db:"end_time"`
	Duration    float64   `json:"duration" db:"duration"`
	Cost        float64   `json:"cost" db:"cost"`
	BillingID   *uuid.UUID `json:"billing_id,omitempty" db:"billing_id"`
}

type BillingTransaction struct {
	ID              uuid.UUID `json:"id" db:"id"`
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	Amount          float64   `json:"amount" db:"amount"`
	Currency        string    `json:"currency" db:"currency"`
	Status          string    `json:"status" db:"status"`
	PaymentMethod   string    `json:"payment_method" db:"payment_method"`
	PaymentProvider string    `json:"payment_provider" db:"payment_provider"`
	ExternalID      string    `json:"external_id,omitempty" db:"external_id"`
	Metadata        string    `json:"metadata,omitempty" db:"metadata"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type PaymentPreference struct {
	Type    string `json:"type"`
	Network string `json:"network,omitempty"`
	Wallet  string `json:"wallet,omitempty"`
	CardNum string `json:"card_num,omitempty"`
	Expiry  string `json:"expiry,omitempty"`
	CVV     string `json:"cvv,omitempty"`
}

type GPUInstance struct {
	ID               string            `json:"id"`
	Provider         string            `json:"provider"`
	GPUName          string            `json:"gpu_name"`
	GPUCount         int               `json:"gpu_count"`
	VRAM             int               `json:"vram_gb"`
	CPUCores         int               `json:"cpu_cores"`
	RAM              int               `json:"ram_gb"`
	Storage          int               `json:"storage_gb"`
	PricePerHour     float64           `json:"price_per_hour"`
	Location         string            `json:"location"`
	Available        bool              `json:"available"`
	Specifications   map[string]interface{} `json:"specifications,omitempty"`
}
