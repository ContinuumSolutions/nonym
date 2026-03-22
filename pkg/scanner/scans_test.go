package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupScanApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	app := fiber.New()
	app.Use(mockAuthMiddleware(1))
	app.Get("/api/v1/scans", HandleListScans)
	app.Post("/api/v1/scans", HandleCreateScan)
	app.Get("/api/v1/scans/:id", HandleGetScan)
	app.Get("/api/v1/scans/:id/status", HandleScanStatus)
	return app
}

func TestHandleListScans_Empty(t *testing.T) {
	app := setupScanApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scans", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["total"].(float64) != 0 {
		t.Errorf("expected 0 scans, got %v", result["total"])
	}
}

func TestHandleGetScan_NotFound(t *testing.T) {
	app := setupScanApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scans/nonexistent-id", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGetScan_Found(t *testing.T) {
	app := setupScanApp(t)
	scan := seedScan(t, 1, "done")

	req := httptest.NewRequest("GET", "/api/v1/scans/"+scan.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] != scan.ID {
		t.Errorf("expected scan ID %s, got %v", scan.ID, result["id"])
	}
}

func TestHandleListScans_Pagination(t *testing.T) {
	app := setupScanApp(t)
	for i := 0; i < 5; i++ {
		seedScan(t, 1, "done")
	}

	req := httptest.NewRequest("GET", "/api/v1/scans?limit=3&offset=0", nil)
	resp, _ := app.Test(req, -1)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	scans := result["scans"].([]interface{})
	if len(scans) != 3 {
		t.Errorf("expected 3 scans with limit=3, got %d", len(scans))
	}
}

func TestHandleCreateScan_NoConnectedVendors(t *testing.T) {
	app := setupScanApp(t)

	// No connected vendors seeded — should return 422.
	req := httptest.NewRequest("POST", "/api/v1/scans", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 422 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 422 with no connected vendors, got %d: %s", resp.StatusCode, b)
	}
}

func TestHandleCreateScan_WithConnectedVendor(t *testing.T) {
	app := setupScanApp(t)
	seedVendorConnection(t, 1, "sentry", "connected")

	req := httptest.NewRequest("POST", "/api/v1/scans", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 202 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["scan_id"].(string); !ok {
		t.Error("expected scan_id in response")
	}
}

func TestHandleScanStatus_DoneScan(t *testing.T) {
	app := setupScanApp(t)
	scan := seedScan(t, 1, "done")

	req := httptest.NewRequest("GET", "/api/v1/scans/"+scan.ID+"/status", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty SSE response body")
	}
}

func TestHandleScanStatus_NotFound(t *testing.T) {
	app := setupScanApp(t)

	req := httptest.NewRequest("GET", "/api/v1/scans/nonexistent/status", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ── insertScan / updateScanStatus DB helpers ──────────────────────────────────

func TestInsertAndGetScan(t *testing.T) {
	setupTestDB(t)
	s := seedScan(t, 1, "pending")

	got, err := getScan(1, s.ID)
	if err != nil {
		t.Fatalf("getScan: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("expected ID %s, got %s", s.ID, got.ID)
	}
	if got.Status != "pending" {
		t.Errorf("expected status pending, got %s", got.Status)
	}
}

func TestUpdateScanStatus(t *testing.T) {
	setupTestDB(t)
	s := seedScan(t, 1, "pending")

	if err := updateScanStatus(s.ID, "done", 5, nil, nil, ""); err != nil {
		t.Fatalf("updateScanStatus: %v", err)
	}

	got, _ := getScan(1, s.ID)
	if got.Status != "done" {
		t.Errorf("expected status done, got %s", got.Status)
	}
	if got.FindingsCount != 5 {
		t.Errorf("expected findings_count 5, got %d", got.FindingsCount)
	}
}

func TestListScans_OrgIsolation(t *testing.T) {
	setupTestDB(t)
	seedScan(t, 1, "done")
	seedScan(t, 2, "done")

	scans, _ := listScans(1, 10, 0)
	if len(scans) != 1 {
		t.Errorf("expected 1 scan for org 1, got %d", len(scans))
	}
}
