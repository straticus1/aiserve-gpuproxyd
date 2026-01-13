package compute

import (
	"fmt"
	"sync"
)

// PortAllocator manages port allocation for compute resources
// Port ranges:
// - 2000-2500: OpenRouter models (500 ports)
// - 3000-15000: Custom models (12,000 ports)
type PortAllocator struct {
	mu sync.Mutex

	minPort      int
	maxPort      int
	allocatedPorts map[int]bool  // port -> allocated
	portQueue    []int          // Available ports
}

// NewPortAllocator creates a new port allocator
func NewPortAllocator(minPort, maxPort int) *PortAllocator {
	pa := &PortAllocator{
		minPort:        minPort,
		maxPort:        maxPort,
		allocatedPorts: make(map[int]bool),
		portQueue:      make([]int, 0, maxPort-minPort+1),
	}

	// Initialize port queue
	for port := minPort; port <= maxPort; port++ {
		pa.portQueue = append(pa.portQueue, port)
	}

	return pa
}

// Allocate assigns an available port
func (pa *PortAllocator) Allocate() (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	if len(pa.portQueue) == 0 {
		return 0, fmt.Errorf("no available ports (range: %d-%d)", pa.minPort, pa.maxPort)
	}

	// Get first available port
	port := pa.portQueue[0]
	pa.portQueue = pa.portQueue[1:]
	pa.allocatedPorts[port] = true

	return port, nil
}

// AllocateInRange assigns a port within a specific range
func (pa *PortAllocator) AllocateInRange(minPort, maxPort int) (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	// Find first available port in range
	for i, port := range pa.portQueue {
		if port >= minPort && port <= maxPort {
			// Remove from queue
			pa.portQueue = append(pa.portQueue[:i], pa.portQueue[i+1:]...)
			pa.allocatedPorts[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", minPort, maxPort)
}

// Free releases an allocated port
func (pa *PortAllocator) Free(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	if !pa.allocatedPorts[port] {
		return // Port not allocated
	}

	delete(pa.allocatedPorts, port)
	pa.portQueue = append(pa.portQueue, port)
}

// IsAllocated checks if a port is allocated
func (pa *PortAllocator) IsAllocated(port int) bool {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	return pa.allocatedPorts[port]
}

// GetAvailableCount returns the number of available ports
func (pa *PortAllocator) GetAvailableCount() int {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	return len(pa.portQueue)
}

// GetAllocatedCount returns the number of allocated ports
func (pa *PortAllocator) GetAllocatedCount() int {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	return len(pa.allocatedPorts)
}

// AllocateOpenRouterPort allocates a port in the OpenRouter range (2000-2500)
func (pa *PortAllocator) AllocateOpenRouterPort() (int, error) {
	return pa.AllocateInRange(2000, 2500)
}

// AllocateCustomModelPort allocates a port in the custom model range (3000-15000)
func (pa *PortAllocator) AllocateCustomModelPort() (int, error) {
	return pa.AllocateInRange(3000, 15000)
}
