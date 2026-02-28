package brain

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

func newTestActivitiesStore(t *testing.T) *activities.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := activities.NewStore(db)
	if err := s.Migrate(); err != nil {
		t.Fatalf("activities Migrate: %v", err)
	}
	return s
}

func setupBrainApp(t *testing.T) *fiber.App {
	t.Helper()
	svc := newTestService(t)
	events := newTestActivitiesStore(t)
	app := fiber.New()
	NewHandler(svc, events).RegisterRoutes(app)
	return app
}

func setupBrainAppWithStore(t *testing.T) (*fiber.App, *Service, *activities.Store) {
	t.Helper()
	svc := newTestService(t)
	events := newTestActivitiesStore(t)
	app := fiber.New()
	NewHandler(svc, events).RegisterRoutes(app)
	return app, svc, events
}

func TestHandlerStatus_200WithExpectedFields(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodGet, "/brain/status", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	for _, field := range []string{"status", "reputation_score", "reputation_tier"} {
		if _, ok := m[field]; !ok {
			t.Errorf("response missing field %q", field)
		}
	}
}

func TestHandlerStatus_ReturnsOnlineByDefault(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodGet, "/brain/status", nil)
	resp, _ := app.Test(req)
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["status"] != "ONLINE" {
		t.Errorf("want status ONLINE for fresh kernel, got %v", m["status"])
	}
}

func TestHandlerSyncAcknowledge_200(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodPost, "/brain/sync-acknowledge", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	// Response should be a KernelSnapshot with a "status" field
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if _, ok := m["status"]; !ok {
		t.Error("response missing 'status' field")
	}
}

func TestHandlerEvents_EmptyArray(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodGet, "/brain/events", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var events []activities.Event
	json.NewDecoder(resp.Body).Decode(&events)
	if len(events) != 0 {
		t.Errorf("want empty array, got %d events", len(events))
	}
}

func TestHandlerEvents_ReturnsCreatedEvents(t *testing.T) {
	app, _, events := setupBrainAppWithStore(t)
	events.Create(activities.Event{EventType: activities.Finance, Narrative: "test event"})

	req := httptest.NewRequest(http.MethodGet, "/brain/events", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var list []activities.Event
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 {
		t.Errorf("want 1 event, got %d", len(list))
	}
}
