package scanner

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupFindingsApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	app := fiber.New()
	app.Use(mockAuthMiddleware(1))
	app.Get("/api/v1/findings", HandleListFindings)
	app.Get("/api/v1/findings/:id", HandleGetFinding)
	app.Patch("/api/v1/findings/:id", HandlePatchFinding)
	return app
}

func TestHandleListFindings_Empty(t *testing.T) {
	app := setupFindingsApp(t)

	req := httptest.NewRequest("GET", "/api/v1/findings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["total"].(float64) != 0 {
		t.Errorf("expected 0 findings, got %v", result["total"])
	}
}

func TestHandleGetFinding_NotFound(t *testing.T) {
	app := setupFindingsApp(t)

	req := httptest.NewRequest("GET", "/api/v1/findings/nonexistent", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGetFinding_Found(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	f := seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	req := httptest.NewRequest("GET", "/api/v1/findings/"+f.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] != f.ID {
		t.Errorf("expected finding ID %s, got %v", f.ID, result["id"])
	}
	if result["data_type"] != "email" {
		t.Errorf("expected data_type email, got %v", result["data_type"])
	}
}

func TestHandleListFindings_VendorFilter(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc1 := seedVendorConnection(t, 1, "sentry", "connected")
	vc2 := seedVendorConnection(t, 1, "datadog", "connected")
	seedFinding(t, 1, scan.ID, vc1.ID, "sentry", "email", "high", "open")
	seedFinding(t, 1, scan.ID, vc2.ID, "datadog", "phone", "medium", "open")

	req := httptest.NewRequest("GET", "/api/v1/findings?vendor=sentry", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	findings := result["findings"].([]interface{})
	if len(findings) != 1 {
		t.Errorf("expected 1 sentry finding, got %d", len(findings))
	}
}

func TestHandleListFindings_RiskFilter(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "ip_address", "medium", "open")

	req := httptest.NewRequest("GET", "/api/v1/findings?risk_level=high", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	findings := result["findings"].([]interface{})
	if len(findings) != 1 {
		t.Errorf("expected 1 high-risk finding, got %d", len(findings))
	}
}

func TestHandleListFindings_StatusFilter(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "resolved")

	req := httptest.NewRequest("GET", "/api/v1/findings?status=open", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	findings := result["findings"].([]interface{})
	if len(findings) != 1 {
		t.Errorf("expected 1 open finding, got %d", len(findings))
	}
}

func TestHandlePatchFinding_Resolve(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	f := seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	payload := map[string]string{"status": "resolved"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PATCH", "/api/v1/findings/"+f.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "resolved" {
		t.Errorf("expected status resolved, got %v", result["status"])
	}
}

func TestHandlePatchFinding_Suppress(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	f := seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	payload := map[string]string{"status": "suppressed"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PATCH", "/api/v1/findings/"+f.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandlePatchFinding_InvalidStatus(t *testing.T) {
	app := setupFindingsApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	f := seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	payload := map[string]string{"status": "invalid_status"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("PATCH", "/api/v1/findings/"+f.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid status, got %d", resp.StatusCode)
	}
}

// ── DB layer tests ────────────────────────────────────────────────────────────

func TestDeduplicateFinding(t *testing.T) {
	setupTestDB(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	// Same signature → should deduplicate (return existing ID).
	existingID, err := deduplicateFinding(1, "sentry", "email", "event.user.email", "POST /login")
	if err != nil {
		t.Fatalf("deduplicateFinding: %v", err)
	}
	if existingID == "" {
		t.Error("expected existing finding ID for duplicate, got empty string")
	}
}

func TestDeduplicateFinding_DifferentLocation(t *testing.T) {
	setupTestDB(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	// Different location → not a duplicate.
	existingID, err := deduplicateFinding(1, "sentry", "email", "different.path", "POST /login")
	if err != nil {
		t.Fatalf("deduplicateFinding: %v", err)
	}
	if existingID != "" {
		t.Error("expected no existing finding for different location")
	}
}

func TestOrgIsolation_Findings(t *testing.T) {
	setupTestDB(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	// Org 2 should see 0 findings.
	findings, _ := listFindings(FindingFilter{OrgID: 2, Limit: 10, Offset: 0})
	if len(findings) != 0 {
		t.Errorf("org isolation failure: org 2 can see org 1 findings")
	}
}

func TestFindingComplianceImpact(t *testing.T) {
	setupTestDB(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	f := seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	got, err := getFinding(1, f.ID)
	if err != nil {
		t.Fatalf("getFinding: %v", err)
	}
	if len(got.ComplianceImpact) == 0 {
		t.Error("expected compliance impact to be persisted and retrieved")
	}
	hasGDPR := false
	for _, ci := range got.ComplianceImpact {
		if ci.Framework == "GDPR" {
			hasGDPR = true
		}
	}
	if !hasGDPR {
		t.Error("expected GDPR in compliance impact for email finding")
	}
}
