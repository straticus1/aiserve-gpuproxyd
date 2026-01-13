package gpu

import (
	"strings"
)

// GPUVendor represents the GPU manufacturer
type GPUVendor string

const (
	VendorNVIDIA GPUVendor = "NVIDIA"
	VendorAMD    GPUVendor = "AMD"
	VendorIntel  GPUVendor = "Intel"
	VendorApple  GPUVendor = "Apple"
)

// GPUTier represents performance/pricing tier
type GPUTier string

const (
	TierEnterprise GPUTier = "enterprise" // H100, H200, MI300X
	TierHighEnd    GPUTier = "high_end"   // A100, V100, MI250X
	TierMidRange   GPUTier = "mid_range"  // RTX 4090, RTX 3090, MI210
	TierBudget     GPUTier = "budget"     // RTX 3060, RX 6600
	TierUnknown    GPUTier = "unknown"
)

// GPUGroup represents a classification of GPU by vendor and model
type GPUGroup struct {
	Vendor      GPUVendor
	Model       string
	Tier        GPUTier
	VRAM        int     // GB
	ComputeCaps string  // CUDA compute capability or equivalent
	PricePerHr  float64 // USD per hour (estimated)
}

// GPU group definitions
var GPUGroups = map[string]GPUGroup{
	// NVIDIA Enterprise
	"H100": {
		Vendor:      VendorNVIDIA,
		Model:       "H100",
		Tier:        TierEnterprise,
		VRAM:        80,
		ComputeCaps: "9.0",
		PricePerHr:  3.00,
	},
	"H200": {
		Vendor:      VendorNVIDIA,
		Model:       "H200",
		Tier:        TierEnterprise,
		VRAM:        141,
		ComputeCaps: "9.0",
		PricePerHr:  3.50,
	},
	"A100": {
		Vendor:      VendorNVIDIA,
		Model:       "A100",
		Tier:        TierHighEnd,
		VRAM:        80,
		ComputeCaps: "8.0",
		PricePerHr:  2.00,
	},
	"A100-40GB": {
		Vendor:      VendorNVIDIA,
		Model:       "A100-40GB",
		Tier:        TierHighEnd,
		VRAM:        40,
		ComputeCaps: "8.0",
		PricePerHr:  1.50,
	},

	// NVIDIA High-End
	"V100": {
		Vendor:      VendorNVIDIA,
		Model:       "V100",
		Tier:        TierHighEnd,
		VRAM:        32,
		ComputeCaps: "7.0",
		PricePerHr:  1.20,
	},
	"Tesla V100": {
		Vendor:      VendorNVIDIA,
		Model:       "Tesla V100",
		Tier:        TierHighEnd,
		VRAM:        32,
		ComputeCaps: "7.0",
		PricePerHr:  1.20,
	},
	"P100": {
		Vendor:      VendorNVIDIA,
		Model:       "P100",
		Tier:        TierHighEnd,
		VRAM:        16,
		ComputeCaps: "6.0",
		PricePerHr:  0.80,
	},

	// NVIDIA Mid-Range
	"RTX 4090": {
		Vendor:      VendorNVIDIA,
		Model:       "RTX 4090",
		Tier:        TierMidRange,
		VRAM:        24,
		ComputeCaps: "8.9",
		PricePerHr:  0.80,
	},
	"RTX 3090": {
		Vendor:      VendorNVIDIA,
		Model:       "RTX 3090",
		Tier:        TierMidRange,
		VRAM:        24,
		ComputeCaps: "8.6",
		PricePerHr:  0.60,
	},
	"RTX 3080": {
		Vendor:      VendorNVIDIA,
		Model:       "RTX 3080",
		Tier:        TierMidRange,
		VRAM:        10,
		ComputeCaps: "8.6",
		PricePerHr:  0.40,
	},

	// NVIDIA Budget
	"RTX 3060": {
		Vendor:      VendorNVIDIA,
		Model:       "RTX 3060",
		Tier:        TierBudget,
		VRAM:        12,
		ComputeCaps: "8.6",
		PricePerHr:  0.25,
	},
	"GTX 1080 Ti": {
		Vendor:      VendorNVIDIA,
		Model:       "GTX 1080 Ti",
		Tier:        TierBudget,
		VRAM:        11,
		ComputeCaps: "6.1",
		PricePerHr:  0.20,
	},

	// AMD Enterprise
	"MI300X": {
		Vendor:      VendorAMD,
		Model:       "MI300X",
		Tier:        TierEnterprise,
		VRAM:        192,
		ComputeCaps: "gfx942",
		PricePerHr:  3.20,
	},
	"MI250X": {
		Vendor:      VendorAMD,
		Model:       "MI250X",
		Tier:        TierHighEnd,
		VRAM:        128,
		ComputeCaps: "gfx90a",
		PricePerHr:  2.50,
	},
	"MI210": {
		Vendor:      VendorAMD,
		Model:       "MI210",
		Tier:        TierMidRange,
		VRAM:        64,
		ComputeCaps: "gfx90a",
		PricePerHr:  1.80,
	},

	// AMD Consumer
	"RX 7900 XTX": {
		Vendor:      VendorAMD,
		Model:       "RX 7900 XTX",
		Tier:        TierMidRange,
		VRAM:        24,
		ComputeCaps: "gfx1100",
		PricePerHr:  0.50,
	},
	"RX 6900 XT": {
		Vendor:      VendorAMD,
		Model:       "RX 6900 XT",
		Tier:        TierMidRange,
		VRAM:        16,
		ComputeCaps: "gfx1030",
		PricePerHr:  0.40,
	},
	"RX 6600": {
		Vendor:      VendorAMD,
		Model:       "RX 6600",
		Tier:        TierBudget,
		VRAM:        8,
		ComputeCaps: "gfx1032",
		PricePerHr:  0.20,
	},

	// Intel
	"Max 1550": {
		Vendor:      VendorIntel,
		Model:       "Max 1550",
		Tier:        TierHighEnd,
		VRAM:        128,
		ComputeCaps: "PVC",
		PricePerHr:  2.00,
	},
	"Arc A770": {
		Vendor:      VendorIntel,
		Model:       "Arc A770",
		Tier:        TierMidRange,
		VRAM:        16,
		ComputeCaps: "DG2",
		PricePerHr:  0.30,
	},

	// Apple Silicon
	"M1 Ultra": {
		Vendor:      VendorApple,
		Model:       "M1 Ultra",
		Tier:        TierHighEnd,
		VRAM:        128,
		ComputeCaps: "Metal 3",
		PricePerHr:  1.00,
	},
	"M2 Ultra": {
		Vendor:      VendorApple,
		Model:       "M2 Ultra",
		Tier:        TierHighEnd,
		VRAM:        192,
		ComputeCaps: "Metal 3",
		PricePerHr:  1.20,
	},
	"M3 Max": {
		Vendor:      VendorApple,
		Model:       "M3 Max",
		Tier:        TierMidRange,
		VRAM:        128,
		ComputeCaps: "Metal 3",
		PricePerHr:  0.80,
	},
}

// ClassifyGPU attempts to classify a GPU by its name
func ClassifyGPU(name string) (GPUGroup, bool) {
	name = strings.TrimSpace(name)

	// Try exact match first
	if group, ok := GPUGroups[name]; ok {
		return group, true
	}

	// Try case-insensitive match
	nameLower := strings.ToLower(name)
	for key, group := range GPUGroups {
		if strings.ToLower(key) == nameLower {
			return group, true
		}
	}

	// Try partial match (contains)
	for key, group := range GPUGroups {
		if strings.Contains(nameLower, strings.ToLower(key)) {
			return group, true
		}
	}

	// Return unknown GPU
	return GPUGroup{
		Vendor: detectVendor(name),
		Model:  name,
		Tier:   TierUnknown,
	}, false
}

// detectVendor tries to determine vendor from GPU name
func detectVendor(name string) GPUVendor {
	nameLower := strings.ToLower(name)

	if strings.Contains(nameLower, "nvidia") ||
	   strings.Contains(nameLower, "tesla") ||
	   strings.Contains(nameLower, "rtx") ||
	   strings.Contains(nameLower, "gtx") ||
	   strings.HasPrefix(nameLower, "a") && (strings.Contains(nameLower, "100") || strings.Contains(nameLower, "40")) ||
	   strings.HasPrefix(nameLower, "h") && (strings.Contains(nameLower, "100") || strings.Contains(nameLower, "200")) ||
	   strings.HasPrefix(nameLower, "v100") ||
	   strings.HasPrefix(nameLower, "p100") {
		return VendorNVIDIA
	}

	if strings.Contains(nameLower, "amd") ||
	   strings.Contains(nameLower, "radeon") ||
	   strings.HasPrefix(nameLower, "rx") ||
	   strings.HasPrefix(nameLower, "mi") {
		return VendorAMD
	}

	if strings.Contains(nameLower, "intel") ||
	   strings.Contains(nameLower, "arc") ||
	   strings.Contains(nameLower, "max") {
		return VendorIntel
	}

	if strings.Contains(nameLower, "apple") ||
	   strings.HasPrefix(nameLower, "m1") ||
	   strings.HasPrefix(nameLower, "m2") ||
	   strings.HasPrefix(nameLower, "m3") {
		return VendorApple
	}

	return GPUVendor("Unknown")
}

// GetGPUsByVendor returns all GPU models for a specific vendor
func GetGPUsByVendor(vendor GPUVendor) []GPUGroup {
	var gpus []GPUGroup
	for _, group := range GPUGroups {
		if group.Vendor == vendor {
			gpus = append(gpus, group)
		}
	}
	return gpus
}

// GetGPUsByTier returns all GPU models for a specific tier
func GetGPUsByTier(tier GPUTier) []GPUGroup {
	var gpus []GPUGroup
	for _, group := range GPUGroups {
		if group.Tier == tier {
			gpus = append(gpus, group)
		}
	}
	return gpus
}

// GetGPUsByVendorAndTier returns GPU models matching both vendor and tier
func GetGPUsByVendorAndTier(vendor GPUVendor, tier GPUTier) []GPUGroup {
	var gpus []GPUGroup
	for _, group := range GPUGroups {
		if group.Vendor == vendor && group.Tier == tier {
			gpus = append(gpus, group)
		}
	}
	return gpus
}

// FilterGPUs filters GPUs by vendor, tier, min VRAM, and max price
type GPUFilter struct {
	Vendor      *GPUVendor
	Tier        *GPUTier
	MinVRAM     int
	MaxPriceHr  float64
	ComputeCaps string
}

// FilterGPUs returns GPUs matching the filter criteria
func FilterGPUs(filter GPUFilter) []GPUGroup {
	var gpus []GPUGroup

	for _, group := range GPUGroups {
		// Check vendor
		if filter.Vendor != nil && group.Vendor != *filter.Vendor {
			continue
		}

		// Check tier
		if filter.Tier != nil && group.Tier != *filter.Tier {
			continue
		}

		// Check min VRAM
		if filter.MinVRAM > 0 && group.VRAM < filter.MinVRAM {
			continue
		}

		// Check max price
		if filter.MaxPriceHr > 0 && group.PricePerHr > filter.MaxPriceHr {
			continue
		}

		// Check compute capability
		if filter.ComputeCaps != "" && group.ComputeCaps != filter.ComputeCaps {
			continue
		}

		gpus = append(gpus, group)
	}

	return gpus
}

// GetEstimatedCost calculates estimated cost for a GPU group
func (g *GPUGroup) GetEstimatedCost(hours float64) float64 {
	return g.PricePerHr * hours
}

// IsEnterprise returns true if GPU is enterprise tier
func (g *GPUGroup) IsEnterprise() bool {
	return g.Tier == TierEnterprise
}

// SupportsModel returns true if GPU has enough VRAM for a model
func (g *GPUGroup) SupportsModel(modelSizeGB int) bool {
	// Reserve 2GB for system overhead
	return g.VRAM >= (modelSizeGB + 2)
}
