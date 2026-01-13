package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/middleware"
	"github.com/google/uuid"
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

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	redis, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	authService := auth.NewService(db, redis, &cfg.Auth)
	guardRails := middleware.NewGuardRails(redis, &cfg.GuardRails)

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	ctx := context.Background()

	switch command {
	case "users":
		listUsers(ctx, db)

	case "create-user":
		if len(args) < 4 {
			log.Fatal("Usage: admin create-user <email> <password> <name>")
		}
		createUser(ctx, authService, args[1], args[2], args[3])

	case "make-admin":
		if len(args) < 2 {
			log.Fatal("Usage: admin make-admin <email>")
		}
		makeAdmin(ctx, db, args[1])

	case "create-apikey":
		if len(args) < 3 {
			log.Fatal("Usage: admin create-apikey <user-email> <key-name>")
		}
		createAPIKey(ctx, db, authService, args[1], args[2])

	case "usage":
		if len(args) < 2 {
			log.Fatal("Usage: admin usage <user-email>")
		}
		showUsage(ctx, db, args[1])

	case "migrate":
		if err := db.Migrate(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("Database migration completed successfully")

	case "stats":
		showStats(ctx, db)

	case "guardrails-status":
		showGuardRailsStatus(cfg)

	case "guardrails-spending":
		if len(args) < 2 {
			log.Fatal("Usage: admin guardrails-spending <user-email>")
		}
		showGuardRailsSpending(ctx, db, guardRails, args[1])

	case "guardrails-reset":
		if len(args) < 2 {
			log.Fatal("Usage: admin guardrails-reset <user-email> [window]")
		}
		window := "all"
		if len(args) >= 3 {
			window = args[2]
		}
		resetGuardRailsSpending(ctx, db, guardRails, args[1], window)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("GPU Proxy Admin Utility")
	fmt.Println("\nUsage:")
	fmt.Println("  admin [flags] <command> [args]")
	fmt.Println("\nFlags:")
	fmt.Println("  -dv, -developer-mode    Enable developer mode")
	fmt.Println("  -dm, -debug-mode        Enable debug mode")
	fmt.Println("\nCommands:")
	fmt.Println("  users                                List all users")
	fmt.Println("  create-user <email> <pass> <name>    Create a new user")
	fmt.Println("  make-admin <email>                   Grant admin privileges to user")
	fmt.Println("  create-apikey <email> <name>         Create API key for user")
	fmt.Println("  usage <email>                        Show usage stats for user")
	fmt.Println("  migrate                              Run database migrations")
	fmt.Println("  stats                                Show system statistics")
	fmt.Println("  guardrails-status                    Show guard rails configuration")
	fmt.Println("  guardrails-spending <email>          Show spending by time window for user")
	fmt.Println("  guardrails-reset <email> [window]    Reset spending tracking (default: all)")
}

func listUsers(ctx context.Context, db *database.PostgresDB) {
	query := `SELECT id, email, name, is_admin, is_active, created_at FROM users ORDER BY created_at DESC`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tEMAIL\tNAME\tADMIN\tACTIVE\tCREATED")
	fmt.Fprintln(w, "---\t---\t---\t---\t---\t---")

	count := 0
	for rows.Next() {
		var id uuid.UUID
		var email, name string
		var isAdmin, isActive bool
		var createdAt time.Time

		if err := rows.Scan(&id, &email, &name, &isAdmin, &isActive, &createdAt); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		adminStr := "No"
		if isAdmin {
			adminStr = "Yes"
		}

		activeStr := "No"
		if isActive {
			activeStr = "Yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			id.String()[:8], email, name, adminStr, activeStr, createdAt.Format("2006-01-02"))

		count++
	}

	w.Flush()
	fmt.Printf("\nTotal: %d users\n", count)
}

func createUser(ctx context.Context, authService *auth.Service, email, password, name string) {
	user, err := authService.Register(ctx, email, password, name)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("User created successfully\n")
	fmt.Printf("ID: %s\n", user.ID)
	fmt.Printf("Email: %s\n", user.Email)
	fmt.Printf("Name: %s\n", user.Name)
}

func makeAdmin(ctx context.Context, db *database.PostgresDB, email string) {
	query := `UPDATE users SET is_admin = true WHERE email = $1 RETURNING id, email, name`

	var id uuid.UUID
	var userEmail, name string

	err := db.Pool.QueryRow(ctx, query, email).Scan(&id, &userEmail, &name)
	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	fmt.Printf("User %s (%s) is now an admin\n", name, userEmail)
}

func createAPIKey(ctx context.Context, db *database.PostgresDB, authService *auth.Service, email, keyName string) {
	query := `SELECT id FROM users WHERE email = $1`

	var userID uuid.UUID
	if err := db.Pool.QueryRow(ctx, query, email).Scan(&userID); err != nil {
		log.Fatalf("User not found: %v", err)
	}

	apiKey, err := authService.CreateAPIKey(ctx, userID, keyName, nil)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Printf("API key created successfully\n")
	fmt.Printf("User: %s\n", email)
	fmt.Printf("Name: %s\n", keyName)
	fmt.Printf("Key: %s\n", apiKey)
	fmt.Println("\nSave this API key securely. It will not be shown again.")
}

func showUsage(ctx context.Context, db *database.PostgresDB, email string) {
	query := `
		SELECT u.name, u.email, uq.max_gpu_hours, uq.used_gpu_hours,
		       uq.max_requests, uq.used_requests, uq.reset_at
		FROM users u
		JOIN usage_quotas uq ON u.id = uq.user_id
		WHERE u.email = $1
	`

	var name, userEmail string
	var maxGPUHours, usedGPUHours float64
	var maxRequests, usedRequests int64
	var resetAt time.Time

	err := db.Pool.QueryRow(ctx, query, email).Scan(
		&name, &userEmail, &maxGPUHours, &usedGPUHours,
		&maxRequests, &usedRequests, &resetAt,
	)
	if err != nil {
		log.Fatalf("Failed to get usage: %v", err)
	}

	fmt.Printf("Usage Statistics for %s (%s)\n", name, userEmail)
	fmt.Println("=====================================")
	fmt.Printf("GPU Hours: %.2f / %.2f (%.1f%%)\n",
		usedGPUHours, maxGPUHours, (usedGPUHours/maxGPUHours)*100)
	fmt.Printf("Requests: %d / %d (%.1f%%)\n",
		usedRequests, maxRequests, (float64(usedRequests)/float64(maxRequests))*100)
	fmt.Printf("Quota resets: %s\n", resetAt.Format("2006-01-02 15:04:05"))
}

func showStats(ctx context.Context, db *database.PostgresDB) {
	var userCount, activeUserCount, apiKeyCount, txCount int64
	var totalGPUHours float64

	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&userCount)
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE is_active = true").Scan(&activeUserCount)
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM api_keys WHERE is_active = true").Scan(&apiKeyCount)
	db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM billing_transactions").Scan(&txCount)
	db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(duration), 0) FROM gpu_usage").Scan(&totalGPUHours)

	fmt.Println("System Statistics")
	fmt.Println("==================")
	fmt.Printf("Total Users: %d\n", userCount)
	fmt.Printf("Active Users: %d\n", activeUserCount)
	fmt.Printf("Active API Keys: %d\n", apiKeyCount)
	fmt.Printf("Total Transactions: %d\n", txCount)
	fmt.Printf("Total GPU Hours Used: %.2f\n", totalGPUHours)
}

func showGuardRailsStatus(cfg *config.Config) {
	fmt.Println("Guard Rails Configuration")
	fmt.Println("=========================")
	fmt.Printf("Enabled: %v\n\n", cfg.GuardRails.Enabled)

	if !cfg.GuardRails.Enabled {
		fmt.Println("Guard rails are currently disabled.")
		fmt.Println("Set GUARDRAILS_ENABLED=true in .env to enable.")
		return
	}

	fmt.Println("Spending Limits by Time Window:")
	fmt.Println("--------------------------------")

	limits := []struct {
		Name  string
		Value float64
	}{
		{"5 minutes", cfg.GuardRails.Max5MinRate},
		{"15 minutes", cfg.GuardRails.Max15MinRate},
		{"30 minutes", cfg.GuardRails.Max30MinRate},
		{"60 minutes (1h)", cfg.GuardRails.Max60MinRate},
		{"90 minutes (1.5h)", cfg.GuardRails.Max90MinRate},
		{"120 minutes (2h)", cfg.GuardRails.Max120MinRate},
		{"240 minutes (4h)", cfg.GuardRails.Max240MinRate},
		{"300 minutes (5h)", cfg.GuardRails.Max300MinRate},
		{"360 minutes (6h)", cfg.GuardRails.Max360MinRate},
		{"400 minutes (6.67h)", cfg.GuardRails.Max400MinRate},
		{"460 minutes (7.67h)", cfg.GuardRails.Max460MinRate},
		{"520 minutes (8.67h)", cfg.GuardRails.Max520MinRate},
		{"640 minutes (10.67h)", cfg.GuardRails.Max640MinRate},
		{"700 minutes (11.67h)", cfg.GuardRails.Max700MinRate},
		{"1440 minutes (24h)", cfg.GuardRails.Max1440MinRate},
		{"48 hours", cfg.GuardRails.Max48HRate},
		{"72 hours", cfg.GuardRails.Max72HRate},
	}

	activeCount := 0
	for _, limit := range limits {
		if limit.Value > 0 {
			fmt.Printf("  %-25s $%.2f\n", limit.Name+":", limit.Value)
			activeCount++
		}
	}

	if activeCount == 0 {
		fmt.Println("\nNo time window limits are currently configured.")
		fmt.Println("Configure limits in .env (e.g., GUARDRAILS_MAX_60MIN_RATE=100.00)")
	} else {
		fmt.Printf("\nTotal active limits: %d\n", activeCount)
	}
}

func showGuardRailsSpending(ctx context.Context, db *database.PostgresDB, gr *middleware.GuardRails, email string) {
	query := `SELECT id, name FROM users WHERE email = $1`

	var userID uuid.UUID
	var userName string
	if err := db.Pool.QueryRow(ctx, query, email).Scan(&userID, &userName); err != nil {
		log.Fatalf("User not found: %v", err)
	}

	info, err := gr.GetSpendingInfo(ctx, userID)
	if err != nil {
		log.Fatalf("Failed to get spending info: %v", err)
	}

	fmt.Printf("Guard Rails Spending for %s (%s)\n", userName, email)
	fmt.Println("========================================")
	fmt.Printf("User ID: %s\n", userID.String())
	fmt.Printf("Timestamp: %s\n\n", info.Timestamp.Format("2006-01-02 15:04:05"))

	if len(info.WindowSpent) == 0 {
		fmt.Println("No spending data recorded yet.")
		return
	}

	fmt.Println("Spending by Time Window:")
	fmt.Println("------------------------")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WINDOW\tSPENT\tSTATUS")
	fmt.Fprintln(w, "------\t-----\t------")

	windows := []string{"5min", "15min", "30min", "60min", "90min", "120min",
		"240min", "300min", "360min", "400min", "460min", "520min",
		"640min", "700min", "1440min", "48h", "72h"}

	for _, window := range windows {
		if spent, ok := info.WindowSpent[window]; ok {
			status := "OK"
			if len(info.Violations) > 0 {
				for _, violation := range info.Violations {
					if len(violation) > len(window) && violation[:len(window)] == window {
						status = "EXCEEDED"
						break
					}
				}
			}
			fmt.Fprintf(w, "%s\t$%.2f\t%s\n", window, spent, status)
		}
	}

	w.Flush()

	if len(info.Violations) > 0 {
		fmt.Println("\nLIMIT VIOLATIONS:")
		for _, violation := range info.Violations {
			fmt.Printf("  ! %s\n", violation)
		}
	}
}

func resetGuardRailsSpending(ctx context.Context, db *database.PostgresDB, gr *middleware.GuardRails, email, window string) {
	query := `SELECT id, name FROM users WHERE email = $1`

	var userID uuid.UUID
	var userName string
	if err := db.Pool.QueryRow(ctx, query, email).Scan(&userID, &userName); err != nil {
		log.Fatalf("User not found: %v", err)
	}

	if err := gr.ResetSpending(ctx, userID, window); err != nil {
		log.Fatalf("Failed to reset spending: %v", err)
	}

	if window == "all" {
		fmt.Printf("Successfully reset all spending tracking for %s (%s)\n", userName, email)
	} else {
		fmt.Printf("Successfully reset %s spending tracking for %s (%s)\n", window, userName, email)
	}
}
