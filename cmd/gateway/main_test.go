package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sovereignprivacy/gateway/pkg/audit"
	"github.com/sovereignprivacy/gateway/pkg/interceptor"
	"github.com/sovereignprivacy/gateway/pkg/ner"
	"github.com/sovereignprivacy/gateway/pkg/router"
)

func TestHealthEndpoint(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status to be 'healthy', got %v", response["status"])
	}
}

func TestProxyWithPIIDetection(t *testing.T) {
	app := setupTestApp()

	// Create a test request with PII data
	requestData := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "My email is john.doe@example.com and my SSN is 123-45-6789",
			},
		},
	}

	requestJSON, _ := json.Marshal(requestData)

	// Mock upstream server
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var receivedData map[string]interface{}
		json.Unmarshal(body, &receivedData)

		// Verify that PII was anonymized
		messages := receivedData["messages"].([]interface{})
		content := messages[0].(map[string]interface{})["content"].(string)

		if strings.Contains(content, "john.doe@example.com") {
			t.Errorf("Original email should have been anonymized")
		}
		if strings.Contains(content, "123-45-6789") {
			t.Errorf("Original SSN should have been anonymized")
		}
		if !strings.Contains(content, "{{EMAIL_") {
			t.Errorf("Email should have been replaced with token")
		}

		// Mock AI response
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "I understand you want to protect your email {{EMAIL_12345}} and SSN {{SSN_67890}}.",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockUpstream.Close()

	// Update router to use mock upstream
	updateMockProvider(mockUpstream.URL)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-openai-key")

	resp, err := app.Test(req, 30*1000) // 30 second timeout
	if err != nil {
		t.Fatalf("Failed to test proxy endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Verify response structure
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("Invalid response structure")
	}

	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content := message["content"].(string)

	// Verify that tokens were de-anonymized back to original content
	if !strings.Contains(content, "john.doe@example.com") {
		t.Errorf("Email should have been restored in response")
	}
	if strings.Contains(content, "{{EMAIL_") {
		t.Errorf("Email token should have been removed from response")
	}
}

func TestStrictModeBlocking(t *testing.T) {
	app := setupTestApp()

	// Enable strict mode
	ner.SetStrictMode(true)
	defer ner.SetStrictMode(false) // Reset after test

	// Create request with high-sensitivity data
	requestData := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "My SSN is 123-45-6789 and credit card is 4111111111111111",
			},
		},
	}

	requestJSON, _ := json.Marshal(requestData)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test strict mode blocking: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		t.Errorf("Expected status 403 (blocked), got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if !strings.Contains(response["error"].(string), "blocked") {
		t.Errorf("Expected blocked error message, got: %v", response["error"])
	}
}

func TestGatewayStatusEndpoint(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/gateway/status", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test status endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if response["status"] != "operational" {
		t.Errorf("Expected status to be 'operational', got %v", response["status"])
	}

	if response["ner_engine"] == nil {
		t.Errorf("Expected ner_engine status in response")
	}

	if response["providers"] == nil {
		t.Errorf("Expected providers status in response")
	}
}

func TestGatewayStatsEndpoint(t *testing.T) {
	app := setupTestApp()

	req := httptest.NewRequest("GET", "/gateway/stats", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test stats endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	// Check required fields exist
	requiredFields := []string{
		"total_requests",
		"blocked_requests",
		"anonymized_requests",
		"success_rate",
		"avg_processing_time",
	}

	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Expected field '%s' in stats response", field)
		}
	}
}

func TestDashboardAPI(t *testing.T) {
	app := setupDashboardApp()

	// Test statistics endpoint
	req := httptest.NewRequest("GET", "/api/statistics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test dashboard statistics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test transactions endpoint
	req = httptest.NewRequest("GET", "/api/transactions?limit=10", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test dashboard transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test settings endpoints
	req = httptest.NewRequest("GET", "/api/settings", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test dashboard settings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// Helper functions for testing

func setupTestApp() *fiber.App {
	// Set up test environment variables
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	// Initialize services
	ner.Initialize()
	audit.Initialize(":memory:") // Use in-memory SQLite for testing

	// Setup router with test configuration
	config := map[string]router.ProviderConfig{
		"openai": {
			BaseURL: "https://api.openai.com",
			Enabled: true,
		},
		"local": {
			BaseURL: "http://localhost:11434",
			Enabled: true,
		},
	}
	router.Initialize(config)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add test routes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "test",
		})
	})

	app.All("/v1/*", interceptor.HandleProxy)
	app.Get("/gateway/status", interceptor.HandleStatus)
	app.Get("/gateway/stats", interceptor.HandleStats)

	return app
}

func setupDashboardApp() *fiber.App {
	// Initialize audit system
	audit.Initialize(":memory:")

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	api := app.Group("/api")
	api.Get("/transactions", audit.HandleGetTransactions)
	api.Get("/statistics", audit.HandleGetStatistics)
	api.Get("/settings", audit.HandleGetSettings)
	api.Put("/settings", audit.HandleUpdateSettings)

	return app
}

func updateMockProvider(url string) {
	// This would update the router configuration to use the mock server
	// In a real implementation, you'd have a way to inject test configurations
	config := map[string]router.ProviderConfig{
		"openai": {
			BaseURL: url,
			Enabled: true,
		},
	}
	router.Initialize(config)
}