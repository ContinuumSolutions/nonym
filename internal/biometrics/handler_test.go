package biometrics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupApp(t *testing.T) *fiber.App {
	t.Helper()
	store := newTestStore(t) // defined in store_test.go
	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)
	return app
}

func TestHandlerGet_404WhenEmpty(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandlerUpdate_Creates(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(map[string]interface{}{
		"mood":         8,
		"stress_level": 3,
		"sleep":        7,
		"energy":       9,
	})
	req := httptest.NewRequest(http.MethodPut, "/biometrics/checkin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var ci CheckIn
	json.NewDecoder(resp.Body).Decode(&ci)
	if ci.Mood != 8 {
		t.Errorf("Mood: want 8, got %d", ci.Mood)
	}
}

func TestHandlerGet_200AfterUpdate(t *testing.T) {
	store := newTestStore(t)
	store.Upsert(&CheckIn{Mood: 7, StressLevel: 4, Sleep: 6, Energy: 8})

	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var ci CheckIn
	json.NewDecoder(resp.Body).Decode(&ci)
	if ci.Mood != 7 {
		t.Errorf("Mood: want 7, got %d", ci.Mood)
	}
}

func TestHandlerUpdate_400ForBadBody(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPut, "/biometrics/checkin", bytes.NewReader([]byte("notjson")))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

// ── GET /biometrics/checkin/history ──────────────────────────────────────────

func TestHandlerHistory_EmptyReturnsEmptyArray(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin/history", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var entries []CheckIn
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 0 {
		t.Errorf("want empty array before any check-ins, got %d entries", len(entries))
	}
}

func TestHandlerHistory_ReturnsEntriesAfterUpdate(t *testing.T) {
	store := newTestStore(t)
	store.Upsert(&CheckIn{Mood: 7, StressLevel: 3, Sleep: 8, Energy: 9})
	store.Upsert(&CheckIn{Mood: 5, StressLevel: 6, Sleep: 6, Energy: 7})

	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin/history", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var entries []CheckIn
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 2 {
		t.Fatalf("want 2 history entries, got %d", len(entries))
	}
	// newest first
	if entries[0].Mood != 5 {
		t.Errorf("want newest entry first (Mood=5), got Mood=%d", entries[0].Mood)
	}
}

func TestHandlerHistory_LimitQueryParam(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 5; i++ {
		store.Upsert(&CheckIn{Mood: i + 1, StressLevel: 5, Sleep: 5, Energy: 5})
	}

	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin/history?limit=2", nil)
	resp, _ := app.Test(req)
	var entries []CheckIn
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 2 {
		t.Errorf("want 2 entries with ?limit=2, got %d", len(entries))
	}
}

func TestHandlerHistory_DefaultLimitIs7(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 10; i++ {
		store.Upsert(&CheckIn{Mood: i + 1, StressLevel: 5, Sleep: 5, Energy: 5})
	}

	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin/history", nil)
	resp, _ := app.Test(req)
	var entries []CheckIn
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 7 {
		t.Errorf("want default 7 entries, got %d", len(entries))
	}
}

func TestHandlerHistory_InvalidLimitIgnored(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin/history?limit=notanumber", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	// Should still return 200 with the default limit applied
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 for invalid limit param, got %d", resp.StatusCode)
	}
}
