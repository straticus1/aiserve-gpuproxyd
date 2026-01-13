package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/aiserve/gpuproxy/internal/middleware"
	grpcServer "github.com/aiserve/gpuproxy/internal/grpc"
	"github.com/gorilla/mux"
)

var (
	developerMode bool
	debugMode     bool
)

func main() {
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

	router := mux.NewRouter()

	router.Use(middleware.Recovery)
	router.Use(middleware.Logger)
	router.Use(middleware.CORS)

	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	}).Methods("GET")

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/auth/register", authHandler.Register).Methods("POST")
	apiRouter.HandleFunc("/auth/login", authHandler.Login).Methods("POST")

	protected := apiRouter.PathPrefix("").Subrouter()
	protected.Use(authMiddleware.RequireAuth)
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

	router.HandleFunc("/agent/discover", agentHandler.HandleAgentDiscovery).Methods("GET")
	router.HandleFunc("/ws", wsHandler.HandleConnection)

	if developerMode {
		router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/admin")))
	}

	// Initialize gRPC server
	grpcSrv := grpcServer.NewServer(authService, gpuService, protocolHandler, billingService, lbService, cuicServer)

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
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
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
