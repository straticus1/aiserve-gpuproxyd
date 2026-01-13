package gpu

import (
	"encoding/json"
	"fmt"
	"sort"
)

// UserGPUPreferences represents user's GPU selection preferences
type UserGPUPreferences struct {
	UserID string `json:"user_id"`

	// Priority-ordered list of preferred GPU groups
	PreferredGPUs []GPUPreference `json:"preferred_gpus"`

	// Vendor preferences (if no specific GPUs specified)
	PreferredVendors []GPUVendor `json:"preferred_vendors,omitempty"`

	// Tier preferences
	PreferredTiers []GPUTier `json:"preferred_tiers,omitempty"`

	// Constraints
	Constraints GPUConstraints `json:"constraints"`

	// Fallback behavior
	AllowFallback     bool `json:"allow_fallback"`      // Allow lower tier if preferred unavailable
	FallbackMaxPricePct float64 `json:"fallback_max_price_pct"` // Max % price increase for fallback (e.g., 150 = 1.5x)
}

// GPUPreference represents a single GPU preference with priority
type GPUPreference struct {
	Model    string  `json:"model"`     // e.g., "H100", "MI300X"
	Priority int     `json:"priority"`  // 1 = highest, higher numbers = lower priority
	MaxPrice float64 `json:"max_price"` // Max price per hour for this GPU
}

// GPUConstraints represents hard requirements
type GPUConstraints struct {
	MinVRAM         int      `json:"min_vram"`          // Minimum VRAM in GB
	MaxPricePerHour float64  `json:"max_price_per_hour"` // Max price per hour
	RequiredVendors []GPUVendor `json:"required_vendors,omitempty"` // Only these vendors
	ExcludedVendors []GPUVendor `json:"excluded_vendors,omitempty"` // Never these vendors
	RequiredTiers   []GPUTier   `json:"required_tiers,omitempty"`   // Only these tiers
	MinComputeCaps  string      `json:"min_compute_caps,omitempty"` // e.g., "8.0" for CUDA
}

// SelectionResult represents the result of GPU selection
type SelectionResult struct {
	GPU           GPUGroup `json:"gpu"`
	MatchReason   string   `json:"match_reason"`   // Why this GPU was selected
	IsFallback    bool     `json:"is_fallback"`    // Whether this is a fallback selection
	Priority      int      `json:"priority"`       // Priority level matched
	EstimatedCost float64  `json:"estimated_cost"` // Per hour cost
}

// DefaultPreferences returns sensible default preferences
func DefaultPreferences() *UserGPUPreferences {
	return &UserGPUPreferences{
		PreferredVendors: []GPUVendor{VendorNVIDIA, VendorAMD},
		PreferredTiers:   []GPUTier{TierHighEnd, TierMidRange},
		Constraints: GPUConstraints{
			MinVRAM:         8,
			MaxPricePerHour: 5.00,
		},
		AllowFallback:     true,
		FallbackMaxPricePct: 150.0, // Allow up to 50% price increase
	}
}

// SelectGPU selects the best GPU based on user preferences
func (prefs *UserGPUPreferences) SelectGPU(availableGPUs []GPUGroup) (*SelectionResult, error) {
	if len(availableGPUs) == 0 {
		return nil, fmt.Errorf("no GPUs available")
	}

	// Filter by hard constraints first
	filtered := prefs.applyConstraints(availableGPUs)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no GPUs match constraints")
	}

	// Try to match preferred GPUs by priority
	if len(prefs.PreferredGPUs) > 0 {
		if result := prefs.matchPreferredGPUs(filtered); result != nil {
			return result, nil
		}
	}

	// Try to match by vendor preference
	if len(prefs.PreferredVendors) > 0 {
		if result := prefs.matchPreferredVendors(filtered); result != nil {
			return result, nil
		}
	}

	// Try to match by tier preference
	if len(prefs.PreferredTiers) > 0 {
		if result := prefs.matchPreferredTiers(filtered); result != nil {
			return result, nil
		}
	}

	// Fallback: select cheapest GPU that meets constraints
	if prefs.AllowFallback {
		cheapest := prefs.selectCheapest(filtered)
		return &SelectionResult{
			GPU:           cheapest,
			MatchReason:   "fallback: cheapest available GPU",
			IsFallback:    true,
			Priority:      999,
			EstimatedCost: cheapest.PricePerHr,
		}, nil
	}

	return nil, fmt.Errorf("no GPU matches preferences and fallback disabled")
}

// applyConstraints filters GPUs by hard constraints
func (prefs *UserGPUPreferences) applyConstraints(gpus []GPUGroup) []GPUGroup {
	var filtered []GPUGroup

	for _, gpu := range gpus {
		// Check min VRAM
		if prefs.Constraints.MinVRAM > 0 && gpu.VRAM < prefs.Constraints.MinVRAM {
			continue
		}

		// Check max price
		if prefs.Constraints.MaxPricePerHour > 0 && gpu.PricePerHr > prefs.Constraints.MaxPricePerHour {
			continue
		}

		// Check required vendors
		if len(prefs.Constraints.RequiredVendors) > 0 {
			found := false
			for _, vendor := range prefs.Constraints.RequiredVendors {
				if gpu.Vendor == vendor {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check excluded vendors
		if len(prefs.Constraints.ExcludedVendors) > 0 {
			excluded := false
			for _, vendor := range prefs.Constraints.ExcludedVendors {
				if gpu.Vendor == vendor {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		// Check required tiers
		if len(prefs.Constraints.RequiredTiers) > 0 {
			found := false
			for _, tier := range prefs.Constraints.RequiredTiers {
				if gpu.Tier == tier {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, gpu)
	}

	return filtered
}

// matchPreferredGPUs tries to match against priority-ordered preferred GPUs
func (prefs *UserGPUPreferences) matchPreferredGPUs(gpus []GPUGroup) *SelectionResult {
	// Sort preferences by priority (lower number = higher priority)
	sortedPrefs := make([]GPUPreference, len(prefs.PreferredGPUs))
	copy(sortedPrefs, prefs.PreferredGPUs)
	sort.Slice(sortedPrefs, func(i, j int) bool {
		return sortedPrefs[i].Priority < sortedPrefs[j].Priority
	})

	for _, pref := range sortedPrefs {
		for _, gpu := range gpus {
			if gpu.Model == pref.Model {
				// Check max price for this specific GPU
				if pref.MaxPrice > 0 && gpu.PricePerHr > pref.MaxPrice {
					continue
				}

				return &SelectionResult{
					GPU:           gpu,
					MatchReason:   fmt.Sprintf("matched preferred GPU: %s (priority %d)", gpu.Model, pref.Priority),
					IsFallback:    false,
					Priority:      pref.Priority,
					EstimatedCost: gpu.PricePerHr,
				}
			}
		}
	}

	return nil
}

// matchPreferredVendors tries to match against vendor preferences
func (prefs *UserGPUPreferences) matchPreferredVendors(gpus []GPUGroup) *SelectionResult {
	for priority, vendor := range prefs.PreferredVendors {
		for _, gpu := range gpus {
			if gpu.Vendor == vendor {
				return &SelectionResult{
					GPU:           gpu,
					MatchReason:   fmt.Sprintf("matched preferred vendor: %s (priority %d)", vendor, priority+1),
					IsFallback:    false,
					Priority:      priority + 1,
					EstimatedCost: gpu.PricePerHr,
				}
			}
		}
	}

	return nil
}

// matchPreferredTiers tries to match against tier preferences
func (prefs *UserGPUPreferences) matchPreferredTiers(gpus []GPUGroup) *SelectionResult {
	for priority, tier := range prefs.PreferredTiers {
		// Find cheapest GPU in this tier
		var bestGPU *GPUGroup
		for i := range gpus {
			if gpus[i].Tier == tier {
				if bestGPU == nil || gpus[i].PricePerHr < bestGPU.PricePerHr {
					bestGPU = &gpus[i]
				}
			}
		}

		if bestGPU != nil {
			return &SelectionResult{
				GPU:           *bestGPU,
				MatchReason:   fmt.Sprintf("matched preferred tier: %s (priority %d)", tier, priority+1),
				IsFallback:    false,
				Priority:      priority + 1,
				EstimatedCost: bestGPU.PricePerHr,
			}
		}
	}

	return nil
}

// selectCheapest returns the cheapest GPU from the list
func (prefs *UserGPUPreferences) selectCheapest(gpus []GPUGroup) GPUGroup {
	cheapest := gpus[0]
	for _, gpu := range gpus[1:] {
		if gpu.PricePerHr < cheapest.PricePerHr {
			cheapest = gpu
		}
	}
	return cheapest
}

// ValidatePreferences checks if preferences are valid
func (prefs *UserGPUPreferences) ValidatePreferences() error {
	// Check for duplicate priorities
	priorities := make(map[int]bool)
	for _, pref := range prefs.PreferredGPUs {
		if pref.Priority < 1 {
			return fmt.Errorf("priority must be >= 1, got %d for GPU %s", pref.Priority, pref.Model)
		}
		if priorities[pref.Priority] {
			return fmt.Errorf("duplicate priority %d", pref.Priority)
		}
		priorities[pref.Priority] = true
	}

	// Check for conflicting constraints
	for _, required := range prefs.Constraints.RequiredVendors {
		for _, excluded := range prefs.Constraints.ExcludedVendors {
			if required == excluded {
				return fmt.Errorf("vendor %s is both required and excluded", required)
			}
		}
	}

	// Check for invalid fallback percentage
	if prefs.AllowFallback && prefs.FallbackMaxPricePct < 100 {
		return fmt.Errorf("fallback_max_price_pct must be >= 100 (100 = no increase), got %.1f", prefs.FallbackMaxPricePct)
	}

	return nil
}

// ToJSON serializes preferences to JSON
func (prefs *UserGPUPreferences) ToJSON() (string, error) {
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON deserializes preferences from JSON
func FromJSON(jsonStr string) (*UserGPUPreferences, error) {
	var prefs UserGPUPreferences
	if err := json.Unmarshal([]byte(jsonStr), &prefs); err != nil {
		return nil, err
	}
	return &prefs, prefs.ValidatePreferences()
}

// Example preference configurations
var ExamplePreferences = map[string]*UserGPUPreferences{
	"cost_optimized": {
		PreferredTiers: []GPUTier{TierBudget, TierMidRange},
		Constraints: GPUConstraints{
			MinVRAM:         8,
			MaxPricePerHour: 0.50,
		},
		AllowFallback:     true,
		FallbackMaxPricePct: 120.0,
	},
	"performance_focused": {
		PreferredGPUs: []GPUPreference{
			{Model: "H100", Priority: 1, MaxPrice: 3.50},
			{Model: "A100", Priority: 2, MaxPrice: 2.50},
			{Model: "V100", Priority: 3, MaxPrice: 1.50},
		},
		Constraints: GPUConstraints{
			MinVRAM: 40,
		},
		AllowFallback:     true,
		FallbackMaxPricePct: 200.0,
	},
	"nvidia_only": {
		PreferredVendors: []GPUVendor{VendorNVIDIA},
		Constraints: GPUConstraints{
			RequiredVendors: []GPUVendor{VendorNVIDIA},
			MinVRAM:         16,
			MaxPricePerHour: 2.00,
		},
		AllowFallback:     true,
		FallbackMaxPricePct: 150.0,
	},
	"amd_preferred": {
		PreferredVendors: []GPUVendor{VendorAMD, VendorNVIDIA},
		PreferredGPUs: []GPUPreference{
			{Model: "MI300X", Priority: 1, MaxPrice: 3.50},
			{Model: "MI250X", Priority: 2, MaxPrice: 2.80},
		},
		Constraints: GPUConstraints{
			MinVRAM:         32,
			MaxPricePerHour: 3.50,
		},
		AllowFallback:     true,
		FallbackMaxPricePct: 150.0,
	},
}
