package ledger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func setupApp(t *testing.T) *fiber.App {
	t.Helper()
	l := newTestLedger(t) // defined in sqlite_test.go
	l.Initialize("ek1-kernel")
	app := fiber.New()
	NewHandler(l, "ek1-kernel").RegisterRoutes(app)
	return app
}

func TestHandlerScore_200WithFields(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/ledger/score", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if _, ok := m["score"]; !ok {
		t.Error("response missing 'score' field")
	}
	if _, ok := m["tier"]; !ok {
		t.Error("response missing 'tier' field")
	}
	if _, ok := m["trust_tax"]; !ok {
		t.Error("response missing 'trust_tax' field")
	}
}

func TestHandlerHistory_200WithEntries(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("ek1-kernel")
	l.LogSuccess("ek1-kernel", 50)

	app := fiber.New()
	NewHandler(l, "ek1-kernel").RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/ledger/history", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var entries []HistoryEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	// Initialize inserts 1 row, LogSuccess inserts 1 more
	if len(entries) < 2 {
		t.Errorf("want at least 2 entries, got %d", len(entries))
	}
}

func TestHandlerHistory_RespectsLimitParam(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("ek1-kernel")
	for i := 0; i < 5; i++ {
		l.LogSuccess("ek1-kernel", 10)
	}
	app := fiber.New()
	NewHandler(l, "ek1-kernel").RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/ledger/history?limit=2", nil)
	resp, _ := app.Test(req)
	var entries []HistoryEntry
	json.NewDecoder(resp.Body).Decode(&entries)
	if len(entries) != 2 {
		t.Errorf("want 2 entries with limit=2, got %d", len(entries))
	}
}
