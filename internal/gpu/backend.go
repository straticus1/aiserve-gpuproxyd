package gpu

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// BackendType represents the type of GPU backend
type BackendType string

const (
	BackendCUDA   BackendType = "cuda"
	BackendROCm   BackendType = "rocm"
	BackendOneAPI BackendType = "oneapi"
	BackendNone   BackendType = "none"
)

// Backend represents a detected GPU backend
type Backend struct {
	Type      BackendType
	Version   string
	Available bool
	Devices   int
	Error     string
}

// DetectBackends detects all available GPU backends on the system
func DetectBackends() []Backend {
	backends := []Backend{
		detectCUDA(),
		detectROCm(),
		detectOneAPI(),
	}

	availableCount := 0
	for _, b := range backends {
		if b.Available {
			availableCount++
			log.Printf("Detected %s backend: version=%s, devices=%d", b.Type, b.Version, b.Devices)
		}
	}

	if availableCount == 0 {
		log.Println("No local GPU backends detected. Running in API-only mode.")
	}

	return backends
}

// GetAvailableBackend returns the first available backend, or BackendNone
func GetAvailableBackend(backends []Backend) BackendType {
	for _, b := range backends {
		if b.Available {
			return b.Type
		}
	}
	return BackendNone
}

// detectCUDA detects NVIDIA CUDA installation
func detectCUDA() Backend {
	backend := Backend{Type: BackendCUDA}

	// Check for nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=count", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		backend.Error = "nvidia-smi not found"
		return backend
	}

	// Count devices
	deviceCount := 0
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	deviceCount = len(lines)

	// Get CUDA version
	cmd = exec.Command("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader")
	versionOutput, err := cmd.Output()
	if err == nil {
		backend.Version = strings.TrimSpace(string(versionOutput))
		if len(lines) > 0 {
			backend.Version = strings.Split(backend.Version, "\n")[0]
		}
	}

	// Check for CUDA toolkit
	if _, err := os.Stat("/usr/local/cuda"); err == nil {
		backend.Available = true
		backend.Devices = deviceCount
	} else if cudaPath := os.Getenv("CUDA_PATH"); cudaPath != "" {
		backend.Available = true
		backend.Devices = deviceCount
	} else {
		backend.Error = "CUDA toolkit not found"
	}

	return backend
}

// detectROCm detects AMD ROCm installation
func detectROCm() Backend {
	backend := Backend{Type: BackendROCm}

	// Check for rocm-smi
	cmd := exec.Command("rocm-smi", "--showproductname")
	output, err := cmd.Output()
	if err != nil {
		backend.Error = "rocm-smi not found"
		return backend
	}

	// Count devices
	deviceCount := 0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "GPU[") {
			deviceCount++
		}
	}

	// Get ROCm version
	if data, err := os.ReadFile("/opt/rocm/.info/version"); err == nil {
		backend.Version = strings.TrimSpace(string(data))
		backend.Available = true
		backend.Devices = deviceCount
	} else if rocmPath := os.Getenv("ROCM_PATH"); rocmPath != "" {
		backend.Available = true
		backend.Devices = deviceCount
		backend.Version = "unknown"
	} else {
		backend.Error = "ROCm installation not found"
	}

	return backend
}

// detectOneAPI detects Intel OneAPI installation
func detectOneAPI() Backend {
	backend := Backend{Type: BackendOneAPI}

	// Check for sycl-ls (OneAPI Level Zero)
	cmd := exec.Command("sycl-ls")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative: clinfo
		cmd = exec.Command("clinfo")
		output, err = cmd.Output()
		if err != nil {
			backend.Error = "sycl-ls and clinfo not found"
			return backend
		}
	}

	// Count Intel GPU devices
	deviceCount := 0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "intel") &&
			(strings.Contains(strings.ToLower(line), "gpu") ||
				strings.Contains(strings.ToLower(line), "graphics")) {
			deviceCount++
		}
	}

	// Check for OneAPI installation
	oneapiPath := os.Getenv("ONEAPI_ROOT")
	if oneapiPath == "" {
		oneapiPath = "/opt/intel/oneapi"
	}

	if _, err := os.Stat(oneapiPath); err == nil {
		backend.Available = true
		backend.Devices = deviceCount
		backend.Version = "installed"

		// Try to get version from setvars.sh
		if data, err := os.ReadFile(oneapiPath + "/version.txt"); err == nil {
			backend.Version = strings.TrimSpace(string(data))
		}
	} else {
		backend.Error = "OneAPI installation not found"
	}

	return backend
}

// GetBackendInfo returns a formatted string with backend information
func GetBackendInfo(backends []Backend) string {
	var info []string
	for _, b := range backends {
		if b.Available {
			info = append(info, fmt.Sprintf("%s (v%s, %d devices)", b.Type, b.Version, b.Devices))
		}
	}
	if len(info) == 0 {
		return "No local GPU backends available"
	}
	return strings.Join(info, ", ")
}
