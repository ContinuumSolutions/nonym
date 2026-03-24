package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func init() {
	Register(&twilioConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type twilioConnector struct{ client *http.Client }

func (t *twilioConnector) Vendor() string { return "twilio" }

func (t *twilioConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	accountSID, _ := vc.Credentials["account_sid"].(string)
	authToken, _ := vc.Credentials["auth_token"].(string)

	if accountSID == "" || authToken == "" {
		return nil, fmt.Errorf("twilio: requires account_sid and auth_token")
	}

	msgs, err := t.fetchMessages(accountSID, authToken)
	if err != nil {
		return nil, fmt.Errorf("twilio: fetch messages: %w", err)
	}
	return t.normalise(accountSID, msgs), nil
}

// ── Twilio API types ──────────────────────────────────────────────────────────

type twilioMessage struct {
	SID  string `json:"sid"`
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

type twilioMessagesResponse struct {
	Messages []twilioMessage `json:"messages"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (t *twilioConnector) fetchMessages(accountSID, authToken string) ([]twilioMessage, error) {
	url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json?PageSize=100", accountSID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(accountSID, authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Twilio API: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result twilioMessagesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result.Messages, nil
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (t *twilioConnector) normalise(accountSID string, msgs []twilioMessage) []NormalizedEvent {
	endpoint := fmt.Sprintf("api.twilio.com/Accounts/%s/Messages", accountSID)
	var out []NormalizedEvent
	for _, m := range msgs {
		if m.From != "" {
			out = append(out, NormalizedEvent{
				VendorID: "twilio", EventID: m.SID, Source: "message.from",
				Text: m.From, Metadata: map[string]string{"endpoint": endpoint},
			})
		}
		if m.To != "" {
			out = append(out, NormalizedEvent{
				VendorID: "twilio", EventID: m.SID, Source: "message.to",
				Text: m.To, Metadata: map[string]string{"endpoint": endpoint},
			})
		}
		if m.Body != "" {
			out = append(out, NormalizedEvent{
				VendorID: "twilio", EventID: m.SID, Source: "message.body",
				Text: m.Body, Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}
