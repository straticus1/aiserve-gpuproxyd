package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// QuotaLimits defines storage and upload limits for a user
type QuotaLimits struct {
	MaxStorageBytes     int64 // Maximum total storage in bytes
	MaxFileSize         int64 // Maximum individual file size in bytes
	MaxUploadsPerHour   int   // Maximum number of uploads per hour
	MaxUploadsPerDay    int   // Maximum number of uploads per day
}

// DefaultQuotaLimits returns sensible default quota limits
func DefaultQuotaLimits() QuotaLimits {
	return QuotaLimits{
		MaxStorageBytes:   100 * 1024 * 1024 * 1024, // 100GB
		MaxFileSize:       10 * 1024 * 1024 * 1024,  // 10GB per file
		MaxUploadsPerHour: 50,
		MaxUploadsPerDay:  500,
	}
}

// PremiumQuotaLimits returns premium tier quota limits
func PremiumQuotaLimits() QuotaLimits {
	return QuotaLimits{
		MaxStorageBytes:   1024 * 1024 * 1024 * 1024, // 1TB
		MaxFileSize:       100 * 1024 * 1024 * 1024,  // 100GB per file
		MaxUploadsPerHour: 500,
		MaxUploadsPerDay:  5000,
	}
}

// UserQuota tracks current usage for a user
type UserQuota struct {
	UserID              uuid.UUID
	CurrentStorageBytes int64
	UploadsLastHour     []time.Time
	UploadsLastDay      []time.Time
	Limits              QuotaLimits
	mu                  sync.RWMutex
}

// QuotaManager manages storage quotas and rate limiting for users
type QuotaManager struct {
	mu     sync.RWMutex
	quotas map[string]*UserQuota // userID -> quota
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager() *QuotaManager {
	qm := &QuotaManager{
		quotas: make(map[string]*UserQuota),
	}

	// Start cleanup goroutine to prune old upload timestamps
	go qm.cleanupLoop()

	return qm
}

// GetOrCreateQuota gets or creates a quota for a user
func (qm *QuotaManager) GetOrCreateQuota(userID uuid.UUID, limits QuotaLimits) *UserQuota {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	userIDStr := userID.String()
	if quota, exists := qm.quotas[userIDStr]; exists {
		return quota
	}

	quota := &UserQuota{
		UserID:          userID,
		UploadsLastHour: make([]time.Time, 0),
		UploadsLastDay:  make([]time.Time, 0),
		Limits:          limits,
	}

	qm.quotas[userIDStr] = quota
	return quota
}

// CheckUploadAllowed checks if a user can upload a file of given size
func (qm *QuotaManager) CheckUploadAllowed(ctx context.Context, userID uuid.UUID, fileSize int64) error {
	quota := qm.GetOrCreateQuota(userID, DefaultQuotaLimits())

	quota.mu.Lock()
	defer quota.mu.Unlock()

	// Check file size limit
	if fileSize > quota.Limits.MaxFileSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed %d bytes", fileSize, quota.Limits.MaxFileSize)
	}

	// Check total storage limit
	if quota.CurrentStorageBytes+fileSize > quota.Limits.MaxStorageBytes {
		return fmt.Errorf("storage quota exceeded: current %d + upload %d > limit %d bytes",
			quota.CurrentStorageBytes, fileSize, quota.Limits.MaxStorageBytes)
	}

	// Clean up old upload timestamps
	now := time.Now()
	quota.UploadsLastHour = filterTimestamps(quota.UploadsLastHour, now.Add(-1*time.Hour))
	quota.UploadsLastDay = filterTimestamps(quota.UploadsLastDay, now.Add(-24*time.Hour))

	// Check hourly rate limit
	if len(quota.UploadsLastHour) >= quota.Limits.MaxUploadsPerHour {
		return fmt.Errorf("hourly upload limit reached: %d/%d uploads",
			len(quota.UploadsLastHour), quota.Limits.MaxUploadsPerHour)
	}

	// Check daily rate limit
	if len(quota.UploadsLastDay) >= quota.Limits.MaxUploadsPerDay {
		return fmt.Errorf("daily upload limit reached: %d/%d uploads",
			len(quota.UploadsLastDay), quota.Limits.MaxUploadsPerDay)
	}

	return nil
}

// RecordUpload records a successful upload
func (qm *QuotaManager) RecordUpload(userID uuid.UUID, fileSize int64) {
	quota := qm.GetOrCreateQuota(userID, DefaultQuotaLimits())

	quota.mu.Lock()
	defer quota.mu.Unlock()

	now := time.Now()
	quota.CurrentStorageBytes += fileSize
	quota.UploadsLastHour = append(quota.UploadsLastHour, now)
	quota.UploadsLastDay = append(quota.UploadsLastDay, now)
}

// RecordDeletion records a file deletion
func (qm *QuotaManager) RecordDeletion(userID uuid.UUID, fileSize int64) {
	quota := qm.GetOrCreateQuota(userID, DefaultQuotaLimits())

	quota.mu.Lock()
	defer quota.mu.Unlock()

	quota.CurrentStorageBytes -= fileSize
	if quota.CurrentStorageBytes < 0 {
		quota.CurrentStorageBytes = 0
	}
}

// GetQuotaInfo returns current quota information for a user
func (qm *QuotaManager) GetQuotaInfo(userID uuid.UUID) map[string]interface{} {
	quota := qm.GetOrCreateQuota(userID, DefaultQuotaLimits())

	quota.mu.RLock()
	defer quota.mu.RUnlock()

	// Clean up timestamps for accurate count
	now := time.Now()
	uploadsLastHour := len(filterTimestamps(quota.UploadsLastHour, now.Add(-1*time.Hour)))
	uploadsLastDay := len(filterTimestamps(quota.UploadsLastDay, now.Add(-24*time.Hour)))

	return map[string]interface{}{
		"user_id": userID.String(),
		"storage": map[string]interface{}{
			"used_bytes":  quota.CurrentStorageBytes,
			"limit_bytes": quota.Limits.MaxStorageBytes,
			"used_pct":    float64(quota.CurrentStorageBytes) / float64(quota.Limits.MaxStorageBytes) * 100,
		},
		"file_size": map[string]interface{}{
			"max_bytes": quota.Limits.MaxFileSize,
		},
		"rate_limits": map[string]interface{}{
			"uploads_last_hour": uploadsLastHour,
			"hourly_limit":      quota.Limits.MaxUploadsPerHour,
			"uploads_last_day":  uploadsLastDay,
			"daily_limit":       quota.Limits.MaxUploadsPerDay,
		},
	}
}

// SetUserLimits updates quota limits for a specific user
func (qm *QuotaManager) SetUserLimits(userID uuid.UUID, limits QuotaLimits) {
	quota := qm.GetOrCreateQuota(userID, limits)

	quota.mu.Lock()
	defer quota.mu.Unlock()

	quota.Limits = limits
}

// cleanupLoop periodically cleans up old upload timestamps
func (qm *QuotaManager) cleanupLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		qm.mu.RLock()
		userIDs := make([]string, 0, len(qm.quotas))
		for userID := range qm.quotas {
			userIDs = append(userIDs, userID)
		}
		qm.mu.RUnlock()

		now := time.Now()
		for _, userID := range userIDs {
			qm.mu.RLock()
			quota, exists := qm.quotas[userID]
			qm.mu.RUnlock()

			if !exists {
				continue
			}

			quota.mu.Lock()
			quota.UploadsLastHour = filterTimestamps(quota.UploadsLastHour, now.Add(-1*time.Hour))
			quota.UploadsLastDay = filterTimestamps(quota.UploadsLastDay, now.Add(-24*time.Hour))
			quota.mu.Unlock()
		}
	}
}

// filterTimestamps removes timestamps older than cutoff
func filterTimestamps(timestamps []time.Time, cutoff time.Time) []time.Time {
	result := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			result = append(result, ts)
		}
	}
	return result
}

// Global quota manager instance
var globalQuotaManager *QuotaManager
var quotaManagerOnce sync.Once

// GetQuotaManager returns the global quota manager instance
func GetQuotaManager() *QuotaManager {
	quotaManagerOnce.Do(func() {
		globalQuotaManager = NewQuotaManager()
	})
	return globalQuotaManager
}
