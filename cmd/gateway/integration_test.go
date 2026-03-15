package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sovereignprivacy/gateway/pkg/audit"
	"github.com/sovereignprivacy/gateway/pkg/auth"
	"github.com/sovereignprivacy/gateway/pkg/ner"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

// GatewayIntegrationTestSuite tests the complete gateway application
type GatewayIntegrationTestSuite struct {
	suite.Suite
	app              *fiber.App
	db               *sql.DB
	mockAIServer     *httptest.Server
	testUser         *auth.User
	testOrg          *auth.Organization
	testUserToken    string
	testAPIKey       string
}

func (suite *GatewayIntegrationTestSuite) SetupSuite() {
	// Setup test database
	testDB, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	suite.db = testDB

	// Initialize all systems
	err = auth.Initialize(testDB)
	suite.Require().NoError(err)

	err = audit.Initialize(":memory:")
	suite.Require().NoError(err)

	err = ner.Initialize()
	suite.Require().NoError(err)

	// Create mock AI server
	suite.mockAIServer = httptest.NewServer(http.HandlerFunc(suite.mockAIHandler))

	// Set environment variables for testing
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("OPENAI_BASE_URL", suite.mockAIServer.URL)
	os.Setenv("ANTHROPIC_BASE_URL", suite.mockAIServer.URL)

	// Create test user and get token
	registerReq := &auth.RegisterRequest{
		Email:    "integration@test.com",
		Password: "testpass123",
		Name:     "Integration Test User",
	}

	suite.testUser, suite.testOrg, err = auth.RegisterUser(registerReq)
	suite.Require().NoError(err)

	loginReq := &auth.LoginRequest{
		Email:    "integration@test.com",
		Password: "testpass123",
	}

	loginResp, err := auth.LoginUser(loginReq, "127.0.0.1", "test-agent")
	suite.Require().NoError(err)
	suite.testUserToken = loginResp.Token

	// Create test API key
	apiKeyReq := &auth.APIKeyCreateRequest{
		Name:        "Integration Test Key",
		Permissions: "read",
	}

	apiKeyResp, err := auth.CreateAPIKey(apiKeyReq, fmt.Sprintf("%d", suite.testUser.ID), suite.testOrg.ID)
	suite.Require().NoError(err)
	suite.testAPIKey = apiKeyResp.APIKey

	// Setup the actual application
	suite.app = setupIntegrationTestApp()
}

func (suite *GatewayIntegrationTestSuite) TearDownSuite() {
	if suite.mockAIServer != nil {
		suite.mockAIServer.Close()
	}
	if suite.db != nil {
		suite.db.Close()
	}
}

func (suite *GatewayIntegrationTestSuite) mockAIHandler(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, _ := io.ReadAll(r.Body)

	// Check for anonymized content in request
	bodyStr := string(body)
	hasTokens := false
	if bytes.Contains(body, []byte("{{EMAIL_")) ||
	   bytes.Contains(body, []byte("{{SSN_")) ||
	   bytes.Contains(body, []byte("{{PHONE_")) {
		hasTokens = true
	}

	switch r.URL.Path {
	case "/v1/chat/completions":
		response := map[string]interface{}{
			"id":      "chatcmpl-test-123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-3.5-turbo",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": fmt.Sprintf("I received your message (anonymized: %v): %s", hasTokens, bodyStr[:min(100, len(bodyStr))]),
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 20,
				"total_tokens":      30,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	case "/v1/completions":
		response := map[string]interface{}{
			"id":      "cmpl-test-123",
			"object":  "text_completion",
			"created": time.Now().Unix(),
			"model":   "text-davinci-003",
			"choices": []map[string]interface{}{
				{
					"text":         "Completion response",
					"index":        0,
					"finish_reason": "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func setupIntegrationTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add basic middleware and routes that would be in the main application
	// This is a simplified version - the actual app setup would be more complex

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "test",
			"timestamp": time.Now().Unix(),
		})
	})

	// Gateway status
	app.Get("/gateway/status", func(c *fiber.Ctx) error {
		nerStatus := ner.GetStatus()
		return c.JSON(fiber.Map{
			"gateway": "operational",
			"ner":     nerStatus,
			"auth":    "operational",
			"audit":   "operational",
		})
	})

	// Auth routes
	authGroup := app.Group("/auth")
	authGroup.Post("/register", func(c *fiber.Ctx) error {
		var req auth.RegisterRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		user, _, err := auth.RegisterUser(&req)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(201).JSON(fiber.Map{
			"message": "User registered successfully",
			"user": map[string]interface{}{
				"id":    user.ID,
				"email": user.Email,
				"name":  user.Name,
			},
		})
	})

	authGroup.Post("/login", func(c *fiber.Ctx) error {
		var req auth.LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}

		response, err := auth.LoginUser(&req, c.IP(), c.Get("User-Agent"))
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(response)
	})

	// API routes with auth middleware
	apiGroup := app.Group("/api")
	apiGroup.Use(func(c *fiber.Ctx) error {
		// Simple auth middleware for testing
		token := c.Get("Authorization")
		if token == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization required"})
		}

		token = token[7:] // Remove "Bearer "
		user, err := auth.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
		}

		c.Locals("user", user)
		return c.Next()
	})

	apiGroup.Get("/transactions", audit.HandleGetTransactions)
	apiGroup.Get("/statistics", audit.HandleGetStatistics)
	apiGroup.Get("/api-keys", auth.HandleGetAPIKeys)
	apiGroup.Post("/api-keys", auth.HandleCreateAPIKey)

	// Gateway proxy routes (simplified)
	app.All("/v1/*", func(c *fiber.Ctx) error {
		// This would be the main proxy logic in the real app
		// For testing, we'll simulate PII detection and forwarding

		// Read request body
		body := c.Body()
		bodyStr := string(body)

		// Detect PII
		processed, details, err := ner.ProcessContent(bodyStr)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "PII processing failed"})
		}

		// For testing, we'll just return a mock response
		// In the real app, this would forward to the AI provider
		if len(details) > 0 {
			// Log transaction with PII detection
			audit.LogTransaction(
				fmt.Sprintf("tx-%d", time.Now().UnixNano()),
				"success",
				"openai",
				200,
				details,
				1, // test organizationID
				1, // test userID
			)

			// Simulate de-anonymization of response
			mockResponse := "Response with detected PII tokens: " + processed
			restored, err := ner.DeAnonymizeContent(mockResponse, details)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "De-anonymization failed"})
			}

			return c.JSON(fiber.Map{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": restored,
						},
					},
				},
				"pii_detected": len(details),
			})
		}

		return c.JSON(fiber.Map{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "No PII detected: " + bodyStr,
					},
				},
			},
			"pii_detected": 0,
		})
	})

	return app
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestGatewayIntegrationSuite(t *testing.T) {
	suite.Run(t, new(GatewayIntegrationTestSuite))
}

func (suite *GatewayIntegrationTestSuite) TestHealthEndpoint() {
	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Equal("ok", response["status"])
	suite.Contains(response, "timestamp")
}

func (suite *GatewayIntegrationTestSuite) TestGatewayStatus() {
	req := httptest.NewRequest("GET", "/gateway/status", nil)
	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Equal("operational", response["gateway"])
	suite.Contains(response, "ner")
	suite.Equal("operational", response["auth"])
	suite.Equal("operational", response["audit"])
}

func (suite *GatewayIntegrationTestSuite) TestUserRegistrationFlow() {
	requestBody := map[string]interface{}{
		"email":    "newuser@test.com",
		"password": "password123",
		"name":     "New Test User",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(201, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "message")
	suite.Contains(response, "user")

	user := response["user"].(map[string]interface{})
	suite.Equal("newuser@test.com", user["email"])
	suite.Equal("New Test User", user["name"])
}

func (suite *GatewayIntegrationTestSuite) TestUserLoginFlow() {
	requestBody := map[string]interface{}{
		"email":    "integration@test.com",
		"password": "testpass123",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "token")
	suite.Contains(response, "expires_at")
	suite.Contains(response, "user")

	token := response["token"].(string)
	suite.NotEmpty(token)
	suite.True(len(token) > 10)
}

func (suite *GatewayIntegrationTestSuite) TestAPIKeyManagement() {
	// Create API key
	requestBody := map[string]interface{}{
		"name":        "Test Integration Key",
		"permissions": "read",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/api/api-keys", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+suite.testUserToken)

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(201, resp.StatusCode)

	var createResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createResponse)
	suite.NoError(err)

	suite.Contains(createResponse, "api_key")
	suite.Contains(createResponse, "id")
	suite.Contains(createResponse, "masked_key")

	// Get API keys
	req = httptest.NewRequest("GET", "/api/api-keys", nil)
	req.Header.Set("Authorization", "Bearer "+suite.testUserToken)

	resp, err = suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var getResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&getResponse)
	suite.NoError(err)

	suite.Contains(getResponse, "api_keys")
	apiKeys := getResponse["api_keys"].([]interface{})
	suite.GreaterOrEqual(len(apiKeys), 1)
}

func (suite *GatewayIntegrationTestSuite) TestPIIDetectionInGateway() {
	// Test request with PII
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "My email is john@example.com and phone is 555-123-4567",
			},
		},
		"model": "gpt-3.5-turbo",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "choices")
	suite.Contains(response, "pii_detected")

	piiDetected := response["pii_detected"].(float64)
	suite.Greater(piiDetected, float64(0), "Should detect PII in the request")

	// Verify original PII data is restored in response
	choices := response["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	content := message["content"].(string)

	suite.Contains(content, "john@example.com")
	suite.Contains(content, "555-123-4567")
}

func (suite *GatewayIntegrationTestSuite) TestRequestWithoutPII() {
	// Test request without PII
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "What is the weather like today?",
			},
		},
		"model": "gpt-3.5-turbo",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "pii_detected")
	piiDetected := response["pii_detected"].(float64)
	suite.Equal(float64(0), piiDetected, "Should not detect PII in clean request")
}

func (suite *GatewayIntegrationTestSuite) TestAuditLoggingIntegration() {
	// Make a request to generate audit log
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Test audit logging with email: test@audit.com",
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	// Check audit logs
	req = httptest.NewRequest("GET", "/api/transactions?limit=5", nil)
	req.Header.Set("Authorization", "Bearer "+suite.testUserToken)

	resp, err = suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var auditResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&auditResponse)
	suite.NoError(err)

	suite.Contains(auditResponse, "transactions")
	transactions := auditResponse["transactions"].([]interface{})
	suite.GreaterOrEqual(len(transactions), 1)

	// Check that recent transaction is logged
	transaction := transactions[0].(map[string]interface{})
	suite.Contains(transaction, "id")
	suite.Contains(transaction, "status")
	suite.Contains(transaction, "provider")
}

func (suite *GatewayIntegrationTestSuite) TestStatisticsEndpoint() {
	// Make some requests to generate statistics
	for i := 0; i < 3; i++ {
		requestBody := map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": fmt.Sprintf("Test request %d", i),
				},
			},
		}

		jsonBody, err := json.Marshal(requestBody)
		suite.NoError(err)

		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err := suite.app.Test(req, -1)
		suite.NoError(err)
		resp.Body.Close()
	}

	// Check statistics
	req := httptest.NewRequest("GET", "/api/statistics", nil)
	req.Header.Set("Authorization", "Bearer "+suite.testUserToken)

	resp, err := suite.app.Test(req, -1)
	suite.NoError(err)
	defer resp.Body.Close()

	suite.Equal(200, resp.StatusCode)

	var statsResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&statsResponse)
	suite.NoError(err)

	// Verify statistics structure
	requiredFields := []string{
		"total_requests",
		"successful_requests",
		"blocked_requests",
		"redacted_requests",
		"success_rate",
		"avg_processing_time",
		"top_providers",
		"recent_activity",
	}

	for _, field := range requiredFields {
		suite.Contains(statsResponse, field, "Statistics should contain field: %s", field)
	}

	suite.Greater(statsResponse["total_requests"], float64(0))
}

func (suite *GatewayIntegrationTestSuite) TestAuthenticationRequired() {
	// Test that protected endpoints require authentication
	protectedEndpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/transactions"},
		{"GET", "/api/statistics"},
		{"GET", "/api/api-keys"},
		{"POST", "/api/api-keys"},
	}

	for _, endpoint := range protectedEndpoints {
		suite.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func() {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(401, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			suite.NoError(err)

			suite.Contains(response, "error")
		})
	}
}

func (suite *GatewayIntegrationTestSuite) TestErrorHandling() {
	// Test various error conditions
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			method:         "POST",
			path:           "/v1/chat/completions",
			body:           "{invalid json}",
			headers:        map[string]string{"Content-Type": "application/json"},
			expectedStatus: 400,
		},
		{
			name:           "Missing Content-Type",
			method:         "POST",
			path:           "/v1/chat/completions",
			body:           `{"test": "value"}`,
			headers:        map[string]string{},
			expectedStatus: 200, // Should still work with default parsing
		},
		{
			name:           "Non-existent endpoint",
			method:         "GET",
			path:           "/nonexistent",
			body:           "",
			headers:        map[string]string{},
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader([]byte(tt.body)))
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			resp, err := suite.app.Test(req, -1)
			suite.NoError(err)
			defer resp.Body.Close()

			suite.Equal(tt.expectedStatus, resp.StatusCode)
		})
	}
}

func (suite *GatewayIntegrationTestSuite) TestConcurrentRequests() {
	// Test handling multiple concurrent requests
	numRequests := 10
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			requestBody := map[string]interface{}{
				"messages": []map[string]interface{}{
					{
						"role":    "user",
						"content": fmt.Sprintf("Concurrent request %d", index),
					},
				},
			}

			jsonBody, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.app.Test(req, -1)
			if err == nil {
				resp.Body.Close()
				results <- resp.StatusCode
			} else {
				results <- 500
			}
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		statusCode := <-results
		if statusCode == 200 {
			successCount++
		}
	}

	// Most requests should succeed
	suite.GreaterOrEqual(successCount, numRequests*8/10) // 80% success rate
}

// Benchmark tests for the complete system
func BenchmarkGatewayCompleteFlow(b *testing.B) {
	// This would benchmark the complete request flow through the gateway
	// Setup would be similar to the integration tests
	b.Skip("Benchmark test - implement if needed for performance testing")
}

func BenchmarkGatewayPIIDetection(b *testing.B) {
	// This would benchmark the PII detection performance in the gateway
	b.Skip("Benchmark test - implement if needed for performance testing")
}