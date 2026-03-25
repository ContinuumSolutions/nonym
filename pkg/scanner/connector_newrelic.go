package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&newrelicConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type newrelicConnector struct{ client *http.Client }

func (n *newrelicConnector) Vendor() string { return "newrelic" }

// TestConnection verifies the User API key via a lightweight NerdGraph identity query.
func (n *newrelicConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	apiKey := credStr(vc, "api_key", "user_key", "token")
	if !strings.HasPrefix(apiKey, "NRAK-") {
		return ConnectionResult{Success: false, Message: "New Relic requires a User API key (NRAK-...)"}
	}
	query := `{"query":"{ actor { user { name email } } }"}`
	req, err := http.NewRequest("POST", "https://api.newrelic.com/graphql", bytes.NewBufferString(query))
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Failed to build request: %v", err)}
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Could not reach New Relic API: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 {
		return ConnectionResult{Success: false, Message: "Invalid New Relic API key — must be a User key (NRAK-...)"}
	}
	if resp.StatusCode >= 400 {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("New Relic API error (HTTP %d)", resp.StatusCode)}
	}
	return ConnectionResult{Success: true, Message: "New Relic credentials validated — account accessible"}
}

// FetchEvents queries New Relic via the NerdGraph API (GraphQL) for recent log entries.
func (n *newrelicConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	apiKey := ""
	for _, k := range []string{"api_key", "user_key", "token"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			apiKey = v
			break
		}
	}
	accountID, _ := vc.Credentials["account_id"].(string)
	if apiKey == "" || accountID == "" {
		return nil, fmt.Errorf("newrelic: requires api_key and account_id")
	}

	logs, err := n.fetchLogs(apiKey, accountID)
	if err != nil {
		return nil, fmt.Errorf("newrelic: fetch logs: %w", err)
	}
	return n.normalise(accountID, logs), nil
}

// ── NerdGraph types ───────────────────────────────────────────────────────────

type newrelicLogEntry struct {
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (n *newrelicConnector) fetchLogs(apiKey, accountID string) ([]newrelicLogEntry, error) {
	// Use NerdGraph (GraphQL) to run a NRQL log query.
	nrql := fmt.Sprintf("SELECT message, timestamp, level FROM Log WHERE accountId = %s LIMIT 100 SINCE 15 minutes ago", accountID)
	query := fmt.Sprintf(`{"query":"{ actor { account(id: %s) { nrql(query: %q) { results } } } }"}`, accountID, nrql)

	req, err := http.NewRequest("POST", "https://api.newrelic.com/graphql", bytes.NewBufferString(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach New Relic API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed (HTTP 403) — check API key type (must be User key: NRAK-...)")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// Parse GraphQL response structure.
	var gqlResp struct {
		Data struct {
			Actor struct {
				Account struct {
					NRQL struct {
						Results []newrelicLogEntry `json:"results"`
					} `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("parse NerdGraph response: %w", err)
	}
	return gqlResp.Data.Actor.Account.NRQL.Results, nil
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (n *newrelicConnector) normalise(accountID string, logs []newrelicLogEntry) []NormalizedEvent {
	endpoint := "api.newrelic.com/graphql (account " + accountID + ")"
	var out []NormalizedEvent
	for i, l := range logs {
		if l.Message == "" {
			continue
		}
		out = append(out, NormalizedEvent{
			VendorID: "newrelic",
			EventID:  fmt.Sprintf("nr_log_%s_%d", accountID, i),
			Source:   "log.message",
			Text:     l.Message,
			Metadata: map[string]string{"endpoint": endpoint, "level": l.Level},
		})
	}
	return out
}
