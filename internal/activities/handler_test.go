package activities

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func setupAppWithStore(t *testing.T) (*fiber.App, *Store) {
	t.Helper()
	store := newTestStore(t)
	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)
	return app, store
}

func TestHandlerList_EmptyReturnsArray(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/activities/events", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var events []Event
	json.NewDecoder(resp.Body).Decode(&events)
	if len(events) != 0 {
		t.Errorf("want empty array, got %d events", len(events))
	}
}

func TestHandlerList_ReturnsCreatedEvents(t *testing.T) {
	app, store := setupAppWithStore(t)
	store.Create(Event{EventType: Finance, Decision: Pending, Narrative: "test"})

	req := httptest.NewRequest(http.MethodGet, "/activities/events", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var events []Event
	json.NewDecoder(resp.Body).Decode(&events)
	if len(events) != 1 {
		t.Errorf("want 1 event, got %d", len(events))
	}
}

func TestHandlerGet_ExistingEvent(t *testing.T) {
	app, store := setupAppWithStore(t)
	ev, _ := store.Create(Event{EventType: Calendar, Narrative: "Meeting"})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/activities/events/%d", ev.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var got Event
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Narrative != "Meeting" {
		t.Errorf("Narrative: want %q, got %q", "Meeting", got.Narrative)
	}
}

func TestHandlerGet_NonExistentReturns404(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/activities/events/99999", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandlerGet_InvalidIDReturns400(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/activities/events/abc", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandlerToggleRead_TogglesReadFlag(t *testing.T) {
	app, store := setupAppWithStore(t)
	ev, _ := store.Create(Event{EventType: Finance, Narrative: "test"})

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/activities/events/%d/read", ev.ID), bytes.NewReader(nil))
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var got Event
	json.NewDecoder(resp.Body).Decode(&got)
	if !got.Read {
		t.Error("want Read=true after toggle")
	}
}

func TestHandlerToggleRead_NonExistentReturns404(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPut, "/activities/events/99999/read", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

