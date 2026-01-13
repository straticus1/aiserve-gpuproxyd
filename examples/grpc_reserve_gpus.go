package main

import (
	"context"
	"fmt"
	"log"
	"os"

	pb "github.com/aiserve/gpuproxy/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	// Get credentials from environment
	email := os.Getenv("GPU_EMAIL")
	password := os.Getenv("GPU_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("GPU_EMAIL and GPU_PASSWORD environment variables required")
	}

	// Connect to gRPC server
	serverAddr := os.Getenv("GPU_GRPC_ADDR")
	if serverAddr == "" {
		serverAddr = "localhost:9090"
	}

	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewGPUProxyServiceClient(conn)

	// Step 1: Login
	fmt.Println("=== Logging in ===")
	loginResp, err := client.Login(context.Background(), &pb.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Printf("Logged in as: %s\n", loginResp.Email)
	fmt.Printf("User ID: %s\n\n", loginResp.UserId)

	// Create authenticated context
	ctx := metadata.AppendToOutgoingContext(
		context.Background(),
		"authorization", "Bearer "+loginResp.Token,
	)

	// Step 2: List available GPUs
	fmt.Println("=== Listing Available GPUs ===")
	instances, err := client.ListGPUInstances(ctx, &pb.ListGPUInstancesRequest{
		Provider: "all",
		MinVram:  16,
		MaxPrice: 3.0,
	})
	if err != nil {
		log.Fatalf("Failed to list instances: %v", err)
	}

	fmt.Printf("Found %d instances matching criteria:\n", instances.TotalCount)
	for i, inst := range instances.Instances {
		if i < 5 { // Show first 5
			fmt.Printf("  %d. %s - %s ($%.2f/hr, %dGB VRAM, %s)\n",
				i+1, inst.Provider, inst.GpuModel,
				inst.PricePerHour, inst.VramGb, inst.Location)
		}
	}
	if instances.TotalCount > 5 {
		fmt.Printf("  ... and %d more\n", instances.TotalCount-5)
	}
	fmt.Println()

	// Step 3: Reserve GPUs
	reserveCount := int32(2) // Reserve 2 GPUs
	fmt.Printf("=== Reserving %d GPUs ===\n", reserveCount)

	reserved, err := client.ReserveGPUs(ctx, &pb.ReserveGPUsRequest{
		Count:    reserveCount,
		Provider: "vast.ai", // Can also use "io.net" or "" for all
		MinVram:  16,
		MaxPrice: 2.5,
	})
	if err != nil {
		log.Fatalf("Failed to reserve GPUs: %v", err)
	}

	fmt.Printf("%s\n", reserved.Message)
	fmt.Printf("Successfully reserved %d GPUs:\n\n", reserved.ReservedCount)

	totalCost := 0.0
	for i, inst := range reserved.ReservedInstances {
		contractID := inst.Metadata["contract_id"]
		fmt.Printf("GPU %d:\n", i+1)
		fmt.Printf("  Instance ID:  %s\n", inst.Id)
		fmt.Printf("  Provider:     %s\n", inst.Provider)
		fmt.Printf("  GPU Model:    %s\n", inst.GpuModel)
		fmt.Printf("  VRAM:         %dGB\n", inst.VramGb)
		fmt.Printf("  GPUs:         %d\n", inst.NumGpus)
		fmt.Printf("  Location:     %s\n", inst.Location)
		fmt.Printf("  Price/Hour:   $%.2f\n", inst.PricePerHour)
		fmt.Printf("  Status:       %s\n", inst.Status)
		fmt.Printf("  Contract ID:  %s\n", contractID)
		fmt.Println()
		totalCost += inst.PricePerHour
	}

	fmt.Printf("Total cost: $%.2f/hour\n", totalCost)
	fmt.Printf("Estimated daily cost: $%.2f\n", totalCost*24)
	fmt.Printf("Estimated weekly cost: $%.2f\n", totalCost*24*7)
	fmt.Println()

	// Step 4: Check load balancer status
	fmt.Println("=== Load Balancer Status ===")
	loadInfo, err := client.GetLoadInfo(ctx, &pb.GetLoadInfoRequest{
		Type: "all",
	})
	if err != nil {
		log.Printf("Failed to get load info: %v", err)
	} else {
		fmt.Printf("Current strategy: %s\n", loadInfo.CurrentStrategy)
		fmt.Printf("Tracked instances: %d\n", len(loadInfo.ServerLoad))
	}
	fmt.Println()

	// Step 5: Get transaction history
	fmt.Println("=== Recent Transactions ===")
	transactions, err := client.GetTransactions(ctx, &pb.GetTransactionsRequest{
		Limit: 5,
	})
	if err != nil {
		log.Printf("Failed to get transactions: %v", err)
	} else {
		fmt.Printf("Showing %d of %d total transactions:\n",
			len(transactions.Transactions), transactions.TotalCount)
		for i, txn := range transactions.Transactions {
			fmt.Printf("  %d. %s - $%.2f %s (%s)\n",
				i+1, txn.Status, txn.Amount, txn.Currency, txn.Id)
		}
	}
	fmt.Println()

	fmt.Println("=== Reservation Complete ===")
	fmt.Println("Your GPUs are now reserved and ready to use!")
	fmt.Printf("Total reserved: %d GPUs\n", reserved.ReservedCount)
	fmt.Printf("Total cost: $%.2f/hour\n", totalCost)
}
