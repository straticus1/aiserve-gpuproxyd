package gpu

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/aiserve/gpuproxy/pkg/ionet"
	"github.com/aiserve/gpuproxy/pkg/vastai"
)

type Provider string

const (
	ProviderVastAI Provider = "vast.ai"
	ProviderIONet  Provider = "io.net"
	ProviderAll    Provider = "all"
)

type Service struct {
	vastClient  *vastai.Client
	ionetClient *ionet.Client
	config      *config.GPUConfig
}

func NewService(cfg *config.GPUConfig) *Service {
	var vastClient *vastai.Client
	var ionetClient *ionet.Client

	if cfg.VastAIAPIKey != "" {
		vastClient = vastai.NewClient(cfg.VastAIAPIKey, cfg.Timeout)
	}

	if cfg.IONetAPIKey != "" {
		ionetClient = ionet.NewClient(cfg.IONetAPIKey, cfg.Timeout)
	}

	return &Service{
		vastClient:  vastClient,
		ionetClient: ionetClient,
		config:      cfg,
	}
}

func (s *Service) ListInstances(ctx context.Context, provider Provider) ([]models.GPUInstance, error) {
	switch provider {
	case ProviderVastAI:
		if s.vastClient == nil {
			return nil, fmt.Errorf("vast.ai client not configured")
		}
		return s.vastClient.ListInstances(ctx)

	case ProviderIONet:
		if s.ionetClient == nil {
			return nil, fmt.Errorf("io.net client not configured")
		}
		return s.ionetClient.ListInstances(ctx)

	case ProviderAll:
		return s.listAllInstances(ctx)

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

func (s *Service) listAllInstances(ctx context.Context) ([]models.GPUInstance, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allInstances []models.GPUInstance
	var errors []error

	if s.vastClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			instances, err := s.vastClient.ListInstances(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("vast.ai: %w", err))
			} else {
				allInstances = append(allInstances, instances...)
			}
		}()
	}

	if s.ionetClient != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			instances, err := s.ionetClient.ListInstances(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("io.net: %w", err))
			} else {
				allInstances = append(allInstances, instances...)
			}
		}()
	}

	wg.Wait()

	if len(errors) > 0 && len(allInstances) == 0 {
		return nil, fmt.Errorf("all providers failed: %v", errors)
	}

	sort.Slice(allInstances, func(i, j int) bool {
		return allInstances[i].PricePerHour < allInstances[j].PricePerHour
	})

	return allInstances, nil
}

func (s *Service) CreateInstance(ctx context.Context, provider Provider, instanceID string, config map[string]interface{}) (string, error) {
	switch provider {
	case ProviderVastAI:
		if s.vastClient == nil {
			return "", fmt.Errorf("vast.ai client not configured")
		}
		imageURL, _ := config["image"].(string)
		if imageURL == "" {
			imageURL = "nvidia/cuda:12.0.0-base-ubuntu22.04"
		}
		return s.vastClient.CreateInstance(ctx, instanceID, imageURL)

	case ProviderIONet:
		if s.ionetClient == nil {
			return "", fmt.Errorf("io.net client not configured")
		}
		return s.ionetClient.CreateInstance(ctx, instanceID, config)

	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

func (s *Service) DestroyInstance(ctx context.Context, provider Provider, instanceID string) error {
	switch provider {
	case ProviderVastAI:
		if s.vastClient == nil {
			return fmt.Errorf("vast.ai client not configured")
		}
		return s.vastClient.DestroyInstance(ctx, instanceID)

	case ProviderIONet:
		if s.ionetClient == nil {
			return fmt.Errorf("io.net client not configured")
		}
		return s.ionetClient.DestroyInstance(ctx, instanceID)

	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

func (s *Service) GetInstanceStatus(ctx context.Context, provider Provider, instanceID string) (string, error) {
	switch provider {
	case ProviderIONet:
		if s.ionetClient == nil {
			return "", fmt.Errorf("io.net client not configured")
		}
		return s.ionetClient.GetInstanceStatus(ctx, instanceID)

	default:
		return "", fmt.Errorf("status check not supported for provider: %s", provider)
	}
}

func (s *Service) FilterInstances(instances []models.GPUInstance, filters map[string]interface{}) []models.GPUInstance {
	var filtered []models.GPUInstance

	for _, instance := range instances {
		if s.matchesFilters(instance, filters) {
			filtered = append(filtered, instance)
		}
	}

	return filtered
}

func (s *Service) matchesFilters(instance models.GPUInstance, filters map[string]interface{}) bool {
	if minVRAM, ok := filters["min_vram"].(int); ok {
		if instance.VRAM < minVRAM {
			return false
		}
	}

	if maxPrice, ok := filters["max_price"].(float64); ok {
		if instance.PricePerHour > maxPrice {
			return false
		}
	}

	if gpuModel, ok := filters["gpu_model"].(string); ok {
		if instance.GPUName != gpuModel {
			return false
		}
	}

	if location, ok := filters["location"].(string); ok {
		if instance.Location != location {
			return false
		}
	}

	return true
}
