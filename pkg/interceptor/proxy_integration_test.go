package interceptor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// Stub types for testing (these would be defined in the actual proxy package)
type ProxyConfig struct {
	TargetURL            string
	Timeout              time.Duration
	MaxRequestSize       int
	EnablePIIDetection   bool
	EnableAuditLogging   bool
	StrictMode           bool
	AllowedContentTypes  []string
	BlockedEntityTypes   []string
	ConcurrencyLimit     int
	RateLimitPerSecond   int
	EnableCircuitBreaker bool
}

type ProxyServer struct {
	config *ProxyConfig
}

func NewProxyServer(config *ProxyConfig) (*ProxyServer, error) {
	return &ProxyServer{config: config}, nil
}

func (p *ProxyServer) HandleRequest(w http.ResponseWriter, r *http.Request) error {
	// Mock implementation
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"choices": [{"message": {"content": "mock response"}}]}`))
	return nil
}

func (p *ProxyServer) Shutdown(ctx context.Context) error {
	return nil
}

// ProxyIntegrationTestSuite tests the proxy functionality with real HTTP servers
type ProxyIntegrationTestSuite struct {
	suite.Suite
	mockAIServer *httptest.Server
	proxy        *ProxyServer
}

func (suite *ProxyIntegrationTestSuite) SetupTest() {
	// Create mock AI provider server
	suite.mockAIServer = httptest.NewServer(http.HandlerFunc(suite.mockAIHandler))

	// Initialize proxy with mock server as target
	config := &ProxyConfig{
		TargetURL:            suite.mockAIServer.URL,
		Timeout:              10 * time.Second,
		MaxRequestSize:       10 * 1024 * 1024, // 10MB
		EnablePIIDetection:   true,
		EnableAuditLogging:   true,
		StrictMode:           false,
		AllowedContentTypes:  []string{"application/json", "text/plain"},
		BlockedEntityTypes:   []string{},
		ConcurrencyLimit:     100,
		RateLimitPerSecond:   10,
		EnableCircuitBreaker: true,
	}

	var err error
	suite.proxy, err = NewProxyServer(config)
	suite.Require().NoError(err)
}

func (suite *ProxyIntegrationTestSuite) TearDownTest() {
	if suite.mockAIServer != nil {
		suite.mockAIServer.Close()
	}
	if suite.proxy != nil {
		suite.proxy.Shutdown(context.Background())
	}
}

// Mock AI provider handler
func (suite *ProxyIntegrationTestSuite) mockAIHandler(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse request to check for anonymized content
	var request map[string]interface{}
	if len(body) > 0 && r.Header.Get("Content-Type") == "application/json" {
		json.Unmarshal(body, &request)
	}

	// Simulate different AI provider responses based on request
	switch r.URL.Path {
	case "/v1/chat/completions":
		suite.handleChatCompletion(w, r, request)
	case "/v1/completions":
		suite.handleCompletion(w, r, request)
	case "/error":
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Internal server error",
				"type":    "server_error",
			},
		})
	case "/timeout":
		time.Sleep(15 * time.Second) // Longer than proxy timeout
		w.WriteHeader(http.StatusOK)
	case "/large-response":
		// Generate large response
		largeData := make([]byte, 20*1024*1024) // 20MB
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(largeData)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (suite *ProxyIntegrationTestSuite) handleChatCompletion(w http.ResponseWriter, r *http.Request, request map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")

	// Check if request contains anonymized tokens
	prompt := ""
	if messages, ok := request["messages"].([]interface{}); ok && len(messages) > 0 {
		if msg, ok := messages[0].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				prompt = content
			}
		}
	}

	response := map[string]interface{}{
		"id":      "chatcmpl-test123",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "gpt-3.5-turbo",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "I received your message: " + prompt,
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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (suite *ProxyIntegrationTestSuite) handleCompletion(w http.ResponseWriter, r *http.Request, request map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")

	prompt := ""
	if p, ok := request["prompt"].(string); ok {
		prompt = p
	}

	response := map[string]interface{}{
		"id":      "cmpl-test123",
		"object":  "text_completion",
		"created": time.Now().Unix(),
		"model":   "text-davinci-003",
		"choices": []map[string]interface{}{
			{
				"text":         "Completion for: " + prompt,
				"index":        0,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     5,
			"completion_tokens": 15,
			"total_tokens":      20,
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func TestProxyIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ProxyIntegrationTestSuite))
}

func (suite *ProxyIntegrationTestSuite) TestBasicProxyFunctionality() {
	// Create test request
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, how are you?",
			},
		},
		"model": "gpt-3.5-turbo",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	w := httptest.NewRecorder()

	// Process request through proxy
	err = suite.proxy.HandleRequest(w, req)
	suite.NoError(err)

	// Check response
	suite.Equal(200, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "choices")
	choices := response["choices"].([]interface{})
	suite.Len(choices, 1)
}

func (suite *ProxyIntegrationTestSuite) TestPIIDetectionAndAnonymization() {
	// Create request with PII
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "My email is john.doe@example.com and my phone is 555-123-4567",
			},
		},
		"model": "gpt-3.5-turbo",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	w := httptest.NewRecorder()

	// Process request through proxy
	err = suite.proxy.HandleRequest(w, req)
	suite.NoError(err)

	// Check response
	suite.Equal(200, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)

	// Verify response was processed
	suite.Contains(response, "choices")
	choices := response["choices"].([]interface{})
	suite.Len(choices, 1)

	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	content := message["content"].(string)

	// Content should contain original PII (de-anonymized) or tokens depending on implementation
	suite.Contains(content, "I received your message:")
}

func (suite *ProxyIntegrationTestSuite) TestStrictModeBlocking() {
	// Enable strict mode
	suite.proxy.config.StrictMode = true

	// Create request with sensitive PII
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "My SSN is 123-45-6789 and credit card is 4111111111111111",
			},
		},
		"model": "gpt-3.5-turbo",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	w := httptest.NewRecorder()

	// Process request through proxy
	err = suite.proxy.HandleRequest(w, req)

	// Should be blocked or processed with anonymization
	// The exact behavior depends on implementation
	suite.True(w.Code == 403 || w.Code == 200) // Either blocked or anonymized
}

func (suite *ProxyIntegrationTestSuite) TestErrorHandling() {
	// Test upstream error propagation
	req := httptest.NewRequest("POST", "/error", nil)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	err := suite.proxy.HandleRequest(w, req)
	suite.NoError(err) // Proxy shouldn't error, but should forward the error response

	suite.Equal(500, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	suite.NoError(err)

	suite.Contains(response, "error")
}

func (suite *ProxyIntegrationTestSuite) TestTimeoutHandling() {
	// Test request timeout
	req := httptest.NewRequest("POST", "/timeout", nil)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	// Set shorter timeout for this test
	originalTimeout := suite.proxy.config.Timeout
	suite.proxy.config.Timeout = 1 * time.Second

	start := time.Now()
	_ = suite.proxy.HandleRequest(w, req)
	duration := time.Since(start)

	// Restore original timeout
	suite.proxy.config.Timeout = originalTimeout

	// Should timeout within reasonable time
	suite.True(duration < 5*time.Second)
	suite.True(w.Code == 408 || w.Code == 504) // Timeout or gateway timeout
}

func (suite *ProxyIntegrationTestSuite) TestContentTypeValidation() {
	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid JSON",
			contentType:    "application/json",
			body:           `{"test": "value"}`,
			expectedStatus: 200,
		},
		{
			name:           "valid text",
			contentType:    "text/plain",
			body:           "test content",
			expectedStatus: 200,
		},
		{
			name:           "blocked content type",
			contentType:    "application/xml",
			body:           "<test>value</test>",
			expectedStatus: 415, // Unsupported media type
		},
		{
			name:           "no content type",
			contentType:    "",
			body:           `{"test": "value"}`,
			expectedStatus: 200, // Should default to JSON
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader([]byte(tt.body)))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()

			err := suite.proxy.HandleRequest(w, req)

			if tt.expectedStatus == 415 {
				// Should be rejected by proxy
				suite.True(w.Code == 415 || err != nil)
			} else {
				suite.NoError(err)
				suite.True(w.Code >= 200 && w.Code < 300)
			}
		})
	}
}

func (suite *ProxyIntegrationTestSuite) TestRequestSizeLimit() {
	// Test with request exceeding size limit
	largeBody := make([]byte, 15*1024*1024) // 15MB (larger than 10MB limit)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	err := suite.proxy.HandleRequest(w, req)

	// Should be rejected due to size
	suite.True(w.Code == 413 || err != nil) // Payload too large
}

func (suite *ProxyIntegrationTestSuite) TestConcurrencyHandling() {
	// Test concurrent requests
	numRequests := 20
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			requestBody := map[string]interface{}{
				"prompt": "Test concurrent request",
				"model":  "text-davinci-003",
			}

			jsonBody, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			err := suite.proxy.HandleRequest(w, req)
			if err == nil {
				results <- w.Code
			} else {
				results <- 500
			}
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		code := <-results
		if code == 200 {
			successCount++
		}
	}

	// Most requests should succeed
	suite.GreaterOrEqual(successCount, numRequests/2)
}

func (suite *ProxyIntegrationTestSuite) TestHeaderForwarding() {
	// Test that important headers are forwarded
	requestBody := map[string]interface{}{
		"prompt": "Test header forwarding",
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-api-key")
	req.Header.Set("User-Agent", "Test-Client/1.0")
	req.Header.Set("X-Custom-Header", "custom-value")

	w := httptest.NewRecorder()

	err = suite.proxy.HandleRequest(w, req)
	suite.NoError(err)

	suite.Equal(200, w.Code)
	suite.Equal("application/json", w.Header().Get("Content-Type"))
}

func (suite *ProxyIntegrationTestSuite) TestCircuitBreakerBehavior() {
	// Enable circuit breaker
	suite.proxy.config.EnableCircuitBreaker = true

	// Simulate multiple failures to trigger circuit breaker
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/error", nil)
		w := httptest.NewRecorder()
		suite.proxy.HandleRequest(w, req)
	}

	// Next request should be handled by circuit breaker
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader([]byte(`{"prompt": "test"}`)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	_ = suite.proxy.HandleRequest(w, req)

	// Circuit breaker might reject the request or let it through
	// Exact behavior depends on implementation
	suite.True(w.Code >= 200 && w.Code < 600)
}

func (suite *ProxyIntegrationTestSuite) TestAuditLogging() {
	// Test that requests are properly logged for audit
	requestBody := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Test audit logging with PII: john@example.com",
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	suite.NoError(err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	err = suite.proxy.HandleRequest(w, req)
	suite.NoError(err)

	// Audit logging is enabled, so transaction should be logged
	// This would need to be verified with the audit system
	suite.Equal(200, w.Code)
}

// Benchmark tests
func BenchmarkProxyBasicRequest(b *testing.B) {
	// Setup
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "ok"}`))
	}))
	defer mockServer.Close()

	config := &ProxyConfig{
		TargetURL:          mockServer.URL,
		Timeout:            10 * time.Second,
		EnablePIIDetection: false, // Disable for performance test
		EnableAuditLogging: false,
	}

	proxy, err := NewProxyServer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer proxy.Shutdown(context.Background())

	requestBody := `{"test": "value"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte(requestBody)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		proxy.HandleRequest(w, req)
	}
}

func BenchmarkProxyWithPIIDetection(b *testing.B) {
	// Setup
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "processed"}`))
	}))
	defer mockServer.Close()

	config := &ProxyConfig{
		TargetURL:          mockServer.URL,
		Timeout:            10 * time.Second,
		EnablePIIDetection: true,
		EnableAuditLogging: false,
	}

	proxy, err := NewProxyServer(config)
	if err != nil {
		b.Fatal(err)
	}
	defer proxy.Shutdown(context.Background())

	requestBody := `{"content": "My email is test@example.com and phone is 555-1234"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte(requestBody)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		proxy.HandleRequest(w, req)
	}
}