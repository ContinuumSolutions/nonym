package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&hubspotConnector{
		baseURL: "https://api.hubapi.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type hubspotConnector struct {
	baseURL string
	client  *http.Client
}

func (h *hubspotConnector) Vendor() string { return "hubspot" }

func (h *hubspotConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := ""
	for _, k := range []string{"access_token", "token", "api_key"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			token = v
			break
		}
	}
	if token == "" {
		return nil, fmt.Errorf("hubspot: no access_token in credentials")
	}

	contacts, err := h.fetchContacts(token)
	if err != nil {
		return nil, fmt.Errorf("hubspot: fetch contacts: %w", err)
	}
	return h.normalise(contacts), nil
}

// ── HubSpot API types ─────────────────────────────────────────────────────────

type hubspotContact struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
}

type hubspotContactsResponse struct {
	Results []hubspotContact `json:"results"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (h *hubspotConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", h.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — check private app token scopes", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (h *hubspotConnector) fetchContacts(token string) ([]hubspotContact, error) {
	properties := "email,firstname,lastname,phone,address,city,country"
	var result hubspotContactsResponse
	err := h.get(token, "/crm/v3/objects/contacts?limit=100&properties="+properties, &result)
	return result.Results, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (h *hubspotConnector) normalise(contacts []hubspotContact) []NormalizedEvent {
	var out []NormalizedEvent
	endpoint := "api.hubapi.com/crm/v3/objects/contacts"

	piiFields := []string{"email", "firstname", "lastname", "phone", "address", "city"}

	for _, c := range contacts {
		for _, field := range piiFields {
			if v, ok := c.Properties[field]; ok {
				if s, ok := stringify(v); ok {
					out = append(out, NormalizedEvent{
						VendorID: "hubspot",
						EventID:  c.ID,
						Source:   "contact.properties." + field,
						Text:     s,
						Metadata: map[string]string{"endpoint": endpoint},
					})
				}
			}
		}
	}
	return out
}
