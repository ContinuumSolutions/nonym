package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&zohoDeskConnector{
		baseURL: "https://desk.zoho.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type zohoDeskConnector struct {
	baseURL string
	client  *http.Client
}

func (z *zohoDeskConnector) Vendor() string { return "zoho-desk" }

// TestConnection verifies by fetching the organization details.
func (z *zohoDeskConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "access_token")
	orgID := credStr(vc, "org_id")
	if token == "" || orgID == "" {
		return ConnectionResult{Success: false, Message: "Zoho Desk requires access_token and org_id"}
	}
	if err := z.get(token, orgID, "/api/v1/organizations", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Zoho Desk connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Zoho Desk token validated — organization accessible"}
}

// FetchEvents retrieves recent tickets and scans subject and description for PII.
func (z *zohoDeskConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "access_token")
	orgID := credStr(vc, "org_id")
	if token == "" || orgID == "" {
		return nil, fmt.Errorf("zoho-desk: access_token and org_id are required")
	}
	tickets, err := z.fetchTickets(token, orgID)
	if err != nil {
		return nil, fmt.Errorf("zoho-desk: fetch tickets: %w", err)
	}
	return z.normalise(tickets), nil
}

// ── Zoho Desk API types ───────────────────────────────────────────────────────

type zohoDeskTicket struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

type zohoDeskTicketsResponse struct {
	Data []zohoDeskTicket `json:"data"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (z *zohoDeskConnector) get(token, orgID, path string, out interface{}) error {
	req, err := http.NewRequest("GET", z.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Zoho-oauthtoken "+token)
	req.Header.Set("orgId", orgID)
	req.Header.Set("Accept", "application/json")

	resp, err := z.client.Do(req)
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

func (z *zohoDeskConnector) fetchTickets(token, orgID string) ([]zohoDeskTicket, error) {
	var result zohoDeskTicketsResponse
	err := z.get(token, orgID, "/api/v1/tickets?limit=100&sortBy=-createdTime", &result)
	return result.Data, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (z *zohoDeskConnector) normalise(tickets []zohoDeskTicket) []NormalizedEvent {
	endpoint := "desk.zoho.com/api/v1/tickets"
	var out []NormalizedEvent
	for _, t := range tickets {
		fields := []struct{ src, val string }{
			{"ticket.subject", t.Subject},
			{"ticket.description", t.Description},
			{"ticket.email", t.Email},
			{"ticket.phone", t.Phone},
		}
		for _, f := range fields {
			if f.val == "" {
				continue
			}
			out = append(out, NormalizedEvent{
				VendorID: "zoho-desk",
				EventID:  t.ID,
				Source:   f.src,
				Text:     f.val,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}
