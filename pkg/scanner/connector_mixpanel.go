package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&mixpanelConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type mixpanelConnector struct{ client *http.Client }

func (m *mixpanelConnector) Vendor() string { return "mixpanel" }

// TestConnection verifies credentials by calling the engage API with page_size=1.
func (m *mixpanelConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	user := credStr(vc, "service_account_user", "service_account")
	secret := credStr(vc, "service_account_secret", "secret")
	projectID, _ := vc.Credentials["project_id"].(string)
	if user == "" || secret == "" || projectID == "" {
		return ConnectionResult{Success: false, Message: "Mixpanel requires service_account, secret, and project_id"}
	}
	url := fmt.Sprintf("https://mixpanel.com/api/2.0/engage?project_id=%s&page_size=1", projectID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Failed to build request: %v", err)}
	}
	req.SetBasicAuth(user, secret)
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Could not reach Mixpanel API: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return ConnectionResult{Success: false, Message: "Invalid Mixpanel credentials — check service account and secret"}
	}
	if resp.StatusCode >= 400 {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Mixpanel API error (HTTP %d)", resp.StatusCode)}
	}
	return ConnectionResult{Success: true, Message: "Mixpanel credentials validated — project accessible"}
}

func (m *mixpanelConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	user := ""
	for _, k := range []string{"service_account_user", "service_account"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			user = v
			break
		}
	}
	secret := ""
	for _, k := range []string{"service_account_secret", "secret"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			secret = v
			break
		}
	}
	projectID, _ := vc.Credentials["project_id"].(string)

	if user == "" || secret == "" || projectID == "" {
		return nil, fmt.Errorf("mixpanel: requires service_account_user, service_account_secret, and project_id")
	}

	profiles, err := m.fetchProfiles(user, secret, projectID)
	if err != nil {
		return nil, fmt.Errorf("mixpanel: fetch profiles: %w", err)
	}
	return m.normalise(projectID, profiles), nil
}

// ── Mixpanel API types ────────────────────────────────────────────────────────

type mixpanelProfile struct {
	DistinctID string                 `json:"$distinct_id"`
	Properties map[string]interface{} `json:"$properties"`
}

type mixpanelProfilesResponse struct {
	Results []mixpanelProfile `json:"results"`
	Status  string            `json:"status"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (m *mixpanelConnector) fetchProfiles(user, secret, projectID string) ([]mixpanelProfile, error) {
	url := fmt.Sprintf("https://mixpanel.com/api/2.0/engage?project_id=%s", projectID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, secret)
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Mixpanel API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("authentication failed — check service account credentials and project ID")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result mixpanelProfilesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result.Results, nil
}

// ── Normalisation ─────────────────────────────────────────────────────────────

var mixpanelPIIProps = map[string]bool{
	"$email": true, "$name": true, "$phone": true, "$first_name": true,
	"$last_name": true, "$city": true, "$country_code": true,
	"email": true, "name": true, "phone": true,
}

func (m *mixpanelConnector) normalise(projectID string, profiles []mixpanelProfile) []NormalizedEvent {
	endpoint := "mixpanel.com/api/2.0/engage?project_id=" + projectID
	var out []NormalizedEvent

	for _, p := range profiles {
		// The distinct_id itself may be an email address.
		out = append(out, NormalizedEvent{
			VendorID: "mixpanel", EventID: p.DistinctID,
			Source: "profile.$distinct_id", Text: p.DistinctID,
			Metadata: map[string]string{"endpoint": endpoint},
		})

		for k, v := range p.Properties {
			if !mixpanelPIIProps[k] {
				continue
			}
			if s, ok := stringify(v); ok {
				out = append(out, NormalizedEvent{
					VendorID: "mixpanel", EventID: p.DistinctID,
					Source: "profile.properties." + k, Text: s,
					Metadata: map[string]string{"endpoint": endpoint},
				})
			}
		}
	}
	return out
}
