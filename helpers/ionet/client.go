package ionet

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

// Client wraps IO.net API interactions
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new IO.net API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.io.net/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// List returns available GPU instances
func (c *Client) List(ctx context.Context, opts common.ListOptions) ([]common.GPUInstance, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/devices", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters for filtering
	q := req.URL.Query()
	if opts.GPUModel != "" {
		q.Add("gpu_model", opts.GPUModel)
	}
	if opts.Region != "" {
		q.Add("region", opts.Region)
	}
	if opts.Status != "" {
		q.Add("status", opts.Status)
	}
	if opts.Limit > 0 {
		q.Add("limit", fmt.Sprintf("%d", opts.Limit))
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var ionetDevices struct {
		Devices []struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			Status      string  `json:"status"`
			GPUModel    string  `json:"gpu_model"`
			GPUCount    int     `json:"gpu_count"`
			GPUMemory   int     `json:"gpu_memory_gb"`
			CPUCores    int     `json:"cpu_cores"`
			RAM         int     `json:"ram_gb"`
			Disk        int     `json:"disk_gb"`
			PublicIP    string  `json:"public_ip"`
			Region      string  `json:"region"`
			Datacenter  string  `json:"datacenter"`
			PricePerHr  float64 `json:"price_per_hour"`
			Available   bool    `json:"available"`
		} `json:"devices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ionetDevices); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instances := make([]common.GPUInstance, 0, len(ionetDevices.Devices))
	for _, device := range ionetDevices.Devices {
		// Apply additional filters
		if opts.MaxCostPerHr > 0 && device.PricePerHr > opts.MaxCostPerHr {
			continue
		}

		status := device.Status
		if device.Available {
			status = "available"
		}

		instance := common.GPUInstance{
			ID:          device.ID,
			Provider:    common.ProviderBudgetIONet,
			Status:      status,
			GPUModel:    device.GPUModel,
			GPUCount:    device.GPUCount,
			VRAM:        device.GPUMemory,
			CPUCores:    device.CPUCores,
			RAM:         device.RAM,
			Disk:        device.Disk,
			PublicIP:    device.PublicIP,
			Region:      device.Region,
			Datacenter:  device.Datacenter,
			CostPerHour: device.PricePerHr,
			ProviderData: map[string]interface{}{
				"name":      device.Name,
				"available": device.Available,
			},
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// Reserve creates a new GPU instance reservation
func (c *Client) Reserve(ctx context.Context, req common.ReservationRequest) (*common.GPUInstance, error) {
	// First, find matching devices
	listOpts := common.ListOptions{
		GPUModel:     req.GPUModel,
		Region:       req.PreferredRegion,
		MaxCostPerHr: req.MaxCostPerHour,
		Status:       "available",
		Limit:        1,
	}

	devices, err := c.List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices found matching criteria")
	}

	bestDevice := devices[0]

	// Create reservation
	reserveReq := map[string]interface{}{
		"device_id": bestDevice.ID,
		"duration":  int(req.Duration.Hours()),
		"image":     req.Image,
	}

	if req.Env != nil {
		reserveReq["env"] = req.Env
	}

	if req.Labels != nil {
		reserveReq["labels"] = req.Labels
	}

	reqBody, err := json.Marshal(reserveReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/reservations",
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ReservationID string `json:"reservation_id"`
		DeviceID      string `json:"device_id"`
		Status        string `json:"status"`
		SSHHost       string `json:"ssh_host"`
		SSHPort       int    `json:"ssh_port"`
		SSHKey        string `json:"ssh_key"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Get full instance details
	instance := bestDevice
	instance.ID = result.ReservationID
	instance.Status = result.Status
	instance.SSHPort = result.SSHPort
	instance.SSHKey = result.SSHKey
	instance.CreatedAt = time.Now()
	instance.StartedAt = time.Now()

	return &instance, nil
}

// Release terminates a GPU instance reservation
func (c *Client) Release(ctx context.Context, instanceID string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		"DELETE",
		fmt.Sprintf("%s/reservations/%s", c.baseURL, instanceID),
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Status returns the current status of a reservation
func (c *Client) Status(ctx context.Context, instanceID string) (*common.GPUInstance, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s/reservations/%s", c.baseURL, instanceID),
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

	var reservation struct {
		ID          string    `json:"id"`
		DeviceID    string    `json:"device_id"`
		Status      string    `json:"status"`
		GPUModel    string    `json:"gpu_model"`
		GPUCount    int       `json:"gpu_count"`
		GPUMemory   int       `json:"gpu_memory_gb"`
		CPUCores    int       `json:"cpu_cores"`
		RAM         int       `json:"ram_gb"`
		Disk        int       `json:"disk_gb"`
		PublicIP    string    `json:"public_ip"`
		SSHPort     int       `json:"ssh_port"`
		Region      string    `json:"region"`
		PricePerHr  float64   `json:"price_per_hour"`
		TotalCost   float64   `json:"total_cost"`
		CreatedAt   time.Time `json:"created_at"`
		ExpiresAt   time.Time `json:"expires_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&reservation); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instance := &common.GPUInstance{
		ID:          reservation.ID,
		Provider:    common.ProviderBudgetIONet,
		Status:      reservation.Status,
		GPUModel:    reservation.GPUModel,
		GPUCount:    reservation.GPUCount,
		VRAM:        reservation.GPUMemory,
		CPUCores:    reservation.CPUCores,
		RAM:         reservation.RAM,
		Disk:        reservation.Disk,
		PublicIP:    reservation.PublicIP,
		SSHPort:     reservation.SSHPort,
		Region:      reservation.Region,
		CostPerHour: reservation.PricePerHr,
		TotalCost:   reservation.TotalCost,
		CreatedAt:   reservation.CreatedAt,
	}

	return instance, nil
}

// Destroy is an alias for Release
func (c *Client) Destroy(ctx context.Context, instanceID string) error {
	return c.Release(ctx, instanceID)
}
