package vastai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/helpers/common"
)

// Client wraps Vast.ai API interactions
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new Vast.ai API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://console.vast.ai/api/v0",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// List returns available GPU instances
func (c *Client) List(ctx context.Context, opts common.ListOptions) ([]common.GPUInstance, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/bundles", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var vastOffers []struct {
		ID              int     `json:"id"`
		GPUName         string  `json:"gpu_name"`
		NumGPUs         int     `json:"num_gpus"`
		GPURAMTotal     int     `json:"gpu_ram"`
		CPUCores        int     `json:"cpu_cores"`
		CPURAMTotal     int     `json:"cpu_ram"`
		DiskSpace       int     `json:"disk_space"`
		DPHBase         float64 `json:"dph_base"`
		PublicIPAddr    string  `json:"public_ipaddr"`
		Datacenter      string  `json:"geolocation"`
		MachineID       int     `json:"machine_id"`
		HostingType     int     `json:"hosting_type"`
		InetUpBilled    float64 `json:"inet_up_billed"`
		InetDownBilled  float64 `json:"inet_down_billed"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vastOffers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instances := make([]common.GPUInstance, 0, len(vastOffers))
	for _, offer := range vastOffers {
		// Apply filters
		if opts.GPUModel != "" && offer.GPUName != opts.GPUModel {
			continue
		}
		if opts.Region != "" && offer.Datacenter != opts.Region {
			continue
		}
		if opts.MaxCostPerHr > 0 && offer.DPHBase > opts.MaxCostPerHr {
			continue
		}

		instance := common.GPUInstance{
			ID:          fmt.Sprintf("%d", offer.ID),
			Provider:    common.ProviderBudgetVastAI,
			Status:      "available",
			GPUModel:    offer.GPUName,
			GPUCount:    offer.NumGPUs,
			VRAM:        offer.GPURAMTotal / 1024, // Convert MB to GB
			CPUCores:    offer.CPUCores,
			RAM:         offer.CPURAMTotal / 1024, // Convert MB to GB
			Disk:        offer.DiskSpace,
			PublicIP:    offer.PublicIPAddr,
			Region:      offer.Datacenter,
			CostPerHour: offer.DPHBase,
			ProviderData: map[string]interface{}{
				"machine_id":        offer.MachineID,
				"hosting_type":      offer.HostingType,
				"inet_up_billed":    offer.InetUpBilled,
				"inet_down_billed":  offer.InetDownBilled,
			},
		}
		instances = append(instances, instance)

		if opts.Limit > 0 && len(instances) >= opts.Limit {
			break
		}
	}

	return instances, nil
}

// Reserve creates a new GPU instance reservation
func (c *Client) Reserve(ctx context.Context, req common.ReservationRequest) (*common.GPUInstance, error) {
	// First, find matching offers
	listOpts := common.ListOptions{
		GPUModel:     req.GPUModel,
		Region:       req.PreferredRegion,
		MaxCostPerHr: req.MaxCostPerHour,
		Limit:        1,
	}

	offers, err := c.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find offers: %w", err)
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no offers found matching criteria")
	}

	bestOffer := offers[0]

	// Create instance from offer
	createReq := map[string]interface{}{
		"client_id": "me",
		"image":     req.Image,
		"disk":      req.MinDisk,
		"label":     req.Labels["name"],
		"onstart":   "",
		"runtype":   "ssh",
		"image_login": "",
	}

	if req.Env != nil {
		createReq["env"] = req.Env
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"PUT",
		fmt.Sprintf("%s/asks/%s/", c.baseURL, bestOffer.ID),
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Success     bool   `json:"success"`
		NewContract int    `json:"new_contract"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("reservation failed")
	}

	// Get instance details
	instance := bestOffer
	instance.ID = fmt.Sprintf("%d", result.NewContract)
	instance.Status = "running"
	instance.CreatedAt = time.Now()
	instance.StartedAt = time.Now()

	return &instance, nil
}

// Release terminates a GPU instance
func (c *Client) Release(ctx context.Context, instanceID string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		"DELETE",
		fmt.Sprintf("%s/instances/%s/", c.baseURL, instanceID),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Status returns the current status of an instance
func (c *Client) Status(ctx context.Context, instanceID string) (*common.GPUInstance, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/instances/%s/", c.baseURL, instanceID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var vastInstance struct {
		ID              int     `json:"id"`
		ActualStatus    string  `json:"actual_status"`
		GPUName         string  `json:"gpu_name"`
		NumGPUs         int     `json:"num_gpus"`
		GPURAMTotal     int     `json:"gpu_ram"`
		CPUCores        int     `json:"cpu_cores"`
		CPURAMTotal     int     `json:"cpu_ram"`
		DiskSpace       int     `json:"disk_space"`
		DPHBase         float64 `json:"dph_base"`
		PublicIPAddr    string  `json:"public_ipaddr"`
		SSHHost         string  `json:"ssh_host"`
		SSHPort         int     `json:"ssh_port"`
		Datacenter      string  `json:"geolocation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vastInstance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instance := &common.GPUInstance{
		ID:          fmt.Sprintf("%d", vastInstance.ID),
		Provider:    common.ProviderBudgetVastAI,
		Status:      vastInstance.ActualStatus,
		GPUModel:    vastInstance.GPUName,
		GPUCount:    vastInstance.NumGPUs,
		VRAM:        vastInstance.GPURAMTotal / 1024,
		CPUCores:    vastInstance.CPUCores,
		RAM:         vastInstance.CPURAMTotal / 1024,
		Disk:        vastInstance.DiskSpace,
		PublicIP:    vastInstance.PublicIPAddr,
		SSHPort:     vastInstance.SSHPort,
		Region:      vastInstance.Datacenter,
		CostPerHour: vastInstance.DPHBase,
	}

	return instance, nil
}

// Destroy is an alias for Release
func (c *Client) Destroy(ctx context.Context, instanceID string) error {
	return c.Release(ctx, instanceID)
}
