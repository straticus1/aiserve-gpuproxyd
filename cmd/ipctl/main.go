package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
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

	command := os.Args[1]
	ctx := context.Background()

	switch command {
	case "config":
		handleConfig(ctx, db, os.Args[2:])
	case "allowlist":
		handleAllowlist(ctx, db, os.Args[2:])
	case "denylist":
		handleDenylist(ctx, db, os.Args[2:])
	case "check":
		handleCheck(ctx, db, os.Args[2:])
	case "log":
		handleLog(ctx, db, os.Args[2:])
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`ipctl - IP Access Control Management Tool

Usage:
  ipctl <command> [options]

Commands:
  config      Manage IP access configuration
  allowlist   Manage IP allowlist
  denylist    Manage IP denylist
  check       Check if an IP would be allowed
  log         View IP access logs
  help        Show this help message

Config Commands:
  ipctl config get --user-email <email>
  ipctl config set --user-email <email> --mode <disabled|allowlist|denylist|strict>
  ipctl config enable-allowlist --user-email <email>
  ipctl config enable-denylist --user-email <email>
  ipctl config enable-audit --user-email <email>

Allowlist Commands:
  ipctl allowlist list --user-email <email>
  ipctl allowlist add --user-email <email> --ip <ip_address> [--description <text>] [--range <cidr>]
  ipctl allowlist remove --user-email <email> --ip <ip_address>

Denylist Commands:
  ipctl denylist list --user-email <email>
  ipctl denylist add --user-email <email> --ip <ip_address> [--reason <text>] [--expires <hours>] [--range <cidr>]
  ipctl denylist remove --user-email <email> --ip <ip_address>

Check Command:
  ipctl check --user-email <email> --ip <ip_address>

Log Command:
  ipctl log --user-email <email> [--limit <number>]

Examples:
  # Set allowlist mode for user
  ipctl config set --user-email user@example.com --mode allowlist

  # Add IP to allowlist
  ipctl allowlist add --user-email user@example.com --ip 203.0.113.5 --description "Office IP"

  # Add IP range to denylist
  ipctl denylist add --user-email user@example.com --ip 192.168.1.100 --range "192.168.1.0/24" --reason "Suspicious activity"

  # Check if IP is allowed
  ipctl check --user-email user@example.com --ip 203.0.113.5

  # View access logs
  ipctl log --user-email user@example.com --limit 50
`)
}

func getUserID(ctx context.Context, db *database.PostgresDB, email string) (string, error) {
	query := `SELECT id FROM users WHERE email = $1`
	var userID string
	err := db.Pool.QueryRow(ctx, query, email).Scan(&userID)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	return userID, nil
}

func handleConfig(ctx context.Context, db *database.PostgresDB, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ipctl config <get|set|enable-allowlist|enable-denylist|enable-audit> --user-email <email>")
		os.Exit(1)
	}

	subcommand := args[0]
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	userEmail := fs.String("user-email", "", "User email")
	mode := fs.String("mode", "", "IP access mode (disabled, allowlist, denylist, strict)")
	fs.Parse(args[1:])

	if *userEmail == "" {
		fmt.Println("Error: --user-email is required")
		os.Exit(1)
	}

	userID, err := getUserID(ctx, db, *userEmail)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	switch subcommand {
	case "get":
		query := `
			SELECT mode, allowlist_enabled, denylist_enabled, block_on_no_match, audit_log_enabled
			FROM ip_access_config WHERE user_id = $1
		`
		var config models.IPAccessConfig
		err := db.Pool.QueryRow(ctx, query, userID).Scan(
			&config.Mode, &config.AllowlistEnabled, &config.DenylistEnabled,
			&config.BlockOnNoMatch, &config.AuditLogEnabled,
		)
		if err != nil {
			if err.Error() == "no rows in result set" {
				fmt.Println("No IP access configuration found (default: disabled)")
				return
			}
			log.Fatalf("Error: %v", err)
		}

		fmt.Printf("IP Access Configuration for %s:\n", *userEmail)
		fmt.Printf("  Mode:              %s\n", config.Mode)
		fmt.Printf("  Allowlist Enabled: %v\n", config.AllowlistEnabled)
		fmt.Printf("  Denylist Enabled:  %v\n", config.DenylistEnabled)
		fmt.Printf("  Block on No Match: %v\n", config.BlockOnNoMatch)
		fmt.Printf("  Audit Log Enabled: %v\n", config.AuditLogEnabled)

	case "set":
		if *mode == "" {
			fmt.Println("Error: --mode is required")
			os.Exit(1)
		}

		query := `
			INSERT INTO ip_access_config (id, user_id, mode, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE SET mode = EXCLUDED.mode, updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query, uuid.New().String(), userID, *mode, time.Now(), time.Now())
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("IP access mode set to '%s' for %s\n", *mode, *userEmail)

	case "enable-allowlist":
		query := `
			INSERT INTO ip_access_config (id, user_id, allowlist_enabled, created_at, updated_at)
			VALUES ($1, $2, TRUE, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET allowlist_enabled = TRUE, updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query, uuid.New().String(), userID, time.Now(), time.Now())
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Allowlist enabled for %s\n", *userEmail)

	case "enable-denylist":
		query := `
			INSERT INTO ip_access_config (id, user_id, denylist_enabled, created_at, updated_at)
			VALUES ($1, $2, TRUE, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET denylist_enabled = TRUE, updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query, uuid.New().String(), userID, time.Now(), time.Now())
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Denylist enabled for %s\n", *userEmail)

	case "enable-audit":
		query := `
			INSERT INTO ip_access_config (id, user_id, audit_log_enabled, created_at, updated_at)
			VALUES ($1, $2, TRUE, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET audit_log_enabled = TRUE, updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query, uuid.New().String(), userID, time.Now(), time.Now())
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Audit logging enabled for %s\n", *userEmail)

	default:
		fmt.Printf("Unknown config subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleAllowlist(ctx context.Context, db *database.PostgresDB, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ipctl allowlist <list|add|remove> --user-email <email>")
		os.Exit(1)
	}

	subcommand := args[0]
	fs := flag.NewFlagSet("allowlist", flag.ExitOnError)
	userEmail := fs.String("user-email", "", "User email")
	ip := fs.String("ip", "", "IP address")
	ipRange := fs.String("range", "", "CIDR range (e.g., 192.168.1.0/24)")
	description := fs.String("description", "", "Description")
	fs.Parse(args[1:])

	if *userEmail == "" {
		fmt.Println("Error: --user-email is required")
		os.Exit(1)
	}

	userID, err := getUserID(ctx, db, *userEmail)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	switch subcommand {
	case "list":
		query := `
			SELECT id, ip_address, ip_range, description, is_active, created_at
			FROM ip_allowlist WHERE user_id = $1 ORDER BY created_at DESC
		`
		rows, err := db.Pool.Query(ctx, query, userID)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "IP ADDRESS\tCIDR RANGE\tDESCRIPTION\tACTIVE\tCREATED\n")

		for rows.Next() {
			var id, ipAddr string
			var ipRng, desc *string
			var isActive bool
			var createdAt time.Time

			rows.Scan(&id, &ipAddr, &ipRng, &desc, &isActive, &createdAt)

			cidrStr := "-"
			if ipRng != nil {
				cidrStr = *ipRng
			}
			descStr := "-"
			if desc != nil {
				descStr = *desc
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\n",
				ipAddr, cidrStr, descStr, isActive, createdAt.Format("2006-01-02 15:04"))
		}
		w.Flush()

	case "add":
		if *ip == "" {
			fmt.Println("Error: --ip is required")
			os.Exit(1)
		}

		var ipRngPtr *string
		if *ipRange != "" {
			ipRngPtr = ipRange
		}

		query := `
			INSERT INTO ip_allowlist (id, user_id, ip_address, ip_range, description, is_active, created_at, updated_at, created_by)
			VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8)
			ON CONFLICT (user_id, ip_address) DO UPDATE SET
				ip_range = EXCLUDED.ip_range,
				description = EXCLUDED.description,
				is_active = TRUE,
				updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query,
			uuid.New().String(), userID, *ip, ipRngPtr, *description,
			time.Now(), time.Now(), "CLI:"+*userEmail,
		)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Added %s to allowlist for %s\n", *ip, *userEmail)

	case "remove":
		if *ip == "" {
			fmt.Println("Error: --ip is required")
			os.Exit(1)
		}

		query := `DELETE FROM ip_allowlist WHERE user_id = $1 AND ip_address = $2`
		result, err := db.Pool.Exec(ctx, query, userID, *ip)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if result.RowsAffected() == 0 {
			fmt.Printf("IP %s not found in allowlist\n", *ip)
		} else {
			fmt.Printf("Removed %s from allowlist for %s\n", *ip, *userEmail)
		}

	default:
		fmt.Printf("Unknown allowlist subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleDenylist(ctx context.Context, db *database.PostgresDB, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ipctl denylist <list|add|remove> --user-email <email>")
		os.Exit(1)
	}

	subcommand := args[0]
	fs := flag.NewFlagSet("denylist", flag.ExitOnError)
	userEmail := fs.String("user-email", "", "User email")
	ip := fs.String("ip", "", "IP address")
	ipRange := fs.String("range", "", "CIDR range")
	reason := fs.String("reason", "", "Reason for blocking")
	expiresHours := fs.Int("expires", 0, "Expires in hours (0 = never)")
	fs.Parse(args[1:])

	if *userEmail == "" {
		fmt.Println("Error: --user-email is required")
		os.Exit(1)
	}

	userID, err := getUserID(ctx, db, *userEmail)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	switch subcommand {
	case "list":
		query := `
			SELECT id, ip_address, ip_range, reason, is_active, expires_at, created_at
			FROM ip_denylist WHERE user_id = $1 ORDER BY created_at DESC
		`
		rows, err := db.Pool.Query(ctx, query, userID)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		defer rows.Close()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "IP ADDRESS\tCIDR RANGE\tREASON\tACTIVE\tEXPIRES\tCREATED\n")

		for rows.Next() {
			var id, ipAddr string
			var ipRng, rsn *string
			var isActive bool
			var expiresAt *time.Time
			var createdAt time.Time

			rows.Scan(&id, &ipAddr, &ipRng, &rsn, &isActive, &expiresAt, &createdAt)

			cidrStr := "-"
			if ipRng != nil {
				cidrStr = *ipRng
			}
			rsnStr := "-"
			if rsn != nil {
				rsnStr = *rsn
			}
			expStr := "Never"
			if expiresAt != nil {
				expStr = expiresAt.Format("2006-01-02 15:04")
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\t%s\n",
				ipAddr, cidrStr, rsnStr, isActive, expStr, createdAt.Format("2006-01-02 15:04"))
		}
		w.Flush()

	case "add":
		if *ip == "" {
			fmt.Println("Error: --ip is required")
			os.Exit(1)
		}

		var ipRngPtr *string
		if *ipRange != "" {
			ipRngPtr = ipRange
		}

		var expiresAt *time.Time
		if *expiresHours > 0 {
			exp := time.Now().Add(time.Duration(*expiresHours) * time.Hour)
			expiresAt = &exp
		}

		query := `
			INSERT INTO ip_denylist (id, user_id, ip_address, ip_range, reason, is_active, expires_at, created_at, updated_at, created_by)
			VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8, $9)
			ON CONFLICT (user_id, ip_address) DO UPDATE SET
				ip_range = EXCLUDED.ip_range,
				reason = EXCLUDED.reason,
				is_active = TRUE,
				expires_at = EXCLUDED.expires_at,
				updated_at = EXCLUDED.updated_at
		`
		_, err := db.Pool.Exec(ctx, query,
			uuid.New().String(), userID, *ip, ipRngPtr, *reason, expiresAt,
			time.Now(), time.Now(), "CLI:"+*userEmail,
		)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		fmt.Printf("Added %s to denylist for %s\n", *ip, *userEmail)

	case "remove":
		if *ip == "" {
			fmt.Println("Error: --ip is required")
			os.Exit(1)
		}

		query := `DELETE FROM ip_denylist WHERE user_id = $1 AND ip_address = $2`
		result, err := db.Pool.Exec(ctx, query, userID, *ip)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		if result.RowsAffected() == 0 {
			fmt.Printf("IP %s not found in denylist\n", *ip)
		} else {
			fmt.Printf("Removed %s from denylist for %s\n", *ip, *userEmail)
		}

	default:
		fmt.Printf("Unknown denylist subcommand: %s\n", subcommand)
		os.Exit(1)
	}
}

func handleCheck(ctx context.Context, db *database.PostgresDB, args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	userEmail := fs.String("user-email", "", "User email")
	ip := fs.String("ip", "", "IP address to check")
	fs.Parse(args)

	if *userEmail == "" || *ip == "" {
		fmt.Println("Error: --user-email and --ip are required")
		os.Exit(1)
	}

	userID, err := getUserID(ctx, db, *userEmail)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Check denylist
	denyQuery := `
		SELECT COUNT(*) FROM ip_denylist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
		  AND (expires_at IS NULL OR expires_at > NOW())
	`
	var denyCount int
	db.Pool.QueryRow(ctx, denyQuery, userID, *ip).Scan(&denyCount)

	// Check allowlist
	allowQuery := `
		SELECT COUNT(*) FROM ip_allowlist
		WHERE user_id = $1 AND ip_address = $2 AND is_active = TRUE
	`
	var allowCount int
	db.Pool.QueryRow(ctx, allowQuery, userID, *ip).Scan(&allowCount)

	fmt.Printf("IP Access Check for %s from %s:\n", *userEmail, *ip)
	if denyCount > 0 {
		fmt.Println("  Result: BLOCKED (in denylist)")
	} else if allowCount > 0 {
		fmt.Println("  Result: ALLOWED (in allowlist)")
	} else {
		fmt.Println("  Result: ALLOWED (no restrictions)")
	}
}

func handleLog(ctx context.Context, db *database.PostgresDB, args []string) {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	userEmail := fs.String("user-email", "", "User email")
	limit := fs.Int("limit", 20, "Number of log entries")
	fs.Parse(args)

	if *userEmail == "" {
		fmt.Println("Error: --user-email is required")
		os.Exit(1)
	}

	userID, err := getUserID(ctx, db, *userEmail)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	query := `
		SELECT ip_address, action, result, reason, endpoint, method, created_at
		FROM ip_access_log
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.Pool.Query(ctx, query, userID, *limit)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "IP ADDRESS\tACTION\tRESULT\tREASON\tENDPOINT\tMETHOD\tTIME\n")

	for rows.Next() {
		var ipAddr, action, result, endpoint, method string
		var reason *string
		var createdAt time.Time

		rows.Scan(&ipAddr, &action, &result, &reason, &endpoint, &method, &createdAt)

		rsnStr := "-"
		if reason != nil {
			rsnStr = *reason
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			ipAddr, action, result, rsnStr, endpoint, method, createdAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
}
