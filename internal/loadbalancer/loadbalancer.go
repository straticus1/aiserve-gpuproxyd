package loadbalancer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aiserve/gpuproxy/internal/models"
)

type Strategy string

const (
	StrategyRoundRobin         Strategy = "round_robin"
	StrategyEqualWeighted      Strategy = "equal_weighted"
	StrategyWeightedRoundRobin Strategy = "weighted_round_robin"
	StrategyLeastConnections   Strategy = "least_connections"
	StrategyLeastResponseTime  Strategy = "least_response_time"
)

type LoadBalancer interface {
	SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error)
	RecordConnection(instanceID string)
	RecordDisconnection(instanceID string)
	RecordResponseTime(instanceID string, duration time.Duration)
	GetLoad(instanceID string) *InstanceLoad
	GetAllLoads() map[string]*InstanceLoad
}

type InstanceLoad struct {
	InstanceID       string        `json:"instance_id"`
	Provider         string        `json:"provider"`
	ActiveConnections int          `json:"active_connections"`
	TotalConnections int64         `json:"total_connections"`
	AvgResponseTime  time.Duration `json:"avg_response_time"`
	LastResponseTime time.Duration `json:"last_response_time"`
	Weight           float64       `json:"weight"`
	LastUsed         time.Time     `json:"last_used"`
}

type BaseLoadBalancer struct {
	strategy  Strategy
	loads     map[string]*InstanceLoad
	mu        sync.RWMutex
	roundRobinIndex int
}

func NewLoadBalancer(strategy Strategy) LoadBalancer {
	switch strategy {
	case StrategyRoundRobin:
		return &RoundRobinLB{BaseLoadBalancer: newBase(strategy)}
	case StrategyEqualWeighted:
		return &EqualWeightedLB{BaseLoadBalancer: newBase(strategy)}
	case StrategyWeightedRoundRobin:
		return &WeightedRoundRobinLB{BaseLoadBalancer: newBase(strategy)}
	case StrategyLeastConnections:
		return &LeastConnectionsLB{BaseLoadBalancer: newBase(strategy)}
	case StrategyLeastResponseTime:
		return &LeastResponseTimeLB{BaseLoadBalancer: newBase(strategy)}
	default:
		return &RoundRobinLB{BaseLoadBalancer: newBase(StrategyRoundRobin)}
	}
}

func newBase(strategy Strategy) *BaseLoadBalancer {
	return &BaseLoadBalancer{
		strategy: strategy,
		loads:    make(map[string]*InstanceLoad),
	}
}

func (lb *BaseLoadBalancer) ensureLoad(instanceID, provider string) *InstanceLoad {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	return lb.ensureLoadUnsafe(instanceID, provider)
}

// ensureLoadUnsafe must be called while holding the lock
func (lb *BaseLoadBalancer) ensureLoadUnsafe(instanceID, provider string) *InstanceLoad {
	if load, exists := lb.loads[instanceID]; exists {
		return load
	}

	load := &InstanceLoad{
		InstanceID:       instanceID,
		Provider:         provider,
		ActiveConnections: 0,
		TotalConnections:  0,
		Weight:           1.0,
		LastUsed:         time.Now(),
	}
	lb.loads[instanceID] = load
	return load
}

func (lb *BaseLoadBalancer) RecordConnection(instanceID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if load, exists := lb.loads[instanceID]; exists {
		load.ActiveConnections++
		load.TotalConnections++
		load.LastUsed = time.Now()
	}
}

func (lb *BaseLoadBalancer) RecordDisconnection(instanceID string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if load, exists := lb.loads[instanceID]; exists {
		if load.ActiveConnections > 0 {
			load.ActiveConnections--
		}
	}
}

func (lb *BaseLoadBalancer) RecordResponseTime(instanceID string, duration time.Duration) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if load, exists := lb.loads[instanceID]; exists {
		if load.AvgResponseTime == 0 {
			load.AvgResponseTime = duration
		} else {
			load.AvgResponseTime = (load.AvgResponseTime + duration) / 2
		}
		load.LastResponseTime = duration
	}
}

func (lb *BaseLoadBalancer) GetLoad(instanceID string) *InstanceLoad {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if load, exists := lb.loads[instanceID]; exists {
		return load
	}
	return nil
}

func (lb *BaseLoadBalancer) GetAllLoads() map[string]*InstanceLoad {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	loads := make(map[string]*InstanceLoad)
	for id, load := range lb.loads {
		loadCopy := *load
		loads[id] = &loadCopy
	}
	return loads
}

type RoundRobinLB struct {
	*BaseLoadBalancer
}

func (lb *RoundRobinLB) SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	index := lb.roundRobinIndex % len(instances)
	lb.roundRobinIndex++

	selected := &instances[index]
	lb.ensureLoadUnsafe(selected.ID, selected.Provider)
	return selected, nil
}

type EqualWeightedLB struct {
	*BaseLoadBalancer
}

func (lb *EqualWeightedLB) SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	minLoad := int64(1<<63 - 1)
	var selected *models.GPUInstance

	for i := range instances {
		load := lb.ensureLoadUnsafe(instances[i].ID, instances[i].Provider)
		if load.TotalConnections < minLoad {
			minLoad = load.TotalConnections
			selected = &instances[i]
		}
	}

	if selected == nil {
		selected = &instances[0]
	}

	return selected, nil
}

type WeightedRoundRobinLB struct {
	*BaseLoadBalancer
	currentWeight float64
}

func (lb *WeightedRoundRobinLB) SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	var selected *models.GPUInstance
	maxWeight := -1.0

	for i := range instances {
		load := lb.ensureLoadUnsafe(instances[i].ID, instances[i].Provider)

		weight := lb.calculateWeight(&instances[i])
		load.Weight = weight

		if weight > maxWeight {
			maxWeight = weight
			selected = &instances[i]
		}
	}

	if selected == nil {
		selected = &instances[0]
	}

	return selected, nil
}

func (lb *WeightedRoundRobinLB) calculateWeight(instance *models.GPUInstance) float64 {
	baseWeight := 1.0

	if instance.VRAM >= 80 {
		baseWeight = 3.0
	} else if instance.VRAM >= 40 {
		baseWeight = 2.0
	} else if instance.VRAM >= 24 {
		baseWeight = 1.5
	}

	if instance.PricePerHour < 1.0 {
		baseWeight *= 1.2
	}

	return baseWeight
}

type LeastConnectionsLB struct {
	*BaseLoadBalancer
}

func (lb *LeastConnectionsLB) SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	minConnections := int(1<<31 - 1)
	var selected *models.GPUInstance

	for i := range instances {
		load := lb.ensureLoadUnsafe(instances[i].ID, instances[i].Provider)
		if load.ActiveConnections < minConnections {
			minConnections = load.ActiveConnections
			selected = &instances[i]
		}
	}

	if selected == nil {
		selected = &instances[0]
	}

	return selected, nil
}

type LeastResponseTimeLB struct {
	*BaseLoadBalancer
}

func (lb *LeastResponseTimeLB) SelectInstance(instances []models.GPUInstance) (*models.GPUInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	minResponseTime := time.Duration(1<<63 - 1)
	var selected *models.GPUInstance

	for i := range instances {
		load := lb.ensureLoadUnsafe(instances[i].ID, instances[i].Provider)

		if load.AvgResponseTime == 0 {
			selected = &instances[i]
			break
		}

		if load.AvgResponseTime < minResponseTime {
			minResponseTime = load.AvgResponseTime
			selected = &instances[i]
		}
	}

	if selected == nil {
		selected = &instances[0]
	}

	return selected, nil
}

type LoadBalancerService struct {
	balancer LoadBalancer
	strategy Strategy
}

func NewLoadBalancerService(strategy Strategy) *LoadBalancerService {
	return &LoadBalancerService{
		balancer: NewLoadBalancer(strategy),
		strategy: strategy,
	}
}

func (s *LoadBalancerService) SelectInstance(ctx context.Context, instances []models.GPUInstance) (*models.GPUInstance, error) {
	return s.balancer.SelectInstance(instances)
}

func (s *LoadBalancerService) TrackConnection(instanceID string) {
	s.balancer.RecordConnection(instanceID)
}

func (s *LoadBalancerService) TrackDisconnection(instanceID string) {
	s.balancer.RecordDisconnection(instanceID)
}

func (s *LoadBalancerService) TrackResponseTime(instanceID string, duration time.Duration) {
	s.balancer.RecordResponseTime(instanceID, duration)
}

func (s *LoadBalancerService) GetInstanceLoad(instanceID string) *InstanceLoad {
	return s.balancer.GetLoad(instanceID)
}

func (s *LoadBalancerService) GetAllLoads() map[string]*InstanceLoad {
	return s.balancer.GetAllLoads()
}

func (s *LoadBalancerService) GetStrategy() Strategy {
	return s.strategy
}

func (s *LoadBalancerService) SetStrategy(strategy Strategy) {
	s.strategy = strategy
	s.balancer = NewLoadBalancer(strategy)
}
