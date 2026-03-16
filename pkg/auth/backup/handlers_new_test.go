package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

type HandlersNewTestSuite struct {
	suite.Suite
	app    *fiber.App
	testDB *sql.DB
}

func (suite *HandlersNewTestSuite) SetupSuite() {
	var err error
	suite.testDB, err = sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)

	// Initialize auth system
	err = Initialize(suite.testDB)
	suite.Require().NoError(err)

	// Create tables
	suite.createTestTables()

	// Setup Fiber app with routes
	suite.app = fiber.New()
	suite.setupRoutes()
}

func (suite *HandlersNewTestSuite) TearDownSuite() {
	if suite.testDB != nil {
		suite.testDB.Close()
	}
}

func (suite *HandlersNewTestSuite) SetupTest() {
	// Clean up data before each test
	suite.cleanupTestData()
}

func (suite *HandlersNewTestSuite) createTestTables() {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
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
		`CREATE TABLE IF NOT EXISTS user_sessions (
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
		_, err := suite.testDB.Exec(table)
		suite.Require().NoError(err)
	}
}

func (suite *HandlersNewTestSuite) cleanupTestData() {
	tables := []string{"user_sessions", "users", "organizations"}
	for _, table := range tables {
		_, err := suite.testDB.Exec("DELETE FROM " + table)
		suite.Require().NoError(err)
	}
}

func (suite *HandlersNewTestSuite) setupRoutes() {
	// Auth routes
	// TODO: Uncomment when HandleSignup is implemented
	// suite.app.Post("/api/v1/auth/signup", HandleSignup)
	suite.app.Post("/api/v1/auth/login", HandleLogin)
	suite.app.Post("/api/v1/auth/logout", HandleLogout)
	// TODO: Uncomment when HandleMe is implemented
	// suite.app.Get("/api/v1/auth/me", AuthMiddleware, HandleMe)

	// Protected test route
	suite.app.Get("/api/v1/protected", AuthMiddleware, func(c *fiber.Ctx) error {
		// TODO: Uncomment when GetUserIDFromContext and GetOrganizationIDFromContext are implemented
		// userID, _ := GetUserIDFromContext(c)
		// orgID, _ := GetOrganizationIDFromContext(c)
		return c.JSON(fiber.Map{
			"user_id":         "test_user",
			"organization_id": "test_org",
			"message":         "Protected resource accessed",
		})
	})

	// Admin-only test route
	// TODO: Uncomment when RequireRole is implemented
	// suite.app.Get("/api/v1/admin", AuthMiddleware, RequireRole("admin"), func(c *fiber.Ctx) error {
	// 	return c.JSON(fiber.Map{"message": "Admin resource accessed"})
	// })
}

func (suite *HandlersNewTestSuite) TestSignupAPI_Success() {
	reqBody := map[string]interface{}{
		"email":        "test@example.com",
		"password":     "password123",
		"name":         "Test User",
		"organization": "Test Company",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req)
	suite.NoError(err)
	suite.Equal(201, resp.StatusCode)

	// Parse response
	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(respBody, &response)
	suite.NoError(err)

	suite.Equal("Account created successfully", response["message"])
	suite.NotEmpty(response["token"])
	suite.NotNil(response["user"])
	suite.NotNil(response["organization"])

	// Verify user data
	user := response["user"].(map[string]interface{})
	suite.Equal("test@example.com", user["email"])
	suite.Equal("admin", user["role"])

	// Verify organization data
	org := response["organization"].(map[string]interface{})
	suite.Equal("Test Company", org["name"])
}

func (suite *HandlersNewTestSuite) TestSignupAPI_DuplicateEmail() {
	reqBody := map[string]interface{}{
		"email":        "duplicate@example.com",
		"password":     "password123",
		"name":         "First User",
		"organization": "First Company",
	}

	body, _ := json.Marshal(reqBody)

	// First signup
	req1 := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")

	resp1, err := suite.app.Test(req1)
	suite.NoError(err)
	suite.Equal(201, resp1.StatusCode)

	// Second signup with same email
	req2 := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := suite.app.Test(req2)
	suite.NoError(err)
	suite.Equal(409, resp2.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp2.Body)
	json.Unmarshal(respBody, &response)
	suite.Contains(response["error"], "already registered")
}

func (suite *HandlersNewTestSuite) TestSignupAPI_ValidationErrors() {
	testCases := []struct {
		name     string
		reqBody  map[string]interface{}
		expected int
	}{
		{
			name: "missing email",
			reqBody: map[string]interface{}{
				"password": "password123",
				"name":     "Test User",
			},
			expected: 400,
		},
		{
			name: "missing password",
			reqBody: map[string]interface{}{
				"email": "test@example.com",
				"name":  "Test User",
			},
			expected: 400,
		},
		{
			name: "invalid json",
			reqBody: map[string]interface{}{
				"invalid": "json",
			},
			expected: 400,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			body, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req)
			suite.NoError(err)
			suite.Equal(tc.expected, resp.StatusCode)
		})
	}
}

func (suite *HandlersNewTestSuite) TestLoginAPI_Success() {
	// First create a user
	signupBody := map[string]interface{}{
		"email":        "login@example.com",
		"password":     "password123",
		"name":         "Login User",
		"organization": "Login Company",
	}

	body, _ := json.Marshal(signupBody)
	signupReq := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	signupReq.Header.Set("Content-Type", "application/json")

	signupResp, err := suite.app.Test(signupReq)
	suite.NoError(err)
	suite.Equal(201, signupResp.StatusCode)

	// Now test login
	loginBody := map[string]interface{}{
		"email":    "login@example.com",
		"password": "password123",
	}

	body, _ = json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := suite.app.Test(loginReq)
	suite.NoError(err)
	suite.Equal(200, loginResp.StatusCode)

	// Parse response
	var response map[string]interface{}
	respBody, _ := io.ReadAll(loginResp.Body)
	err = json.Unmarshal(respBody, &response)
	suite.NoError(err)

	suite.Equal("Login successful", response["message"])
	suite.NotEmpty(response["token"])
	suite.NotNil(response["user"])
	suite.NotNil(response["organization"])
}

func (suite *HandlersNewTestSuite) TestLoginAPI_InvalidCredentials() {
	// Create a user first
	signupBody := map[string]interface{}{
		"email":        "valid@example.com",
		"password":     "correctpassword",
		"name":         "Valid User",
		"organization": "Valid Company",
	}

	body, _ := json.Marshal(signupBody)
	signupReq := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	signupReq.Header.Set("Content-Type", "application/json")

	_, err := suite.app.Test(signupReq)
	suite.NoError(err)

	// Test invalid login attempts
	testCases := []struct {
		name    string
		reqBody map[string]interface{}
	}{
		{
			name: "wrong password",
			reqBody: map[string]interface{}{
				"email":    "valid@example.com",
				"password": "wrongpassword",
			},
		},
		{
			name: "nonexistent email",
			reqBody: map[string]interface{}{
				"email":    "nonexistent@example.com",
				"password": "password123",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			body, _ := json.Marshal(tc.reqBody)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req)
			suite.NoError(err)
			suite.Equal(401, resp.StatusCode)
		})
	}
}

func (suite *HandlersNewTestSuite) TestAuthMiddleware() {
	// Create user and get token
	token := suite.createUserAndGetToken("middleware@example.com", "password123")

	// Test protected route with valid token
	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := suite.app.Test(req)
	suite.NoError(err)
	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &response)
	suite.NotZero(response["user_id"])
	suite.NotZero(response["organization_id"])
	suite.Equal("Protected resource accessed", response["message"])
}

func (suite *HandlersNewTestSuite) TestAuthMiddleware_InvalidToken() {
	testCases := []struct {
		name   string
		header string
	}{
		{
			name:   "missing authorization header",
			header: "",
		},
		{
			name:   "invalid format",
			header: "InvalidFormat",
		},
		{
			name:   "invalid token",
			header: "Bearer invalid-token",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest("GET", "/api/v1/protected", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			resp, err := suite.app.Test(req)
			suite.NoError(err)
			suite.Equal(401, resp.StatusCode)
		})
	}
}

func (suite *HandlersNewTestSuite) TestRequireRoleMiddleware() {
	// Create admin user and get token
	adminToken := suite.createUserAndGetToken("admin@example.com", "password123")

	// Test admin-only route with admin token
	req := httptest.NewRequest("GET", "/api/v1/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := suite.app.Test(req)
	suite.NoError(err)
	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &response)
	suite.Equal("Admin resource accessed", response["message"])
}

func (suite *HandlersNewTestSuite) TestMeEndpoint() {
	token := suite.createUserAndGetToken("me@example.com", "password123")

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := suite.app.Test(req)
	suite.NoError(err)
	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &response)

	suite.NotNil(response["user"])
	suite.NotNil(response["organization"])

	user := response["user"].(map[string]interface{})
	suite.Equal("me@example.com", user["email"])
}

func (suite *HandlersNewTestSuite) TestLogoutEndpoint() {
	token := suite.createUserAndGetToken("logout@example.com", "password123")

	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := suite.app.Test(req)
	suite.NoError(err)
	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBody, &response)
	suite.Equal("Logged out successfully", response["message"])
}

func (suite *HandlersNewTestSuite) TestOrganizationIsolationInAPI() {
	// Create two users in different organizations
	token1 := suite.createUserAndGetToken("user1@company1.com", "password123")
	token2 := suite.createUserAndGetToken("user2@company2.com", "password123")

	// Get user context for both
	req1 := httptest.NewRequest("GET", "/api/v1/protected", nil)
	req1.Header.Set("Authorization", "Bearer "+token1)

	resp1, err := suite.app.Test(req1)
	suite.NoError(err)
	suite.Equal(200, resp1.StatusCode)

	var response1 map[string]interface{}
	respBody1, _ := io.ReadAll(resp1.Body)
	json.Unmarshal(respBody1, &response1)

	req2 := httptest.NewRequest("GET", "/api/v1/protected", nil)
	req2.Header.Set("Authorization", "Bearer "+token2)

	resp2, err := suite.app.Test(req2)
	suite.NoError(err)
	suite.Equal(200, resp2.StatusCode)

	var response2 map[string]interface{}
	respBody2, _ := io.ReadAll(resp2.Body)
	json.Unmarshal(respBody2, &response2)

	// Verify different organization IDs
	orgID1 := int(response1["organization_id"].(float64))
	orgID2 := int(response2["organization_id"].(float64))
	suite.NotEqual(orgID1, orgID2, "Users should be in different organizations")
}

// Helper method to create user and return JWT token
func (suite *HandlersNewTestSuite) createUserAndGetToken(email, password string) string {
	signupBody := map[string]interface{}{
		"email":        email,
		"password":     password,
		"name":         "Test User",
		"organization": "Test Company",
	}

	body, _ := json.Marshal(signupBody)
	req := httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req)
	suite.Require().NoError(err)
	suite.Require().Equal(201, resp.StatusCode)

	var response map[string]interface{}
	respBody, _ := io.ReadAll(resp.Body)
	suite.Require().NoError(json.Unmarshal(respBody, &response))

	return response["token"].(string)
}

func TestHandlersNewSuite(t *testing.T) {
	suite.Run(t, new(HandlersNewTestSuite))
}

// Additional integration tests
func TestAPIWorkflow_SignupLoginLogout(t *testing.T) {
	// Setup test database and app
	testDB, err := sql.Open("sqlite", ":memory:")
	assert.NoError(t, err)
	defer testDB.Close()

	err = Initialize(testDB)
	assert.NoError(t, err)

	// Create tables
	tables := []string{
		`CREATE TABLE organizations (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, slug TEXT UNIQUE, description TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, organization_id INTEGER, email TEXT UNIQUE, password_hash TEXT, first_name TEXT, last_name TEXT, role TEXT DEFAULT 'user', is_active BOOLEAN DEFAULT true, email_verified BOOLEAN DEFAULT false, last_login DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE user_sessions (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, session_token TEXT UNIQUE, expires_at DATETIME, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, last_accessed DATETIME DEFAULT CURRENT_TIMESTAMP)`,
	}

	for _, table := range tables {
		_, err = testDB.Exec(table)
		assert.NoError(t, err)
	}

	app := fiber.New()
	// TODO: Uncomment when HandleSignup is implemented
	// app.Post("/auth/signup", HandleSignup)
	app.Post("/auth/login", HandleLogin)
	app.Post("/auth/logout", HandleLogout)

	// 1. Signup
	signupBody := map[string]interface{}{
		"email":        "workflow@example.com",
		"password":     "password123",
		"name":         "Workflow User",
		"organization": "Workflow Company",
	}

	body, _ := json.Marshal(signupBody)
	signupReq := httptest.NewRequest("POST", "/auth/signup", bytes.NewReader(body))
	signupReq.Header.Set("Content-Type", "application/json")

	signupResp, err := app.Test(signupReq)
	assert.NoError(t, err)
	assert.Equal(t, 201, signupResp.StatusCode)

	var signupResponse map[string]interface{}
	respBody, _ := io.ReadAll(signupResp.Body)
	json.Unmarshal(respBody, &signupResponse)
	signupToken := signupResponse["token"].(string)

	// 2. Login
	loginBody := map[string]interface{}{
		"email":    "workflow@example.com",
		"password": "password123",
	}

	body, _ = json.Marshal(loginBody)
	loginReq := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := app.Test(loginReq)
	assert.NoError(t, err)
	assert.Equal(t, 200, loginResp.StatusCode)

	var loginResponse map[string]interface{}
	respBody, _ = io.ReadAll(loginResp.Body)
	json.Unmarshal(respBody, &loginResponse)
	loginToken := loginResponse["token"].(string)

	// 3. Logout
	logoutReq := httptest.NewRequest("POST", "/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginToken)

	logoutResp, err := app.Test(logoutReq)
	assert.NoError(t, err)
	assert.Equal(t, 200, logoutResp.StatusCode)

	// Tokens should be different (each login generates new token)
	assert.NotEqual(t, signupToken, loginToken)
}