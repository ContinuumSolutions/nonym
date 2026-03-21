package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupVendorTestApp creates a Fiber app with all vendor endpoints wired up.
func setupVendorTestApp(t *testing.T) *fiber.App {
	t.Helper()
	testDB := setupTestDB(t)

	// Create vendor_integrations table in the test DB.
	db = testDB
	if err := InitializeVendorTables(); err != nil {
		t.Fatalf("InitializeVendorTables: %v", err)
	}

	app := fiber.New()
	app.Use(mockAuthMiddleware(1, 1))
	app.Get("/api/v1/vendors", HandleListVendors)
	app.Get("/api/v1/vendors/configured", HandleGetConfiguredVendors)
	app.Post("/api/v1/vendors/setup", HandleSetupVendor)
	app.Get("/api/v1/vendors/:id", HandleGetVendor)
	app.Delete("/api/v1/vendors/configured/:vendor_id", HandleRemoveVendor)
	return app
}

func TestHandleListVendors(t *testing.T) {
	app := setupVendorTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/vendors", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	vendors, ok := result["vendors"].([]interface{})
	if !ok {
		t.Fatal("response missing vendors array")
	}
	if len(vendors) == 0 {
		t.Fatal("expected at least one vendor")
	}

	total, ok := result["total"].(float64)
	if !ok || int(total) != len(vendors) {
		t.Errorf("total field mismatch: %v vs %d", total, len(vendors))
	}

	// Each vendor should have required fields.
	first := vendors[0].(map[string]interface{})
	for _, field := range []string{"id", "name", "category", "known_hosts", "data_types"} {
		if _, exists := first[field]; !exists {
			t.Errorf("vendor missing field %q", field)
		}
	}
}

func TestHandleGetVendor_Found(t *testing.T) {
	app := setupVendorTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/vendors/sentry", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var vendor map[string]interface{}
	json.Unmarshal(body, &vendor)

	if vendor["id"] != "sentry" {
		t.Errorf("expected vendor id sentry, got %v", vendor["id"])
	}
	if vendor["name"] != "Sentry" {
		t.Errorf("expected vendor name Sentry, got %v", vendor["name"])
	}
	if vendor["nonym_sdk"] != "@nonym/sentry" {
		t.Errorf("expected nonym_sdk @nonym/sentry, got %v", vendor["nonym_sdk"])
	}
}

func TestHandleGetVendor_NotFound(t *testing.T) {
	app := setupVendorTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/vendors/nonexistent-xyz", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleSetupVendor_ValidSentry(t *testing.T) {
	app := setupVendorTestApp(t)

	payload := map[string]string{"vendor_id": "sentry", "method": "sdk_wrapper"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, respBody)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	integration, ok := result["integration"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing integration object")
	}
	if integration["vendor_id"] != "sentry" {
		t.Errorf("expected vendor_id sentry, got %v", integration["vendor_id"])
	}
	if integration["method"] != "sdk_wrapper" {
		t.Errorf("expected method sdk_wrapper, got %v", integration["method"])
	}
	if integration["status"] != "active" {
		t.Errorf("expected status active, got %v", integration["status"])
	}

	if _, hasInstructions := result["setup_instructions"]; !hasInstructions {
		t.Error("response missing setup_instructions")
	}
}

func TestHandleSetupVendor_DefaultMethod(t *testing.T) {
	app := setupVendorTestApp(t)

	// Method omitted — should default to "proxy".
	payload := map[string]string{"vendor_id": "datadog"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	integration := result["integration"].(map[string]interface{})
	if integration["method"] != "proxy" {
		t.Errorf("expected default method proxy, got %v", integration["method"])
	}
}

func TestHandleSetupVendor_UnknownVendor(t *testing.T) {
	app := setupVendorTestApp(t)

	payload := map[string]string{"vendor_id": "unknown-xyz"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for unknown vendor, got %d", resp.StatusCode)
	}
}

func TestHandleSetupVendor_MissingVendorID(t *testing.T) {
	app := setupVendorTestApp(t)

	payload := map[string]string{"method": "proxy"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing vendor_id, got %d", resp.StatusCode)
	}
}

func TestHandleSetupVendor_InvalidMethod(t *testing.T) {
	app := setupVendorTestApp(t)

	payload := map[string]string{"vendor_id": "sentry", "method": "invalid_method"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid method, got %d", resp.StatusCode)
	}
}

func TestHandleGetConfiguredVendors_Empty(t *testing.T) {
	app := setupVendorTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/vendors/configured", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	integrations, ok := result["integrations"].([]interface{})
	if !ok {
		t.Fatal("response missing integrations array")
	}
	if len(integrations) != 0 {
		t.Errorf("expected 0 integrations, got %d", len(integrations))
	}
}

func TestHandleSetupThenList(t *testing.T) {
	app := setupVendorTestApp(t)

	// Setup sentry.
	payload, _ := json.Marshal(map[string]string{"vendor_id": "sentry"})
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 201 {
		t.Fatalf("setup failed with %d", resp.StatusCode)
	}

	// Setup posthog.
	payload, _ = json.Marshal(map[string]string{"vendor_id": "posthog"})
	req = httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	if resp.StatusCode != 201 {
		t.Fatalf("setup failed with %d", resp.StatusCode)
	}

	// List — should have 2.
	req = httptest.NewRequest("GET", "/api/v1/vendors/configured", nil)
	resp, _ = app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	integrations := result["integrations"].([]interface{})
	if len(integrations) != 2 {
		t.Errorf("expected 2 integrations, got %d", len(integrations))
	}
}

func TestHandleRemoveVendor(t *testing.T) {
	app := setupVendorTestApp(t)

	// Setup then remove.
	payload, _ := json.Marshal(map[string]string{"vendor_id": "sentry"})
	req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	app.Test(req, -1)

	req = httptest.NewRequest("DELETE", "/api/v1/vendors/configured/sentry", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 on delete, got %d", resp.StatusCode)
	}

	// List — should be empty again.
	req = httptest.NewRequest("GET", "/api/v1/vendors/configured", nil)
	resp, _ = app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	integrations := result["integrations"].([]interface{})
	if len(integrations) != 0 {
		t.Errorf("expected 0 integrations after delete, got %d", len(integrations))
	}
}

func TestHandleSetupVendor_Idempotent(t *testing.T) {
	app := setupVendorTestApp(t)

	setup := func(method string) int {
		payload, _ := json.Marshal(map[string]string{"vendor_id": "sentry", "method": method})
		req := httptest.NewRequest("POST", "/api/v1/vendors/setup", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		return resp.StatusCode
	}

	if code := setup("proxy"); code != 201 {
		t.Fatalf("first setup failed: %d", code)
	}
	// Second call should update (upsert), not return an error.
	if code := setup("sdk_wrapper"); code != 201 {
		t.Fatalf("second setup (upsert) failed: %d", code)
	}

	// Should still only have one integration.
	req := httptest.NewRequest("GET", "/api/v1/vendors/configured", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	integrations := result["integrations"].([]interface{})
	if len(integrations) != 1 {
		t.Errorf("expected 1 integration after upsert, got %d", len(integrations))
	}
}
