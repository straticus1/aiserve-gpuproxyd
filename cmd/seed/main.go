package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aiserve/gpuproxy/internal/auth"
	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
)

var (
	configFile string
	dryRun     bool
)

func main() {
	flag.StringVar(&configFile, "config", ".local_admin", "Path to local admin config file")
	flag.BoolVar(&dryRun, "dry-run", false, "Print what would be created without actually creating")
	flag.Parse()

	log.Printf("Loading configuration from: %s", configFile)

	// Load main application config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect to Redis
	redis, err := database.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Create auth service
	authService := auth.NewService(db, redis, &cfg.Auth)

	// Parse the .local_admin config file
	admins, clients, err := parseConfigFile(configFile)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	if dryRun {
		log.Println("\n=== DRY RUN MODE - No changes will be made ===\n")
		printPlan(admins, clients)
		return
	}

	ctx := context.Background()

	// Seed admin users
	log.Println("\n=== Seeding Admin Users ===")
	for name, password := range admins {
		email := fmt.Sprintf("%s@admin.local", name) // XXX: CHANGEME
		if err := seedUser(ctx, db, authService, email, password, name, true); err != nil {
			log.Printf("Warning: Failed to seed admin %s: %v", name, err)
		}
	}

	// Seed client users
	log.Println("\n=== Seeding Client Users ===")
	for name, password := range clients {
		email := fmt.Sprintf("%s@client.local", name) // XXX: CHANGEME
		if err := seedUser(ctx, db, authService, email, password, name, false); err != nil {
			log.Printf("Warning: Failed to seed client %s: %v", name, err)
		}
	}

	log.Println("\n=== Seeding Complete ===")
	log.Println("\nYou can now use these credentials for testing:")
	log.Println("  Admin users: ryan@admin.local, david@admin.local")
	log.Println("  Client users: client1@client.local, client2@client.local, client3@client.local")
	log.Println("\nTo create API keys for these users, use:")
	log.Println("  ./admin create-apikey <email> <key-name>")
}

func parseConfigFile(path string) (map[string]string, map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	admins := make(map[string]string)
	clients := make(map[string]string)
	currentSection := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			log.Printf("Warning: Skipping invalid line: %s", line)
			continue
		}

		name := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])

		switch currentSection {
		case "admins":
			admins[name] = password
		case "clients":
			clients[name] = password
		default:
			log.Printf("Warning: Unknown section '%s', skipping entry: %s", currentSection, name)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading config file: %w", err)
	}

	return admins, clients, nil
}

func printPlan(admins, clients map[string]string) {
	fmt.Println("Would create the following users:\n")

	fmt.Println("Admins:")
	for name := range admins {
		fmt.Printf("  - %s@admin.local (admin=true)\n", name)
	}

	fmt.Println("\nClients:")
	for name := range clients {
		fmt.Printf("  - %s@client.local (admin=false)\n", name)
	}

	fmt.Println("\nTo proceed, run without --dry-run flag")
}

func seedUser(ctx context.Context, db *database.PostgresDB, authService *auth.Service, email, password, name string, isAdmin bool) error {
	// Check if user already exists
	var exists bool
	err := db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if user exists: %w", err)
	}

	if exists {
		log.Printf("  ✓ User %s (%s) already exists, skipping", name, email)
		return nil
	}

	// Create the user
	user, err := authService.Register(ctx, email, password, name)
	if err != nil {
		return fmt.Errorf("failed to register user: %w", err)
	}

	// Make admin if needed
	if isAdmin {
		query := `UPDATE users SET is_admin = true WHERE id = $1`
		_, err = db.Pool.Exec(ctx, query, user.ID)
		if err != nil {
			return fmt.Errorf("failed to grant admin privileges: %w", err)
		}
	}

	// Create a default API key for the user
	apiKey, err := authService.CreateAPIKey(ctx, user.ID, fmt.Sprintf("%s-default-key", name), nil) // XXX: CHANGEME
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	adminStatus := "client"
	if isAdmin {
		adminStatus = "admin"
	}

	log.Printf("  ✓ Created %s user: %s (%s)", adminStatus, name, email)
	log.Printf("    API Key: %s", apiKey) // XXX: CHANGEME - This key is for testing only!

	return nil
}
