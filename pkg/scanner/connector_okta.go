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
	Register(&oktaConnector{
		client: &http.Client{Timeout: 30 * time.Second},
	})
}

type oktaConnector struct{ client *http.Client }

func (o *oktaConnector) Vendor() string { return "okta" }

// DetectRegion derives the region from the org URL.
func (o *oktaConnector) DetectRegion(vc *VendorConnection) string {
	orgURL := strings.ToLower(credStr(vc, "org_url"))
	switch {
	case strings.Contains(orgURL, ".okta-emea.com") || strings.Contains(orgURL, ".eu.okta.com"):
		return "EU"
	case strings.Contains(orgURL, ".oktapreview.com"):
		return "" // sandbox — no fixed region
	default:
		return "US"
	}
}

// TestConnection verifies by fetching a single user from the directory.
func (o *oktaConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	orgURL, apiToken := credStr(vc, "org_url"), credStr(vc, "api_token")
	if orgURL == "" || apiToken == "" {
		return ConnectionResult{Success: false, Message: "Okta requires org_url and api_token"}
	}
	orgURL = strings.TrimRight(orgURL, "/")
	if err := o.get(orgURL, apiToken, "/api/v1/users?limit=1", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Okta connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Okta token validated — directory accessible"}
}

// FetchEvents scans Okta users and recent system log entries for PII.
func (o *oktaConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	orgURL := strings.TrimRight(credStr(vc, "org_url"), "/")
	apiToken := credStr(vc, "api_token")
	if orgURL == "" || apiToken == "" {
		return nil, fmt.Errorf("okta: org_url and api_token are required")
	}

	users, err := o.fetchUsers(orgURL, apiToken)
	if err != nil {
		return nil, fmt.Errorf("okta: fetch users: %w", err)
	}

	logs, _ := o.fetchLogs(orgURL, apiToken) // non-fatal

	var all []NormalizedEvent
	all = append(all, o.normaliseUsers(orgURL, users)...)
	all = append(all, o.normaliseLogs(orgURL, logs)...)
	return all, nil
}

// ── Okta API types ────────────────────────────────────────────────────────────

type oktaUser struct {
	ID      string `json:"id"`
	Profile struct {
		Login     string `json:"login"`
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		MobilePhone string `json:"mobilePhone"`
	} `json:"profile"`
}

type oktaLogEvent struct {
	UUID        string `json:"uuid"`
	DisplayMessage string `json:"displayMessage"`
	Actor struct {
		DisplayName string `json:"displayName"`
		AlternateID string `json:"alternateId"`
	} `json:"actor"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (o *oktaConnector) get(orgURL, token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", orgURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "SSWS "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
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

func (o *oktaConnector) fetchUsers(orgURL, token string) ([]oktaUser, error) {
	var users []oktaUser
	err := o.get(orgURL, token, "/api/v1/users?limit=100&filter=status+eq+%22ACTIVE%22", &users)
	return users, err
}

func (o *oktaConnector) fetchLogs(orgURL, token string) ([]oktaLogEvent, error) {
	var logs []oktaLogEvent
	err := o.get(orgURL, token, "/api/v1/logs?limit=100&sortOrder=DESCENDING", &logs)
	return logs, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (o *oktaConnector) normaliseUsers(orgURL string, users []oktaUser) []NormalizedEvent {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(orgURL, "https://"), "http://") + "/api/v1/users"
	var out []NormalizedEvent
	for _, u := range users {
		fields := []struct{ src, val string }{
			{"user.profile.login", u.Profile.Login},
			{"user.profile.email", u.Profile.Email},
			{"user.profile.first_name", u.Profile.FirstName},
			{"user.profile.last_name", u.Profile.LastName},
			{"user.profile.mobile_phone", u.Profile.MobilePhone},
		}
		for _, f := range fields {
			if f.val == "" {
				continue
			}
			out = append(out, NormalizedEvent{
				VendorID: "okta", EventID: u.ID, Source: f.src,
				Text:     f.val,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}

func (o *oktaConnector) normaliseLogs(orgURL string, logs []oktaLogEvent) []NormalizedEvent {
	endpoint := strings.TrimPrefix(strings.TrimPrefix(orgURL, "https://"), "http://") + "/api/v1/logs"
	var out []NormalizedEvent
	for _, l := range logs {
		for _, text := range []string{l.Actor.DisplayName, l.Actor.AlternateID} {
			if text == "" {
				continue
			}
			out = append(out, NormalizedEvent{
				VendorID: "okta", EventID: l.UUID, Source: "log.actor",
				Text:     text,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}
