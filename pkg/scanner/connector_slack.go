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
	Register(&slackConnector{client: &http.Client{Timeout: 10 * time.Second}})
}

type slackConnector struct{ client *http.Client }

func (s *slackConnector) Vendor() string { return "slack" }

// TestConnection validates the bot token format and verifies it against the
// Slack auth.test API endpoint.
func (s *slackConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "bot_token")
	if token == "" {
		return ConnectionResult{Success: false, Message: "Slack requires a bot_token"}
	}
	if !strings.HasPrefix(token, "xoxb-") && !strings.HasPrefix(token, "xoxp-") {
		return ConnectionResult{Success: false, Message: "Slack bot token must start with xoxb- (or xoxp- for user tokens)"}
	}

	resp, err := s.callAPI(token, "auth.test", nil)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Slack connection failed: %v", err)}
	}
	if !resp.OK {
		msg := resp.Error
		if msg == "" {
			msg = "auth.test returned ok=false"
		}
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Slack auth failed: %s", msg)}
	}
	return ConnectionResult{Success: true, Message: "Slack bot token validated — workspace accessible"}
}

// FetchEvents fetches recent messages from joined channels and scans them for PII.
func (s *slackConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "bot_token")
	if token == "" {
		return nil, fmt.Errorf("slack: bot_token is required")
	}

	channels, err := s.listChannels(token)
	if err != nil {
		return nil, fmt.Errorf("slack: list channels: %w", err)
	}

	var all []NormalizedEvent
	for _, ch := range channels {
		msgs, err := s.fetchMessages(token, ch.ID)
		if err != nil {
			continue // skip channels we can't read
		}
		for _, m := range msgs {
			if m.Text == "" {
				continue
			}
			all = append(all, NormalizedEvent{
				VendorID: "slack",
				EventID:  ch.ID + ":" + m.TS,
				Source:   "channel.message",
				Text:     m.Text,
				Metadata: map[string]string{
					"endpoint": "slack.com/api/conversations.history",
					"channel":  ch.Name,
				},
			})
		}
		if len(all) >= 500 {
			break
		}
	}
	return all, nil
}

// ── Slack API types ───────────────────────────────────────────────────────────

type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type slackChannelsResponse struct {
	slackResponse
	Channels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channels"`
}

type slackHistoryResponse struct {
	slackResponse
	Messages []struct {
		Text string `json:"text"`
		TS   string `json:"ts"`
	} `json:"messages"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (s *slackConnector) callAPI(token, method string, out interface{}) (*slackResponse, error) {
	req, err := http.NewRequest("GET", "https://slack.com/api/"+method, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var base slackResponse
	if err := json.Unmarshal(body, &base); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
	}
	return &base, nil
}

func (s *slackConnector) listChannels(token string) ([]struct {
	ID   string
	Name string
}, error) {
	var result slackChannelsResponse
	if _, err := s.callAPI(token, "conversations.list?limit=100&types=public_channel,private_channel", &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("conversations.list: %s", result.Error)
	}
	out := make([]struct{ ID, Name string }, len(result.Channels))
	for i, ch := range result.Channels {
		out[i].ID = ch.ID
		out[i].Name = ch.Name
	}
	return out, nil
}

func (s *slackConnector) fetchMessages(token, channelID string) ([]struct {
	Text string
	TS   string
}, error) {
	var result slackHistoryResponse
	if _, err := s.callAPI(token, "conversations.history?channel="+channelID+"&limit=50", &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("conversations.history: %s", result.Error)
	}
	out := make([]struct{ Text, TS string }, len(result.Messages))
	for i, m := range result.Messages {
		out[i].Text = m.Text
		out[i].TS = m.TS
	}
	return out, nil
}
