package vastai

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
	VastAPIBaseURL = "https://console.vast.ai/api/v0"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type Instance struct {
	ID              int     `json:"id"`
	GPUName         string  `json:"gpu_name"`
	NumGPUs         int     `json:"num_gpus"`
	GPUMemoryMB     int     `json:"gpu_ram"`
	CPUCores        int     `json:"cpu_cores"`
	CPURam          int     `json:"cpu_ram"`
	DiskSpace       int     `json:"disk_space"`
	DPHTotal        float64 `json:"dph_total"`
	Location        string  `json:"geolocation"`
	Available       bool    `json:"rentable"`
	Verified        bool    `json:"verified"`
	DirectPortCount int     `json:"direct_port_count"`
	InternetSpeed   float64 `json:"inet_down"`
	Score           float64 `json:"score"`
}

type SearchResponse struct {
	Offers []Instance `json:"offers"`
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
	url := fmt.Sprintf("%s/bundles", VastAPIBaseURL)

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

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	instances := make([]models.GPUInstance, 0, len(searchResp.Offers))
	for _, offer := range searchResp.Offers {
		if !offer.Available {
			continue
		}

		instances = append(instances, models.GPUInstance{
			ID:           fmt.Sprintf("vast-%d", offer.ID),
			Provider:     "vast.ai",
			GPUName:      offer.GPUName,
			GPUCount:     offer.NumGPUs,
			VRAM:         offer.GPUMemoryMB / 1024,
			CPUCores:     offer.CPUCores,
			RAM:          offer.CPURam / 1024,
			Storage:      offer.DiskSpace,
			PricePerHour: offer.DPHTotal,
			Location:     offer.Location,
			Available:    offer.Available,
			Specifications: map[string]interface{}{
				"verified":          offer.Verified,
				"direct_port_count": offer.DirectPortCount,
				"internet_speed":    offer.InternetSpeed,
				"score":             offer.Score,
			},
		})
	}

	return instances, nil
}

func (c *Client) CreateInstance(ctx context.Context, instanceID string, imageURL string) (string, error) {
	url := fmt.Sprintf("%s/asks/%s", VastAPIBaseURL, instanceID)

	payload := map[string]interface{}{
		"image":  imageURL,
		"disk":   10,
		"runtype": "ssh",
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

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	contractID, ok := result["new_contract"].(float64)
	if !ok {
		return "", fmt.Errorf("failed to get contract ID from response")
	}

	return fmt.Sprintf("%d", int(contractID)), nil
}

func (c *Client) DestroyInstance(ctx context.Context, contractID string) error {
	url := fmt.Sprintf("%s/instances/%s", VastAPIBaseURL, contractID)

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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
