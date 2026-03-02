package integrations

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
	NewHandler(store, "http://localhost:3000", "http://localhost:8080").RegisterRoutes(app)
	return app
}

func setupAppWithStore(t *testing.T) (*fiber.App, *Store) {
	t.Helper()
	store := newTestStore(t)
	app := fiber.New()
	NewHandler(store, "http://localhost:3000", "http://localhost:8080").RegisterRoutes(app)
	return app, store
}

func TestHandlerList_EmptyWithoutSeed(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/integrations/services", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var svcs []Service
	json.NewDecoder(resp.Body).Decode(&svcs)
	if len(svcs) != 0 {
		t.Errorf("want empty array, got %d services", len(svcs))
	}
}

func TestHandlerList_ReturnsSeededServices(t *testing.T) {
	app, store := setupAppWithStore(t)
	store.Seed()

	req := httptest.NewRequest(http.MethodGet, "/integrations/services", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var svcs []Service
	json.NewDecoder(resp.Body).Decode(&svcs)
	if len(svcs) == 0 {
		t.Error("want services after Seed, got 0")
	}
}

func TestHandlerGet_ExistingService(t *testing.T) {
	app, store := setupAppWithStore(t)
	svc, _ := store.CreateCustom(&Service{
		Name: "Test", Category: "Finance",
		APIEndpoint: "https://test.com", APIKey: "key1234",
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/integrations/services/%d", svc.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var got Service
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Name != "Test" {
		t.Errorf("Name: want %q, got %q", "Test", got.Name)
	}
}

func TestHandlerGet_NotFound(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/integrations/services/99999", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandlerGet_InvalidID(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/integrations/services/abc", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandlerCreateCustom_201(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(map[string]string{
		"name":         "Test API",
		"category":     "Finance",
		"api_endpoint": "https://api.test.com",
		"api_key":      "secret1234",
	})
	req := httptest.NewRequest(http.MethodPost, "/integrations/services/custom", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("want 201, got %d", resp.StatusCode)
	}
	var svc Service
	json.NewDecoder(resp.Body).Decode(&svc)
	if svc.ID == 0 {
		t.Error("want non-zero ID in response")
	}
	if svc.Status != Connected {
		t.Errorf("want Connected, got %v", svc.Status)
	}
}

func TestHandlerCreateCustom_400MissingFields(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(map[string]string{"name": "No Category"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/services/custom", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for missing required fields, got %d", resp.StatusCode)
	}
}

func TestHandlerStartConnect_200(t *testing.T) {
	app, store := setupAppWithStore(t)
	store.Seed()
	svcs, _ := store.List()
	id := svcs[0].ID

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/integrations/services/%d/connect", id), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var svc Service
	json.NewDecoder(resp.Body).Decode(&svc)
	if svc.Status != Pending {
		t.Errorf("want Pending, got %v", svc.Status)
	}
}

func TestHandlerStartConnect_404(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPost, "/integrations/services/99999/connect", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandlerStartConnect_400InvalidID(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodPost, "/integrations/services/abc/connect", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandlerCompleteConnect_200APIKey(t *testing.T) {
	app, store := setupAppWithStore(t)
	store.Seed()
	svcs, _ := store.List()
	id := svcs[0].ID

	body, _ := json.Marshal(ConnectInput{APIKey: "new-key-5678"})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/integrations/services/%d/connect", id), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var svc Service
	json.NewDecoder(resp.Body).Decode(&svc)
	if svc.Status != Connected {
		t.Errorf("want Connected, got %v", svc.Status)
	}
}

func TestHandlerCompleteConnect_400NoCreds(t *testing.T) {
	app, store := setupAppWithStore(t)
	svc, _ := store.CreateCustom(&Service{
		Name: "X", Category: "Y", APIEndpoint: "https://x.com", APIKey: "key1234",
	})
	body, _ := json.Marshal(ConnectInput{}) // no credentials provided
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/integrations/services/%d/connect", svc.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for no credentials, got %d", resp.StatusCode)
	}
}

func TestHandlerUninstall_200(t *testing.T) {
	app, store := setupAppWithStore(t)
	svc, _ := store.CreateCustom(&Service{
		Name: "X", Category: "Y", APIEndpoint: "https://x.com", APIKey: "key1234",
	})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/integrations/services/%d/connect", svc.ID), nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var got Service
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Status != Disconnected {
		t.Errorf("want Disconnected after uninstall, got %v", got.Status)
	}
}

func TestHandlerUninstall_404(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodDelete, "/integrations/services/99999/connect", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}
