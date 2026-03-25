package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&rollbarConnector{
		baseURL: "https://api.rollbar.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type rollbarConnector struct {
	baseURL string
	client  *http.Client
}

func (r *rollbarConnector) Vendor() string { return "rollbar" }

// TestConnection verifies the access token by listing projects.
func (r *rollbarConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "access_token", "token", "api_key")
	if len(token) < 8 {
		return ConnectionResult{Success: false, Message: "Rollbar access token is missing or too short"}
	}
	req, err := http.NewRequest("GET", r.baseURL+"/api/1/projects/?access_token="+token, nil)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Failed to build request: %v", err)}
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Could not reach Rollbar API: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return ConnectionResult{Success: false, Message: "Invalid Rollbar token — check that it has read scope"}
	}
	if resp.StatusCode >= 400 {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Rollbar API error (HTTP %d)", resp.StatusCode)}
	}
	return ConnectionResult{Success: true, Message: "Rollbar token validated — account accessible"}
}

func (r *rollbarConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "access_token", "token", "api_key")
	if token == "" {
		return nil, fmt.Errorf("rollbar: no access_token in credentials")
	}

	items, err := r.fetchItems(token)
	if err != nil {
		return nil, err
	}
	return r.normalise(items), nil
}

// ── Rollbar API types ─────────────────────────────────────────────────────────

type rollbarItem struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Environment string `json:"environment"`
	Level       string `json:"level"`
}

type rollbarItemsResponse struct {
	Result struct {
		Items []rollbarItem `json:"items"`
	} `json:"result"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (r *rollbarConnector) fetchItems(token string) ([]rollbarItem, error) {
	url := r.baseURL + "/api/1/items/?access_token=" + token + "&status=active&level=error&page_size=100"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Rollbar API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed (HTTP %d) — check that the token has read scope", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result rollbarItemsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse items response: %w", err)
	}
	return result.Result.Items, nil
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (r *rollbarConnector) normalise(items []rollbarItem) []NormalizedEvent {
	var out []NormalizedEvent
	for _, item := range items {
		id := fmt.Sprintf("%d", item.ID)
		if item.Title != "" {
			out = append(out, NormalizedEvent{
				VendorID: "rollbar",
				EventID:  id,
				Source:   "item.title",
				Text:     item.Title,
				Metadata: map[string]string{"endpoint": "api.rollbar.com", "level": item.Level},
			})
		}
	}
	return out
}
