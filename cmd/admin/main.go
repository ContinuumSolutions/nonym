package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/lib/pq"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var (
	db          *sql.DB
	dbConnStr   string
	interactive bool
)

func init() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Ignore error - .env file is optional
	}

	// Build database connection string
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "gateway")
	password := getEnv("DB_PASSWORD", "gateway_password")
	dbname := getEnv("DB_NAME", "gateway")
	sslmode := getEnv("DB_SSL_MODE", "disable")

	dbConnStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
}

func main() {
	var err error
	db, err = sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	rootCmd := &cobra.Command{
		Use:   "admin",
		Short: "Sovereign Privacy Gateway Admin CLI",
		Long:  "Administrative tool for managing users and organizations in Sovereign Privacy Gateway",
	}

	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")

	// User management commands
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	userCmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Run:   createUser,
	})

	userCmd.AddCommand(&cobra.Command{
		Use:   "reset-password [email]",
		Short: "Reset user password",
		Args:  cobra.ExactArgs(1),
		Run:   resetPassword,
	})

	userCmd.AddCommand(&cobra.Command{
		Use:   "list [org-slug]",
		Short: "List users in organization",
		Args:  cobra.MaximumNArgs(1),
		Run:   listUsers,
	})

	userCmd.AddCommand(&cobra.Command{
		Use:   "delete [email]",
		Short: "Delete user account",
		Args:  cobra.ExactArgs(1),
		Run:   deleteUser,
	})

	// Organization management commands
	orgCmd := &cobra.Command{
		Use:   "org",
		Short: "Organization management commands",
	}

	orgCmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new organization",
		Run:   createOrganization,
	})

	orgCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all organizations",
		Run:   listOrganizations,
	})

	orgCmd.AddCommand(&cobra.Command{
		Use:   "delete [org-slug]",
		Short: "Delete organization and all its data",
		Args:  cobra.ExactArgs(1),
		Run:   deleteOrganization,
	})

	rootCmd.AddCommand(userCmd, orgCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
}

func createUser(cmd *cobra.Command, args []string) {
	var email, password, firstName, lastName, role, orgSlug string

	if interactive {
		email = prompt("Email: ")
		password = promptPassword("Password: ")
		firstName = prompt("First Name: ")
		lastName = prompt("Last Name: ")
		role = promptWithDefault("Role (admin/user/viewer)", "user")
		orgSlug = promptWithDefault("Organization slug", "default")
	} else {
		email = prompt("Email: ")
		password = promptPassword("Password: ")
		firstName = prompt("First Name: ")
		lastName = prompt("Last Name: ")
		role = promptWithDefault("Role (admin/user/viewer)", "user")
		orgSlug = promptWithDefault("Organization slug", "default")
	}

	// Validate role
	if role != "admin" && role != "user" && role != "viewer" {
		log.Fatalf("Invalid role. Must be admin, user, or viewer")
	}

	// Get organization ID
	var orgID string
	err := db.QueryRow("SELECT id FROM organizations WHERE slug = $1", orgSlug).Scan(&orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Fatalf("Organization with slug '%s' not found", orgSlug)
		}
		log.Fatalf("Failed to find organization: %v", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create user
	userID := uuid.New()
	_, err = db.Exec(`
		INSERT INTO users (id, organization_id, email, password_hash, first_name, last_name, role, email_verified)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true)
	`, userID, orgID, email, string(hashedPassword), firstName, lastName, role)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			log.Fatalf("User with email '%s' already exists", email)
		}
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("✅ User created successfully!\n")
	fmt.Printf("   ID: %s\n", userID)
	fmt.Printf("   Email: %s\n", email)
	fmt.Printf("   Role: %s\n", role)
	fmt.Printf("   Organization: %s\n", orgSlug)
}

func resetPassword(cmd *cobra.Command, args []string) {
	email := args[0]
	newPassword := promptPassword("New password: ")

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Update password
	result, err := db.Exec(`
		UPDATE users
		SET password_hash = $1, password_reset_token = NULL, password_reset_expires = NULL, updated_at = NOW()
		WHERE email = $2
	`, string(hashedPassword), email)

	if err != nil {
		log.Fatalf("Failed to reset password: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Fatalf("User with email '%s' not found", email)
	}

	fmt.Printf("✅ Password reset successfully for %s\n", email)
}

func listUsers(cmd *cobra.Command, args []string) {
	orgSlug := "default"
	if len(args) > 0 {
		orgSlug = args[0]
	}

	rows, err := db.Query(`
		SELECT u.id, u.email, u.first_name, u.last_name, u.role, u.is_active, u.created_at
		FROM users u
		JOIN organizations o ON u.organization_id = o.id
		WHERE o.slug = $1
		ORDER BY u.created_at DESC
	`, orgSlug)

	if err != nil {
		log.Fatalf("Failed to list users: %v", err)
	}
	defer rows.Close()

	fmt.Printf("Users in organization '%s':\n\n", orgSlug)
	fmt.Printf("%-36s %-30s %-20s %-10s %-8s %s\n", "ID", "EMAIL", "NAME", "ROLE", "ACTIVE", "CREATED")
	fmt.Println(strings.Repeat("-", 120))

	for rows.Next() {
		var id, email, firstName, lastName, role, createdAt string
		var isActive bool

		err := rows.Scan(&id, &email, &firstName, &lastName, &role, &isActive, &createdAt)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		fullName := fmt.Sprintf("%s %s", firstName, lastName)
		activeStr := "Yes"
		if !isActive {
			activeStr = "No"
		}

		fmt.Printf("%-36s %-30s %-20s %-10s %-8s %s\n",
			id, email, fullName, role, activeStr, createdAt[:10])
	}
}

func deleteUser(cmd *cobra.Command, args []string) {
	email := args[0]

	if !confirmDeletion(fmt.Sprintf("Are you sure you want to delete user '%s'?", email)) {
		fmt.Println("Operation cancelled.")
		return
	}

	result, err := db.Exec("DELETE FROM users WHERE email = $1", email)
	if err != nil {
		log.Fatalf("Failed to delete user: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Fatalf("User with email '%s' not found", email)
	}

	fmt.Printf("✅ User '%s' deleted successfully\n", email)
}

func createOrganization(cmd *cobra.Command, args []string) {
	name := prompt("Organization name: ")
	slug := prompt("Organization slug (URL-friendly): ")
	description := prompt("Description (optional): ")

	orgID := uuid.New()
	_, err := db.Exec(`
		INSERT INTO organizations (id, name, slug, description)
		VALUES ($1, $2, $3, $4)
	`, orgID, name, slug, description)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			log.Fatalf("Organization with name '%s' or slug '%s' already exists", name, slug)
		}
		log.Fatalf("Failed to create organization: %v", err)
	}

	fmt.Printf("✅ Organization created successfully!\n")
	fmt.Printf("   ID: %s\n", orgID)
	fmt.Printf("   Name: %s\n", name)
	fmt.Printf("   Slug: %s\n", slug)
}

func listOrganizations(cmd *cobra.Command, args []string) {
	rows, err := db.Query(`
		SELECT o.id, o.name, o.slug, o.description, o.created_at,
		       COUNT(u.id) as user_count
		FROM organizations o
		LEFT JOIN users u ON o.id = u.organization_id
		GROUP BY o.id, o.name, o.slug, o.description, o.created_at
		ORDER BY o.created_at DESC
	`)

	if err != nil {
		log.Fatalf("Failed to list organizations: %v", err)
	}
	defer rows.Close()

	fmt.Printf("Organizations:\n\n")
	fmt.Printf("%-36s %-25s %-15s %-5s %-50s %s\n", "ID", "NAME", "SLUG", "USERS", "DESCRIPTION", "CREATED")
	fmt.Println(strings.Repeat("-", 140))

	for rows.Next() {
		var id, name, slug, description, createdAt string
		var userCount int

		err := rows.Scan(&id, &name, &slug, &description, &createdAt, &userCount)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}

		if len(description) > 47 {
			description = description[:47] + "..."
		}

		fmt.Printf("%-36s %-25s %-15s %-5d %-50s %s\n",
			id, name, slug, userCount, description, createdAt[:10])
	}
}

func deleteOrganization(cmd *cobra.Command, args []string) {
	orgSlug := args[0]

	if orgSlug == "default" {
		log.Fatalf("Cannot delete the default organization")
	}

	if !confirmDeletion(fmt.Sprintf("Are you sure you want to delete organization '%s' and ALL its data?", orgSlug)) {
		fmt.Println("Operation cancelled.")
		return
	}

	// Start transaction
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to start transaction: %v", err)
	}
	defer tx.Rollback()

	// Get organization ID
	var orgID string
	err = tx.QueryRow("SELECT id FROM organizations WHERE slug = $1", orgSlug).Scan(&orgID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Fatalf("Organization with slug '%s' not found", orgSlug)
		}
		log.Fatalf("Failed to find organization: %v", err)
	}

	// Delete organization (cascades will handle related data)
	result, err := tx.Exec("DELETE FROM organizations WHERE id = $1", orgID)
	if err != nil {
		log.Fatalf("Failed to delete organization: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Fatalf("Organization with slug '%s' not found", orgSlug)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Printf("✅ Organization '%s' and all its data deleted successfully\n", orgSlug)
}

// Helper functions
func prompt(message string) string {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func promptWithDefault(message, defaultValue string) string {
	fmt.Printf("%s [%s]: ", message, defaultValue)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func promptPassword(message string) string {
	fmt.Print(message)
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Failed to read password: %v", err)
	}
	fmt.Println()
	return string(password)
}

func confirmDeletion(message string) bool {
	fmt.Printf("%s (type 'yes' to confirm): ", message)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input) == "yes"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
