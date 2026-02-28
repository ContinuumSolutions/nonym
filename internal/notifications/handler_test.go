package notifications

import (
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
	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var items []Notification
	json.NewDecoder(resp.Body).Decode(&items)
	if len(items) != 0 {
		t.Errorf("want empty array, got %d items", len(items))
	}
}

func TestHandlerList_ReturnsOnlyUnread(t *testing.T) {
	app, store := setupAppWithStore(t)
	n, _ := store.Create(Notification{Type: TypeH2HI, Title: "A", Body: "a"})
	store.Create(Notification{Type: TypeOpportunity, Title: "B", Body: "b"})
	store.MarkRead(n.ID) // mark first as read

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	resp, _ := app.Test(req)
	var items []Notification
	json.NewDecoder(resp.Body).Decode(&items)
	if len(items) != 1 {
		t.Errorf("want 1 unread, got %d", len(items))
	}
}

func TestHandlerMarkRead_200(t *testing.T) {
	app, store := setupAppWithStore(t)
	n, _ := store.Create(Notification{Type: TypeH2HI, Title: "X", Body: "y"})

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/notifications/%d/read", n.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var m map[string]bool
	json.NewDecoder(resp.Body).Decode(&m)
	if !m["ok"] {
		t.Error("want ok:true in response")
	}
}

func TestHandlerMarkRead_404ForMissing(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPut, "/notifications/99999/read", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandlerMarkRead_400ForInvalidID(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPut, "/notifications/abc/read", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandlerReadAll_200(t *testing.T) {
	app, store := setupAppWithStore(t)
	store.Create(Notification{Type: TypeH2HI, Title: "1", Body: "a"})
	store.Create(Notification{Type: TypeH2HI, Title: "2", Body: "b"})

	req := httptest.NewRequest(http.MethodPut, "/notifications/read-all", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	// Verify all are read now
	req2 := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	resp2, _ := app.Test(req2)
	var items []Notification
	json.NewDecoder(resp2.Body).Decode(&items)
	if len(items) != 0 {
		t.Errorf("want 0 unread after read-all, got %d", len(items))
	}
}
