package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

var (
	apiURL        string
	apiKey        string
	developerMode bool
	debugMode     bool
)

type GPUInstance struct {
	ID           string  `json:"id"`
	Provider     string  `json:"provider"`
	GPUName      string  `json:"gpu_name"`
	GPUCount     int     `json:"gpu_count"`
	VRAM         int     `json:"vram_gb"`
	PricePerHour float64 `json:"price_per_hour"`
	Location     string  `json:"location"`
	Available    bool    `json:"available"`
}

func main() {
	flag.StringVar(&apiURL, "api", "http://localhost:8080", "API URL")
	flag.StringVar(&apiKey, "key", "", "API key")
	flag.BoolVar(&developerMode, "dv", false, "Enable developer mode")
	flag.BoolVar(&developerMode, "developer-mode", false, "Enable developer mode")
	flag.BoolVar(&debugMode, "dm", false, "Enable debug mode")
	flag.BoolVar(&debugMode, "debug-mode", false, "Enable debug mode")
	flag.Parse()

	if apiKey == "" {
		apiKey = os.Getenv("GPUPROXY_API_KEY")
	}

	if apiKey == "" {
		log.Fatal("API key required. Use -key flag or GPUPROXY_API_KEY environment variable")
	}

	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	if developerMode {
		log.Println("Developer mode enabled")
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "list":
		provider := "all"
		if len(args) > 1 {
			provider = args[1]
		}
		listInstances(provider)

	case "create":
		if len(args) < 3 {
			log.Fatal("Usage: client create <provider> <instance-id>")
		}
		createInstance(args[1], args[2])

	case "destroy":
		if len(args) < 3 {
			log.Fatal("Usage: client destroy <provider> <instance-id>")
		}
		destroyInstance(args[1], args[2])

	case "proxy":
		if len(args) < 3 {
			log.Fatal("Usage: client proxy <protocol> <target-url>")
		}
		proxyRequest(args[1], args[2])

	case "load":
		subcommand := "all"
		if len(args) > 1 {
			subcommand = args[1]
		}
		showLoad(subcommand)

	case "reserve":
		count := 1
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &count)
		}
		reserveInstances(count)

	case "lb-strategy":
		if len(args) < 2 {
			getLoadBalancerStrategy()
		} else {
			setLoadBalancerStrategy(args[1])
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("GPU Proxy CLI Client")
	fmt.Println("\nUsage:")
	fmt.Println("  client [flags] <command> [args]")
	fmt.Println("\nFlags:")
	fmt.Println("  -api <url>              API URL (default: http://localhost:8080)")
	fmt.Println("  -key <key>              API key")
	fmt.Println("  -dv, -developer-mode    Enable developer mode")
	fmt.Println("  -dm, -debug-mode        Enable debug mode")
	fmt.Println("\nCommands:")
	fmt.Println("  list [provider]                      List available GPU instances")
	fmt.Println("                                       provider: all, vast.ai, io.net (default: all)")
	fmt.Println("  create <provider> <instance-id>      Create a GPU instance")
	fmt.Println("  destroy <provider> <instance-id>     Destroy a GPU instance")
	fmt.Println("  reserve <count>                      Reserve multiple GPUs (1-16, default: 1)")
	fmt.Println("  proxy <protocol> <target-url>        Proxy a request through GPU")
	fmt.Println("                                       protocol: http, https, mcp, openinference")
	fmt.Println("  load [type]                          Show load information")
	fmt.Println("                                       type: all, server, provider (default: all)")
	fmt.Println("  lb-strategy [strategy]               Get/set load balancing strategy")
	fmt.Println("                                       strategies: round_robin, equal_weighted,")
	fmt.Println("                                       weighted_round_robin, least_connections,")
	fmt.Println("                                       least_response_time")
}

func listInstances(provider string) {
	url := fmt.Sprintf("%s/api/v1/gpu/instances?provider=%s", apiURL, provider)

	if debugMode {
		log.Printf("GET %s", url)
	}

	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result struct {
		Instances []GPUInstance `json:"instances"`
		Count     int           `json:"count"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	if result.Count == 0 {
		fmt.Println("No instances found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPROVIDER\tGPU\tCOUNT\tVRAM\tPRICE/HR\tLOCATION")
	fmt.Fprintln(w, "---\t---\t---\t---\t---\t---\t---")

	for _, inst := range result.Instances {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%dGB\t$%.2f\t%s\n",
			inst.ID, inst.Provider, inst.GPUName, inst.GPUCount,
			inst.VRAM, inst.PricePerHour, inst.Location)
	}

	w.Flush()
	fmt.Printf("\nTotal: %d instances\n", result.Count)
}

func createInstance(provider, instanceID string) {
	url := fmt.Sprintf("%s/api/v1/gpu/instances/%s/%s", apiURL, provider, instanceID)

	if debugMode {
		log.Printf("POST %s", url)
	}

	config := map[string]interface{}{
		"image": "nvidia/cuda:12.0.0-base-ubuntu22.04",
	}

	configJSON, _ := json.Marshal(config)
	resp, err := makeRequest("POST", url, configJSON)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Instance created successfully\n")
	fmt.Printf("Contract ID: %v\n", result["contract_id"])
	fmt.Printf("Provider: %v\n", result["provider"])
}

func destroyInstance(provider, instanceID string) {
	url := fmt.Sprintf("%s/api/v1/gpu/instances/%s/%s", apiURL, provider, instanceID)

	if debugMode {
		log.Printf("DELETE %s", url)
	}

	resp, err := makeRequest("DELETE", url, nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Instance destroyed: %v\n", result["message"])
}

func proxyRequest(protocol, targetURL string) {
	url := fmt.Sprintf("%s/api/v1/gpu/proxy", apiURL)

	proxyReq := map[string]interface{}{
		"protocol":   protocol,
		"target_url": targetURL,
		"method":     "POST",
		"headers": map[string]string{
			"Content-Type": "application/json",
		},
	}

	if debugMode {
		log.Printf("POST %s", url)
		log.Printf("Proxy request: %+v", proxyReq)
	}

	reqJSON, _ := json.Marshal(proxyReq)
	resp, err := makeRequest("POST", url, reqJSON)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Proxy response:\n")
	pretty, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(pretty))
}

func reserveInstances(count int) {
	if count < 1 || count > 16 {
		log.Fatal("Count must be between 1 and 16")
	}

	url := fmt.Sprintf("%s/api/v1/gpu/instances/reserve", apiURL)

	if debugMode {
		log.Printf("POST %s (count: %d)", url, count)
	}

	req := map[string]interface{}{
		"count":  count,
		"config": map[string]string{"image": "nvidia/cuda:12.0.0-base-ubuntu22.04"},
	}

	reqJSON, _ := json.Marshal(req)
	resp, err := makeRequest("POST", url, reqJSON)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Reserved %v GPUs (requested: %d)\n", result["count"], count)
	if reserved, ok := result["reserved"].([]interface{}); ok {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "INSTANCE\tCONTRACT\tPROVIDER\tGPU\tVRAM\tPRICE")
		fmt.Fprintln(w, "---\t---\t---\t---\t---\t---")

		for _, r := range reserved {
			res := r.(map[string]interface{})
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%vGB\t$%.2f\n",
				res["instance_id"], res["contract_id"], res["provider"],
				res["gpu_model"], res["vram"], res["price"])
		}
		w.Flush()
	}
}

func showLoad(loadType string) {
	switch loadType {
	case "server":
		showServerLoad()
	case "provider":
		showProviderLoad()
	default:
		showServerLoad()
		fmt.Println()
		showProviderLoad()
	}
}

func showServerLoad() {
	url := fmt.Sprintf("%s/api/v1/loadbalancer/loads", apiURL)

	if debugMode {
		log.Printf("GET %s", url)
	}

	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Load Balancing Strategy: %v\n", result["strategy"])
	fmt.Printf("Tracked Instances: %v\n\n", result["count"])

	if loads, ok := result["loads"].(map[string]interface{}); ok {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "INSTANCE\tPROVIDER\tACTIVE\tTOTAL\tAVG RT\tWEIGHT")
		fmt.Fprintln(w, "---\t---\t---\t---\t---\t---")

		for _, load := range loads {
			l := load.(map[string]interface{})
			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%.2f\n",
				l["instance_id"], l["provider"], l["active_connections"],
				l["total_connections"], l["avg_response_time"], l["weight"])
		}
		w.Flush()
	}
}

func showProviderLoad() {
	url := fmt.Sprintf("%s/api/v1/gpu/instances", apiURL)

	if debugMode {
		log.Printf("GET %s", url)
	}

	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result struct {
		Instances []GPUInstance `json:"instances"`
		Count     int           `json:"count"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	vastCount := 0
	ionetCount := 0
	for _, inst := range result.Instances {
		if inst.Provider == "vast.ai" {
			vastCount++
		} else if inst.Provider == "io.net" {
			ionetCount++
		}
	}

	fmt.Println("Provider Load:")
	fmt.Printf("  vast.ai: %d available instances\n", vastCount)
	fmt.Printf("  io.net:  %d available instances\n", ionetCount)
	fmt.Printf("  Total:   %d instances\n", result.Count)
}

func getLoadBalancerStrategy() {
	url := fmt.Sprintf("%s/api/v1/loadbalancer/strategy", apiURL)

	resp, err := makeRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("Current load balancing strategy: %v\n", result["strategy"])
}

func setLoadBalancerStrategy(strategy string) {
	url := fmt.Sprintf("%s/api/v1/loadbalancer/strategy", apiURL)

	req := map[string]string{"strategy": strategy}
	reqJSON, _ := json.Marshal(req)

	resp, err := makeRequest("PUT", url, reqJSON)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Printf("%v\n", result["message"])
	fmt.Printf("New strategy: %v\n", result["strategy"])
}

func makeRequest(method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	if developerMode {
		req.Header.Set("X-Developer-Mode", "true")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
