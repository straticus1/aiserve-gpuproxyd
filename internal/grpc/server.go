package grpc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/loadbalancer"
	"github.com/aiserve/gpuproxy/internal/models"
	pb "github.com/aiserve/gpuproxy/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC GPUProxyService
type Server struct {
	pb.UnimplementedGPUProxyServiceServer
	authService     *auth.Service
	gpuService      *gpu.Service
	protocolHandler *gpu.ProtocolHandler
	billingService  *billing.Service
	lbService       *loadbalancer.LoadBalancerService
	grpcServer      *grpc.Server
}

// NewServer creates a new gRPC server instance
func NewServer(
	authService *auth.Service,
	gpuService *gpu.Service,
	protocolHandler *gpu.ProtocolHandler,
	billingService *billing.Service,
	lbService *loadbalancer.LoadBalancerService,
) *Server {
	return &Server{
		authService:     authService,
		gpuService:      gpuService,
		protocolHandler: protocolHandler,
		billingService:  billingService,
		lbService:       lbService,
	}
}

// Start starts the gRPC server on the specified address
func (s *Server) Start(address string, certFile, keyFile string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption

	// Enable TLS if certificate files are provided
	if certFile != "" && keyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
		log.Printf("gRPC server starting with TLS enabled")
	} else {
		log.Printf("gRPC server starting WITHOUT TLS (insecure)")
	}

	// Add interceptors
	opts = append(opts,
		grpc.UnaryInterceptor(s.authInterceptor),
		grpc.StreamInterceptor(s.streamAuthInterceptor),
	)

	s.grpcServer = grpc.NewServer(opts...)

	pb.RegisterGPUProxyServiceServer(s.grpcServer, s)

	log.Printf("gRPC server listening on %s", address)
	return s.grpcServer.Serve(lis)
}

// StartWithTLSConfig starts the gRPC server with a custom TLS configuration
func (s *Server) StartWithTLSConfig(address string, tlsConfig *tls.Config) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption

	if tlsConfig != nil {
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		log.Printf("gRPC server starting with custom TLS config")
	} else {
		log.Printf("gRPC server starting WITHOUT TLS (insecure)")
	}

	// Add interceptors
	opts = append(opts,
		grpc.UnaryInterceptor(s.authInterceptor),
		grpc.StreamInterceptor(s.streamAuthInterceptor),
	)

	s.grpcServer = grpc.NewServer(opts...)

	pb.RegisterGPUProxyServiceServer(s.grpcServer, s)

	log.Printf("gRPC server listening on %s", address)
	return s.grpcServer.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

// authInterceptor validates authentication for unary RPCs
func (s *Server) authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// Skip auth for Login and HealthCheck
	if info.FullMethod == "/gpuproxy.GPUProxyService/Login" || info.FullMethod == "/gpuproxy.GPUProxyService/HealthCheck" {
		return handler(ctx, req)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	// Check for API key
	apiKeys := md.Get("x-api-key")
	if len(apiKeys) > 0 {
		user, err := s.authService.ValidateAPIKey(ctx, apiKeys[0])
		if err == nil {
			ctx = context.WithValue(ctx, "user_id", user.ID)
			return handler(ctx, req)
		}
	}

	// Check for JWT token
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "missing authorization token")
	}

	// Remove "Bearer " prefix if present
	token := tokens[0]
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	claims, err := auth.ValidateToken(token, s.authService.GetJWTSecret())
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	return handler(ctx, req)
}

// streamAuthInterceptor validates authentication for streaming RPCs
func (s *Server) streamAuthInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	// Check for API key
	apiKeys := md.Get("x-api-key")
	if len(apiKeys) > 0 {
		_, err := s.authService.ValidateAPIKey(ss.Context(), apiKeys[0])
		if err == nil {
			return handler(srv, ss)
		}
	}

	// Check for JWT token
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return status.Errorf(codes.Unauthenticated, "missing authorization token")
	}

	token := tokens[0]
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	_, err := auth.ValidateToken(token, s.authService.GetJWTSecret())
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	return handler(srv, ss)
}

// getUserID extracts user ID from context
func getUserID(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// Login authenticates a user
func (s *Server) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	tokens, user, err := s.authService.Login(ctx, req.Email, req.Password, "", "gRPC")
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	return &pb.LoginResponse{
		Token:  tokens.AccessToken,
		UserId: user.ID.String(),
		Email:  user.Email,
	}, nil
}

// CreateAPIKey creates a new API key
func (s *Server) CreateAPIKey(ctx context.Context, req *pb.CreateAPIKeyRequest) (*pb.CreateAPIKeyResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	apiKey, err := s.authService.CreateAPIKey(ctx, userID, req.Name, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create API key: %v", err)
	}

	return &pb.CreateAPIKeyResponse{
		ApiKey:    apiKey,
		Name:      req.Name,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// ListGPUInstances lists available GPU instances
func (s *Server) ListGPUInstances(ctx context.Context, req *pb.ListGPUInstancesRequest) (*pb.ListGPUInstancesResponse, error) {
	provider := gpu.Provider(req.Provider)
	if provider == "" {
		provider = "all"
	}

	instances, err := s.gpuService.ListInstances(ctx, provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list instances: %v", err)
	}

	// Apply filters
	filters := make(map[string]interface{})
	if req.MinVram > 0 {
		filters["min_vram"] = req.MinVram
	}
	if req.MaxPrice > 0 {
		filters["max_price"] = req.MaxPrice
	}
	if req.GpuModel != "" {
		filters["gpu_model"] = req.GpuModel
	}

	if len(filters) > 0 {
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	// Convert to protobuf format
	pbInstances := make([]*pb.GPUInstance, len(instances))
	for i, inst := range instances {
		metadata := make(map[string]string)
		if inst.Specifications != nil {
			for k, v := range inst.Specifications {
				if str, ok := v.(string); ok {
					metadata[k] = str
				} else {
					metadata[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		status := "available"
		if !inst.Available {
			status = "unavailable"
		}

		pbInstances[i] = &pb.GPUInstance{
			Id:           inst.ID,
			Provider:     inst.Provider,
			Status:       status,
			PricePerHour: inst.PricePerHour,
			VramGb:       int32(inst.VRAM),
			GpuModel:     inst.GPUName,
			NumGpus:      int32(inst.GPUCount),
			Location:     inst.Location,
			Metadata:     metadata,
		}
	}

	return &pb.ListGPUInstancesResponse{
		Instances:  pbInstances,
		TotalCount: int32(len(pbInstances)),
	}, nil
}

// CreateGPUInstance creates a new GPU instance
func (s *Server) CreateGPUInstance(ctx context.Context, req *pb.CreateGPUInstanceRequest) (*pb.CreateGPUInstanceResponse, error) {
	provider := gpu.Provider(req.Provider)

	config := map[string]interface{}{
		"image": req.Image,
		"env":   req.Env,
		"ports": req.Ports,
	}

	contractID, err := s.gpuService.CreateInstance(ctx, provider, req.InstanceId, config)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create instance: %v", err)
	}

	return &pb.CreateGPUInstanceResponse{
		InstanceId: req.InstanceId,
		Provider:   req.Provider,
		Status:     "creating",
		Message:    fmt.Sprintf("Contract ID: %s", contractID),
	}, nil
}

// DestroyGPUInstance destroys a GPU instance
func (s *Server) DestroyGPUInstance(ctx context.Context, req *pb.DestroyGPUInstanceRequest) (*pb.DestroyGPUInstanceResponse, error) {
	provider := gpu.Provider(req.Provider)

	err := s.gpuService.DestroyInstance(ctx, provider, req.InstanceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to destroy instance: %v", err)
	}

	return &pb.DestroyGPUInstanceResponse{
		Success: true,
		Message: "Instance destroyed successfully",
	}, nil
}

// GetGPUInstance gets details of a specific GPU instance
func (s *Server) GetGPUInstance(ctx context.Context, req *pb.GetGPUInstanceRequest) (*pb.GetGPUInstanceResponse, error) {
	provider := gpu.Provider(req.Provider)

	instances, err := s.gpuService.ListInstances(ctx, provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get instance: %v", err)
	}

	for _, inst := range instances {
		if inst.ID == req.InstanceId {
			metadata := make(map[string]string)
			if inst.Specifications != nil {
				for k, v := range inst.Specifications {
					if str, ok := v.(string); ok {
						metadata[k] = str
					} else {
						metadata[k] = fmt.Sprintf("%v", v)
					}
				}
			}

			status := "available"
			if !inst.Available {
				status = "unavailable"
			}

			return &pb.GetGPUInstanceResponse{
				Instance: &pb.GPUInstance{
					Id:           inst.ID,
					Provider:     inst.Provider,
					Status:       status,
					PricePerHour: inst.PricePerHour,
					VramGb:       int32(inst.VRAM),
					GpuModel:     inst.GPUName,
					NumGpus:      int32(inst.GPUCount),
					Location:     inst.Location,
					Metadata:     metadata,
				},
			}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "instance not found")
}

// ProxyRequest proxies a request to a GPU instance
func (s *Server) ProxyRequest(ctx context.Context, req *pb.ProxyRequestMessage) (*pb.ProxyResponse, error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO: Implement proxy request handling
	// For now return not implemented
	return nil, status.Errorf(codes.Unimplemented, "proxy requests not yet implemented via gRPC")
}

// StreamProxyRequest handles streaming proxy requests
func (s *Server) StreamProxyRequest(stream pb.GPUProxyService_StreamProxyRequestServer) error {
	ctx := stream.Context()
	_, err := getUserID(ctx)
	if err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO: Implement streaming proxy requests
	return status.Errorf(codes.Unimplemented, "streaming proxy requests not yet implemented via gRPC")
}

// CreatePayment creates a new payment
func (s *Server) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO: Implement payment creation via gRPC
	return nil, status.Errorf(codes.Unimplemented, "payment creation not yet implemented via gRPC")
}

// GetTransactions retrieves transaction history
func (s *Server) GetTransactions(ctx context.Context, req *pb.GetTransactionsRequest) (*pb.GetTransactionsResponse, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	transactions, err := s.billingService.GetTransactionsByUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get transactions: %v", err)
	}

	// Apply pagination
	limit := int(req.Limit)
	if limit == 0 {
		limit = 50
	}
	offset := int(req.Offset)

	end := offset + limit
	if end > len(transactions) {
		end = len(transactions)
	}
	if offset > len(transactions) {
		offset = len(transactions)
	}

	paginatedTxns := transactions[offset:end]

	pbTransactions := make([]*pb.Transaction, len(paginatedTxns))
	for i, txn := range paginatedTxns {
		pbTransactions[i] = &pb.Transaction{
			Id:          txn.ID.String(),
			Type:        "transaction",
			Amount:      txn.Amount,
			Currency:    txn.Currency,
			Timestamp:   txn.CreatedAt.Unix(),
			Status:      txn.Status,
			Description: "",
		}
	}

	return &pb.GetTransactionsResponse{
		Transactions: pbTransactions,
		TotalCount:   int32(len(transactions)),
	}, nil
}

// GetSpendingInfo retrieves spending information
func (s *Server) GetSpendingInfo(ctx context.Context, req *pb.GetSpendingInfoRequest) (*pb.GetSpendingInfoResponse, error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO: Implement spending info via gRPC (requires access to guard rails middleware)
	return nil, status.Errorf(codes.Unimplemented, "spending info not yet implemented via gRPC")
}

// CheckSpendingLimit checks if a spending amount is allowed
func (s *Server) CheckSpendingLimit(ctx context.Context, req *pb.CheckSpendingLimitRequest) (*pb.CheckSpendingLimitResponse, error) {
	_, err := getUserID(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	// TODO: Implement spending limit checks via gRPC (requires access to guard rails middleware)
	return nil, status.Errorf(codes.Unimplemented, "spending limit checks not yet implemented via gRPC")
}

// SetLoadBalancerStrategy sets the load balancing strategy
func (s *Server) SetLoadBalancerStrategy(ctx context.Context, req *pb.SetLoadBalancerStrategyRequest) (*pb.SetLoadBalancerStrategyResponse, error) {
	strategy := loadbalancer.Strategy(req.Strategy)
	s.lbService.SetStrategy(strategy)

	return &pb.SetLoadBalancerStrategyResponse{
		Strategy: req.Strategy,
		Success:  true,
	}, nil
}

// GetLoadInfo retrieves load balancing information
func (s *Server) GetLoadInfo(ctx context.Context, req *pb.GetLoadInfoRequest) (*pb.GetLoadInfoResponse, error) {
	// TODO: Implement load info via gRPC
	return &pb.GetLoadInfoResponse{
		ServerLoad:      make([]*pb.LoadInfo, 0),
		ProviderLoad:    make([]*pb.LoadInfo, 0),
		CurrentStrategy: string(s.lbService.GetStrategy()),
	}, nil
}

// ReserveGPUs reserves multiple GPUs
func (s *Server) ReserveGPUs(ctx context.Context, req *pb.ReserveGPUsRequest) (*pb.ReserveGPUsResponse, error) {
	count := int(req.Count)
	if count < 1 || count > 16 {
		return nil, status.Errorf(codes.InvalidArgument, "count must be between 1 and 16")
	}

	// Determine provider
	provider := gpu.Provider(req.Provider)
	if provider == "" {
		provider = gpu.ProviderAll
	}

	// List all instances
	instances, err := s.gpuService.ListInstances(ctx, provider)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list instances: %v", err)
	}

	// Apply filters
	filters := make(map[string]interface{})
	if req.MinVram > 0 {
		filters["min_vram"] = req.MinVram
	}
	if req.MaxPrice > 0 {
		filters["max_price"] = req.MaxPrice
	}

	if len(filters) > 0 {
		instances = s.gpuService.FilterInstances(instances, filters)
	}

	// Check if enough instances available
	if len(instances) < count {
		return nil, status.Errorf(codes.FailedPrecondition,
			"not enough instances available. Requested: %d, Available: %d", count, len(instances))
	}

	// Reserve instances
	reserved := make([]*pb.GPUInstance, 0, count)
	errorMessages := make([]string, 0)

	for i := 0; i < count && i < len(instances); i++ {
		var selected *models.GPUInstance

		// Use load balancer to select instance if available
		if s.lbService != nil {
			selected, err = s.lbService.SelectInstance(ctx, instances[i:])
			if err != nil {
				selected = &instances[i]
			}
		} else {
			selected = &instances[i]
		}

		// Clean up instance ID for provider
		instanceID := selected.ID
		providerType := gpu.Provider(selected.Provider)
		if providerType == gpu.ProviderVastAI && len(instanceID) > 5 {
			instanceID = instanceID[5:]
		} else if providerType == gpu.ProviderIONet && len(instanceID) > 6 {
			instanceID = instanceID[6:]
		}

		// Create the instance
		config := make(map[string]interface{})
		contractID, err := s.gpuService.CreateInstance(ctx, providerType, instanceID, config)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", selected.ID, err))
			continue
		}

		// Track connection if load balancer available
		if s.lbService != nil {
			s.lbService.TrackConnection(selected.ID)
		}

		// Convert to protobuf format
		metadata := make(map[string]string)
		metadata["contract_id"] = contractID
		if selected.Specifications != nil {
			for k, v := range selected.Specifications {
				if str, ok := v.(string); ok {
					metadata[k] = str
				} else {
					metadata[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		instanceStatus := "reserved"
		if !selected.Available {
			instanceStatus = "unavailable"
		}

		reserved = append(reserved, &pb.GPUInstance{
			Id:           selected.ID,
			Provider:     selected.Provider,
			Status:       instanceStatus,
			PricePerHour: selected.PricePerHour,
			VramGb:       int32(selected.VRAM),
			GpuModel:     selected.GPUName,
			NumGpus:      int32(selected.GPUCount),
			Location:     selected.Location,
			Metadata:     metadata,
		})
	}

	message := fmt.Sprintf("Successfully reserved %d GPU(s)", len(reserved))
	if len(errorMessages) > 0 {
		message = fmt.Sprintf("Reserved %d/%d GPUs. Errors: %s",
			len(reserved), count, errorMessages[0])
	}

	return &pb.ReserveGPUsResponse{
		ReservedInstances: reserved,
		ReservedCount:     int32(len(reserved)),
		Message:           message,
	}, nil
}

// HealthCheck performs a health check
func (s *Server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	details := map[string]string{
		"service": "gpuproxy-grpc",
		"version": "1.0.0",
	}

	// Convert details to JSON string for now
	detailsJSON, _ := json.Marshal(details)
	var detailsMap map[string]string
	json.Unmarshal(detailsJSON, &detailsMap)

	return &pb.HealthCheckResponse{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
		Details:   detailsMap,
	}, nil
}
