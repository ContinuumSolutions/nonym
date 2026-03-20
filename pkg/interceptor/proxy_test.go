package interceptor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/audit"
	"github.com/ContinuumSolutions/nonym/pkg/ner"
	"github.com/ContinuumSolutions/nonym/pkg/router"
)

func TestHandleProxy_HealthyRequest(t *testing.T) {
	// Initialize dependencies
	setupTestServices()

	// Create mock upstream server
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "Hello! I understand you have questions.",
					},
				},
			},
		})
	}))
	defer mockUpstream.Close()

	// Configure router with mock upstream
	updateRouterForTesting(mockUpstream.URL)

	// Create test request
	requestData := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Hello, how are you today?",
			},
		},
	}
	requestJSON, _ := json.Marshal(requestData)

	app := fiber.New()
	app.Post("/v1/chat/completions", HandleProxy)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-openai-key")

	resp, err := app.Test(req, 10*1000)
	if err != nil {
		t.Fatalf("Failed to test proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}
}

func TestHandleProxy_PIIDetectionAndAnonymization(t *testing.T) {
	setupTestServices()

	// Create mock upstream that verifies anonymization
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var receivedData map[string]interface{}
		json.Unmarshal(body, &receivedData)

		messages := receivedData["messages"].([]interface{})
		content := messages[0].(map[string]interface{})["content"].(string)

		// Verify PII was anonymized
		if strings.Contains(content, "john@example.com") {
			t.Errorf("Email should have been anonymized but wasn't")
		}
		if strings.Contains(content, "123-45-6789") {
			t.Errorf("SSN should have been anonymized but wasn't")
		}
		if !strings.Contains(content, "{{EMAIL_") {
			t.Errorf("Email should have been replaced with token")
		}
		if !strings.Contains(content, "{{SSN_") {
			t.Errorf("SSN should have been replaced with token")
		}

		// Echo the anonymized content back so the gateway can de-anonymize it.
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "I can help with: " + content,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockUpstream.Close()

	updateRouterForTesting(mockUpstream.URL)

	requestData := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "My email is john@example.com and my SSN is 123-45-6789",
			},
		},
	}
	requestJSON, _ := json.Marshal(requestData)

	app := fiber.New()
	app.Post("/v1/chat/completions", HandleProxy)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-openai-key")

	resp, err := app.Test(req, 10*1000)
	if err != nil {
		t.Fatalf("Failed to test proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	// Verify response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	choices := response["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content := message["content"].(string)

	// Verify de-anonymization worked
	if !strings.Contains(content, "john@example.com") {
		t.Errorf("Email should have been restored in response")
	}
	if strings.Contains(content, "{{EMAIL_") {
		t.Errorf("Email token should have been removed from response")
	}
}

func TestHandleProxy_StrictModeBlocking(t *testing.T) {
	setupTestServices()

	// Enable strict mode
	ner.SetStrictMode(true)
	defer ner.SetStrictMode(false)

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

	app := fiber.New()
	app.Post("/v1/chat/completions", HandleProxy)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test strict mode: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 403 {
		t.Errorf("Expected status 403 (blocked), got %d", resp.StatusCode)
	}
}

func TestHandleProxy_ContentTypeHandling(t *testing.T) {
	setupTestServices()

	// Mock upstream that accepts JSON and rejects other content types.
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer mockUpstream.Close()
	updateRouterForTesting(mockUpstream.URL)

	testCases := []struct {
		contentType string
		shouldWork  bool
		description string
	}{
		{"application/json", true, "Standard JSON content"},
		{"application/json; charset=utf-8", true, "JSON with charset"},
		{"text/plain", false, "Plain text should be rejected"},
		{"application/xml", false, "XML should be rejected"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			requestData := map[string]interface{}{
				"model":    "gpt-4",
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			}
			requestJSON, _ := json.Marshal(requestData)

			app := fiber.New()
			app.Post("/v1/chat/completions", HandleProxy)

			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestJSON))
			req.Header.Set("Content-Type", tc.contentType)

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("Failed to test content type: %v", err)
			}
			defer resp.Body.Close()

			if tc.shouldWork && resp.StatusCode >= 400 {
				t.Errorf("Expected success for %s, got %d", tc.contentType, resp.StatusCode)
			}
			if !tc.shouldWork && resp.StatusCode < 400 {
				t.Errorf("Expected error for %s, got %d", tc.contentType, resp.StatusCode)
			}
		})
	}
}

func TestHandleStatus(t *testing.T) {
	setupTestServices()

	app := fiber.New()
	app.Get("/gateway/status", HandleStatus)

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
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify required fields
	requiredFields := []string{"status", "ner_engine", "providers"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Expected field '%s' in status response", field)
		}
	}
}

func TestHandleStats(t *testing.T) {
	setupTestServices()

	app := fiber.New()
	app.Get("/gateway/stats", HandleStats)

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
		t.Fatalf("Failed to read response: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify required fields
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

// Helper functions
func setupTestServices() {
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	ner.Initialize()
	audit.Initialize(":memory:")
	router.Reset()
	router.Initialize(map[string]router.ProviderConfig{
		"openai": {
			BaseURL: "https://api.openai.com",
			Enabled: true,
		},
		"local": {
			BaseURL: "http://localhost:11434",
			Enabled: true,
		},
	})
}

func updateRouterForTesting(mockURL string) {
	router.Reset()
	router.Initialize(map[string]router.ProviderConfig{
		"openai": {
			BaseURL: mockURL,
			Enabled: true,
		},
	})
}
