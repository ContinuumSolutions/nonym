package audit

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ─── Unit tests ───────────────────────────────────────────────────────────────

func TestNormalizeFramework(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"gdpr", "GDPR"},
		{"GDPR", "GDPR"},
		{"hipaa", "HIPAA"},
		{"HIPAA", "HIPAA"},
		{"pci", "PCI-DSS"},
		{"PCI", "PCI-DSS"},
		{"pci-dss", "PCI-DSS"},
		{"PCI-DSS", "PCI-DSS"},
		{"soc2", ""},   // not a supported report framework
		{"", ""},
		{"unknown", ""},
	}

	for _, tc := range cases {
		got := normalizeFramework(tc.input)
		if got != tc.want {
			t.Errorf("normalizeFramework(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseReportTimeRange(t *testing.T) {
	cases := []struct {
		input      string
		wantLabel  string
		wantApprox time.Duration // approximate age of the since time
	}{
		{"7d", "7d", 7 * 24 * time.Hour},
		{"30d", "30d", 30 * 24 * time.Hour},
		{"90d", "90d", 90 * 24 * time.Hour},
		{"invalid", "30d", 30 * 24 * time.Hour}, // fallback
	}

	for _, tc := range cases {
		since, label := parseReportTimeRange(tc.input)
		if label != tc.wantLabel {
			t.Errorf("parseReportTimeRange(%q) label = %q, want %q", tc.input, label, tc.wantLabel)
		}
		// Check that since is approximately the right distance in the past.
		age := time.Since(since)
		tolerance := 5 * time.Second
		if age < tc.wantApprox-tolerance || age > tc.wantApprox+tolerance {
			t.Errorf("parseReportTimeRange(%q) since age = %v, want ~%v", tc.input, age, tc.wantApprox)
		}
	}
}

func TestContainsStr(t *testing.T) {
	cases := []struct {
		slice []string
		s     string
		want  bool
	}{
		{[]string{"GDPR", "HIPAA"}, "GDPR", true},
		{[]string{"GDPR", "HIPAA"}, "SOC2", false},
		{[]string{}, "GDPR", false},
		{nil, "GDPR", false},
	}

	for _, tc := range cases {
		got := containsStr(tc.slice, tc.s)
		if got != tc.want {
			t.Errorf("containsStr(%v, %q) = %v, want %v", tc.slice, tc.s, got, tc.want)
		}
	}
}

func TestRecommendationsFor(t *testing.T) {
	for _, fw := range []string{"GDPR", "HIPAA", "PCI-DSS"} {
		recs := recommendationsFor(fw)
		if len(recs) == 0 {
			t.Errorf("recommendationsFor(%q) returned no recommendations", fw)
		}
		// Framework-specific recommendation should be present.
		found := false
		for _, r := range recs {
			if len(r) > 10 { // any non-trivial string
				found = true
				break
			}
		}
		if !found {
			t.Errorf("recommendationsFor(%q) returned trivial recommendations", fw)
		}
	}
}

// ─── HTTP handler tests ───────────────────────────────────────────────────────

func setupComplianceReportTestApp(t *testing.T) *fiber.App {
	t.Helper()
	testDB := setupTestDB(t)
	db = testDB
	if err := InitializeComplianceTables(); err != nil {
		t.Fatalf("InitializeComplianceTables: %v", err)
	}

	app := fiber.New()
	app.Use(mockAuthMiddleware(1, 1))
	app.Get("/api/v1/compliance/reports", HandleListComplianceReports)
	app.Get("/api/v1/compliance/reports/:framework", HandleGetComplianceReport)
	return app
}

func TestHandleListComplianceReports(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports", nil)
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

	reports, ok := result["reports"].([]interface{})
	if !ok {
		t.Fatal("response missing reports array")
	}
	if len(reports) != 3 {
		t.Errorf("expected 3 reports (GDPR, HIPAA, PCI-DSS), got %d", len(reports))
	}

	// Each report should have required fields.
	for _, r := range reports {
		rm := r.(map[string]interface{})
		for _, field := range []string{"framework", "citation", "description", "endpoint"} {
			if _, exists := rm[field]; !exists {
				t.Errorf("report missing field %q", field)
			}
		}
	}
}

func TestHandleGetComplianceReport_GDPR(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports/gdpr", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var report ComplianceReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("failed to parse report: %v", err)
	}

	if report.Framework != "GDPR" {
		t.Errorf("expected framework GDPR, got %q", report.Framework)
	}
	if report.Citation == "" {
		t.Error("expected non-empty citation")
	}
	if report.OrganizationID != 1 {
		t.Errorf("expected org ID 1, got %d", report.OrganizationID)
	}
	if len(report.Recommendations) == 0 {
		t.Error("expected non-empty recommendations")
	}
	if report.EntityBreakdown == nil {
		t.Error("entity_breakdown should not be nil")
	}
	if report.DailyActivity == nil {
		t.Error("daily_activity should not be nil")
	}
}

func TestHandleGetComplianceReport_HIPAA(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports/hipaa", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var report ComplianceReport
	json.Unmarshal(body, &report)

	if report.Framework != "HIPAA" {
		t.Errorf("expected HIPAA, got %q", report.Framework)
	}
}

func TestHandleGetComplianceReport_PCI(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	for _, path := range []string{"/api/v1/compliance/reports/pci", "/api/v1/compliance/reports/pci-dss"} {
		req := httptest.NewRequest("GET", path, nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for %s, got %d", path, resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var report ComplianceReport
		json.Unmarshal(body, &report)

		if report.Framework != "PCI-DSS" {
			t.Errorf("expected PCI-DSS framework for path %s, got %q", path, report.Framework)
		}
	}
}

func TestHandleGetComplianceReport_UnknownFramework(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports/sox", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for unknown framework, got %d", resp.StatusCode)
	}
}

func TestHandleGetComplianceReport_TimeRange(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	for _, tr := range []string{"7d", "30d", "90d"} {
		req := httptest.NewRequest("GET", "/api/v1/compliance/reports/gdpr?timeRange="+tr, nil)
		resp, _ := app.Test(req, -1)
		if resp.StatusCode != 200 {
			t.Errorf("expected 200 for timeRange=%s, got %d", tr, resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var report ComplianceReport
		json.Unmarshal(body, &report)

		if report.TimeRange != tr {
			t.Errorf("expected time_range %s, got %q", tr, report.TimeRange)
		}
	}
}

func TestHandleGetComplianceReport_SummaryFields(t *testing.T) {
	app := setupComplianceReportTestApp(t)

	// Seed a few transactions so summary counts are non-trivial.
	for i := 0; i < 3; i++ {
		LogTransaction(
			"cr-tx-"+string(rune('0'+i)), "success", "openai", "", 200, nil, 1, 1,
		)
	}

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports/gdpr", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var report ComplianceReport
	json.Unmarshal(body, &report)

	// Compliance score must be in [0, 100].
	if report.Summary.ComplianceScore < 0 || report.Summary.ComplianceScore > 100 {
		t.Errorf("compliance_score out of range: %v", report.Summary.ComplianceScore)
	}

	if report.Summary.TotalRequests < 0 {
		t.Errorf("total_requests should be >= 0, got %d", report.Summary.TotalRequests)
	}
}

func TestHandleGetComplianceReport_RequiresAuth(t *testing.T) {
	testDB := setupTestDB(t)
	db = testDB

	app := fiber.New()
	// No auth middleware — organization_id won't be set.
	app.Get("/api/v1/compliance/reports/:framework", HandleGetComplianceReport)

	req := httptest.NewRequest("GET", "/api/v1/compliance/reports/gdpr", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 without auth, got %d", resp.StatusCode)
	}
}
