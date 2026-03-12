package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

// HandlersTestSuite is the test suite for auth HTTP handlers
type HandlersTestSuite struct {
	suite.Suite
	db  *sql.DB
	app *fiber.App
}

func (suite *HandlersTestSuite) SetupTest() {
	// Create in-memory database for each test
	testDB, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)

	suite.db = testDB

	// Initialize auth system with test database
	err = Initialize(testDB)
	suite.Require().NoError(err)

	// Setup fiber app
	suite.app = fiber.New()

	// Add auth handlers to test routes (these would need to be implemented)
	// For now we'll create mock handlers to test the structure
	suite.app.Post("/api/register", suite.mockRegisterHandler)
	suite.app.Post("/api/login", suite.mockLoginHandler)
	suite.app.Post("/api/logout", suite.mockLogoutHandler)
	suite.app.Get("/api/profile", suite.mockAuthMiddleware, suite.mockProfileHandler)
}

func (suite *HandlersTestSuite) TearDownTest() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// Mock handlers (these should be implemented in the actual handlers.go file)
func (suite *HandlersTestSuite) mockRegisterHandler(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	user, err := RegisterUser(&req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	profile := &UserProfile{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		Active:    user.Active,
		CreatedAt: user.CreatedAt,
	}

	return c.Status(201).JSON(fiber.Map{
		"message": "User registered successfully",
		"user":    profile,
	})
}

func (suite *HandlersTestSuite) mockLoginHandler(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	clientIP := c.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = c.IP()
	}
	userAgent := c.Get("User-Agent")

	response, err := LoginUser(&req, clientIP, userAgent)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(response)
}

func (suite *HandlersTestSuite) mockLogoutHandler(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Authorization header required",
		})
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid authorization format",
		})
	}

	err := LogoutUser(token)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

func (suite *HandlersTestSuite) mockAuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authorization header required",
		})
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid authorization format",
		})
	}

	user, err := ValidateToken(token)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	c.Locals("user", user)
	return c.Next()
}

func (suite *HandlersTestSuite) mockProfileHandler(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "User not found in context",
		})
	}

	profile, err := GetUserProfile(user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to get user profile",
		})
	}

	return c.JSON(fiber.Map{
		"user": profile,
	})
}

func TestHandlersSuite(t *testing.T) {
	suite.Run(t, new(HandlersTestSuite))
}

func (suite *HandlersTestSuite) TestRegisterHandler() {
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		checkFields    []string
		shouldHaveUser bool
	}{
		{
			name: "valid registration",
			requestBody: `{
				"email": "register@test.com",
				"password": "password123",
				"name": "Register Test User"
			}`,
			expectedStatus: 201,
			checkFields:    []string{"message", "user"},
			shouldHaveUser: true,
		},
		{
			name: "registration with organization",
			requestBody: `{
				"email": "org@test.com",
				"password": "password123",
				"name": "Org Test User",
				"organization": "Test Corp"
			}`,
			expectedStatus: 201,
			checkFields:    []string{"message", "user"},
			shouldHaveUser: true,
		},
		{
			name: "registration with firstName and lastName",
			requestBody: `{
				"email": "fullname@test.com",
				"password": "password123",
				"firstName": "John",
				"lastName": "Doe"
			}`,
			expectedStatus: 201,
			checkFields:    []string{"message", "user"},
			shouldHaveUser: true,
		},
		{
			name: "duplicate email",
			requestBody: `{
				"email": "admin@gateway.local",
				"password": "password123",
				"name": "Duplicate User"
			}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
		{
			name: "invalid email",
			requestBody: `{
				"email": "invalid-email",
				"password": "password123",
				"name": "Invalid Email User"
			}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
		{
			name: "short password",
			requestBody: `{
				"email": "short@test.com",
				"password": "123",
				"name": "Short Password User"
			}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
		{
			name: "missing email",
			requestBody: `{
				"password": "password123",
				"name": "No Email User"
			}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
		{
			name: "missing password",
			requestBody: `{
				"email": "nopass@test.com",
				"name": "No Password User"
			}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid json}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
			shouldHaveUser: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/api/register", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			// Check required fields
			for _, field := range tt.checkFields {
				suite.Contains(response, field, "Response should contain field: %s", field)
			}

			if tt.shouldHaveUser {
				user := response["user"].(map[string]interface{})
				suite.Contains(user, "id")
				suite.Contains(user, "email")
				suite.Contains(user, "name")
				suite.Contains(user, "role")
				suite.Contains(user, "active")
				suite.Contains(user, "created_at")

				// Verify password is not included
				suite.NotContains(user, "password")
			}
		})
	}
}

func (suite *HandlersTestSuite) TestLoginHandler() {
	// First register a test user
	registerReq := &RegisterRequest{
		Email:    "login@test.com",
		Password: "testpass123",
		Name:     "Login Test User",
	}

	_, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		checkFields    []string
		shouldHaveToken bool
	}{
		{
			name: "valid login",
			requestBody: `{
				"email": "login@test.com",
				"password": "testpass123"
			}`,
			expectedStatus:  200,
			checkFields:     []string{"token", "expires_at", "user"},
			shouldHaveToken: true,
		},
		{
			name: "admin login",
			requestBody: `{
				"email": "admin@gateway.local",
				"password": "admin123"
			}`,
			expectedStatus:  200,
			checkFields:     []string{"token", "expires_at", "user"},
			shouldHaveToken: true,
		},
		{
			name: "invalid email",
			requestBody: `{
				"email": "nonexistent@test.com",
				"password": "testpass123"
			}`,
			expectedStatus:  401,
			checkFields:     []string{"error"},
			shouldHaveToken: false,
		},
		{
			name: "invalid password",
			requestBody: `{
				"email": "login@test.com",
				"password": "wrongpassword"
			}`,
			expectedStatus:  401,
			checkFields:     []string{"error"},
			shouldHaveToken: false,
		},
		{
			name: "missing email",
			requestBody: `{
				"password": "testpass123"
			}`,
			expectedStatus:  401,
			checkFields:     []string{"error"},
			shouldHaveToken: false,
		},
		{
			name: "missing password",
			requestBody: `{
				"email": "login@test.com"
			}`,
			expectedStatus:  401,
			checkFields:     []string{"error"},
			shouldHaveToken: false,
		},
		{
			name:           "invalid JSON",
			requestBody:    `{invalid json}`,
			expectedStatus: 400,
			checkFields:    []string{"error"},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/api/login", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")
			req.Header.Set("X-Forwarded-For", "127.0.0.1")

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			// Check required fields
			for _, field := range tt.checkFields {
				suite.Contains(response, field, "Response should contain field: %s", field)
			}

			if tt.shouldHaveToken {
				token := response["token"].(string)
				suite.NotEmpty(token)
				suite.True(strings.Contains(token, "."))

				user := response["user"].(map[string]interface{})
				suite.Contains(user, "id")
				suite.Contains(user, "email")
				suite.NotContains(user, "password") // Password should never be in response
			}
		})
	}
}

func (suite *HandlersTestSuite) TestLogoutHandler() {
	// Register and login to get a valid token
	registerReq := &RegisterRequest{
		Email:    "logout@test.com",
		Password: "testpass123",
		Name:     "Logout Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	token, _, err := generateJWTToken(user)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		checkMessage   bool
	}{
		{
			name:           "valid logout",
			authHeader:     "Bearer " + token,
			expectedStatus: 200,
			checkMessage:   true,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			expectedStatus: 400,
			checkMessage:   false,
		},
		{
			name:           "invalid authorization format",
			authHeader:     "InvalidFormat " + token,
			expectedStatus: 400,
			checkMessage:   false,
		},
		{
			name:           "no bearer prefix",
			authHeader:     token,
			expectedStatus: 400,
			checkMessage:   false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/api/logout", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.checkMessage {
				suite.Contains(response, "message")
			} else {
				suite.Contains(response, "error")
			}
		})
	}
}

func (suite *HandlersTestSuite) TestProfileHandler() {
	// Register a test user and get token
	registerReq := &RegisterRequest{
		Email:    "profile@test.com",
		Password: "testpass123",
		Name:     "Profile Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	token, _, err := generateJWTToken(user)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		checkProfile   bool
	}{
		{
			name:           "valid profile request",
			authHeader:     "Bearer " + token,
			expectedStatus: 200,
			checkProfile:   true,
		},
		{
			name:           "missing authorization",
			authHeader:     "",
			expectedStatus: 401,
			checkProfile:   false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid.token.here",
			expectedStatus: 401,
			checkProfile:   false,
		},
		{
			name:           "invalid format",
			authHeader:     "InvalidFormat " + token,
			expectedStatus: 401,
			checkProfile:   false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("GET", "/api/profile", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.checkProfile {
				suite.Contains(response, "user")
				profile := response["user"].(map[string]interface{})
				suite.Equal(user.Email, profile["email"])
				suite.Equal(user.Name, profile["name"])
				suite.NotContains(profile, "password")
			} else {
				suite.Contains(response, "error")
			}
		})
	}
}

func (suite *HandlersTestSuite) TestAuthMiddleware() {
	// Create a test endpoint that requires auth
	testApp := fiber.New()
	testApp.Use(suite.mockAuthMiddleware)
	testApp.Get("/protected", func(c *fiber.Ctx) error {
		user := c.Locals("user").(*User)
		return c.JSON(fiber.Map{
			"message": "Access granted",
			"user_id": user.ID,
		})
	})

	// Register a test user and get token
	registerReq := &RegisterRequest{
		Email:    "middleware@test.com",
		Password: "testpass123",
		Name:     "Middleware Test User",
	}

	user, err := RegisterUser(registerReq)
	suite.Require().NoError(err)

	token, _, err := generateJWTToken(user)
	suite.Require().NoError(err)

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		shouldHaveAccess bool
	}{
		{
			name:             "valid token",
			authHeader:       "Bearer " + token,
			expectedStatus:   200,
			shouldHaveAccess: true,
		},
		{
			name:             "no authorization header",
			authHeader:       "",
			expectedStatus:   401,
			shouldHaveAccess: false,
		},
		{
			name:             "invalid token format",
			authHeader:       "InvalidFormat " + token,
			expectedStatus:   401,
			shouldHaveAccess: false,
		},
		{
			name:             "invalid token",
			authHeader:       "Bearer invalid.token.here",
			expectedStatus:   401,
			shouldHaveAccess: false,
		},
		{
			name:             "expired/malformed token",
			authHeader:       "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.invalid",
			expectedStatus:   401,
			shouldHaveAccess: false,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("GET", "/protected", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := testApp.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			if tt.shouldHaveAccess {
				suite.Contains(response, "message")
				suite.Contains(response, "user_id")
				suite.Equal("Access granted", response["message"])
				suite.Equal(float64(user.ID), response["user_id"])
			} else {
				suite.Contains(response, "error")
			}
		})
	}
}

func (suite *HandlersTestSuite) TestCORSAndHeaders() {
	// Test that handlers properly handle common HTTP headers
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{
		"email": "admin@gateway.local",
		"password": "admin123"
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://frontend.example.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
	req.Header.Set("Accept", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)
	suite.Equal("application/json; charset=utf-8", resp.Header.Get("Content-Type"))
}

func (suite *HandlersTestSuite) TestRequestValidation() {
	// Test various content types
	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid JSON",
			contentType:    "application/json",
			body:           `{"email": "test@test.com", "password": "password123", "name": "Test"}`,
			expectedStatus: 201,
		},
		{
			name:           "empty body",
			contentType:    "application/json",
			body:           "",
			expectedStatus: 400,
		},
		{
			name:           "invalid content type",
			contentType:    "text/plain",
			body:           `{"email": "test@test.com", "password": "password123", "name": "Test"}`,
			expectedStatus: 400,
		},
		{
			name:           "no content type",
			contentType:    "",
			body:           `{"email": "test2@test.com", "password": "password123", "name": "Test"}`,
			expectedStatus: 201, // Fiber often defaults to JSON parsing
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/api/register", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)
		})
	}
}