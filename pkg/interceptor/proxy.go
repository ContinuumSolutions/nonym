package interceptor

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sovereignprivacy/gateway/pkg/audit"
	"github.com/sovereignprivacy/gateway/pkg/ner"
	"github.com/sovereignprivacy/gateway/pkg/router"
)

// ProxyRequest represents an intercepted request
type ProxyRequest struct {
	ID          string            `json:"id"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Provider    string            `json:"provider"`
	Timestamp   time.Time         `json:"timestamp"`
	ClientIP    string            `json:"client_ip"`
	UserAgent   string            `json:"user_agent"`
}

// ProxyResponse represents the response after processing
type ProxyResponse struct {
	ID               string                 `json:"id"`
	StatusCode       int                    `json:"status_code"`
	Headers          map[string]string      `json:"headers"`
	Body             string                 `json:"body"`
	ProcessingTime   time.Duration          `json:"processing_time"`
	RedactionApplied bool                   `json:"redaction_applied"`
	RedactionDetails []ner.RedactionDetail  `json:"redaction_details"`
	OriginalTokens   map[string]string      `json:"original_tokens"`
}

var (
	// Global statistics
	totalRequests     int64
	blockedRequests   int64
	anonymizedRequests int64
)

// HandleProxy is the main entry point for intercepting and processing requests
func HandleProxy(c *fiber.Ctx) error {
	startTime := time.Now()

	// Generate unique request ID
	requestID := uuid.New().String()

	// Extract request details
	request := &ProxyRequest{
		ID:        requestID,
		Method:    c.Method(),
		Path:      c.Path(),
		Headers:   extractHeaders(c),
		Body:      string(c.Body()),
		Timestamp: startTime,
		ClientIP:  c.IP(),
		UserAgent: c.Get("User-Agent"),
	}

	// Determine target provider based on request
	provider, targetURL, err := router.DetermineProvider(request.Path, request.Headers)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Unable to route request",
			"details": err.Error(),
		})
	}

	request.Provider = provider

	// Apply NER analysis and anonymization
	processedBody, redactionDetails, err := ner.ProcessContent(request.Body)
	if err != nil {
		audit.LogTransaction(requestID, "error", fmt.Sprintf("NER processing failed: %v", err), 0, nil)
		return c.Status(500).JSON(fiber.Map{
			"error": "Content processing failed",
		})
	}

	// Check if content should be blocked (strict mode)
	if ner.ShouldBlock(redactionDetails) {
		blockedRequests++
		audit.LogTransaction(requestID, "blocked", "Content blocked due to sensitive data", 0, redactionDetails)
		return c.Status(403).JSON(fiber.Map{
			"error": "Request blocked due to sensitive content",
			"policy": "strict_mode",
		})
	}

	// Create modified request for upstream
	modifiedRequest := createUpstreamRequest(c, processedBody, targetURL)

	// Forward request to target provider
	resp, err := forwardRequest(modifiedRequest)
	if err != nil {
		audit.LogTransaction(requestID, "error", fmt.Sprintf("Upstream request failed: %v", err), 0, redactionDetails)
		return c.Status(502).JSON(fiber.Map{
			"error": "Upstream service unavailable",
		})
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to read response",
		})
	}

	// De-anonymize response (restore original tokens)
	finalResponse, err := ner.DeAnonymizeContent(string(responseBody), redactionDetails)
	if err != nil {
		finalResponse = string(responseBody) // Fallback to original response
	}

	// Prepare response
	proxyResponse := &ProxyResponse{
		ID:               requestID,
		StatusCode:       resp.StatusCode,
		Headers:          extractResponseHeaders(resp),
		Body:             finalResponse,
		ProcessingTime:   time.Since(startTime),
		RedactionApplied: len(redactionDetails) > 0,
		RedactionDetails: redactionDetails,
		OriginalTokens:   ner.ExtractTokenMap(redactionDetails),
	}

	// Update statistics
	totalRequests++
	if len(redactionDetails) > 0 {
		anonymizedRequests++
	}

	// Log transaction
	audit.LogTransaction(requestID, "success", provider, resp.StatusCode, redactionDetails)

	// Copy response headers
	for key, value := range proxyResponse.Headers {
		c.Set(key, value)
	}

	// Return processed response
	c.Status(resp.StatusCode)
	return c.SendString(finalResponse)
}

// HandleStatus returns the current status of the gateway
func HandleStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "operational",
		"uptime": time.Since(time.Now()).String(), // This would be properly calculated
		"version": "1.0.0",
		"ner_engine": ner.GetStatus(),
		"providers": router.GetProviderStatus(),
	})
}

// HandleStats returns processing statistics
func HandleStats(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"total_requests":     totalRequests,
		"blocked_requests":   blockedRequests,
		"anonymized_requests": anonymizedRequests,
		"success_rate":       calculateSuccessRate(),
		"avg_processing_time": calculateAvgProcessingTime(),
	})
}

// Helper functions

func extractHeaders(c *fiber.Ctx) map[string]string {
	headers := make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = string(value)
	})
	return headers
}

func extractResponseHeaders(resp *http.Response) map[string]string {
	headers := make(map[string]string)
	for key, values := range resp.Header {
		headers[key] = strings.Join(values, ", ")
	}
	return headers
}

func createUpstreamRequest(c *fiber.Ctx, modifiedBody string, targetURL *url.URL) *http.Request {
	// Create new request with modified body
	var bodyReader io.Reader
	if modifiedBody != "" {
		bodyReader = strings.NewReader(modifiedBody)
	}

	req, _ := http.NewRequest(c.Method(), targetURL.String(), bodyReader)

	// Copy headers (excluding some that should be managed by the proxy)
	c.Request().Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		if !isProxyHeader(keyStr) {
			req.Header.Set(keyStr, string(value))
		}
	})

	// Update content length if body was modified
	if modifiedBody != "" && modifiedBody != string(c.Body()) {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(modifiedBody)))
	}

	return req
}

func forwardRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
		},
	}

	return client.Do(req)
}

func isProxyHeader(header string) bool {
	proxyHeaders := []string{
		"Connection",
		"Proxy-Connection",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	}

	header = strings.ToLower(header)
	for _, ph := range proxyHeaders {
		if strings.ToLower(ph) == header {
			return true
		}
	}
	return false
}

func calculateSuccessRate() float64 {
	if totalRequests == 0 {
		return 100.0
	}
	return float64(totalRequests-blockedRequests) / float64(totalRequests) * 100.0
}

func calculateAvgProcessingTime() float64 {
	// This would be calculated from audit logs
	return 25.5 // Placeholder milliseconds
}