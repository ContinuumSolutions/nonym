package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&zendeskConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type zendeskConnector struct{ client *http.Client }

func (z *zendeskConnector) Vendor() string { return "zendesk" }

// TestConnection verifies credentials by calling the current-user endpoint.
func (z *zendeskConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	subdomain, _ := vc.Credentials["subdomain"].(string)
	email, _ := vc.Credentials["email"].(string)
	apiToken, _ := vc.Credentials["api_token"].(string)
	if subdomain == "" || email == "" || apiToken == "" {
		return ConnectionResult{Success: false, Message: "Zendesk requires subdomain, email, and api_token"}
	}
	if err := z.get(subdomain, email, apiToken, "/api/v2/users/me.json", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Zendesk connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Zendesk credentials validated — account accessible"}
}

func (z *zendeskConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	subdomain, _ := vc.Credentials["subdomain"].(string)
	email, _ := vc.Credentials["email"].(string)
	apiToken, _ := vc.Credentials["api_token"].(string)

	if subdomain == "" || email == "" || apiToken == "" {
		return nil, fmt.Errorf("zendesk: requires subdomain, email, and api_token")
	}

	tickets, err := z.fetchTickets(subdomain, email, apiToken)
	if err != nil {
		return nil, fmt.Errorf("zendesk: fetch tickets: %w", err)
	}
	users, err := z.fetchUsers(subdomain, email, apiToken)
	if err != nil {
		// non-fatal
		users = nil
	}

	var all []NormalizedEvent
	all = append(all, z.normaliseTickets(subdomain, tickets)...)
	all = append(all, z.normaliseUsers(subdomain, users)...)
	return all, nil
}

// ── Zendesk API types ─────────────────────────────────────────────────────────

type zendeskTicket struct {
	ID          int64  `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	RequesterID int64  `json:"requester_id"`
}

type zendeskTicketsResponse struct {
	Tickets []zendeskTicket `json:"tickets"`
}

type zendeskUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type zendeskUsersResponse struct {
	Users []zendeskUser `json:"users"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (z *zendeskConnector) get(subdomain, email, token, path string, out interface{}) error {
	url := fmt.Sprintf("https://%s.zendesk.com%s", subdomain, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(email+"/token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — verify email/token combination", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (z *zendeskConnector) fetchTickets(subdomain, email, token string) ([]zendeskTicket, error) {
	var result zendeskTicketsResponse
	err := z.get(subdomain, email, token, "/api/v2/tickets?sort_by=created_at&sort_order=desc&per_page=100", &result)
	return result.Tickets, err
}

func (z *zendeskConnector) fetchUsers(subdomain, email, token string) ([]zendeskUser, error) {
	var result zendeskUsersResponse
	err := z.get(subdomain, email, token, "/api/v2/users?role=end-user&per_page=50", &result)
	return result.Users, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (z *zendeskConnector) normaliseTickets(subdomain string, tickets []zendeskTicket) []NormalizedEvent {
	endpoint := subdomain + ".zendesk.com/api/v2/tickets"
	var out []NormalizedEvent
	for _, t := range tickets {
		id := fmt.Sprintf("%d", t.ID)
		for _, pair := range [][2]string{
			{"ticket.subject", t.Subject},
			{"ticket.description", t.Description},
		} {
			if pair[1] != "" {
				out = append(out, NormalizedEvent{
					VendorID: "zendesk", EventID: id, Source: pair[0],
					Text: pair[1], Metadata: map[string]string{"endpoint": endpoint},
				})
			}
		}
	}
	return out
}

func (z *zendeskConnector) normaliseUsers(subdomain string, users []zendeskUser) []NormalizedEvent {
	endpoint := subdomain + ".zendesk.com/api/v2/users"
	var out []NormalizedEvent
	emit := func(id, field, text string) {
		if text != "" {
			out = append(out, NormalizedEvent{
				VendorID: "zendesk", EventID: id, Source: field,
				Text: text, Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	for _, u := range users {
		id := fmt.Sprintf("%d", u.ID)
		emit(id, "user.name", u.Name)
		emit(id, "user.email", u.Email)
		emit(id, "user.phone", u.Phone)
	}
	return out
}
