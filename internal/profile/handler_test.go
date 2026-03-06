package profile

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
	NewHandler(store, nil, nil).RegisterRoutes(app)
	return app
}

func TestHandlerGet_ReturnsDefaults(t *testing.T) {
	app := setupApp(t)
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var p Profile
	json.NewDecoder(resp.Body).Decode(&p)
	if p.KernelName != "EK-1" {
		t.Errorf("KernelName: want EK-1, got %q", p.KernelName)
	}
}

func TestHandlerUpdatePreferences_PersistsAndReturns(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(DecisionPreference{
		TimeSovereignty:    9,
		FinacialGrowth:     7,
		HealthRecovery:     8,
		ReputationBuilding: 6,
		PrivacyProtection:  5,
		Autonomy:           4,
	})
	req := httptest.NewRequest(http.MethodPut, "/profile/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var p Profile
	json.NewDecoder(resp.Body).Decode(&p)
	if p.Preferences.TimeSovereignty != 9 {
		t.Errorf("TimeSovereignty: want 9, got %d", p.Preferences.TimeSovereignty)
	}
}

func TestHandlerUpdatePreferences_400ForOutOfRange(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(map[string]int{
		"time_sovereignty": 11, // out of 1-10 range
	})
	req := httptest.NewRequest(http.MethodPut, "/profile/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for out-of-range preference, got %d", resp.StatusCode)
	}
}

func TestHandlerUpdateConnection_PersistsAndReturns(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(ConnectionSetting{
		KernelName:  "test-kernel",
		APIEndpoint: "https://example.com",
		Timezone:    "Europe/London",
	})
	req := httptest.NewRequest(http.MethodPut, "/profile/connection", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
	var p Profile
	json.NewDecoder(resp.Body).Decode(&p)
	if p.KernelName != "test-kernel" {
		t.Errorf("KernelName: want %q, got %q", "test-kernel", p.KernelName)
	}
}

func TestHandlerUpdateConnection_400WhenKernelNameMissing(t *testing.T) {
	app := setupApp(t)
	body, _ := json.Marshal(ConnectionSetting{KernelName: ""})
	req := httptest.NewRequest(http.MethodPut, "/profile/connection", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 when kernel_name is empty, got %d", resp.StatusCode)
	}
}
