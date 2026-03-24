package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&pagerdutyConnector{
		baseURL: "https://api.pagerduty.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type pagerdutyConnector struct {
	baseURL string
	client  *http.Client
}

func (p *pagerdutyConnector) Vendor() string { return "pagerduty" }

func (p *pagerdutyConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	apiKey := ""
	for _, k := range []string{"api_key", "token"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			apiKey = v
			break
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("pagerduty: no api_key in credentials")
	}

	incidents, err := p.fetchIncidents(apiKey)
	if err != nil {
		return nil, fmt.Errorf("pagerduty: fetch incidents: %w", err)
	}
	return p.normalise(incidents), nil
}

// ── PagerDuty API types ───────────────────────────────────────────────────────

type pagerdutyIncident struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type pagerdutyIncidentsResponse struct {
	Incidents []pagerdutyIncident `json:"incidents"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (p *pagerdutyConnector) get(apiKey, path string, out interface{}) error {
	req, err := http.NewRequest("GET", p.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token token="+apiKey)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed (HTTP 401) — check API key")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (p *pagerdutyConnector) fetchIncidents(apiKey string) ([]pagerdutyIncident, error) {
	var result pagerdutyIncidentsResponse
	err := p.get(apiKey, "/incidents?limit=100&sort_by=created_at:desc", &result)
	return result.Incidents, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (p *pagerdutyConnector) normalise(incidents []pagerdutyIncident) []NormalizedEvent {
	var out []NormalizedEvent
	for _, i := range incidents {
		if i.Title != "" {
			out = append(out, NormalizedEvent{
				VendorID: "pagerduty", EventID: i.ID, Source: "incident.title",
				Text: i.Title, Metadata: map[string]string{"endpoint": "api.pagerduty.com/incidents"},
			})
		}
		if i.Summary != "" && i.Summary != i.Title {
			out = append(out, NormalizedEvent{
				VendorID: "pagerduty", EventID: i.ID, Source: "incident.summary",
				Text: i.Summary, Metadata: map[string]string{"endpoint": "api.pagerduty.com/incidents"},
			})
		}
	}
	return out
}
