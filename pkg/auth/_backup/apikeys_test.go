package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// APIKeyTestSuite is the test suite for API key functionality
type APIKeyTestSuite struct {
	suite.Suite
	db             *sql.DB
	userID         string
	organizationID int
	app            *fiber.App
}

func (suite *APIKeyTestSuite) SetupTest() {
	// Create in-memory database for each test
	testDB, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)

	suite.db = testDB

	// Initialize auth system with test database
	err = Initialize(testDB)
	suite.Require().NoError(err)

	// Create a test user
	registerReq := &RegisterRequest{
		Email:    "apikey@test.com",
		Password: "testpass123",
		Name:     "API Key Test User",
	}

	user, org, err := RegisterUser(registerReq)
	suite.Require().NoError(err)
	suite.userID = fmt.Sprintf("%d", user.ID)
	suite.organizationID = org.ID

	// Setup fiber app for HTTP tests
	suite.app = fiber.New()
	suite.app.Use(suite.mockAuthMiddleware)
	suite.app.Get("/api/api-keys", HandleGetAPIKeys)
	suite.app.Post("/api/api-keys", HandleCreateAPIKey)
	suite.app.Patch("/api/api-keys/:id/revoke", HandleRevokeAPIKey)
	suite.app.Delete("/api/api-keys/:id", HandleDeleteAPIKey)
}

func (suite *APIKeyTestSuite) TearDownTest() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// Mock middleware that sets user context
func (suite *APIKeyTestSuite) mockAuthMiddleware(c *fiber.Ctx) error {
	user := &User{
		ID:    1,
		Email: "apikey@test.com",
		Name:  "API Key Test User",
		Role:  "user",
	}
	c.Locals("user", user)
	return c.Next()
}

func TestAPIKeySuite(t *testing.T) {
	suite.Run(t, new(APIKeyTestSuite))
}

func (suite *APIKeyTestSuite) TestGenerateAPIKey() {
	key, err := generateAPIKey()
	suite.NoError(err)
	suite.NotEmpty(key)
	suite.True(strings.HasPrefix(key, "spg_"))
	suite.Equal(68, len(key)) // "spg_" (4) + 64 hex chars
}

func (suite *APIKeyTestSuite) TestHashAPIKey() {
	key := "spg_test1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	hash, err := hashAPIKey(key)
	suite.NoError(err)
	suite.NotEmpty(hash)
	suite.NotEqual(key, hash)

	// Test with bcrypt verification
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	suite.NoError(err)
}

func (suite *APIKeyTestSuite) TestMaskAPIKey() {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal API key",
			input:    "spg_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expected: "spg_123••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••cdef",
		},
		{
			name:     "short key",
			input:    "spg_123",
			expected: "spg_123",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := maskAPIKey(tt.input)
			suite.Equal(tt.expected, result)
		})
	}
}

func (suite *APIKeyTestSuite) TestCreateAPIKey() {
	tests := []struct {
		name        string
		request     *APIKeyCreateRequest
		shouldErr   bool
		errMsg      string
		checkExpiry bool
	}{
		{
			name: "valid API key with read permissions",
			request: &APIKeyCreateRequest{
				Name:        "Test Read Key",
				Permissions: "read",
			},
			shouldErr: false,
		},
		{
			name: "valid API key with write permissions",
			request: &APIKeyCreateRequest{
				Name:        "Test Write Key",
				Permissions: "write",
			},
			shouldErr: false,
		},
		{
			name: "valid API key with admin permissions",
			request: &APIKeyCreateRequest{
				Name:        "Test Admin Key",
				Permissions: "admin",
			},
			shouldErr: false,
		},
		{
			name: "API key with expiry date",
			request: &APIKeyCreateRequest{
				Name:        "Test Expiry Key",
				Permissions: "read",
				ExpiryDate:  "2025-12-31",
			},
			shouldErr:   false,
			checkExpiry: true,
		},
		{
			name: "invalid permissions",
			request: &APIKeyCreateRequest{
				Name:        "Invalid Key",
				Permissions: "invalid",
			},
			shouldErr: true,
			errMsg:    "invalid permissions",
		},
		{
			name: "invalid expiry date format",
			request: &APIKeyCreateRequest{
				Name:        "Bad Date Key",
				Permissions: "read",
				ExpiryDate:  "invalid-date",
			},
			shouldErr: true,
			errMsg:    "invalid expiry date format",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			response, err := CreateAPIKey(tt.request, suite.userID, suite.organizationID)

			if tt.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errMsg)
				suite.Nil(response)
			} else {
				suite.NoError(err)
				suite.NotNil(response)
				suite.Equal(tt.request.Name, response.Name)
				suite.NotEmpty(response.ID)
				suite.NotEmpty(response.APIKey)
				suite.True(strings.HasPrefix(response.APIKey, "spg_"))
				suite.NotEmpty(response.MaskedKey)

				if tt.checkExpiry {
					suite.NotNil(response.ExpiresAt)
					expectedDate, _ := time.Parse("2006-01-02", tt.request.ExpiryDate)
					suite.Equal(expectedDate.Format("2006-01-02"), response.ExpiresAt.Format("2006-01-02"))
				}
			}
		})
	}
}

func (suite *APIKeyTestSuite) TestGetUserAPIKeys() {
	// Create some test API keys
	keys := []APIKeyCreateRequest{
		{Name: "Key 1", Permissions: "read"},
		{Name: "Key 2", Permissions: "write"},
		{Name: "Key 3", Permissions: "admin"},
	}

	for _, keyReq := range keys {
		_, err := CreateAPIKey(&keyReq, suite.userID, suite.organizationID)
		suite.Require().NoError(err)
	}

	// Test getting API keys
	apiKeys, err := GetUserAPIKeys(suite.userID, suite.organizationID)
	suite.NoError(err)
	suite.Len(apiKeys, 3)

	// Check ordering (should be by created_at DESC)
	suite.Equal("Key 3", apiKeys[0].Name) // Last created should be first

	// Test with non-existent user
	apiKeys, err = GetUserAPIKeys("999", 999)
	suite.NoError(err)
	suite.Empty(apiKeys)
}

func (suite *APIKeyTestSuite) TestValidateAPIKey() {
	// Create a test API key
	createReq := &APIKeyCreateRequest{
		Name:        "Validation Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	tests := []struct {
		name      string
		apiKey    string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid API key",
			apiKey:    response.APIKey,
			shouldErr: false,
		},
		{
			name:      "invalid API key",
			apiKey:    "spg_invalidkey1234567890abcdef1234567890abcdef1234567890abcdef",
			shouldErr: true,
			errMsg:    "invalid or expired API key",
		},
		{
			name:      "empty API key",
			apiKey:    "",
			shouldErr: true,
			errMsg:    "invalid or expired API key",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			user, err := ValidateAPIKey(tt.apiKey)

			if tt.shouldErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errMsg)
				suite.Nil(user)
			} else {
				suite.NoError(err)
				suite.NotNil(user)
				suite.Equal("apikey@test.com", user.Email)
			}
		})
	}
}

func (suite *APIKeyTestSuite) TestValidateExpiredAPIKey() {
	// Create an API key with past expiry date
	createReq := &APIKeyCreateRequest{
		Name:        "Expired Test Key",
		Permissions: "read",
		ExpiryDate:  "2020-01-01", // Past date
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	// Try to validate expired key
	user, err := ValidateAPIKey(response.APIKey)
	suite.Error(err)
	suite.Contains(err.Error(), "invalid or expired API key")
	suite.Nil(user)
}

func (suite *APIKeyTestSuite) TestRevokeAPIKey() {
	// Create a test API key
	createReq := &APIKeyCreateRequest{
		Name:        "Revoke Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	// Test revoking the key
	err = RevokeAPIKey(response.ID, suite.userID, suite.organizationID)
	suite.NoError(err)

	// Try to validate revoked key
	user, err := ValidateAPIKey(response.APIKey)
	suite.Error(err)
	suite.Contains(err.Error(), "invalid or expired API key")
	suite.Nil(user)

	// Test revoking non-existent key
	err = RevokeAPIKey("non-existent-key", suite.userID, suite.organizationID)
	suite.Error(err)
	suite.Contains(err.Error(), "API key not found or access denied")

	// Test revoking with wrong user
	err = RevokeAPIKey(response.ID, "999", 999)
	suite.Error(err)
	suite.Contains(err.Error(), "API key not found or access denied")
}

func (suite *APIKeyTestSuite) TestDeleteAPIKey() {
	// Create a test API key
	createReq := &APIKeyCreateRequest{
		Name:        "Delete Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	// Test deleting the key
	err = DeleteAPIKey(response.ID, suite.userID, suite.organizationID)
	suite.NoError(err)

	// Verify key is deleted
	apiKeys, err := GetUserAPIKeys(suite.userID, suite.organizationID)
	suite.NoError(err)
	for _, key := range apiKeys {
		suite.NotEqual(response.ID, key.ID, "Deleted key should not appear in user's keys")
	}

	// Test deleting non-existent key
	err = DeleteAPIKey("non-existent-key", suite.userID, suite.organizationID)
	suite.Error(err)
	suite.Contains(err.Error(), "API key not found or access denied")
}

// HTTP Handler Tests

func (suite *APIKeyTestSuite) TestHandleGetAPIKeys() {
	// Create some test API keys
	for i := 0; i < 3; i++ {
		createReq := &APIKeyCreateRequest{
			Name:        fmt.Sprintf("HTTP Test Key %d", i+1),
			Permissions: "read",
		}
		_, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
		suite.Require().NoError(err)
	}

	req := httptest.NewRequest("GET", "/api/api-keys", nil)
	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	apiKeys := response["api_keys"].([]interface{})
	suite.Len(apiKeys, 3)
}

func (suite *APIKeyTestSuite) TestHandleCreateAPIKey() {
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		shouldHaveKey  bool
	}{
		{
			name: "valid create request",
			requestBody: `{
				"name": "HTTP Created Key",
				"permissions": "read"
			}`,
			expectedStatus: 201,
			shouldHaveKey:  true,
		},
		{
			name: "missing name",
			requestBody: `{
				"permissions": "read"
			}`,
			expectedStatus: 400,
			shouldHaveKey:  false,
		},
		{
			name: "missing permissions",
			requestBody: `{
				"name": "No Perms Key"
			}`,
			expectedStatus: 400,
			shouldHaveKey:  false,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid json}`,
			expectedStatus: 400,
			shouldHaveKey:  false,
		},
		{
			name: "invalid permissions",
			requestBody: `{
				"name": "Invalid Perms Key",
				"permissions": "invalid"
			}`,
			expectedStatus: 400,
			shouldHaveKey:  false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/api/api-keys", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.shouldHaveKey {
				suite.Contains(response, "api_key")
				suite.Contains(response, "id")
				suite.Contains(response, "name")
				suite.Contains(response, "masked_key")
			} else {
				suite.Contains(response, "error")
			}
		})
	}
}

func (suite *APIKeyTestSuite) TestHandleRevokeAPIKey() {
	// Create a test API key
	createReq := &APIKeyCreateRequest{
		Name:        "HTTP Revoke Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		keyID          string
		expectedStatus int
	}{
		{
			name:           "valid revoke request",
			keyID:          response.ID,
			expectedStatus: 200,
		},
		{
			name:           "non-existent key",
			keyID:          "non-existent-key",
			expectedStatus: 400,
		},
		{
			name:           "empty key ID",
			keyID:          "",
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url := fmt.Sprintf("/api/api-keys/%s/revoke", tt.keyID)
			req := httptest.NewRequest("PATCH", url, nil)

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var respData map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&respData)
			suite.NoError(err)

			if tt.expectedStatus == 200 {
				suite.Contains(respData, "message")
			} else {
				suite.Contains(respData, "error")
			}
		})
	}
}

func (suite *APIKeyTestSuite) TestHandleDeleteAPIKey() {
	// Create a test API key
	createReq := &APIKeyCreateRequest{
		Name:        "HTTP Delete Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		keyID          string
		expectedStatus int
	}{
		{
			name:           "valid delete request",
			keyID:          response.ID,
			expectedStatus: 200,
		},
		{
			name:           "non-existent key",
			keyID:          "non-existent-key",
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			url := fmt.Sprintf("/api/api-keys/%s", tt.keyID)
			req := httptest.NewRequest("DELETE", url, nil)

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var respData map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&respData)
			suite.NoError(err)

			if tt.expectedStatus == 200 {
				suite.Contains(respData, "message")
			} else {
				suite.Contains(respData, "error")
			}
		})
	}
}

func (suite *APIKeyTestSuite) TestAPIKeyMiddleware() {
	// Create test API key
	createReq := &APIKeyCreateRequest{
		Name:        "Middleware Test Key",
		Permissions: "read",
	}

	response, err := CreateAPIKey(createReq, suite.userID, suite.organizationID)
	suite.Require().NoError(err)

	// Create test app with middleware
	app := fiber.New()
	app.Use(APIKeyMiddleware)
	app.Get("/test", func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(*User)
		if !ok {
			return c.Status(401).JSON(fiber.Map{"error": "No user"})
		}
		authMethod := c.Locals("auth_method")
		return c.JSON(fiber.Map{
			"user":        user.Email,
			"auth_method": authMethod,
		})
	})

	tests := []struct {
		name           string
		apiKey         string
		expectedStatus int
		checkAuthMethod bool
	}{
		{
			name:            "valid API key",
			apiKey:          response.APIKey,
			expectedStatus:  200,
			checkAuthMethod: true,
		},
		{
			name:           "invalid API key",
			apiKey:         "spg_invalid1234567890abcdef1234567890abcdef1234567890abcdef",
			expectedStatus: 401,
		},
		{
			name:           "no API key (should continue to next)",
			apiKey:         "",
			expectedStatus: 401, // Will fail at our test handler
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			resp, err := app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			if tt.checkAuthMethod {
				var respData map[string]interface{}
				err = json.NewDecoder(resp.Body).Decode(&respData)
				suite.NoError(err)
				suite.Equal("api_key", respData["auth_method"])
				suite.Equal("apikey@test.com", respData["user"])
			}
		})
	}
}

// Benchmark tests
func BenchmarkGenerateAPIKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = generateAPIKey()
	}
}

func BenchmarkHashAPIKey(b *testing.B) {
	key := "spg_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	for i := 0; i < b.N; i++ {
		_, _ = hashAPIKey(key)
	}
}

func BenchmarkMaskAPIKey(b *testing.B) {
	key := "spg_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	for i := 0; i < b.N; i++ {
		_ = maskAPIKey(key)
	}
}