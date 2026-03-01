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
	for _, field := range []string{"status", "reputation_score", "reputation_tier", "stage_progress", "time_saved_today"} {
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

func TestHandlerStatus_StageProgressShadow100(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodGet, "/brain/status", nil)
	resp, _ := app.Test(req)
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)

	sp, ok := m["stage_progress"].(map[string]interface{})
	if !ok {
		t.Fatal("stage_progress should be an object")
	}
	if sp["shadow"] != float64(100) {
		t.Errorf("shadow: want 100, got %v", sp["shadow"])
	}
	if sp["hand"] != float64(0) {
		t.Errorf("hand: want 0, got %v", sp["hand"])
	}
	if sp["voice"] != float64(0) {
		t.Errorf("voice: want 0, got %v", sp["voice"])
	}
}

func TestHandlerStatus_TimeSavedTodayZeroWithNoEvents(t *testing.T) {
	app := setupBrainApp(t)
	req := httptest.NewRequest(http.MethodGet, "/brain/status", nil)
	resp, _ := app.Test(req)
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["time_saved_today"] != float64(0) {
		t.Errorf("time_saved_today: want 0 with no events, got %v", m["time_saved_today"])
	}
}

func TestHandlerStatus_TimeSavedTodayCountsHandledEvents(t *testing.T) {
	svc := newTestService(t)
	events := newTestActivitiesStore(t)
	app := fiber.New()
	NewHandler(svc, events).RegisterRoutes(app)

	// 2 accepted + 1 automated = 3 handled → 45 minutes
	events.Create(activities.Event{EventType: activities.Communication, Decision: activities.Accepted})
	events.Create(activities.Event{EventType: activities.Finance, Decision: activities.Automated})
	events.Create(activities.Event{EventType: activities.Calendar, Decision: activities.Accepted})
	// Pending and Cancelled should not count
	events.Create(activities.Event{EventType: activities.Communication, Decision: activities.Pending})
	events.Create(activities.Event{EventType: activities.Billing, Decision: activities.Cancelled})

	req := httptest.NewRequest(http.MethodGet, "/brain/status", nil)
	resp, _ := app.Test(req)
	var m map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&m)
	if m["time_saved_today"] != float64(45) {
		t.Errorf("time_saved_today: want 45 (3×15 min), got %v", m["time_saved_today"])
	}
}

