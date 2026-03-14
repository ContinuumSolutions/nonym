package auth

import (
	"testing"
	"time"
	"database/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestUUIDSupport tests that our models properly handle UUID strings
func TestUUIDSupport(t *testing.T) {
	t.Run("User model handles UUID strings", func(t *testing.T) {
		// Test creating a user with UUID strings
		user := &User{
			ID:             "550e8400-e29b-41d4-a716-446655440000",
			Email:          "test@example.com",
			Name:           "Test User",
			Role:           "user",
			OrganizationID: "550e8400-e29b-41d4-a716-446655440001",
			Active:         true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// Verify UUID strings are properly assigned
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", user.ID)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440001", user.OrganizationID)
		assert.IsType(t, "", user.ID) // Ensure it's a string
		assert.IsType(t, "", user.OrganizationID) // Ensure it's a string
	})

	t.Run("Organization model handles UUID strings", func(t *testing.T) {
		// Test creating an organization with UUID strings
		org := &Organization{
			ID:          "550e8400-e29b-41d4-a716-446655440002",
			Name:        "Test Organization",
			Slug:        "test-org",
			Description: "Test description",
			OwnerID:     "550e8400-e29b-41d4-a716-446655440000",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Verify UUID strings are properly assigned
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440002", org.ID)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", org.OwnerID)
		assert.IsType(t, "", org.ID) // Ensure it's a string
		assert.IsType(t, "", org.OwnerID) // Ensure it's a string
	})

	t.Run("UserProfile model handles UUID strings", func(t *testing.T) {
		// Test creating a user profile with UUID strings
		profile := &UserProfile{
			ID:             "550e8400-e29b-41d4-a716-446655440003",
			Email:          "profile@example.com",
			Name:           "Profile User",
			Role:           "admin",
			OrganizationID: "550e8400-e29b-41d4-a716-446655440004",
			Active:         true,
			CreatedAt:      time.Now(),
		}

		// Verify UUID strings are properly assigned
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440003", profile.ID)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440004", profile.OrganizationID)
		assert.IsType(t, "", profile.ID) // Ensure it's a string
		assert.IsType(t, "", profile.OrganizationID) // Ensure it's a string
	})
}

// TestGenerateJWTWithUUIDs tests JWT generation with UUID strings
func TestGenerateJWTWithUUIDs(t *testing.T) {
	// Initialize a test secret
	jwtSecret = []byte("test-secret-key-for-testing")

	userID := "550e8400-e29b-41d4-a716-446655440000"
	orgID := "550e8400-e29b-41d4-a716-446655440001"
	email := "test@example.com"
	role := "user"

	token, expiresAt, err := generateJWT(userID, orgID, email, role)

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.True(t, expiresAt.After(time.Now()))

	// Verify token is a valid string
	assert.IsType(t, "", token)
}

// TestSQLiteWithUUIDs tests that SQLite can handle UUID strings properly
func TestSQLiteWithUUIDs(t *testing.T) {
	// Create in-memory SQLite database for testing
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Create a simple test table with string ID columns
	_, err = db.Exec(`
		CREATE TABLE test_users (
			id TEXT PRIMARY KEY,
			organization_id TEXT NOT NULL,
			email TEXT NOT NULL,
			name TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	// Test inserting UUID strings
	userID := "550e8400-e29b-41d4-a716-446655440000"
	orgID := "550e8400-e29b-41d4-a716-446655440001"
	email := "test@example.com"
	name := "Test User"

	_, err = db.Exec(
		"INSERT INTO test_users (id, organization_id, email, name) VALUES (?, ?, ?, ?)",
		userID, orgID, email, name,
	)
	require.NoError(t, err)

	// Test retrieving UUID strings
	var retrievedUserID, retrievedOrgID, retrievedEmail, retrievedName string
	err = db.QueryRow(
		"SELECT id, organization_id, email, name FROM test_users WHERE id = ?",
		userID,
	).Scan(&retrievedUserID, &retrievedOrgID, &retrievedEmail, &retrievedName)

	require.NoError(t, err)
	assert.Equal(t, userID, retrievedUserID)
	assert.Equal(t, orgID, retrievedOrgID)
	assert.Equal(t, email, retrievedEmail)
	assert.Equal(t, name, retrievedName)

	// Verify they're strings
	assert.IsType(t, "", retrievedUserID)
	assert.IsType(t, "", retrievedOrgID)
}

// TestRegisterRequestWithUUIDs tests registration requests handle UUID strings
func TestRegisterRequestWithUUIDs(t *testing.T) {
	existingOrgID := "550e8400-e29b-41d4-a716-446655440000"

	req := &RegisterRequest{
		Email:          "newuser@example.com",
		Password:       "password123",
		Name:           "New User",
		Organization:   "Test Org",
		OrganizationID: &existingOrgID,
	}

	// Verify the organization ID is properly handled as a string
	assert.Equal(t, existingOrgID, *req.OrganizationID)
	assert.IsType(t, "", *req.OrganizationID)
}

// TestLoginRequestWithUUIDs tests login requests handle UUID strings
func TestLoginRequestWithUUIDs(t *testing.T) {
	orgID := "550e8400-e29b-41d4-a716-446655440000"

	req := &LoginRequest{
		Email:          "user@example.com",
		Password:       "password123",
		OrganizationID: &orgID,
	}

	// Verify the organization ID is properly handled as a string
	assert.Equal(t, orgID, *req.OrganizationID)
	assert.IsType(t, "", *req.OrganizationID)
}