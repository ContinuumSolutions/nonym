package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&airtableConnector{
		baseURL: "https://api.airtable.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type airtableConnector struct {
	baseURL string
	client  *http.Client
}

func (a *airtableConnector) Vendor() string { return "airtable" }

// TestConnection verifies the token via the /v0/meta/whoami endpoint.
func (a *airtableConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "api_key", "token")
	if !strings.HasPrefix(token, "pat") {
		return ConnectionResult{Success: false, Message: "Airtable personal access token must start with 'pat'"}
	}
	if err := a.get(token, "/v0/meta/whoami", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Airtable connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Airtable token validated — account accessible"}
}

// FetchEvents lists bases, samples table records, and scans field values for PII.
func (a *airtableConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "api_key", "token")
	if token == "" {
		return nil, fmt.Errorf("airtable: api_key is required")
	}

	bases, err := a.listBases(token)
	if err != nil {
		return nil, fmt.Errorf("airtable: list bases: %w", err)
	}

	var all []NormalizedEvent
	for _, base := range bases {
		tables, err := a.listTables(token, base.ID)
		if err != nil {
			continue
		}
		for _, table := range tables {
			records, err := a.listRecords(token, base.ID, table.ID)
			if err != nil {
				continue
			}
			all = append(all, a.normalise(base, table, records)...)
			if len(all) >= 500 {
				return all, nil
			}
		}
	}
	return all, nil
}

// ── Airtable API types ────────────────────────────────────────────────────────

type airtableBase struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type airtableTable struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type airtableRecord struct {
	ID     string                 `json:"id"`
	Fields map[string]interface{} `json:"fields"`
}

type airtableBasesResponse struct {
	Bases []airtableBase `json:"bases"`
}

type airtableTablesResponse struct {
	Tables []airtableTable `json:"tables"`
}

type airtableRecordsResponse struct {
	Records []airtableRecord `json:"records"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (a *airtableConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", a.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(req)
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

func (a *airtableConnector) listBases(token string) ([]airtableBase, error) {
	var result airtableBasesResponse
	err := a.get(token, "/v0/meta/bases", &result)
	return result.Bases, err
}

func (a *airtableConnector) listTables(token, baseID string) ([]airtableTable, error) {
	var result airtableTablesResponse
	err := a.get(token, "/v0/meta/bases/"+baseID+"/tables", &result)
	return result.Tables, err
}

func (a *airtableConnector) listRecords(token, baseID, tableID string) ([]airtableRecord, error) {
	var result airtableRecordsResponse
	err := a.get(token, fmt.Sprintf("/v0/%s/%s?maxRecords=50", baseID, tableID), &result)
	return result.Records, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (a *airtableConnector) normalise(base airtableBase, table airtableTable, records []airtableRecord) []NormalizedEvent {
	endpoint := fmt.Sprintf("api.airtable.com/v0/%s/%s", base.ID, table.ID)
	var out []NormalizedEvent
	for _, rec := range records {
		for field, val := range rec.Fields {
			s, ok := stringify(val)
			if !ok || s == "" {
				continue
			}
			out = append(out, NormalizedEvent{
				VendorID: "airtable",
				EventID:  rec.ID,
				Source:   fmt.Sprintf("record.%s.%s", table.Name, field),
				Text:     s,
				Metadata: map[string]string{
					"endpoint": endpoint,
					"base":     base.Name,
					"table":    table.Name,
				},
			})
		}
	}
	return out
}
