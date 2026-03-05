package harvest

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// newTestHandler builds a Handler with empty integrations (no installed services),
// so scanner.Scan returns an empty result without calling the AI client.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	// integrations store — no services installed
	intKey := make([]byte, 32)
	intStore := integrations.NewStore(openTestDB(t), intKey)
	intStore.Migrate()

	// datasync engine — no adapters
	engine := datasync.NewEngine(intStore, nil)

	// activities store — scanner writes events here
	actStore := activities.NewStore(openTestDB(t))
	actStore.Migrate()

	// ai client — not called since no signals are produced
	aiClient := ai.NewClient("http://localhost:11434", "llama3.2")

	scanner := NewScanner(engine, aiClient, actStore)

	harvestStore := NewStore(openTestDB(t))
	harvestStore.Migrate()

	notifsStore := notifications.NewStore(openTestDB(t))
	notifsStore.Migrate()

	return NewHandler(scanner, harvestStore, notifsStore)
}

func setupHarvestApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	newTestHandler(t).RegisterRoutes(app)
	return app
}

func TestHandlerResults_NoScanReturnsMessage(t *testing.T) {
	app := setupHarvestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/harvest/results", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if _, ok := m["message"]; !ok {
		t.Error("want 'message' field when no scan has run")
	}
}

func TestHandlerScan_EmptyServicesReturnsAccepted(t *testing.T) {
	app := setupHarvestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/harvest/scan", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("want 202, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["status"] != "started" {
		t.Errorf("want status=started, got %v", m["status"])
	}
}

func TestHandlerScan_SavedResultAvailableViaResults(t *testing.T) {
	h := newTestHandler(t)
	app := fiber.New()
	h.RegisterRoutes(app)

	// Fire async scan
	req1 := httptest.NewRequest(http.MethodPost, "/harvest/scan", nil)
	resp1, _ := app.Test(req1)
	if resp1.StatusCode != http.StatusAccepted {
		t.Errorf("scan: want 202, got %d", resp1.StatusCode)
	}

	// Poll /harvest/status until running=false (max 5s).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/harvest/status", nil)
		resp, _ := app.Test(req)
		var st map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&st)
		if running, _ := st["running"].(bool); !running {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Now fetch the stored result
	req2 := httptest.NewRequest(http.MethodGet, "/harvest/results", nil)
	resp2, _ := app.Test(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("results: want 200, got %d", resp2.StatusCode)
	}
	var result HarvestResult
	json.NewDecoder(resp2.Body).Decode(&result)
	if result.ScannedAt.IsZero() {
		t.Error("want non-zero ScannedAt in stored result")
	}
}
