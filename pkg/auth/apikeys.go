package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
)

// APIKey represents an API key for the gateway
type APIKey struct {
	ID             string     `json:"id" db:"id"`
	Name           string     `json:"name" db:"name"`
	KeyHash        string     `json:"-" db:"key_hash"`
	MaskedKey      string     `json:"masked_key" db:"masked_key"`
	Permissions    string     `json:"permissions" db:"permissions"`
	UserID         string     `json:"user_id" db:"user_id"`
	OrganizationID int        `json:"organization_id" db:"organization_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at" db:"expires_at"`
	Status         string     `json:"status" db:"status"`
	LastUsed       *time.Time `json:"last_used,omitempty" db:"last_used"`
}

// APIKeyCreateRequest represents the request to create an API key
type APIKeyCreateRequest struct {
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	ExpiryDate  string `json:"expiryDate,omitempty"`
}

// APIKeyCreateResponse represents the response after creating an API key
type APIKeyCreateResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"` // Only returned once
	MaskedKey string    `json:"masked_key"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// generateAPIKey generates a new API key
func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "spg_" + hex.EncodeToString(bytes), nil
}

// hashAPIKey creates a bcrypt hash of the API key
func hashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	return string(hash), err
}

// maskAPIKey creates a masked version of the API key for display
func maskAPIKey(key string) string {
	if len(key) < 8 {
		return key
	}
	prefix := key[:7] // "spg_" + 3 chars
	suffix := key[len(key)-4:]
	return prefix + strings.Repeat("•", 24) + suffix
}

// encryptAPIKey encrypts an API key for secure storage
func encryptAPIKey(apiKey, userID string) (string, error) {
	// Derive encryption key from user ID and a salt
	salt := []byte("spg-apikey-salt-2024")
	key := pbkdf2.Key([]byte(userID), salt, 10000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(apiKey), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAPIKey decrypts an API key from storage
func decryptAPIKey(encryptedKey, userID string) (string, error) {
	// Derive the same encryption key
	salt := []byte("spg-apikey-salt-2024")
	key := pbkdf2.Key([]byte(userID), salt, 10000, 32, sha256.New)

	data, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// CreateAPIKey creates a new API key with organization context
func CreateAPIKey(req *APIKeyCreateRequest, userID string, organizationID string) (*APIKeyCreateResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validate permissions
	validPermissions := map[string]bool{
		"read":  true,
		"write": true,
		"admin": true,
	}
	if !validPermissions[req.Permissions] {
		return nil, fmt.Errorf("invalid permissions")
	}

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash the API key (for authentication)
	keyHash, err := hashAPIKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to hash API key: %w", err)
	}

	// Encrypt the API key (for secure retrieval)
	encryptedKey, err := encryptAPIKey(apiKey, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API key: %w", err)
	}

	// Parse expiry date
	var expiresAt *time.Time
	if req.ExpiryDate != "" {
		parsed, err := time.Parse("2006-01-02", req.ExpiryDate)
		if err != nil {
			return nil, fmt.Errorf("invalid expiry date format")
		}
		expiresAt = &parsed
	}

	// Generate unique ID
	id := fmt.Sprintf("key_%d", time.Now().UnixNano())

	// Insert into database
	query := `INSERT INTO api_keys (id, name, key_hash, encrypted_key, masked_key, permissions, user_id, organization_id, expires_at, status)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.Exec(query, id, req.Name, keyHash, encryptedKey, maskAPIKey(apiKey), req.Permissions, userID, organizationID, expiresAt, "active")
	if err != nil {
		return nil, fmt.Errorf("failed to store API key: %w", err)
	}

	return &APIKeyCreateResponse{
		ID:        id,
		Name:      req.Name,
		APIKey:    apiKey, // Only returned once
		MaskedKey: maskAPIKey(apiKey),
		ExpiresAt: expiresAt,
	}, nil
}

// GetUserAPIKeys retrieves all API keys for a user within their organization
func GetUserAPIKeys(userID string, organizationID string) ([]APIKey, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, name, masked_key, permissions, organization_id, created_at, expires_at, status, last_used
			  FROM api_keys WHERE user_id = ? AND organization_id = ? ORDER BY created_at DESC`

	rows, err := db.Query(query, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var apiKeys []APIKey
	for rows.Next() {
		var key APIKey
		err := rows.Scan(&key.ID, &key.Name, &key.MaskedKey, &key.Permissions,
			&key.OrganizationID, &key.CreatedAt, &key.ExpiresAt, &key.Status, &key.LastUsed)
		if err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		key.UserID = userID
		apiKeys = append(apiKeys, key)
	}

	return apiKeys, nil
}

// ValidateAPIKey validates an API key and returns the associated user with organization context
func ValidateAPIKey(apiKey string) (*User, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Query all active API keys with organization context
	query := `SELECT ak.id, ak.key_hash, ak.permissions, ak.expires_at, ak.user_id, ak.organization_id,
			  u.id, u.email, u.name, u.role, u.organization_id, u.active
			  FROM api_keys ak
			  JOIN users u ON ak.user_id = CAST(u.id AS TEXT)
			  WHERE ak.status = 'active' AND ak.organization_id = u.organization_id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var keyID, keyHash, permissions, userIDStr string
		var expiresAt *time.Time
		var keyOrgID string
		var user User

		err := rows.Scan(&keyID, &keyHash, &permissions, &expiresAt, &userIDStr, &keyOrgID,
			&user.ID, &user.Email, &user.Name, &user.Role, &user.OrganizationID, &user.Active)
		if err != nil {
			continue
		}

		// Check if key matches
		if bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(apiKey)) == nil {
			// Check if key is expired
			if expiresAt != nil && time.Now().After(*expiresAt) {
				continue
			}

			// Verify organization consistency
			if keyOrgID != user.OrganizationID {
				continue
			}

			// Update last used timestamp
			_, _ = db.Exec("UPDATE api_keys SET last_used = ? WHERE id = ?", time.Now(), keyID)

			return &user, nil
		}
	}

	return nil, fmt.Errorf("invalid or expired API key")
}

// RevokeAPIKey revokes an API key within organization scope
func RevokeAPIKey(keyID, userID string, organizationID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `UPDATE api_keys SET status = 'revoked' WHERE id = ? AND user_id = ? AND organization_id = ?`
	result, err := db.Exec(query, keyID, userID, organizationID)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or access denied")
	}

	return nil
}

// DeleteAPIKey permanently deletes an API key within organization scope
func DeleteAPIKey(keyID, userID string, organizationID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	query := `DELETE FROM api_keys WHERE id = ? AND user_id = ? AND organization_id = ?`
	result, err := db.Exec(query, keyID, userID, organizationID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("API key not found or access denied")
	}

	return nil
}

// HTTP Handlers

// HandleGetAPIKeys handles GET /api/api-keys
func HandleGetAPIKeys(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	apiKeys, err := GetUserAPIKeys(user.ID, user.OrganizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch API keys",
		})
	}

	return c.JSON(fiber.Map{
		"api_keys": apiKeys,
	})
}

// HandleCreateAPIKey handles POST /api/api-keys
func HandleCreateAPIKey(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req APIKeyCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" || req.Permissions == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Name and permissions are required",
		})
	}

	response, err := CreateAPIKey(&req, user.ID, user.OrganizationID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(201).JSON(response)
}

// HandleRevokeAPIKey handles PATCH /api/api-keys/:id/revoke
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

	err := RevokeAPIKey(keyID, user.ID, user.OrganizationID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "API key revoked successfully",
	})
}

// HandleDeleteAPIKey handles DELETE /api/api-keys/:id
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

	err := DeleteAPIKey(keyID, user.ID, user.OrganizationID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "API key deleted successfully",
	})
}

// HandleGetFullAPIKey handles GET /api/api-keys/:id/full - returns the full API key for copying
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

	// Get the encrypted API key from database
	userIDStr := user.ID
	query := `SELECT encrypted_key, status FROM api_keys WHERE id = ? AND user_id = ? AND organization_id = ?`

	var encryptedKey, status string
	err := db.QueryRow(query, keyID, userIDStr, user.OrganizationID).Scan(&encryptedKey, &status)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "API key not found or access denied",
		})
	}

	if status != "active" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot copy inactive API key",
		})
	}

	// Decrypt the API key
	fullKey, err := decryptAPIKey(encryptedKey, userIDStr)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to decrypt API key",
		})
	}

	return c.JSON(fiber.Map{
		"api_key": fullKey,
		"warning": "Keep this API key secure. Anyone with access to this key can use your gateway.",
	})
}

// APIKeyMiddleware validates API key for gateway endpoints
func APIKeyMiddleware(c *fiber.Ctx) error {
	// Check for API key in X-API-Key header
	apiKey := c.Get("X-API-Key")

	// SECURITY ENHANCEMENT: Require API key for all proxy requests
	if apiKey == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
			"message": "This Sovereign Privacy Gateway requires an API key for access. Please include your SPG API key in the X-API-Key header.",
			"documentation": "Visit the dashboard to generate your API key at /integrations",
			"migration_notice": "If you're upgrading from an earlier version, API keys are now required for security. This change protects your gateway from unauthorized access.",
		})
	}

	// Validate API key
	user, err := ValidateAPIKey(apiKey)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid API key",
			"message": "The provided API key is invalid, expired, or revoked. Please check your key and try again.",
			"documentation": "Visit the dashboard to manage your API keys at /integrations",
		})
	}

	// Store user in context for audit logging and access control
	c.Locals("user", user)
	c.Locals("auth_method", "api_key")

	return c.Next()
}
