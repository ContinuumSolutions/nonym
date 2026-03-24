package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&datadogConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type datadogConnector struct{ client *http.Client }

func (d *datadogConnector) Vendor() string { return "datadog" }

// TestConnection validates that both api_key and app_key are present.
func (d *datadogConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	if credStr(vc, "api_key") == "" || credStr(vc, "app_key") == "" {
		return ConnectionResult{Success: false, Message: "Datadog requires both api_key and app_key"}
	}
	return ConnectionResult{Success: true, Message: "Datadog credentials validated (format check passed)"}
}

func (d *datadogConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	apiKey, _ := vc.Credentials["api_key"].(string)
	appKey, _ := vc.Credentials["app_key"].(string)
	if apiKey == "" || appKey == "" {
		return nil, fmt.Errorf("datadog: missing api_key or app_key in credentials")
	}
	site := "datadoghq.com"
	if s, ok := vc.Credentials["site"].(string); ok && s != "" {
		site = s
	}

	logs, err := d.fetchLogs(apiKey, appKey, site)
	if err != nil {
		return nil, fmt.Errorf("datadog: fetch logs: %w", err)
	}
	return d.normalise(logs, site), nil
}

// ── Datadog log types ─────────────────────────────────────────────────────────

type datadogLogEvent struct {
	ID         string                 `json:"id"`
	Attributes datadogLogAttributes   `json:"attributes"`
}

type datadogLogAttributes struct {
	Message    string                 `json:"message"`
	Service    string                 `json:"service"`
	Host       string                 `json:"host"`
	Attributes map[string]interface{} `json:"attributes"`
}

type datadogLogsResponse struct {
	Data []datadogLogEvent `json:"data"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (d *datadogConnector) fetchLogs(apiKey, appKey, site string) ([]datadogLogEvent, error) {
	// Query the last 15 minutes of logs, capped at 100 events.
	now := time.Now().UTC()
	from := now.Add(-15 * time.Minute)

	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"from":  from.Format(time.RFC3339),
			"to":    now.Format(time.RFC3339),
			"query": "*",
		},
		"page": map[string]interface{}{"limit": 100},
		"sort": "-timestamp",
	}
	body, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://api.%s/api/v2/logs/events/search", site)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed (HTTP 403) — check that the API key has logs_read scope")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var result datadogLogsResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result.Data, nil
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (d *datadogConnector) normalise(events []datadogLogEvent, site string) []NormalizedEvent {
	endpoint := "api." + site
	var out []NormalizedEvent

	emit := func(id, field, text string) {
		if text == "" {
			return
		}
		out = append(out, NormalizedEvent{
			VendorID: "datadog",
			EventID:  id,
			Source:   field,
			Text:     text,
			Metadata: map[string]string{"endpoint": endpoint},
		})
	}

	for _, ev := range events {
		emit(ev.ID, "log.message", ev.Attributes.Message)
		emit(ev.ID, "log.service", ev.Attributes.Service)
		for k, v := range ev.Attributes.Attributes {
			if s, ok := stringify(v); ok {
				emit(ev.ID, "log.attributes."+k, s)
			}
		}
	}
	log.Printf("datadog connector: normalised %d events into %d scannable fields", len(events), len(out))
	return out
}
