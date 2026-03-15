package auth

import (
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"
)

// TestNewAuthSystemIntegration tests the new auth system end-to-end
func TestNewAuthSystemIntegration(t *testing.T) {
	// Setup test database
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer testDB.Close()

	// Initialize auth system
	oldDB := db
	db = testDB

	// Initialize JWT secret
	jwtSecret = []byte("test-secret")

	defer func() {
		db = oldDB
	}()

	// Create tables
	tables := []string{
		`CREATE TABLE organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			is_active BOOLEAN DEFAULT true,
			email_verified BOOLEAN DEFAULT false,
			last_login DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (organization_id) REFERENCES organizations(id)
		)`,
		`CREATE TABLE user_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			session_token TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, table := range tables {
		if _, err := testDB.Exec(table); err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	t.Run("SignupUser creates user and organization atomically", func(t *testing.T) {
		req := &RegisterRequest{
			Email:        "test@example.com",
			Password:     "password123",
			Name:         "Test User",
			Organization: "Test Company",
		}

		response, err := SignupUser(req, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("SignupUser failed: %v", err)
		}

		// Verify response
		if response.Token == "" {
			t.Error("Token should not be empty")
		}
		if response.User == nil {
			t.Error("User should not be nil")
		}
		if response.Organization == nil {
			t.Error("Organization should not be nil")
		}
		if response.User.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", response.User.Email)
		}
		if response.User.Role != "admin" {
			t.Errorf("Expected role admin, got %s", response.User.Role)
		}

		// Verify database state
		var userCount, orgCount int
		err = testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.Email).Scan(&userCount)
		if err != nil || userCount != 1 {
			t.Errorf("Expected 1 user, found %d", userCount)
		}

		err = testDB.QueryRow("SELECT COUNT(*) FROM organizations WHERE name = ?", req.Organization).Scan(&orgCount)
		if err != nil || orgCount != 1 {
			t.Errorf("Expected 1 organization, found %d", orgCount)
		}
	})

	t.Run("SignupUser handles duplicate email", func(t *testing.T) {
		// First signup should succeed (already done above)
		// Second signup with same email should fail
		req := &RegisterRequest{
			Email:        "test@example.com", // Same email as above
			Password:     "differentpassword",
			Name:         "Different User",
			Organization: "Different Company",
		}

		_, err := SignupUser(req, "127.0.0.1", "test-agent")
		if err == nil {
			t.Error("Expected error for duplicate email")
		}

		// Verify only one organization exists
		var orgCount int
		err = testDB.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&orgCount)
		if err != nil || orgCount != 1 {
			t.Errorf("Expected 1 organization after failed signup, found %d", orgCount)
		}
	})

	t.Run("AuthenticateUser works after signup", func(t *testing.T) {
		req := &LoginRequest{
			Email:    "test@example.com",
			Password: "password123",
		}

		response, err := AuthenticateUser(req, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("AuthenticateUser failed: %v", err)
		}

		// Verify response
		if response.Token == "" {
			t.Error("Token should not be empty")
		}
		if response.User.Email != "test@example.com" {
			t.Errorf("Expected email test@example.com, got %s", response.User.Email)
		}
	})

	t.Run("AuthenticateUser fails with wrong password", func(t *testing.T) {
		req := &LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		_, err := AuthenticateUser(req, "127.0.0.1", "test-agent")
		if err == nil {
			t.Error("Expected error for wrong password")
		}
	})

	t.Run("Organization isolation works", func(t *testing.T) {
		// Create first user in first organization
		req1 := &RegisterRequest{
			Email:        "user1@company1.com",
			Password:     "password123",
			Name:         "User One",
			Organization: "Company One",
		}

		response1, err := SignupUser(req1, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("First signup failed: %v", err)
		}

		// Create second user in different organization
		req2 := &RegisterRequest{
			Email:        "user2@company2.com",
			Password:     "password123",
			Name:         "User Two",
			Organization: "Company Two",
		}

		response2, err := SignupUser(req2, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("Second signup failed: %v", err)
		}

		// Verify different organization IDs
		if response2.User.OrganizationID == response1.User.OrganizationID { // Should have different org IDs
			t.Error("Users should have different organization IDs")
		}

		// Verify database has two organizations
		var orgCount int
		err = testDB.QueryRow("SELECT COUNT(*) FROM organizations").Scan(&orgCount)
		if err != nil || orgCount != 2 {
			t.Errorf("Expected 2 organizations, found %d", orgCount)
		}
	})

	t.Run("JWT token validation works", func(t *testing.T) {
		// Use the token from first user
		req := &RegisterRequest{
			Email:        "jwt@example.com",
			Password:     "password123",
			Name:         "JWT User",
			Organization: "JWT Company",
		}

		signupResp, err := SignupUser(req, "127.0.0.1", "test-agent")
		if err != nil {
			t.Fatalf("Signup failed: %v", err)
		}

		// Validate the token
		user, err := ValidateToken(signupResp.Token)
		if err != nil {
			t.Fatalf("Token validation failed: %v", err)
		}

		if user.Email != "jwt@example.com" {
			t.Errorf("Expected email jwt@example.com, got %s", user.Email)
		}
		if user.OrganizationID != signupResp.User.OrganizationID {
			t.Errorf("Organization ID mismatch: expected %d, got %d",
				signupResp.User.OrganizationID, user.OrganizationID)
		}
	})

	fmt.Println("✅ All integration tests passed!")
}