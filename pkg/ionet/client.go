package ionet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/internal/models"
)

const (
	IONetAPIBaseURL = "https://api.io.net/v1"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type Device struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	GPU         GPU     `json:"gpu"`
	CPU         CPU     `json:"cpu"`
	Memory      Memory  `json:"memory"`
	Storage     Storage `json:"storage"`
	PricePerHr  float64 `json:"price_per_hour"`
	Location    string  `json:"location"`
	Status      string  `json:"status"`
	Performance float64 `json:"performance_score"`
}

type GPU struct {
	Model string `json:"model"`
	Count int    `json:"count"`
	VRAM  int    `json:"vram_gb"`
}

type CPU struct {
	Cores int `json:"cores"`
	Model string `json:"model"`
}

type Memory struct {
	Total int `json:"total_gb"`
}

type Storage struct {
	Total int `json:"total_gb"`
}

type ListDevicesResponse struct {
	Devices []Device `json:"devices"`
	Total   int      `json:"total"`
}

func NewClient(apiKey string, timeout time.Duration) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) ListInstances(ctx context.Context) ([]models.GPUInstance, error) {
	url := fmt.Sprintf("%s/devices", IONetAPIBaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var listResp ListDevicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instances := make([]models.GPUInstance, 0, len(listResp.Devices))
	for _, device := range listResp.Devices {
		if device.Status != "available" {
			continue
		}

		instances = append(instances, models.GPUInstance{
			ID:           fmt.Sprintf("ionet-%s", device.ID),
			Provider:     "io.net",
			GPUName:      device.GPU.Model,
			GPUCount:     device.GPU.Count,
			VRAM:         device.GPU.VRAM,
			CPUCores:     device.CPU.Cores,
			RAM:          device.Memory.Total,
			Storage:      device.Storage.Total,
			PricePerHour: device.PricePerHr,
			Location:     device.Location,
			Available:    device.Status == "available",
			Specifications: map[string]interface{}{
				"cpu_model":         device.CPU.Model,
				"performance_score": device.Performance,
				"device_name":       device.Name,
			},
		})
	}

	return instances, nil
}

func (c *Client) CreateInstance(ctx context.Context, deviceID string, config map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/instances", IONetAPIBaseURL)

	payload := map[string]interface{}{
		"device_id": deviceID,
		"config":    config,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	instanceID, ok := result["instance_id"].(string)
	if !ok {
		return "", fmt.Errorf("failed to get instance ID from response")
	}

	return instanceID, nil
}

func (c *Client) DestroyInstance(ctx context.Context, instanceID string) error {
	url := fmt.Sprintf("%s/instances/%s", IONetAPIBaseURL, instanceID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) GetInstanceStatus(ctx context.Context, instanceID string) (string, error) {
	url := fmt.Sprintf("%s/instances/%s", IONetAPIBaseURL, instanceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	status, ok := result["status"].(string)
	if !ok {
		return "", fmt.Errorf("failed to get status from response")
	}

	return status, nil
}
