package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&postmarkConnector{
		baseURL: "https://api.postmarkapp.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type postmarkConnector struct {
	baseURL string
	client  *http.Client
}

func (p *postmarkConnector) Vendor() string { return "postmark" }

// TestConnection verifies the server token via the /server endpoint.
func (p *postmarkConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "server_token")
	if len(token) < 8 {
		return ConnectionResult{Success: false, Message: "Postmark server token is missing or too short"}
	}
	if err := p.get(token, "/server", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Postmark connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Postmark server token validated — server accessible"}
}

// FetchEvents retrieves recent outbound messages and scans recipient / subject / body fields.
func (p *postmarkConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "server_token")
	if token == "" {
		return nil, fmt.Errorf("postmark: server_token is required")
	}
	messages, err := p.fetchMessages(token)
	if err != nil {
		return nil, fmt.Errorf("postmark: fetch messages: %w", err)
	}
	return p.normalise(messages), nil
}

// ── Postmark API types ────────────────────────────────────────────────────────

type postmarkMessage struct {
	MessageID string `json:"MessageID"`
	From      string `json:"From"`
	To        string `json:"To"`
	Cc        string `json:"Cc"`
	Subject   string `json:"Subject"`
	TextBody  string `json:"TextBody"`
}

type postmarkMessagesResponse struct {
	Messages    []postmarkMessage `json:"Messages"`
	TotalCount  int               `json:"TotalCount"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (p *postmarkConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", p.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Postmark-Server-Token", token)
	req.Header.Set("Accept", "application/json")

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

func (p *postmarkConnector) fetchMessages(token string) ([]postmarkMessage, error) {
	var result postmarkMessagesResponse
	err := p.get(token, "/messages/outbound?count=100&offset=0", &result)
	return result.Messages, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (p *postmarkConnector) normalise(messages []postmarkMessage) []NormalizedEvent {
	endpoint := "api.postmarkapp.com/messages/outbound"
	var out []NormalizedEvent
	for _, m := range messages {
		fields := []struct{ src, val string }{
			{"message.from", m.From},
			{"message.to", m.To},
			{"message.cc", m.Cc},
			{"message.subject", m.Subject},
			{"message.text_body", m.TextBody},
		}
		for _, f := range fields {
			if f.val == "" {
				continue
			}
			out = append(out, NormalizedEvent{
				VendorID: "postmark",
				EventID:  m.MessageID,
				Source:   f.src,
				Text:     f.val,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}
