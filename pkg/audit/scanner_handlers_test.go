package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ─── Unit tests for internal helpers ─────────────────────────────────────────

func TestMatchesFingerprint(t *testing.T) {
	cases := []struct {
		key      string
		patterns []string
		want     bool
	}{
		{"SENTRY_DSN", []string{"SENTRY_"}, true},
		{"sentry_dsn", []string{"SENTRY_"}, true},       // case-insensitive
		{"DD_API_KEY", []string{"DD_"}, true},
		{"sentryDsn", []string{"sentryDsn"}, true},      // exact key match
		{"sentryDSN", []string{"sentryDsn"}, true},      // case-insensitive exact
		{"POSTHOG_KEY", []string{"POSTHOG_"}, true},
		{"REACT_APP_FOO", []string{"SENTRY_"}, false},
		{"", []string{"SENTRY_"}, false},
	}

	for _, tc := range cases {
		got := matchesFingerprint(tc.key, tc.patterns)
		if got != tc.want {
			t.Errorf("matchesFingerprint(%q, %v) = %v, want %v", tc.key, tc.patterns, got, tc.want)
		}
	}
}

func TestMaskValue(t *testing.T) {
	cases := []struct {
		input    string
		wantFull bool // true if we just check the length is same
	}{
		{"short", false},          // len ≤ 8 → all stars
		{"12345678", false},       // len == 8 → all stars
		{"1234567890abcdef", false}, // len > 8 → partial masking
	}

	for _, tc := range cases {
		got := maskValue(tc.input)
		if len(got) != len(tc.input) {
			t.Errorf("maskValue(%q) changed length: got %d, want %d", tc.input, len(got), len(tc.input))
		}
		if got == tc.input {
			t.Errorf("maskValue(%q) did not mask anything", tc.input)
		}
	}
}

func TestMaskDSN(t *testing.T) {
	dsn := "https://abc123def456@o123456.ingest.sentry.io/789"
	masked := maskDSN(dsn)

	if masked == dsn {
		t.Error("maskDSN should change the input")
	}
	// Host portion after @ should be preserved.
	if len(masked) != len(dsn) {
		t.Errorf("maskDSN changed length: got %d, want %d", len(masked), len(dsn))
	}
}

func TestScanForVendors_SentryDetected(t *testing.T) {
	kv := map[string]string{
		"SENTRY_DSN": "https://abc@o123.ingest.sentry.io/456",
	}
	findings := scanForVendors(kv)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for Sentry")
	}

	var sentryFinding *ScanFinding
	for i := range findings {
		if findings[i].VendorID == "sentry" {
			sentryFinding = &findings[i]
			break
		}
	}
	if sentryFinding == nil {
		t.Fatal("sentry finding not present")
	}
	if sentryFinding.Risk != "high" {
		t.Errorf("expected high risk, got %q", sentryFinding.Risk)
	}
	if sentryFinding.Protected {
		t.Error("should not be marked as protected (no 'nonym' in value)")
	}
}

func TestScanForVendors_MultipleVendors(t *testing.T) {
	kv := map[string]string{
		"SENTRY_DSN":  "https://key@sentry.io/1",
		"DD_API_KEY":  "abc123",
		"POSTHOG_KEY": "phc_abc",
	}
	findings := scanForVendors(kv)

	found := make(map[string]bool)
	for _, f := range findings {
		found[f.VendorID] = true
	}

	for _, expected := range []string{"sentry", "datadog", "posthog"} {
		if !found[expected] {
			t.Errorf("expected finding for %s", expected)
		}
	}
}

func TestScanForVendors_NonymProtected(t *testing.T) {
	// A value containing "nonym" should be marked as protected.
	kv := map[string]string{
		"SENTRY_DSN": "https://key@nonym.example.com/sentry/456",
	}
	findings := scanForVendors(kv)

	for _, f := range findings {
		if f.VendorID == "sentry" && !f.Protected {
			t.Error("sentry DSN pointing to nonym should be marked protected")
		}
	}
}

func TestScanForVendors_NoVendors(t *testing.T) {
	kv := map[string]string{
		"DATABASE_URL":   "postgres://user:pass@localhost/db",
		"NODE_ENV":       "production",
		"REACT_APP_NAME": "MyApp",
	}
	findings := scanForVendors(kv)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// ─── HTTP handler tests ───────────────────────────────────────────────────────

func setupScannerTestApp(t *testing.T) *fiber.App {
	t.Helper()
	testDB := setupTestDB(t)
	db = testDB

	app := fiber.New()
	app.Use(mockAuthMiddleware(1, 1))
	app.Post("/api/v1/scanner", HandleScanVendors)
	app.Get("/api/v1/scanner/sentry", HandleScanSentry)
	return app
}

func TestHandleScanVendors_EmptyPayload(t *testing.T) {
	app := setupScannerTestApp(t)

	req := httptest.NewRequest("POST", "/api/v1/scanner", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result ScanResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for empty payload, got %d", len(result.Findings))
	}
}

func TestHandleScanVendors_WithSentry(t *testing.T) {
	app := setupScannerTestApp(t)

	payload := map[string]interface{}{
		"env": map[string]string{
			"SENTRY_DSN": "https://key@o123.ingest.sentry.io/456",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/scanner", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result ScanResult
	json.Unmarshal(respBody, &result)

	if len(result.Findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	if result.Unprotected == 0 {
		t.Error("expected at least one unprotected finding")
	}
}

func TestHandleScanVendors_ResponseShape(t *testing.T) {
	app := setupScannerTestApp(t)

	req := httptest.NewRequest("POST", "/api/v1/scanner", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	for _, field := range []string{"scanned_at", "total_vendors", "findings", "protected", "unprotected", "instructions"} {
		if _, ok := result[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

func TestHandleScanSentry_NoQueryParam(t *testing.T) {
	app := setupScannerTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/sentry", nil)
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

	if _, ok := result["vendor"]; !ok {
		t.Error("response missing vendor field")
	}
	if _, ok := result["setup_guide"]; !ok {
		t.Error("response missing setup_guide field")
	}
	// No DSN provided — finding should be nil/absent.
	if finding, ok := result["finding"]; ok && finding != nil {
		t.Errorf("expected nil finding when no DSN provided, got %v", finding)
	}
}

func TestHandleScanSentry_WithValidDSN(t *testing.T) {
	app := setupScannerTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/sentry?dsn=https://abc123@o123456.ingest.sentry.io/789", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	finding, ok := result["finding"].(map[string]interface{})
	if !ok || finding == nil {
		t.Fatal("expected non-nil finding for a valid Sentry DSN")
	}
	if finding["vendor_id"] != "sentry" {
		t.Errorf("expected vendor_id sentry, got %v", finding["vendor_id"])
	}
	if finding["risk"] != "high" {
		t.Errorf("expected high risk, got %v", finding["risk"])
	}
}

func TestHandleScanSentry_WithInvalidDSN(t *testing.T) {
	app := setupScannerTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/sentry?dsn=not-a-real-dsn", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	// Invalid DSN — finding should be nil.
	if finding, ok := result["finding"]; ok && finding != nil {
		t.Errorf("expected nil finding for invalid DSN, got %v", finding)
	}
}
