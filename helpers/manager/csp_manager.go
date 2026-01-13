package manager

import (
	"context"
	"fmt"
	"sort"

	"github.com/aiserve/gpuproxy/helpers/aws"
	"github.com/aiserve/gpuproxy/helpers/common"
	"github.com/aiserve/gpuproxy/helpers/ionet"
	"github.com/aiserve/gpuproxy/helpers/oracle"
	"github.com/aiserve/gpuproxy/helpers/vastai"
)

// CSPManager manages all cloud service providers
type CSPManager struct {
	// Budget CSPs
	vastAIClient *vastai.Client
	ionetClient  *ionet.Client

	// Major CSPs
	awsClient    *aws.Client
	oracleClient *oracle.Client

	// Configuration
	preferBudget bool
	maxBudgetGPUs int
	maxMajorGPUs  int
}

// NewCSPManager creates a unified CSP manager
func NewCSPManager(
	vastAIKey, ionetKey string,
	awsRegion, awsAccessKey, awsSecret, awsSession string,
	oracleTenancy, oracleUser, oracleFingerprint, oracleKey, oracleRegion, oracleCompartment string,
) *CSPManager {
	return &CSPManager{
		// Budget CSPs
		vastAIClient: vastai.NewClient(vastAIKey),
		ionetClient:  ionet.NewClient(ionetKey),

		// Major CSPs
		awsClient:    aws.NewClient(awsRegion, awsAccessKey, awsSecret, awsSession),
		oracleClient: oracle.NewClient(oracleTenancy, oracleUser, oracleFingerprint, oracleKey, oracleRegion, oracleCompartment),

		// Defaults
		preferBudget:  true,
		maxBudgetGPUs: 500, // 500 Vast.ai + 500 IO.net = 1000 total
		maxMajorGPUs:  500, // AWS + Oracle combined
	}
}

// SetPreference configures budget vs major CSP preference
func (m *CSPManager) SetPreference(preferBudget bool) {
	m.preferBudget = preferBudget
}

// ListAll returns GPU instances from all providers
func (m *CSPManager) ListAll(ctx context.Context, opts common.ListOptions) (map[string][]common.GPUInstance, error) {
	results := make(map[string][]common.GPUInstance)

	// Budget CSPs
	if vastInstances, err := m.vastAIClient.List(ctx, opts); err == nil {
		results["vastai"] = vastInstances
	}

	if ionetInstances, err := m.ionetClient.List(ctx, opts); err == nil {
		results["ionet"] = ionetInstances
	}

	// Major CSPs (using mock data for now)
	if awsInstances, err := m.awsClient.MockList(ctx, opts); err == nil {
		results["aws"] = awsInstances
	}

	if oracleInstances, err := m.oracleClient.MockList(ctx, opts); err == nil {
		results["oracle"] = oracleInstances
	}

	return results, nil
}

// ListBudgetCSPs returns instances from budget providers only
func (m *CSPManager) ListBudgetCSPs(ctx context.Context, opts common.ListOptions) ([]common.GPUInstance, error) {
	var allInstances []common.GPUInstance

	vastInstances, err := m.vastAIClient.List(ctx, opts)
	if err == nil {
		allInstances = append(allInstances, vastInstances...)
	}

	ionetInstances, err := m.ionetClient.List(ctx, opts)
	if err == nil {
		allInstances = append(allInstances, ionetInstances...)
	}

	// Sort by cost (cheapest first)
	sort.Slice(allInstances, func(i, j int) bool {
		return allInstances[i].CostPerHour < allInstances[j].CostPerHour
	})

	return allInstances, nil
}

// ListMajorCSPs returns instances from major providers only
func (m *CSPManager) ListMajorCSPs(ctx context.Context, opts common.ListOptions) ([]common.GPUInstance, error) {
	var allInstances []common.GPUInstance

	// Using mock data for major CSPs until SDK integration
	awsInstances, err := m.awsClient.MockList(ctx, opts)
	if err == nil {
		allInstances = append(allInstances, awsInstances...)
	}

	oracleInstances, err := m.oracleClient.MockList(ctx, opts)
	if err == nil {
		allInstances = append(allInstances, oracleInstances...)
	}

	// Sort by cost (cheapest first)
	sort.Slice(allInstances, func(i, j int) bool {
		return allInstances[i].CostPerHour < allInstances[j].CostPerHour
	})

	return allInstances, nil
}

// Reserve intelligently selects and reserves a GPU instance
func (m *CSPManager) Reserve(ctx context.Context, req common.ReservationRequest) (*common.GPUInstance, error) {
	// Step 1: Determine provider category preference
	if req.PreferBudget || m.preferBudget {
		// Try budget CSPs first
		instance, err := m.reserveBudgetCSP(ctx, req)
		if err == nil {
			return instance, nil
		}

		// Fallback to major CSPs if allowed
		if req.AllowFallback {
			return m.reserveMajorCSP(ctx, req)
		}

		return nil, fmt.Errorf("no budget CSP available and fallback disabled")
	}

	// Try major CSPs first
	instance, err := m.reserveMajorCSP(ctx, req)
	if err == nil {
		return instance, nil
	}

	// Fallback to budget CSPs if allowed
	if req.AllowFallback {
		return m.reserveBudgetCSP(ctx, req)
	}

	return nil, fmt.Errorf("no major CSP available and fallback disabled")
}

// reserveBudgetCSP reserves from Vast.ai or IO.net
func (m *CSPManager) reserveBudgetCSP(ctx context.Context, req common.ReservationRequest) (*common.GPUInstance, error) {
	// List available instances from both budget CSPs
	opts := common.ListOptions{
		GPUModel:     req.GPUModel,
		Region:       req.PreferredRegion,
		MaxCostPerHr: req.MaxCostPerHour,
		Limit:        10,
	}

	vastInstances, vastErr := m.vastAIClient.List(ctx, opts)
	ionetInstances, ionetErr := m.ionetClient.List(ctx, opts)

	// Combine and sort by cost
	var allInstances []common.GPUInstance
	if vastErr == nil {
		allInstances = append(allInstances, vastInstances...)
	}
	if ionetErr == nil {
		allInstances = append(allInstances, ionetInstances...)
	}

	if len(allInstances) == 0 {
		return nil, fmt.Errorf("no budget CSP instances available")
	}

	sort.Slice(allInstances, func(i, j int) bool {
		return allInstances[i].CostPerHour < allInstances[j].CostPerHour
	})

	// Try to reserve the cheapest available
	bestInstance := allInstances[0]

	switch bestInstance.Provider {
	case common.ProviderBudgetVastAI:
		return m.vastAIClient.Reserve(ctx, req)
	case common.ProviderBudgetIONet:
		return m.ionetClient.Reserve(ctx, req)
	default:
		return nil, fmt.Errorf("unknown budget provider: %s", bestInstance.Provider)
	}
}

// reserveMajorCSP reserves from AWS or Oracle
func (m *CSPManager) reserveMajorCSP(ctx context.Context, req common.ReservationRequest) (*common.GPUInstance, error) {
	// For now, return error since AWS/Oracle SDK not fully integrated
	return nil, fmt.Errorf("major CSP reservation not yet implemented - requires AWS/Oracle SDK integration")
}

// Release terminates an instance regardless of provider
func (m *CSPManager) Release(ctx context.Context, provider common.ProviderType, instanceID string) error {
	switch provider {
	case common.ProviderBudgetVastAI:
		return m.vastAIClient.Release(ctx, instanceID)
	case common.ProviderBudgetIONet:
		return m.ionetClient.Release(ctx, instanceID)
	case common.ProviderMajorAWS:
		return m.awsClient.Release(ctx, instanceID)
	case common.ProviderMajorOracle:
		return m.oracleClient.Release(ctx, instanceID)
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

// Status gets instance status regardless of provider
func (m *CSPManager) Status(ctx context.Context, provider common.ProviderType, instanceID string) (*common.GPUInstance, error) {
	switch provider {
	case common.ProviderBudgetVastAI:
		return m.vastAIClient.Status(ctx, instanceID)
	case common.ProviderBudgetIONet:
		return m.ionetClient.Status(ctx, instanceID)
	case common.ProviderMajorAWS:
		return m.awsClient.Status(ctx, instanceID)
	case common.ProviderMajorOracle:
		return m.oracleClient.Status(ctx, instanceID)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// GetStats returns statistics across all providers
func (m *CSPManager) GetStats(ctx context.Context) map[string]interface{} {
	budgetInstances, _ := m.ListBudgetCSPs(ctx, common.ListOptions{})
	majorInstances, _ := m.ListMajorCSPs(ctx, common.ListOptions{})

	budgetCost := 0.0
	for _, inst := range budgetInstances {
		budgetCost += inst.CostPerHour
	}

	majorCost := 0.0
	for _, inst := range majorInstances {
		majorCost += inst.CostPerHour
	}

	return map[string]interface{}{
		"budget_csps": map[string]interface{}{
			"count":           len(budgetInstances),
			"total_cost_hour": budgetCost,
			"providers":       []string{"vastai", "ionet"},
		},
		"major_csps": map[string]interface{}{
			"count":           len(majorInstances),
			"total_cost_hour": majorCost,
			"providers":       []string{"aws", "oracle"},
		},
		"total_instances": len(budgetInstances) + len(majorInstances),
		"total_cost_hour": budgetCost + majorCost,
		"preference":      map[string]bool{"budget_first": m.preferBudget},
	}
}
