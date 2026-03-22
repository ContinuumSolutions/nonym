package scanner

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupOverviewApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	app := fiber.New()
	app.Use(mockAuthMiddleware(1))
	app.Get("/api/v1/scanner/overview", HandleScannerOverview)
	app.Get("/api/v1/scanner/flows", HandleScannerFlows)
	return app
}

func TestHandleScannerOverview_Empty(t *testing.T) {
	app := setupOverviewApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/overview", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["vendors_connected"].(float64) != 0 {
		t.Errorf("expected 0 vendors connected, got %v", result["vendors_connected"])
	}
	if result["risk_score"].(float64) != 100 {
		t.Errorf("expected risk_score 100 with no findings, got %v", result["risk_score"])
	}

	findings := result["findings"].(map[string]interface{})
	if findings["total"].(float64) != 0 {
		t.Errorf("expected 0 total findings, got %v", findings["total"])
	}

	compliance := result["compliance"].(map[string]interface{})
	for _, fw := range []string{"GDPR", "SOC2", "HIPAA"} {
		fwStatus := compliance[fw].(map[string]interface{})
		if fwStatus["status"] != "ok" {
			t.Errorf("expected %s status ok with no findings, got %v", fw, fwStatus["status"])
		}
	}
}

func TestHandleScannerOverview_WithFindings(t *testing.T) {
	app := setupOverviewApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "ip_address", "medium", "open")

	req := httptest.NewRequest("GET", "/api/v1/scanner/overview", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["vendors_connected"].(float64) != 1 {
		t.Errorf("expected 1 vendor connected, got %v", result["vendors_connected"])
	}

	findings := result["findings"].(map[string]interface{})
	if findings["total"].(float64) != 2 {
		t.Errorf("expected 2 total findings, got %v", findings["total"])
	}
	if findings["high"].(float64) != 1 {
		t.Errorf("expected 1 high finding, got %v", findings["high"])
	}

	riskScore := result["risk_score"].(float64)
	if riskScore >= 100 {
		t.Errorf("risk score should decrease with findings, got %v", riskScore)
	}
}

func TestHandleScannerOverview_ComplianceWarning(t *testing.T) {
	app := setupOverviewApp(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	// Add multiple email findings — each has GDPR impact.
	for i := 0; i < 3; i++ {
		seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")
	}

	req := httptest.NewRequest("GET", "/api/v1/scanner/overview", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	compliance := result["compliance"].(map[string]interface{})
	gdpr := compliance["GDPR"].(map[string]interface{})
	if gdpr["violations"].(float64) == 0 {
		t.Error("expected GDPR violations with email findings")
	}
	if gdpr["status"] == "ok" {
		t.Error("GDPR status should not be ok with email findings")
	}
}

func TestHandleScannerFlows_Empty(t *testing.T) {
	app := setupOverviewApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/flows", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	nodes := result["nodes"].([]interface{})
	// Should always have at least the "app" node.
	if len(nodes) < 1 {
		t.Error("expected at least one node (app)")
	}
	appNode := nodes[0].(map[string]interface{})
	if appNode["type"] != "app" {
		t.Errorf("expected first node to be app, got %v", appNode["type"])
	}
}

func TestHandleScannerFlows_WithVendors(t *testing.T) {
	app := setupOverviewApp(t)
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	scan := seedScan(t, 1, "done")
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "email", "high", "open")

	req := httptest.NewRequest("GET", "/api/v1/scanner/flows", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	nodes := result["nodes"].([]interface{})
	// app node + sentry node.
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes (app + sentry), got %d", len(nodes))
	}

	edges := result["edges"].([]interface{})
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}

	edge := edges[0].(map[string]interface{})
	if edge["from"] != "app" {
		t.Errorf("expected edge from 'app', got %v", edge["from"])
	}
	if edge["to"] != vc.ID {
		t.Errorf("expected edge to vendor connection ID, got %v", edge["to"])
	}
}

// ── buildComplianceSnapshot ───────────────────────────────────────────────────

func TestBuildComplianceSnapshot_NoFindings(t *testing.T) {
	setupTestDB(t)
	snap := buildComplianceSnapshot(1)
	if snap.GDPR.Status != "ok" {
		t.Errorf("expected GDPR ok, got %s", snap.GDPR.Status)
	}
	if snap.HIPAA.Status != "ok" {
		t.Errorf("expected HIPAA ok, got %s", snap.HIPAA.Status)
	}
	if snap.SOC2.Status != "ok" {
		t.Errorf("expected SOC2 ok, got %s", snap.SOC2.Status)
	}
}

func TestBuildComplianceSnapshot_HealthFinding(t *testing.T) {
	setupTestDB(t)
	scan := seedScan(t, 1, "done")
	vc := seedVendorConnection(t, 1, "sentry", "connected")
	// Health data → HIPAA impact.
	seedFinding(t, 1, scan.ID, vc.ID, "sentry", "health", "high", "open")

	snap := buildComplianceSnapshot(1)
	if snap.HIPAA.Violations == 0 {
		t.Error("expected HIPAA violations for health finding")
	}
}
