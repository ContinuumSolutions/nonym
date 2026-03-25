package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupReportsApp(t *testing.T) *fiber.App {
	t.Helper()
	setupTestDB(t)

	app := fiber.New()
	app.Use(mockAuthMiddleware(1))
	app.Get("/api/v1/reports", HandleListReports)
	app.Post("/api/v1/reports/generate", HandleGenerateReport)
	app.Get("/api/v1/reports/:id", HandleGetReport)
	app.Get("/api/v1/reports/:id/download", HandleDownloadReport)
	// Public share endpoint — no auth middleware.
	app.Get("/api/v1/reports/share/:token", HandleGetSharedReport)
	return app
}

func seedReport(t *testing.T, orgID int, framework, status string) Report {
	t.Helper()
	r := &Report{
		ID:        newID(),
		OrgID:     orgID,
		Framework: framework,
		TimeRange: "last_30_days",
		Options:   map[string]interface{}{},
		Status:    status,
	}
	if err := insertReport(r); err != nil {
		t.Fatalf("seedReport: %v", err)
	}
	// For done reports, add a share token.
	if status == "done" {
		token := newShareToken()
		updateReport(r.ID, "done", "", token, nil, nil)
		r.ShareToken = token
	}
	return *r
}

func TestHandleListReports_Empty(t *testing.T) {
	app := setupReportsApp(t)

	req := httptest.NewRequest("GET", "/api/v1/reports", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["total"].(float64) != 0 {
		t.Errorf("expected 0 reports, got %v", result["total"])
	}
}

func TestHandleGenerateReport_Success(t *testing.T) {
	app := setupReportsApp(t)

	payload := map[string]interface{}{
		"framework":  "GDPR",
		"time_range": "last_30_days",
		"options":    map[string]interface{}{},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/reports/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 202 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["report_id"].(string); !ok {
		t.Error("expected report_id in response")
	}
}

func TestHandleGenerateReport_InvalidFramework(t *testing.T) {
	app := setupReportsApp(t)

	payload := map[string]interface{}{
		"framework":  "INVALID",
		"time_range": "last_30_days",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/reports/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for invalid framework, got %d", resp.StatusCode)
	}
}

func TestHandleGenerateReport_MissingTimeRange(t *testing.T) {
	app := setupReportsApp(t)

	payload := map[string]interface{}{
		"framework": "GDPR",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/v1/reports/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for missing time_range, got %d", resp.StatusCode)
	}
}

func TestHandleGetReport_NotFound(t *testing.T) {
	app := setupReportsApp(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/nonexistent", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleGetReport_Found(t *testing.T) {
	app := setupReportsApp(t)
	r := seedReport(t, 1, "GDPR", "pending")

	req := httptest.NewRequest("GET", "/api/v1/reports/"+r.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["framework"] != "GDPR" {
		t.Errorf("expected GDPR framework, got %v", result["framework"])
	}
}

func TestHandleDownloadReport_NotReady(t *testing.T) {
	app := setupReportsApp(t)
	r := seedReport(t, 1, "GDPR", "pending")

	req := httptest.NewRequest("GET", "/api/v1/reports/"+r.ID+"/download", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422 for pending report, got %d", resp.StatusCode)
	}
}

func TestHandleGetSharedReport_ValidToken(t *testing.T) {
	setupTestDB(t)
	r := seedReport(t, 1, "GDPR", "done")

	// Use a bare app without auth to test the public endpoint.
	app := fiber.New()
	app.Get("/api/v1/reports/share/:token", HandleGetSharedReport)

	req := httptest.NewRequest("GET", "/api/v1/reports/share/"+r.ShareToken, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for valid share token, got %d: %s", resp.StatusCode, b)
	}
}

func TestHandleGetSharedReport_InvalidToken(t *testing.T) {
	app := setupReportsApp(t)

	req := httptest.NewRequest("GET", "/api/v1/reports/share/invalid-token-xyz", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 for invalid token, got %d", resp.StatusCode)
	}
}

// ── Valid frameworks ──────────────────────────────────────────────────────────

func TestHandleGenerateReport_AllFrameworks(t *testing.T) {
	app := setupReportsApp(t)
	for _, fw := range []string{"GDPR", "SOC2", "HIPAA", "Custom"} {
		payload := map[string]interface{}{
			"framework":  fw,
			"time_range": "last_30_days",
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/reports/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 202 {
			b, _ := io.ReadAll(resp.Body)
			t.Errorf("framework %s failed: %d %s", fw, resp.StatusCode, b)
		}
	}
}

// ── newShareToken ─────────────────────────────────────────────────────────────

func TestNewShareToken(t *testing.T) {
	t1 := newShareToken()
	t2 := newShareToken()
	if len(t1) != 64 { // 32 bytes → 64 hex chars
		t.Errorf("expected 64-char token, got %d", len(t1))
	}
	if t1 == t2 {
		t.Error("share tokens should be unique")
	}
}
