package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/ner"
)

const (
	testOrgID  = "1"
	testUserID = 1
)

func TestInitializeAuditSystem(t *testing.T) {
	err := Initialize(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize audit system: %v", err)
	}

	// Verify database is accessible by running a simple query
	_, err = GetStatistics("1")
	if err != nil {
		t.Errorf("Failed to get statistics after initialization: %v", err)
	}
}

func TestLogTransaction(t *testing.T) {
	Initialize(":memory:")

	// Test logging a transaction
	redactionDetails := []ner.RedactionDetail{
		{
			EntityType:   ner.EntityEmail,
			OriginalText: "john@example.com",
			RedactedText: "{{EMAIL_12345}}",
			Confidence:   0.95,
			StartIndex:   10,
			EndIndex:     25,
		},
	}

	LogTransaction("test-123", "success", "openai", "", 200, redactionDetails, 1, 1)

	// Verify transaction was stored
	transactions, err := GetTransactions(1, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(transactions) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(transactions))
	}

	stored := transactions[0]
	if stored.ID != "test-123" {
		t.Errorf("Expected transaction ID test-123, got %s", stored.ID)
	}
	if stored.Status != "success" {
		t.Errorf("Expected status success, got %s", stored.Status)
	}
	if stored.Provider != "openai" {
		t.Errorf("Expected provider openai, got %s", stored.Provider)
	}
	if stored.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", stored.StatusCode)
	}
	if stored.RedactionCount != 1 {
		t.Errorf("Expected redaction count 1, got %d", stored.RedactionCount)
	}
}

func TestLogTransactionWithoutRedactions(t *testing.T) {
	Initialize(":memory:")

	// Test logging without redactions
	LogTransaction("test-456", "success", "anthropic", "", 200, nil, 1, 1)

	transactions, err := GetTransactions(1, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(transactions) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(transactions))
	}

	stored := transactions[0]
	if stored.RedactionCount != 0 {
		t.Errorf("Expected redaction count 0, got %d", stored.RedactionCount)
	}
	if len(stored.RedactionDetails) != 0 {
		t.Errorf("Expected empty redaction details, got %d items", len(stored.RedactionDetails))
	}
}

func TestLogBlockedTransaction(t *testing.T) {
	Initialize(":memory:")

	// Log a blocked transaction
	LogTransaction("blocked-123", "blocked", "local", "", 403, []ner.RedactionDetail{
		{EntityType: "ssn", OriginalText: "123-45-6789", RedactedText: "{{SSN_ABC}}"},
	}, 1, 1)

	// Verify in statistics
	stats, err := GetStatistics("1")
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	if stats.BlockedRequests != 1 {
		t.Errorf("Expected 1 blocked request in stats, got %d", stats.BlockedRequests)
	}
	if stats.TotalRequests != 1 {
		t.Errorf("Expected 1 total request in stats, got %d", stats.TotalRequests)
	}
}

func TestGetStatistics(t *testing.T) {
	Initialize(":memory:")

	// Log some test transactions
	LogTransaction("tx1", "success", "openai", "", 200, []ner.RedactionDetail{
		{EntityType: "email", OriginalText: "test@example.com", RedactedText: "{{EMAIL_A}}"},
	}, 1, 1)
	LogTransaction("tx2", "success", "local", "", 200, nil, 1, 1)
	LogTransaction("tx3", "error", "openai", "", 500, []ner.RedactionDetail{
		{EntityType: "ssn", OriginalText: "123-45-6789", RedactedText: "{{SSN_B}}"},
	}, 1, 1)
	LogTransaction("blocked1", "blocked", "anthropic", "", 403, []ner.RedactionDetail{
		{EntityType: "credit_card", OriginalText: "4111111111111111", RedactedText: "{{CARD_C}}"},
	}, 1, 1)

	stats, err := GetStatistics("1")
	if err != nil {
		t.Fatalf("Failed to get statistics: %v", err)
	}

	if stats.TotalRequests != 4 {
		t.Errorf("Expected 4 total requests, got %d", stats.TotalRequests)
	}
	if stats.SuccessfulRequests != 2 {
		t.Errorf("Expected 2 successful requests, got %d", stats.SuccessfulRequests)
	}
	if stats.BlockedRequests != 1 {
		t.Errorf("Expected 1 blocked request, got %d", stats.BlockedRequests)
	}
	if stats.RedactedRequests != 3 {
		t.Errorf("Expected 3 redacted requests (tx1, tx3, blocked1), got %d", stats.RedactedRequests)
	}
	if stats.SuccessRate != 50.0 { // 2 successful out of 4 total
		t.Errorf("Expected success rate 50%%, got %.1f%%", stats.SuccessRate)
	}

	// Check top providers
	if len(stats.TopProviders) == 0 {
		t.Errorf("Expected provider statistics")
	} else {
		// OpenAI should be top with 2 requests
		found := false
		for _, provider := range stats.TopProviders {
			if provider.Provider == "openai" && provider.Requests == 2 {
				found = true
				if provider.Percent != 50.0 { // 2 out of 4 = 50%
					t.Errorf("Expected OpenAI percentage 50%%, got %.1f%%", provider.Percent)
				}
				break
			}
		}
		if !found {
			t.Errorf("OpenAI provider stats not found or incorrect")
		}
	}
}

func TestHandleGetTransactions(t *testing.T) {
	Initialize(":memory:")

	// Log some test transactions
	for i := 0; i < 5; i++ {
		LogTransaction(fmt.Sprintf("tx%d", i), "success", "openai", "", 200, nil, 1, 1)
	}

	app := fiber.New()
	// Add middleware to simulate authentication context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("organization_id", "1")
		return c.Next()
	})
	app.Get("/api/transactions", HandleGetTransactions)

	// Test default limit
	req := httptest.NewRequest("GET", "/api/transactions", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test transactions endpoint: %v", err)
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

	transactions := response["transactions"].([]interface{})
	if len(transactions) != 5 {
		t.Errorf("Expected 5 transactions, got %d", len(transactions))
	}

	// Test with limit parameter
	req = httptest.NewRequest("GET", "/api/transactions?limit=3", nil)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test transactions with limit: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	transactions = response["transactions"].([]interface{})
	if len(transactions) != 3 {
		t.Errorf("Expected 3 transactions with limit, got %d", len(transactions))
	}
}

func TestHandleGetStatistics(t *testing.T) {
	Initialize(":memory:")

	// Log some test data
	LogTransaction("tx1", "success", "openai", "", 200, []ner.RedactionDetail{
		{EntityType: "email", OriginalText: "test@example.com", RedactedText: "{{EMAIL_A}}"},
		{EntityType: "phone", OriginalText: "555-123-4567", RedactedText: "{{PHONE_B}}"},
	}, 1, 1)
	LogTransaction("blocked1", "blocked", "anthropic", "", 403, nil, 1, 1)

	app := fiber.New()
	// Add middleware to simulate authentication context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("organization_id", "1")
		return c.Next()
	})
	app.Get("/api/statistics", HandleGetStatistics)

	req := httptest.NewRequest("GET", "/api/statistics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test statistics endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var stats map[string]interface{}
	err = json.Unmarshal(body, &stats)
	if err != nil {
		t.Fatalf("Failed to parse statistics: %v", err)
	}

	// Verify required fields exist
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
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field '%s' in statistics", field)
		}
	}

	// Verify values
	if stats["total_requests"].(float64) != 2 {
		t.Errorf("Expected 2 total requests, got %v", stats["total_requests"])
	}
	if stats["blocked_requests"].(float64) != 1 {
		t.Errorf("Expected 1 blocked request, got %v", stats["blocked_requests"])
	}
	if stats["redacted_requests"].(float64) != 1 {
		t.Errorf("Expected 1 redacted request, got %v", stats["redacted_requests"])
	}
}

func TestHandleSettingsEndpoints(t *testing.T) {
	Initialize(":memory:")

	app := fiber.New()
	app.Get("/api/settings", HandleGetSettings)
	app.Put("/api/settings", HandleUpdateSettings)

	// Test GET settings
	req := httptest.NewRequest("GET", "/api/settings", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test get settings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 for get settings, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read settings response: %v", err)
	}

	var settings map[string]interface{}
	err = json.Unmarshal(body, &settings)
	if err != nil {
		t.Fatalf("Failed to parse settings: %v", err)
	}

	// Verify required setting fields
	requiredFields := []string{
		"strict_mode",
		"allowed_providers",
		"blocked_entities",
		"retention_days",
		"log_level",
	}

	for _, field := range requiredFields {
		if _, exists := settings[field]; !exists {
			t.Errorf("Expected field '%s' in settings", field)
		}
	}

	// Test PUT settings
	newSettings := map[string]interface{}{
		"strict_mode":       true,
		"allowed_providers": []string{"openai", "anthropic"},
		"blocked_entities":  []string{"ssn", "credit_card"},
		"retention_days":    7,
		"log_level":         "debug",
	}

	settingsJSON, _ := json.Marshal(newSettings)
	req = httptest.NewRequest("PUT", "/api/settings", strings.NewReader(string(settingsJSON)))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatalf("Failed to test update settings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200 for update settings, got %d", resp.StatusCode)
	}
}

func TestTransactionPagination(t *testing.T) {
	Initialize(":memory:")

	// Log 10 transactions
	for i := 0; i < 10; i++ {
		LogTransaction(fmt.Sprintf("page-tx-%d", i), "success", "openai", "", 200, nil, 1, 1)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// Test pagination
	page1, err := GetTransactions(5, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get page 1: %v", err)
	}
	if len(page1) != 5 {
		t.Errorf("Expected 5 transactions in page 1, got %d", len(page1))
	}

	page2, err := GetTransactions(5, 5, "1")
	if err != nil {
		t.Fatalf("Failed to get page 2: %v", err)
	}
	if len(page2) != 5 {
		t.Errorf("Expected 5 transactions in page 2, got %d", len(page2))
	}

	// Verify pages have different transactions
	page1IDs := make(map[string]bool)
	for _, tx := range page1 {
		page1IDs[tx.ID] = true
	}

	for _, tx := range page2 {
		if page1IDs[tx.ID] {
			t.Errorf("Transaction %s appears in both pages", tx.ID)
		}
	}
}

func TestLogTransactionVendorName(t *testing.T) {
	Initialize(":memory:")

	LogTransaction("vendor-tx-1", "success", "openai", "sentry", 200, nil, 1, 1)
	LogTransaction("vendor-tx-2", "success", "anthropic", "", 200, nil, 1, 1)

	transactions, err := GetTransactions(10, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(transactions))
	}

	// Results are newest-first; vendor-tx-2 is last inserted so index 0.
	for _, tx := range transactions {
		switch tx.ID {
		case "vendor-tx-1":
			if tx.VendorName != "sentry" {
				t.Errorf("Expected vendor_name sentry, got %q", tx.VendorName)
			}
		case "vendor-tx-2":
			if tx.VendorName != "" {
				t.Errorf("Expected empty vendor_name, got %q", tx.VendorName)
			}
		}
	}
}

func TestRedactionDetailsStorage(t *testing.T) {
	Initialize(":memory:")

	redactionDetails := []ner.RedactionDetail{
		{
			EntityType:   "email",
			OriginalText: "john@example.com",
			RedactedText: "{{EMAIL_12345}}",
			Confidence:   0.95,
		},
		{
			EntityType:   "phone",
			OriginalText: "555-123-4567",
			RedactedText: "{{PHONE_67890}}",
			Confidence:   0.88,
		},
	}

	LogTransaction("redaction-test", "success", "openai", "", 200, redactionDetails, 1, 1)

	transactions, err := GetTransactions(1, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(transactions) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(transactions))
	}

	stored := transactions[0]
	if len(stored.RedactionDetails) != 2 {
		t.Errorf("Expected 2 redaction details, got %d", len(stored.RedactionDetails))
	}

	// Check details are preserved
	emailFound := false
	phoneFound := false

	for _, detail := range stored.RedactionDetails {
		if detail.EntityType == "email" && detail.OriginalText == "john@example.com" {
			emailFound = true
			if detail.Confidence != 0.95 {
				t.Errorf("Expected email confidence 0.95, got %f", detail.Confidence)
			}
		}
		if detail.EntityType == "phone" && detail.OriginalText == "555-123-4567" {
			phoneFound = true
			if detail.Confidence != 0.88 {
				t.Errorf("Expected phone confidence 0.88, got %f", detail.Confidence)
			}
		}
	}

	if !emailFound {
		t.Errorf("Email redaction detail not found")
	}
	if !phoneFound {
		t.Errorf("Phone redaction detail not found")
	}
}
