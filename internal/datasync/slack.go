package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type SlackAdapter struct{}

func (a *SlackAdapter) Slug() string { return "slack" }

func (a *SlackAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Fetch DMs and mentions using conversations.history on all joined channels.
	// Step 1: list joined channels (public + private + DMs).
	channelListURL := "https://slack.com/api/conversations.list?types=public_channel,private_channel,im&limit=200&exclude_archived=true"
	listBody, err := authGet(ctx, channelListURL, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("slack: list channels: %w", err)
	}

	var listResp struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return nil, fmt.Errorf("slack: decode channels: %w", err)
	}
	if !listResp.OK {
		return nil, fmt.Errorf("slack: conversations.list: %s", listResp.Error)
	}

	oldest := strconv.FormatInt(since.Unix(), 10)
	var signals []RawSignal

	for _, ch := range listResp.Channels {
		url := fmt.Sprintf(
			"https://slack.com/api/conversations.history?channel=%s&oldest=%s&limit=20",
			ch.ID, oldest,
		)
		body, err := authGet(ctx, url, creds.OAuthAccessToken)
		if err != nil {
			continue
		}

		var histResp struct {
			OK       bool   `json:"ok"`
			Messages []struct {
				Text string `json:"text"`
				User string `json:"user"`
				Ts   string `json:"ts"` // Unix float as string e.g. "1700000000.123456"
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &histResp); err != nil || !histResp.OK {
			continue
		}

		for _, msg := range histResp.Messages {
			occurred := tsToTime(msg.Ts)
			title := fmt.Sprintf("Message in #%s", ch.Name)
			if ch.Name == "" {
				title = "Direct Message"
			}

			signals = append(signals, RawSignal{
				ServiceSlug: a.Slug(),
				Category:    "Communication",
				Title:       title,
				Body:        msg.Text,
				Metadata: map[string]string{
					"channel": ch.ID,
					"user":    msg.User,
					"ts":      msg.Ts,
				},
				OccurredAt: occurred,
			})
		}
	}
	return signals, nil
}

// tsToTime converts a Slack timestamp string ("1700000000.123456") to time.Time.
func tsToTime(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	// Split on dot; take whole seconds.
	for i, ch := range ts {
		if ch == '.' {
			secs, err := strconv.ParseInt(ts[:i], 10, 64)
			if err != nil {
				return time.Now()
			}
			return time.Unix(secs, 0)
		}
	}
	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(secs, 0)
}
