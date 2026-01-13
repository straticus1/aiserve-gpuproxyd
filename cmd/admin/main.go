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
