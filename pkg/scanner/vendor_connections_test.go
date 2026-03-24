package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

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
	app.Post("/api/v1/vendor-connections/test", HandleTestCredentials)
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

// TestHandleTestVendorConnection_InvalidFormat verifies that a format-invalid token
// updates the connection status to "error" and returns success=false.
func TestHandleTestVendorConnection_InvalidFormat(t *testing.T) {
	app := setupVendorConnectionApp(t)
	// Sentry token shorter than 8 chars → format failure, no real API call.
	now := time.Now()
	vc := VendorConnection{
		ID: newID(), OrgID: 1, Vendor: "sentry", DisplayName: "sentry",
		Status: "disconnected", AuthType: "api_key",
		Credentials: map[string]interface{}{"token": "short"},
		Settings:    map[string]interface{}{},
		CreatedAt:   now, UpdatedAt: now,
	}
	if err := insertVendorConnection(&vc); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/"+vc.ID+"/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] == true {
		t.Error("expected success=false for short sentry token")
	}
	conn, ok := result["connection"].(map[string]interface{})
	if !ok {
		t.Fatal("expected connection object in response")
	}
	if conn["status"] != "error" {
		t.Errorf("expected connection status 'error', got %v", conn["status"])
	}
}

// TestHandleTestVendorConnection_FormatPass verifies that a format-valid credential
// (Stripe short key, no real API call) updates the connection to "connected".
func TestHandleTestVendorConnection_FormatPass(t *testing.T) {
	app := setupVendorConnectionApp(t)
	// Stripe key starts with sk_, but < 20 chars → format-only pass, no real network call.
	now := time.Now()
	vc := VendorConnection{
		ID: newID(), OrgID: 1, Vendor: "stripe", DisplayName: "stripe",
		Status: "disconnected", AuthType: "api_key",
		Credentials: map[string]interface{}{"api_key": "sk_test_abc"},
		Settings:    map[string]interface{}{},
		CreatedAt:   now, UpdatedAt: now,
	}
	if err := insertVendorConnection(&vc); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/"+vc.ID+"/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("expected success=true for format-valid stripe key, got: %v", result["message"])
	}
	conn, ok := result["connection"].(map[string]interface{})
	if !ok {
		t.Fatal("expected connection object in response")
	}
	if conn["status"] != "connected" {
		t.Errorf("expected status 'connected', got %v", conn["status"])
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

// ── testConnection delegation ─────────────────────────────────────────────────

// TestTestConnection_DelegatesToConnector verifies that testConnection delegates
// to the registered connector rather than reimplementing validation.
func TestTestConnection_DelegatesToConnector(t *testing.T) {
	// Stripe short key: format-only pass (< 20 chars), no real API call.
	vc := &VendorConnection{Vendor: "stripe", Credentials: map[string]interface{}{"api_key": "sk_test_abc"}}
	result := testConnection(vc)
	if !result.Success {
		t.Errorf("testConnection should delegate to stripe connector and return success: %s", result.Message)
	}
}

func TestTestConnection_UnknownVendor_NoCreds(t *testing.T) {
	vc := &VendorConnection{Vendor: "nonexistent", Credentials: map[string]interface{}{}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for unknown vendor with no credentials")
	}
}

func TestTestConnection_UnknownVendor_WithCreds(t *testing.T) {
	vc := &VendorConnection{Vendor: "nonexistent", Credentials: map[string]interface{}{"api_key": "somekey"}}
	result := testConnection(vc)
	if !result.Success {
		t.Errorf("expected success for unknown vendor with credentials: %s", result.Message)
	}
}

// ── HandleTestCredentials ─────────────────────────────────────────────────────

func TestHandleTestCredentials_FormatValid(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"vendor":      "stripe",
		"credentials": map[string]interface{}{"api_key": "sk_test_abc"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("expected success=true for format-valid stripe key: %v", result["message"])
	}
}

func TestHandleTestCredentials_FormatInvalid(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"vendor":      "sentry",
		"credentials": map[string]interface{}{"token": "short"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] == true {
		t.Error("expected success=false for short sentry token")
	}
}

func TestHandleTestCredentials_MissingVendor(t *testing.T) {
	app := setupVendorConnectionApp(t)

	payload := map[string]interface{}{
		"credentials": map[string]interface{}{"api_key": "sk_test_abc"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing vendor, got %d", resp.StatusCode)
	}
}

func TestHandleTestCredentials_UpdatesExistingConnection(t *testing.T) {
	app := setupVendorConnectionApp(t)
	// Seed a stripe connection first.
	vc := seedVendorConnection(t, 1, "stripe", "disconnected")
	_ = vc

	// Test with a format-valid stripe key — should update the existing connection.
	payload := map[string]interface{}{
		"vendor":      "stripe",
		"credentials": map[string]interface{}{"api_key": "sk_test_abc"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/vendor-connections/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["success"] != true {
		t.Errorf("expected success=true: %v", result["message"])
	}
	// The existing connection should be returned with updated status.
	conn, ok := result["connection"].(map[string]interface{})
	if !ok {
		t.Fatal("expected connection in response when existing connection exists")
	}
	if conn["status"] != "connected" {
		t.Errorf("expected existing connection updated to 'connected', got %v", conn["status"])
	}
}

// ── Connector format validation ───────────────────────────────────────────────

func TestTestConnection_Rollbar_ShortToken(t *testing.T) {
	vc := &VendorConnection{Vendor: "rollbar", Credentials: map[string]interface{}{"access_token": "short"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for short rollbar token")
	}
}

func TestTestConnection_Bugsnag_ShortToken(t *testing.T) {
	vc := &VendorConnection{Vendor: "bugsnag", Credentials: map[string]interface{}{"auth_token": "short"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for short bugsnag token")
	}
}

func TestTestConnection_Zendesk_MissingFields(t *testing.T) {
	vc := &VendorConnection{Vendor: "zendesk", Credentials: map[string]interface{}{"subdomain": "myco"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for zendesk missing email and api_token")
	}
}

func TestTestConnection_SendGrid_BadPrefix(t *testing.T) {
	vc := &VendorConnection{Vendor: "sendgrid", Credentials: map[string]interface{}{"api_key": "notsg.key"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for sendgrid key without SG. prefix")
	}
}

func TestTestConnection_Twilio_BadSID(t *testing.T) {
	vc := &VendorConnection{Vendor: "twilio", Credentials: map[string]interface{}{
		"account_sid": "BADsid", "auth_token": "validtokenhere123",
	}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for twilio account_sid not starting with AC")
	}
}

func TestTestConnection_PagerDuty_ShortKey(t *testing.T) {
	vc := &VendorConnection{Vendor: "pagerduty", Credentials: map[string]interface{}{"api_key": "short"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for short pagerduty key")
	}
}

func TestTestConnection_PostHog_MissingProject(t *testing.T) {
	vc := &VendorConnection{Vendor: "posthog", Credentials: map[string]interface{}{"api_key": "phc_validkey"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for posthog missing project_id")
	}
}

func TestTestConnection_Mixpanel_MissingCreds(t *testing.T) {
	vc := &VendorConnection{Vendor: "mixpanel", Credentials: map[string]interface{}{"service_account": "user"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for mixpanel missing secret and project_id")
	}
}

func TestTestConnection_NewRelic_BadKey(t *testing.T) {
	vc := &VendorConnection{Vendor: "newrelic", Credentials: map[string]interface{}{
		"api_key": "notNRAK-key", "account_id": "12345",
	}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for newrelic key without NRAK- prefix")
	}
}

func TestTestConnection_Algolia_MissingAppID(t *testing.T) {
	vc := &VendorConnection{Vendor: "algolia", Credentials: map[string]interface{}{"api_key": "validkeyhere"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for algolia missing app_id")
	}
}

func TestTestConnection_Algolia_Valid(t *testing.T) {
	vc := &VendorConnection{Vendor: "algolia", Credentials: map[string]interface{}{
		"app_id": "MYAPPID", "api_key": "validkeyhere",
	}}
	result := testConnection(vc)
	if !result.Success {
		t.Errorf("expected success for algolia with app_id and api_key: %s", result.Message)
	}
}

func TestTestConnection_Intercom_ShortToken(t *testing.T) {
	vc := &VendorConnection{Vendor: "intercom", Credentials: map[string]interface{}{"access_token": "short"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for short intercom token")
	}
}

func TestTestConnection_HubSpot_ShortToken(t *testing.T) {
	vc := &VendorConnection{Vendor: "hubspot", Credentials: map[string]interface{}{"access_token": "short"}}
	result := testConnection(vc)
	if result.Success {
		t.Error("expected failure for short hubspot token")
	}
}
