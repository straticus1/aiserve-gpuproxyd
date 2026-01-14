package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aiserve/gpuproxy/internal/api"
	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/loadbalancer"
	"github.com/aiserve/gpuproxy/internal/mcp"
	"github.com/aiserve/gpuproxy/internal/a2a"
	"github.com/aiserve/gpuproxy/internal/acp"
	"github.com/aiserve/gpuproxy/internal/cuic"
	"github.com/aiserve/gpuproxy/internal/fipa"
	"github.com/aiserve/gpuproxy/internal/kqml"
	"github.com/aiserve/gpuproxy/internal/langchain"
	"github.com/aiserve/gpuproxy/internal/logging"
	"github.com/aiserve/gpuproxy/internal/metrics"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/aiserve/gpuproxy/internal/models"
	grpcServer "github.com/aiserve/gpuproxy/internal/grpc"
	"github.com/gorilla/mux"
)

var (
	developerMode bool
	debugMode     bool
)

func main() {
	// Go runtime optimizations for high-concurrency workloads
	setupRuntimeOptimizations()

	flag.BoolVar(&developerMode, "dv", false, "Enable developer mode")
	flag.BoolVar(&developerMode, "developer-mode", false, "Enable developer mode")
	flag.BoolVar(&debugMode, "dm", false, "Enable debug mode")
	flag.BoolVar(&debugMode, "debug-mode", false, "Enable debug mode")
	flag.Parse()

	if debugMode {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	if developerMode {
		log.Println("Developer mode enabled")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logging
	logCfg := logging.SyslogConfig{
		Enabled:  cfg.Logging.SyslogEnabled,
		Network:  cfg.Logging.SyslogNetwork,
		Address:  cfg.Logging.SyslogAddress,
		Tag:      cfg.Logging.SyslogTag,
		Facility: cfg.Logging.SyslogFacility,
		FilePath: cfg.Logging.LogFile,
	}

	if err := logging.Initialize(logCfg); err != nil {
		log.Printf("Warning: Failed to initialize logging: %v", err)
	}
	defer func() {
		if logger := logging.GetLogger(); logger != nil {
			logger.Close()
		}
	}()

	if debugMode {
		log.Printf("Configuration loaded: %+v", cfg.Server)
	}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	redis, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Detect local GPU backends
	log.Println("Detecting local GPU backends...")
	backends := gpu.DetectBackends()
	availableBackend := gpu.GetAvailableBackend(backends)

	if availableBackend != gpu.BackendNone {
		log.Printf("Local GPU backend available: %s", gpu.GetBackendInfo(backends))
		log.Printf("Using local backend: %s (preferred: %s)", availableBackend, cfg.GPU.PreferredBackend)
	} else {
		if cfg.GPU.VastAIAPIKey != "" || cfg.GPU.IONetAPIKey != "" {
			log.Println("No local GPU backend detected. Using cloud providers only.")
		} else {
			log.Println("WARNING: No local GPU backends and no cloud provider API keys configured.")
			log.Println("Server will start but GPU operations will fail until providers are configured.")
		}
	}

	authService := auth.NewService(db, redis, &cfg.Auth)
	billingService := billing.NewService(db, &cfg.Billing)
	gpuService := gpu.NewService(&cfg.GPU)
	protocolHandler := gpu.NewProtocolHandler(cfg.GPU.Timeout)
	lbService := loadbalancer.NewLoadBalancerService(loadbalancer.Strategy(cfg.LoadBalancer.Strategy))

	authMiddleware := middleware.NewAuthMiddleware(authService, cfg.Auth.JWTSecret)
	rateLimiter := middleware.NewRateLimiter(redis)
	guardRails := middleware.NewGuardRails(redis, &cfg.GuardRails)
	ipAccessControl := middleware.NewIPAccessControl(db.Pool)
	mcpServer := mcp.NewMCPServer(gpuService, billingService, authService, guardRails)
	a2aServer := a2a.NewA2AServer(gpuService, billingService, authService, guardRails)
	acpServer := acp.NewACPServer(gpuService, billingService, authService, guardRails)
	cuicServer := cuic.NewCUICServer(gpuService, billingService, authService, guardRails)
	fipaServer := fipa.NewFIPAServer(gpuService, billingService, authService, guardRails)
	kqmlServer := kqml.NewKQMLServer(gpuService, billingService, authService, guardRails)
	langchainServer := langchain.NewLangChainServer(gpuService, billingService, authService, guardRails)

	authHandler := api.NewAuthHandler(authService)
	billingHandler := api.NewBillingHandler(billingService)
	gpuHandler := api.NewGPUHandler(gpuService, protocolHandler, lbService)
	gpuPrefsHandler := api.NewGPUPreferencesHandler(db)
	lbHandler := api.NewLoadBalancerHandler(lbService)
	userHandler := api.NewUserHandler(db)
	wsHandler := api.NewWebSocketHandler()
	guardRailsHandler := api.NewGuardRailsHandler(guardRails)
	mcpHandler := api.NewMCPHandler(mcpServer)
	agentHandler := api.NewAgentHandler(a2aServer, acpServer, cuicServer, fipaServer, kqmlServer, langchainServer)
	ipAccessHandler := api.NewIPAccessHandler(db)

	// Initialize model serving if enabled
	var modelServeHandler *api.ModelServeHandler
	if cfg.ModelServing.Enabled {
		// Configure model registry storage path
		modelRegistry := models.GetModelRegistry()
		modelRegistry.SetStorageRoot(cfg.ModelServing.StoragePath)

		// Create model serving handler
		modelServeHandler = api.NewModelServeHandler()
		log.Printf("Model serving enabled. Storage path: %s", cfg.ModelServing.StoragePath)
	}

	// Initialize structured logger
	logLevel := logging.INFO
	if debugMode {
		logLevel = logging.DEBUG
	}
	logging.InitStructuredLogger("gpuproxy", logLevel)

	// Start metrics collection
	m := metrics.GetMetrics()
	m.StartCollection(context.Background())

	router := mux.NewRouter()

	router.Use(middleware.Recovery)
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.CORS)

	// Observability endpoints
	observabilityHandler := api.NewObservabilityHandler(db, redis)
	router.HandleFunc("/health", observabilityHandler.HandleHealth).Methods("GET")
	router.HandleFunc("/metrics", observabilityHandler.HandleMetrics).Methods("GET")
	router.HandleFunc("/stats", observabilityHandler.HandleStats).Methods("GET")
	router.HandleFunc("/polling", observabilityHandler.HandlePolling).Methods("GET")
	router.HandleFunc("/monitor", observabilityHandler.HandleMonitor).Methods("GET")

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/auth/register", authHandler.Register).Methods("POST")
	apiRouter.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	protected := apiRouter.PathPrefix("").Subrouter()
	protected.Use(authMiddleware.RequireAuth)
	protected.Use(ipAccessControl.Middleware()) // IP access control after auth
	protected.Use(rateLimiter.Limit(100))
	protected.Use(guardRails.Middleware())

	protected.HandleFunc("/auth/apikey", authHandler.CreateAPIKey).Methods("POST")

	protected.HandleFunc("/user/export", userHandler.ExportAccount).Methods("GET")

	protected.HandleFunc("/gpu/instances", gpuHandler.ListInstances).Methods("GET")
	protected.HandleFunc("/gpu/instances/batch", gpuHandler.BatchCreateInstances).Methods("POST")
	protected.HandleFunc("/gpu/instances/reserve", gpuHandler.ReserveInstances).Methods("POST")
	protected.HandleFunc("/gpu/instances/{provider}/{instanceId}", gpuHandler.CreateInstance).Methods("POST")
	protected.HandleFunc("/gpu/instances/{provider}/{instanceId}", gpuHandler.DestroyInstance).Methods("DELETE")
	protected.HandleFunc("/gpu/proxy", gpuHandler.ProxyRequest).Methods("POST")

	// GPU Preferences endpoints
	protected.HandleFunc("/gpu/preferences", gpuPrefsHandler.GetPreferences).Methods("GET")
	protected.HandleFunc("/gpu/preferences", gpuPrefsHandler.SetPreferences).Methods("POST")
	protected.HandleFunc("/gpu/preferences/test", gpuPrefsHandler.TestSelection).Methods("POST")
	protected.HandleFunc("/gpu/available", gpuPrefsHandler.GetAvailableGPUs).Methods("GET")
	protected.HandleFunc("/gpu/groups", gpuPrefsHandler.GetGPUGroups).Methods("GET")
	protected.HandleFunc("/gpu/classify", gpuPrefsHandler.ClassifyGPU).Methods("GET")

	// Public GPU info endpoints (no auth required)
	apiRouter.HandleFunc("/gpu/examples", gpuPrefsHandler.GetExamplePreferences).Methods("GET")

	protected.HandleFunc("/loadbalancer/loads", lbHandler.GetLoads).Methods("GET")
	protected.HandleFunc("/loadbalancer/load", lbHandler.GetInstanceLoad).Methods("GET")
	protected.HandleFunc("/loadbalancer/strategy", lbHandler.GetStrategy).Methods("GET")
	protected.HandleFunc("/loadbalancer/strategy", lbHandler.SetStrategy).Methods("PUT")

	protected.HandleFunc("/billing/payment", billingHandler.CreatePayment).Methods("POST")
	protected.HandleFunc("/billing/transactions", billingHandler.GetTransactions).Methods("GET")

	protected.HandleFunc("/guardrails/spending", guardRailsHandler.GetSpendingInfo).Methods("GET")
	protected.HandleFunc("/guardrails/spending/record", guardRailsHandler.RecordSpending).Methods("POST")
	protected.HandleFunc("/guardrails/spending/check", guardRailsHandler.CheckSpending).Methods("POST")
	protected.HandleFunc("/guardrails/spending/reset", guardRailsHandler.ResetSpending).Methods("POST")

	// IP Access Control endpoints
	protected.HandleFunc("/ip-access/config", ipAccessHandler.GetConfig).Methods("GET")
	protected.HandleFunc("/ip-access/config", ipAccessHandler.UpdateConfig).Methods("PUT")
	protected.HandleFunc("/ip-access/allowlist", ipAccessHandler.ListAllowlist).Methods("GET")
	protected.HandleFunc("/ip-access/allowlist", ipAccessHandler.AddAllowlist).Methods("POST")
	protected.HandleFunc("/ip-access/allowlist/{id}", ipAccessHandler.RemoveAllowlist).Methods("DELETE")
	protected.HandleFunc("/ip-access/denylist", ipAccessHandler.ListDenylist).Methods("GET")
	protected.HandleFunc("/ip-access/denylist", ipAccessHandler.AddDenylist).Methods("POST")
	protected.HandleFunc("/ip-access/denylist/{id}", ipAccessHandler.RemoveDenylist).Methods("DELETE")
	protected.HandleFunc("/ip-access/check", ipAccessHandler.CheckIP).Methods("POST")
	protected.HandleFunc("/ip-access/log", ipAccessHandler.GetAccessLog).Methods("GET")

	protected.HandleFunc("/mcp", mcpHandler.HandleMCP).Methods("POST")
	protected.HandleFunc("/mcp/sse", mcpHandler.HandleSSE).Methods("GET")

	protected.HandleFunc("/a2a", agentHandler.HandleA2A).Methods("POST")
	protected.HandleFunc("/acp", agentHandler.HandleACP).Methods("POST")
	protected.HandleFunc("/cuic", agentHandler.HandleCUIC).Methods("POST")
	protected.HandleFunc("/fipa", agentHandler.HandleFIPA).Methods("POST")
	protected.HandleFunc("/kqml", agentHandler.HandleKQML).Methods("POST")
	protected.HandleFunc("/langchain", agentHandler.HandleLangChain).Methods("POST")
	protected.HandleFunc("/langchain/tools", agentHandler.HandleLangChainTools).Methods("GET")

	protected.HandleFunc("/agent", agentHandler.HandleUnifiedAgent).Methods("POST")

	// Model serving endpoints (if enabled)
	if cfg.ModelServing.Enabled && modelServeHandler != nil {
		protected.HandleFunc("/models/upload", modelServeHandler.UploadModel).Methods("POST")
		protected.HandleFunc("/models", modelServeHandler.ListModels).Methods("GET")
		protected.HandleFunc("/models/{model_id}", modelServeHandler.GetModel).Methods("GET")
		protected.HandleFunc("/models/{model_id}", modelServeHandler.DeleteModel).Methods("DELETE")
		protected.HandleFunc("/models/{model_id}/predict", modelServeHandler.PredictModel).Methods("POST")
		protected.HandleFunc("/models/{model_id}/metrics", modelServeHandler.GetModelMetrics).Methods("GET")

		// Public endpoint for supported formats (no auth required)
		apiRouter.HandleFunc("/models/formats", modelServeHandler.SupportedFormats).Methods("GET")
	}

	router.HandleFunc("/agent/discover", agentHandler.HandleAgentDiscovery).Methods("GET")
	router.HandleFunc("/ws", wsHandler.HandleConnection)

	// Serve admin dashboard at /admin
	router.PathPrefix("/admin").Handler(http.StripPrefix("/admin", http.FileServer(http.Dir("./web/admin"))))

	// Serve main website (must be last to catch all other routes)
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))

	// Initialize gRPC server with IP access control
	grpcSrv := grpcServer.NewServer(authService, gpuService, protocolHandler, billingService, lbService, cuicServer, ipAccessControl)

	// Format address properly for IPv6 (needs brackets)
	grpcHost := cfg.Server.Host
	if strings.Contains(grpcHost, ":") {
		grpcHost = "[" + grpcHost + "]"
	}
	grpcAddr := fmt.Sprintf("%s:%d", grpcHost, cfg.Server.GRPCPort)

	go func() {
		log.Printf("Starting gRPC server on %s", grpcAddr)
		if err := grpcSrv.Start(grpcAddr, cfg.Server.GRPCTLSCert, cfg.Server.GRPCTLSKey); err != nil {
			log.Fatalf("gRPC server failed: %v", err)
		}
	}()

	// Format address properly for IPv6 (needs brackets)
	httpHost := cfg.Server.Host
	if strings.Contains(httpHost, ":") {
		httpHost = "[" + httpHost + "]"
	}
	addr := fmt.Sprintf("%s:%d", httpHost, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,

		// Request timeouts optimized for GPU workloads
		ReadTimeout:       30 * time.Second,  // Increased from 15s for GPU operations
		ReadHeaderTimeout: 10 * time.Second,  // Protect against slow clients (Slowloris)
		WriteTimeout:      120 * time.Second, // Increased from 15s for streaming responses
		IdleTimeout:       120 * time.Second, // Increased from 60s for persistent connections

		// Resource limits
		MaxHeaderBytes: 1 << 20, // 1MB max headers (prevent memory exhaustion)
	}

	go func() {
		log.Printf("Starting HTTP server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown gRPC server
	grpcSrv.Stop()
	log.Println("gRPC server stopped")

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP server forced to shutdown: %v", err)
	}

	log.Println("Servers exited gracefully")
}

func setupRuntimeOptimizations() {
	// GOMAXPROCS tuning for container environments
	// In Docker, Go may not detect CPU limits correctly
	numCPU := runtime.NumCPU()
	if cpuLimit := os.Getenv("CPU_LIMIT"); cpuLimit != "" {
		if limit, err := strconv.Atoi(cpuLimit); err == nil && limit > 0 {
			numCPU = limit
		}
	}
	runtime.GOMAXPROCS(numCPU)
	log.Printf("GOMAXPROCS set to %d", numCPU)

	// GC tuning for high-throughput, low-latency
	// Increase GC target percentage to reduce GC frequency
	// Default is 100, we use 200 for high-throughput
	debug.SetGCPercent(200)

	// Set memory limit if specified (Go 1.19+)
	if memLimit := os.Getenv("GOMEMLIMIT"); memLimit != "" {
		if limit := parseMemoryLimit(memLimit); limit > 0 {
			debug.SetMemoryLimit(limit)
			log.Printf("Go memory limit set to %s", memLimit)
		}
	}

	log.Println("Runtime optimizations applied")
}

func parseMemoryLimit(limit string) int64 {
	// Parse "2GB", "512MB", etc.
	var value int64
	var unit string
	if n, err := fmt.Sscanf(limit, "%d%s", &value, &unit); n != 2 || err != nil {
		return 0
	}

	switch strings.ToUpper(unit) {
	case "GB", "G":
		return value * 1024 * 1024 * 1024
	case "MB", "M":
		return value * 1024 * 1024
	case "KB", "K":
		return value * 1024
	default:
		return value
	}
}
