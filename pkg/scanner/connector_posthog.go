package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&posthogConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type posthogConnector struct{ client *http.Client }

func (p *posthogConnector) Vendor() string { return "posthog" }

func (p *posthogConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	apiKey, _ := vc.Credentials["api_key"].(string)
	projectID, _ := vc.Credentials["project_id"].(string)
	host, _ := vc.Credentials["host"].(string)
	if host == "" {
		host = "https://app.posthog.com"
	}
	if apiKey == "" || projectID == "" {
		return nil, fmt.Errorf("posthog: requires api_key and project_id")
	}

	events, err := p.fetchEvents(host, apiKey, projectID)
	if err != nil {
		return nil, fmt.Errorf("posthog: fetch events: %w", err)
	}
	persons, err := p.fetchPersons(host, apiKey, projectID)
	if err != nil {
		persons = nil
	}

	var all []NormalizedEvent
	all = append(all, p.normaliseEvents(projectID, events)...)
	all = append(all, p.normalisePersons(projectID, persons)...)
	return all, nil
}

// ── PostHog API types ─────────────────────────────────────────────────────────

type posthogEvent struct {
	ID         string                 `json:"id"`
	Event      string                 `json:"event"`
	Properties map[string]interface{} `json:"properties"`
}

type posthogEventsResponse struct {
	Results []posthogEvent `json:"results"`
}

type posthogPerson struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
}

type posthogPersonsResponse struct {
	Results []posthogPerson `json:"results"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (p *posthogConnector) get(host, apiKey, path string, out interface{}) error {
	req, err := http.NewRequest("GET", host+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (p *posthogConnector) fetchEvents(host, apiKey, projectID string) ([]posthogEvent, error) {
	var result posthogEventsResponse
	err := p.get(host, apiKey, fmt.Sprintf("/api/projects/%s/events/?limit=100", projectID), &result)
	return result.Results, err
}

func (p *posthogConnector) fetchPersons(host, apiKey, projectID string) ([]posthogPerson, error) {
	var result posthogPersonsResponse
	err := p.get(host, apiKey, fmt.Sprintf("/api/projects/%s/persons/?limit=50", projectID), &result)
	return result.Results, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

var posthogPIIProps = map[string]bool{
	"$email": true, "$name": true, "$phone": true,
	"email": true, "name": true, "phone": true,
	"$ip": true, "$current_url": true,
}

func (p *posthogConnector) normaliseEvents(projectID string, events []posthogEvent) []NormalizedEvent {
	endpoint := "app.posthog.com/api/projects/" + projectID + "/events"
	var out []NormalizedEvent
	for _, ev := range events {
		for k, v := range ev.Properties {
			if !posthogPIIProps[k] {
				continue
			}
			if s, ok := stringify(v); ok {
				out = append(out, NormalizedEvent{
					VendorID: "posthog", EventID: ev.ID,
					Source: "event.properties." + k, Text: s,
					Metadata: map[string]string{"endpoint": endpoint, "event": ev.Event},
				})
			}
		}
	}
	return out
}

func (p *posthogConnector) normalisePersons(projectID string, persons []posthogPerson) []NormalizedEvent {
	endpoint := "app.posthog.com/api/projects/" + projectID + "/persons"
	var out []NormalizedEvent
	for _, person := range persons {
		for k, v := range person.Properties {
			if !posthogPIIProps[k] {
				continue
			}
			if s, ok := stringify(v); ok {
				out = append(out, NormalizedEvent{
					VendorID: "posthog", EventID: person.ID,
					Source: "person.properties." + k, Text: s,
					Metadata: map[string]string{"endpoint": endpoint},
				})
			}
		}
	}
	return out
}
