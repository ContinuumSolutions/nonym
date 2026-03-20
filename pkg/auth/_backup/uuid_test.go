package auth

import (
	"testing"
	"time"
	"database/sql"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"
)

// TestUUIDSupport tests that our models properly handle UUID strings
func TestUUIDSupport(t *testing.T) {
	t.Run("User model handles UUID strings", func(t *testing.T) {
		// Test creating a user with UUID types
		userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
		orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")

		user := &models.User{
			ID:             userID,
			Email:          "test@example.com",
			FirstName:      "Test",
			LastName:       "User",
			Role:           models.RoleUser,
			OrganizationID: orgID,
			IsActive:       true,
			EmailVerified:  true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// Verify UUID types are properly assigned
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, orgID, user.OrganizationID)
		assert.IsType(t, uuid.UUID{}, user.ID) // Ensure it's a UUID
		assert.IsType(t, uuid.UUID{}, user.OrganizationID) // Ensure it's a UUID
	})

	t.Run("Organization model handles UUID strings", func(t *testing.T) {
		// Test creating an organization with UUID types
		orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

		org := &models.Organization{
			ID:          orgID,
			Name:        "Test Organization",
			Slug:        "test-org",
			Description: "Test description",
			IsActive:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Verify UUID types are properly assigned
		assert.Equal(t, orgID, org.ID)
		assert.IsType(t, uuid.UUID{}, org.ID) // Ensure it's a UUID
	})

	t.Run("UserProfile model handles UUID strings", func(t *testing.T) {
		// Test creating a user profile with UUID types
		profileID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440003")

		profile := &models.UserProfile{
			ID:            profileID,
			Email:         "profile@example.com",
			FirstName:     "Profile",
			LastName:      "User",
			FullName:      "Profile User",
			Role:          models.RoleAdmin,
			IsActive:      true,
			EmailVerified: true,
			CreatedAt:     time.Now(),
		}

		// Verify UUID types are properly assigned
		assert.Equal(t, profileID, profile.ID)
		assert.IsType(t, uuid.UUID{}, profile.ID) // Ensure it's a UUID
	})
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

// TestRegisterRequestWithUUIDs tests registration requests handle UUID types
func TestRegisterRequestWithUUIDs(t *testing.T) {
	existingOrgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	req := &models.RegisterRequest{
		Email:          "newuser@example.com",
		Password:       "password123",
		FirstName:      "New",
		LastName:       "User",
		Organization:   "Test Org",
		OrganizationID: &existingOrgID,
	}

	// Verify the organization ID is properly handled as a UUID
	assert.Equal(t, existingOrgID, *req.OrganizationID)
	assert.IsType(t, uuid.UUID{}, *req.OrganizationID)
}

// TestLoginRequestWithUUIDs tests login requests handle UUID types
func TestLoginRequestWithUUIDs(t *testing.T) {
	orgID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	req := &models.LoginRequest{
		Email:          "user@example.com",
		Password:       "password123",
		OrganizationID: &orgID,
		RememberMe:     false,
	}

	// Verify the organization ID is properly handled as a UUID
	assert.Equal(t, orgID, *req.OrganizationID)
	assert.IsType(t, uuid.UUID{}, *req.OrganizationID)
}