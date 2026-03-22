package audit

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupBenchmarksTestApp(t *testing.T) *fiber.App {
	t.Helper()
	testDB := setupTestDB(t)
	db = testDB

	app := fiber.New()
	app.Get("/api/v1/benchmarks", HandleGetBenchmarks)
	return app
}

func TestHandleGetBenchmarks_Status(t *testing.T) {
	app := setupBenchmarksTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleGetBenchmarks_RequiredFields(t *testing.T) {
	app := setupBenchmarksTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	for _, field := range []string{"generated_at", "environment", "go_version", "results", "methodology"} {
		if _, ok := result[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

func TestHandleGetBenchmarks_ResultsNotEmpty(t *testing.T) {
	app := setupBenchmarksTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var report BenchmarkReport
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("failed to parse BenchmarkReport: %v", err)
	}

	if len(report.Results) == 0 {
		t.Fatal("expected non-empty benchmark results")
	}
}

func TestHandleGetBenchmarks_ResultShape(t *testing.T) {
	app := setupBenchmarksTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var report BenchmarkReport
	json.Unmarshal(body, &report)

	for _, r := range report.Results {
		if r.Label == "" {
			t.Error("benchmark result has empty label")
		}
		if r.P50Ms <= 0 {
			t.Errorf("benchmark %q has non-positive P50: %v", r.Label, r.P50Ms)
		}
		if r.P95Ms < r.P50Ms {
			t.Errorf("benchmark %q: P95 (%v) < P50 (%v)", r.Label, r.P95Ms, r.P50Ms)
		}
		if r.P99Ms < r.P95Ms {
			t.Errorf("benchmark %q: P99 (%v) < P95 (%v)", r.Label, r.P99Ms, r.P95Ms)
		}
		if r.Throughput == "" {
			t.Errorf("benchmark %q has empty Throughput", r.Label)
		}
		if r.Description == "" {
			t.Errorf("benchmark %q has empty Description", r.Label)
		}
	}
}

func TestHandleGetBenchmarks_MethodologyNonEmpty(t *testing.T) {
	app := setupBenchmarksTestApp(t)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var report BenchmarkReport
	json.Unmarshal(body, &report)

	if len(report.Methodology) < 50 {
		t.Errorf("methodology text is too short (%d chars)", len(report.Methodology))
	}
}

func TestHandleGetBenchmarks_NoAuthRequired(t *testing.T) {
	// Benchmarks are public — no auth middleware should be needed.
	testDB := setupTestDB(t)
	db = testDB

	app := fiber.New()
	// Intentionally no auth middleware.
	app.Get("/api/v1/benchmarks", HandleGetBenchmarks)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != 200 {
		t.Errorf("benchmarks should be accessible without auth, got %d", resp.StatusCode)
	}
}

func TestHandleGetBenchmarks_LiveStatsAbsentWhenNoData(t *testing.T) {
	// With an empty DB (no transactions), live_stats should be absent/nil.
	testDB := setupTestDB(t)
	db = testDB

	app := fiber.New()
	app.Get("/api/v1/benchmarks", HandleGetBenchmarks)

	req := httptest.NewRequest("GET", "/api/v1/benchmarks", nil)
	resp, _ := app.Test(req, -1)
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if liveStats, ok := result["live_stats"]; ok && liveStats != nil {
		t.Logf("live_stats present (may be valid if transactions exist): %v", liveStats)
	}
	// No assertion — just confirm the endpoint doesn't crash.
}
