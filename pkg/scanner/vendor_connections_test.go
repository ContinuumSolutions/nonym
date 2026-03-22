package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupVendorConnectionApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	app := fiber.New()
	app.Use(mockAuthMiddleware(1))
	app.Get("/api/v1/vendor-connections", HandleListVendorConnections)
	app.Post("/api/v1/vendor-connections", HandleCreateVendorConnection)
	app.Delete("/api/v1/vendor-connections/:id", HandleDeleteVendorConnection)
	app.Post("/api/v1/vendor-connections/:id/test", HandleTestVendorConnection)
	return app
}

func TestHandleListVendorConnections_Empty(t *testing.T) {
	app := setupVendorConnectionApp(t)

	req := httptest.NewRequest("GET", "/api/v1/vendor-connections", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if total := body["total"].(float64); total != 0 {
		t.Errorf("expected 0 connections, got %v", total)
	}
}

func TestHandleCreateVendorConnection_Success(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"vendor":    "sentry",
		"auth_type": "api_key",
		"credentials": map[string]interface{}{
			"token": "testsentrytoken12345",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}

	var vc map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&vc)
	if vc["vendor"] != "sentry" {
		t.Errorf("expected vendor sentry, got %v", vc["vendor"])
	}
	// Credentials should be masked.
	creds := vc["credentials"].(map[string]interface{})
	if creds["token"] == "testsentrytoken12345" {
		t.Error("credentials should be masked in response")
	}
}

func TestHandleCreateVendorConnection_MissingVendor(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"auth_type":   "api_key",
		"credentials": map[string]interface{}{"token": "abc"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing vendor, got %d", resp.StatusCode)
	}
}

func TestHandleCreateVendorConnection_MissingCredentials(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"vendor":      "sentry",
		"auth_type":   "api_key",
		"credentials": map[string]interface{}{},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing credentials, got %d", resp.StatusCode)
	}
}

func TestHandleCreateVendorConnection_InvalidAuthType(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"vendor":      "sentry",
		"auth_type":   "bad_type",
		"credentials": map[string]interface{}{"token": "abc"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid auth_type, got %d", resp.StatusCode)
	}
}

func TestHandleCreateThenList(t *testing.T) {
	app := setupVendorConnectionApp(t)

	// Create two connections.
	for _, vendor := range []string{"sentry", "datadog"} {
		var apiKey string
		var payload map[string]interface{}
		if vendor == "datadog" {
			payload = map[string]interface{}{
				"vendor":    vendor,
				"auth_type": "api_key",
				"credentials": map[string]interface{}{
					"api_key": "ddapikey12345",
					"app_key": "ddappkey12345",
				},
			}
		} else {
			_ = apiKey
			payload = map[string]interface{}{
				"vendor":    vendor,
				"auth_type": "api_key",
				"credentials": map[string]interface{}{
					"token": "sentrytoken12345",
				},
			}
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/vendor-connections", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 201 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("failed to create %s: %d %s", vendor, resp.StatusCode, b)
		}
	}

	// List.
	req := httptest.NewRequest("GET", "/api/v1/vendor-connections", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	connections := result["connections"].([]interface{})
	if len(connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(connections))
	}
	if result["total"].(float64) != 2 {
		t.Errorf("expected total 2, got %v", result["total"])
	}
}

func TestHandleDeleteVendorConnection(t *testing.T) {
	app := setupVendorConnectionApp(t)
	vc := seedVendorConnection(t, 1, "sentry", "disconnected")

	req := httptest.NewRequest("DELETE", "/api/v1/vendor-connections/"+vc.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// List should now be empty.
	req = httptest.NewRequest("GET", "/api/v1/vendor-connections", nil)
	resp, _ = app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["total"].(float64) != 0 {
		t.Errorf("expected 0 connections after delete, got %v", result["total"])
	}
}

func TestHandleTestVendorConnection_Sentry_Valid(t *testing.T) {
	app := setupVendorConnectionApp(t)
	vc := seedVendorConnection(t, 1, "sentry", "disconnected")

	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/"+vc.ID+"/test", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("expected success true, got %v", result["success"])
	}
}

func TestHandleTestVendorConnection_NotFound(t *testing.T) {
	app := setupVendorConnectionApp(t)

	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/nonexistent/test", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleListVendorConnections_StatusFilter(t *testing.T) {
	app := setupVendorConnectionApp(t)
	seedVendorConnection(t, 1, "sentry", "connected")
	seedVendorConnection(t, 1, "datadog", "disconnected")

	req := httptest.NewRequest("GET", "/api/v1/vendor-connections?status=connected", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	connections := result["connections"].([]interface{})
	if len(connections) != 1 {
		t.Errorf("expected 1 connected vendor, got %d", len(connections))
	}
}

// ── testConnection helper ─────────────────────────────────────────────────────

func TestTestConnection_Sentry_MissingToken(t *testing.T) {
	vc := &VendorConnection{Vendor: "sentry", Credentials: map[string]interface{}{}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for sentry with no token")
	}
}

func TestTestConnection_Datadog_MissingAppKey(t *testing.T) {
	vc := &VendorConnection{
		Vendor:      "datadog",
		Credentials: map[string]interface{}{"api_key": "abc123"},
	}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for datadog missing app_key")
	}
}

func TestTestConnection_Stripe_BadPrefix(t *testing.T) {
	vc := &VendorConnection{
		Vendor:      "stripe",
		Credentials: map[string]interface{}{"api_key": "notsk_key"},
	}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for stripe key without sk_ prefix")
	}
}

func TestTestConnection_Stripe_ValidKey(t *testing.T) {
	vc := &VendorConnection{
		Vendor:      "stripe",
		Credentials: map[string]interface{}{"api_key": "sk_test_abc123"},
	}
	result := testConnection(vc)
	if !result.Success {
		t.Errorf("expected success for stripe valid key, got: %s", result.Message)
	}
}

func TestMaskCredentials(t *testing.T) {
	vc := &VendorConnection{
		Credentials: map[string]interface{}{
			"api_key": "sk-abcdefghijklmnopqrstuvwxyz",
		},
	}
	maskCredentials(vc)
	if vc.Credentials["api_key"] == "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Error("credentials should be masked after maskCredentials")
	}
}
