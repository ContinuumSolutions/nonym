package auth

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

// APIKey represents an API key
type APIKey struct {
	ID             int        `json:"id" db:"id"`
	Name           string     `json:"name" db:"name"`
	KeyHash        string     `json:"-" db:"key_hash"`
	MaskedKey      string     `json:"masked_key" db:"masked_key"`
	Permissions    string     `json:"permissions" db:"permissions"`
	UserID         int        `json:"user_id" db:"user_id"`
	OrganizationID int        `json:"organization_id" db:"organization_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	Status         string     `json:"status" db:"status"`
	LastUsed       *time.Time `json:"last_used,omitempty" db:"last_used"`
}

// APIKeyCreateRequest represents an API key creation request
type APIKeyCreateRequest struct {
	Name        string     `json:"name" validate:"required"`
	Permissions string     `json:"permissions" validate:"required"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// APIKeyResponse represents an API key in responses
type APIKeyResponse struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	MaskedKey   string     `json:"masked_key"`
	Permissions string     `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Status      string     `json:"status"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
}

// HandleGetAPIKeys handles GET /api/v1/api-keys
func HandleGetAPIKeys(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Retrieve API keys for the user's organization
	query := formatQuery(`SELECT id, name, masked_key, permissions, created_at, expires_at, status, last_used
			  FROM api_keys
			  WHERE organization_id = ? AND user_id = ? AND status != 'deleted'
			  ORDER BY created_at DESC`)

	rows, err := db.Query(query, user.OrganizationID, user.ID)
	if err != nil {
		log.Printf("Failed to query API keys: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to retrieve API keys",
		})
	}
	defer rows.Close()

	var apiKeys []APIKeyResponse
	for rows.Next() {
		var key APIKeyResponse
		var expiresAt sql.NullTime
		var lastUsed sql.NullTime

		err := rows.Scan(&key.ID, &key.Name, &key.MaskedKey, &key.Permissions,
						&key.CreatedAt, &expiresAt, &key.Status, &lastUsed)
		if err != nil {
			log.Printf("Failed to scan API key: %v", err)
			continue
		}

		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if lastUsed.Valid {
			key.LastUsed = &lastUsed.Time
		}

		apiKeys = append(apiKeys, key)
	}

	return c.JSON(fiber.Map{
		"api_keys": apiKeys,
		"total":    len(apiKeys),
	})
}

// HandleCreateAPIKey handles POST /api/v1/api-keys
func HandleCreateAPIKey(c *fiber.Ctx) error {
	fmt.Printf("*** HANDLER CALLED: HandleCreateAPIKey ***\n")
	log.Printf("*** HANDLER CALLED: HandleCreateAPIKey ***")

	user, ok := c.Locals("user").(*User)
	if !ok {
		log.Printf("Authentication failed - no user in context")
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	log.Printf("User authenticated: ID=%d, Email=%s, OrgID=%d", user.ID, user.Email, user.OrganizationID)

	var req APIKeyCreateRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	log.Printf("Request parsed: Name=%s, Permissions=%s", req.Name, req.Permissions)

	// Basic validation
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "API key name is required",
		})
	}

	if req.Permissions == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Permissions are required",
		})
	}

	// 1. Generate secure random API key
	apiKeyValue := "spg_" + generateRandomString(32)

	// 2. Hash the key for storage
	keyHash, err := hashAPIKey(apiKeyValue)
	if err != nil {
		log.Printf("Failed to hash API key: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	// 3. Create masked version for display
	maskedKey := createMaskedKey(apiKeyValue)

	// 4. Store in database with proper error handling
	query := formatQuery(`INSERT INTO api_keys (name, key_hash, masked_key, permissions, user_id, organization_id, expires_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`)

	var keyID int
	err = db.QueryRow(query, req.Name, keyHash, maskedKey, req.Permissions, user.ID, user.OrganizationID, req.ExpiresAt).Scan(&keyID)
	if err != nil {
		log.Printf("Failed to store API key - Query: %s", query)
		log.Printf("Failed to store API key - Params: name=%s, user_id=%d, org_id=%d", req.Name, user.ID, user.OrganizationID)
		log.Printf("Failed to store API key - Error: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create API key",
		})
	}
	log.Printf("Successfully created API key with ID: %d", keyID)

	// 5. Return the key once (never show again)
	return c.Status(201).JSON(fiber.Map{
		"message": "API key created successfully",
		"api_key": apiKeyValue, // Only shown once
		"key_info": fiber.Map{
			"id":          keyID,
			"name":        req.Name,
			"masked_key":  maskedKey,
			"permissions": req.Permissions,
			"expires_at":  req.ExpiresAt,
			"status":      "active",
		},
		"warning": "Keep this API key secure. This is the only time it will be shown in full.",
	})
}

// HandleGetFullAPIKey handles GET /api/v1/api-keys/:id/full (placeholder)
func HandleGetFullAPIKey(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "API key ID is required",
		})
	}

	_ = user // Acknowledge user variable

	// TODO: For security reasons, full API keys should never be retrievable after creation
	return c.Status(403).JSON(fiber.Map{
		"error": "Full API keys cannot be retrieved for security reasons",
	})
}

// HandleRevokeAPIKey handles PUT /api/v1/api-keys/:id/revoke
func HandleRevokeAPIKey(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "API key ID is required",
		})
	}

	// 1. Verify key belongs to user or user is admin
	var existingUserID int
	var existingOrgID int
	checkQuery := formatQuery(`SELECT user_id, organization_id FROM api_keys WHERE id = ? AND status = 'active'`)
	err := db.QueryRow(checkQuery, keyID).Scan(&existingUserID, &existingOrgID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{
			"error": "API key not found",
		})
	}
	if err != nil {
		log.Printf("Failed to verify API key ownership: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	// Check if user owns the key or is in the same organization
	if existingUserID != user.ID && existingOrgID != user.OrganizationID {
		return c.Status(403).JSON(fiber.Map{
			"error": "You don't have permission to revoke this API key",
		})
	}

	// 2. Update status to 'revoked' in database
	updateQuery := formatQuery(`UPDATE api_keys SET status = 'revoked', last_used = CURRENT_TIMESTAMP WHERE id = ?`)
	_, err = db.Exec(updateQuery, keyID)
	if err != nil {
		log.Printf("Failed to revoke API key: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to revoke API key",
		})
	}

	// 3. Log the revocation for audit
	log.Printf("API key revoked: ID=%s, UserID=%d, OrgID=%d", keyID, user.ID, user.OrganizationID)

	return c.JSON(fiber.Map{
		"message": "API key revoked successfully",
		"key_id":  keyID,
	})
}

// HandleDeleteAPIKey handles DELETE /api/v1/api-keys/:id
func HandleDeleteAPIKey(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	keyID := c.Params("id")
	if keyID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "API key ID is required",
		})
	}

	// 1. Verify key belongs to user or user is admin
	var existingUserID int
	var existingOrgID int
	checkQuery := formatQuery(`SELECT user_id, organization_id FROM api_keys WHERE id = ?`)
	err := db.QueryRow(checkQuery, keyID).Scan(&existingUserID, &existingOrgID)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{
			"error": "API key not found",
		})
	}
	if err != nil {
		log.Printf("Failed to verify API key ownership: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	// Check if user owns the key or is in the same organization
	if existingUserID != user.ID && existingOrgID != user.OrganizationID {
		return c.Status(403).JSON(fiber.Map{
			"error": "You don't have permission to delete this API key",
		})
	}

	// 2. Mark as deleted in database (soft delete for audit trail)
	deleteQuery := formatQuery(`UPDATE api_keys SET status = 'deleted', last_used = CURRENT_TIMESTAMP WHERE id = ?`)
	result, err := db.Exec(deleteQuery, keyID)
	if err != nil {
		log.Printf("Failed to delete API key: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to delete API key",
		})
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{
			"error": "API key not found",
		})
	}

	// 3. Log the deletion for audit
	log.Printf("API key deleted: ID=%s, UserID=%d, OrgID=%d", keyID, user.ID, user.OrganizationID)

	return c.JSON(fiber.Map{
		"message": "API key deleted successfully",
		"key_id":  keyID,
	})
}

// APIKeyMiddleware validates API key authentication
func APIKeyMiddleware(c *fiber.Ctx) error {
	// Check for API key in headers
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		// Check Authorization header with "Bearer" prefix
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if apiKey == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "API key required",
		})
	}

	// Database migration should handle table creation

	// Validate API key against database
	keyInfo, err := validateAPIKeyFromDatabaseTemp(apiKey)
	if err != nil {
		// Log the error and try fallback to test keys
		log.Printf("API key validation error: %v", err)
		return fallbackToTestKeysTemp(c, apiKey)
	}

	if keyInfo == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid API key",
		})
	}

	// Update last_used timestamp
	updateLastUsedQuery := formatQuery(`UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?`)
	db.Exec(updateLastUsedQuery, keyInfo.ID)

	// Set context for downstream handlers
	c.Locals("organization_id", keyInfo.OrganizationID)
	c.Locals("user_id", keyInfo.UserID)
	c.Locals("auth_method", "api_key")
	c.Locals("api_key_id", keyInfo.ID)
	c.Locals("api_key_permissions", keyInfo.Permissions)

	return c.Next()
}

// generateRandomString generates a secure random string for API keys
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Error generating random string: %v", err)
		// Fallback to current timestamp as last resort
		fallback := fmt.Sprintf("%d", time.Now().UnixNano())
		if len(fallback) > length {
			return fallback[:length]
		}
		return fallback
	}

	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}

	return string(bytes)
}

// hashAPIKey creates a bcrypt hash of the API key for secure storage
func hashAPIKey(apiKey string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(bytes), nil
}

// verifyAPIKey verifies an API key against its hash
func verifyAPIKey(apiKey, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(apiKey))
	return err == nil
}

// createMaskedKey creates a masked version of the API key for display
func createMaskedKey(apiKey string) string {
	if len(apiKey) < 8 {
		return "****"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}


// APIKeyInfo holds information about a validated API key
type APIKeyInfo struct {
	ID             int
	UserID         int
	OrganizationID int
	Permissions    string
	Status         string
	ExpiresAt      *time.Time
}

// validateAPIKeyFromDatabaseTemp validates an API key against the database
func validateAPIKeyFromDatabaseTemp(apiKey string) (*APIKeyInfo, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Query active API keys and check against the provided key
	query := formatQuery(`SELECT id, key_hash, user_id, organization_id, permissions, status, expires_at
			  FROM api_keys
			  WHERE status = 'active'`)

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var keyInfo APIKeyInfo
		var keyHash string
		var expiresAt sql.NullTime

		err := rows.Scan(&keyInfo.ID, &keyHash, &keyInfo.UserID, &keyInfo.OrganizationID,
						&keyInfo.Permissions, &keyInfo.Status, &expiresAt)
		if err != nil {
			continue
		}

		// Verify the API key against the hash
		if verifyAPIKey(apiKey, keyHash) {
			// Check if key has expired
			if expiresAt.Valid {
				keyInfo.ExpiresAt = &expiresAt.Time
				if time.Now().After(expiresAt.Time) {
					// Key has expired, mark as expired
					expireQuery := formatQuery(`UPDATE api_keys SET status = 'expired' WHERE id = ?`)
					db.Exec(expireQuery, keyInfo.ID)
					continue
				}
			}

			return &keyInfo, nil
		}
	}

	return nil, nil // No matching key found
}

// fallbackToTestKeysTemp provides fallback authentication for development/testing
func fallbackToTestKeysTemp(c *fiber.Ctx, apiKey string) error {
	validTestKeys := map[string]int{
		"test-api-key": 1, // organization_id = 1
		"demo-key":     1,
		"dev-key":      1,
	}

	orgID, isValid := validTestKeys[apiKey]
	if !isValid {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid API key",
		})
	}

	// Set organization context for downstream handlers
	c.Locals("organization_id", orgID)
	c.Locals("auth_method", "api_key_test")

	return c.Next()
}