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
		"feeling":      8,
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
	if ci.Feeling != 8 {
		t.Errorf("Feeling: want 8, got %d", ci.Feeling)
	}
}

func TestHandlerGet_200AfterUpdate(t *testing.T) {
	store := newTestStore(t)
	store.Upsert(&CheckIn{Feeling: 7, StressLevel: 4, Sleep: 6, Energy: 8})

	app := fiber.New()
	NewHandler(store).RegisterRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/biometrics/checkin", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var ci CheckIn
	json.NewDecoder(resp.Body).Decode(&ci)
	if ci.Feeling != 7 {
		t.Errorf("Feeling: want 7, got %d", ci.Feeling)
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
