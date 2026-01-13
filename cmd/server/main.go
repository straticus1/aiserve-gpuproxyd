package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aiserve/gpuproxy/internal/api"
	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/billing"
	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/gpu"
	"github.com/aiserve/gpuproxy/internal/loadbalancer"
	"github.com/aiserve/gpuproxy/internal/middleware"
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

	authService := auth.NewService(db, redis, &cfg.Auth)
	billingService := billing.NewService(db, &cfg.Billing)
	gpuService := gpu.NewService(&cfg.GPU)
	protocolHandler := gpu.NewProtocolHandler(cfg.GPU.Timeout)
	lbService := loadbalancer.NewLoadBalancerService(loadbalancer.Strategy(cfg.LoadBalancer.Strategy))

	authHandler := api.NewAuthHandler(authService)
	billingHandler := api.NewBillingHandler(billingService)
	gpuHandler := api.NewGPUHandler(gpuService, protocolHandler, lbService)
	lbHandler := api.NewLoadBalancerHandler(lbService)
	userHandler := api.NewUserHandler(db)
	wsHandler := api.NewWebSocketHandler()

	authMiddleware := middleware.NewAuthMiddleware(authService, cfg.Auth.JWTSecret)
	rateLimiter := middleware.NewRateLimiter(redis)

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

	protected.HandleFunc("/auth/apikey", authHandler.CreateAPIKey).Methods("POST")

	protected.HandleFunc("/user/export", userHandler.ExportAccount).Methods("GET")

	protected.HandleFunc("/gpu/instances", gpuHandler.ListInstances).Methods("GET")
	protected.HandleFunc("/gpu/instances/batch", gpuHandler.BatchCreateInstances).Methods("POST")
	protected.HandleFunc("/gpu/instances/reserve", gpuHandler.ReserveInstances).Methods("POST")
	protected.HandleFunc("/gpu/instances/{provider}/{instanceId}", gpuHandler.CreateInstance).Methods("POST")
	protected.HandleFunc("/gpu/instances/{provider}/{instanceId}", gpuHandler.DestroyInstance).Methods("DELETE")
	protected.HandleFunc("/gpu/proxy", gpuHandler.ProxyRequest).Methods("POST")

	protected.HandleFunc("/loadbalancer/loads", lbHandler.GetLoads).Methods("GET")
	protected.HandleFunc("/loadbalancer/load", lbHandler.GetInstanceLoad).Methods("GET")
	protected.HandleFunc("/loadbalancer/strategy", lbHandler.GetStrategy).Methods("GET")
	protected.HandleFunc("/loadbalancer/strategy", lbHandler.SetStrategy).Methods("PUT")

	protected.HandleFunc("/billing/payment", billingHandler.CreatePayment).Methods("POST")
	protected.HandleFunc("/billing/transactions", billingHandler.GetTransactions).Methods("GET")

	router.HandleFunc("/ws", wsHandler.HandleConnection)

	if developerMode {
		router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/admin")))
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Printf("Starting GPU Proxy server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}
