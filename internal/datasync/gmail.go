package datasync

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type GmailAdapter struct{}

func (a *GmailAdapter) Slug() string { return "gmail" }

func (a *GmailAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// List unread message IDs received after `since`.
	query := fmt.Sprintf("is:unread after:%d", since.Unix())
	listURL := fmt.Sprintf(
		"https://gmail.googleapis.com/gmail/v1/users/me/messages?q=%s&maxResults=25",
		url.QueryEscape(query),
	)

	listBody, err := authGet(ctx, listURL, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("gmail: list messages: %w", err)
	}

	var listResp struct {
		Messages []struct{ ID string `json:"id"` } `json:"messages"`
	}
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return nil, fmt.Errorf("gmail: decode list: %w", err)
	}

	var signals []RawSignal
	for _, msg := range listResp.Messages {
		url := fmt.Sprintf(
			"https://gmail.googleapis.com/gmail/v1/users/me/messages/%s?format=metadata&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Date",
			msg.ID,
		)
		body, err := authGet(ctx, url, creds.OAuthAccessToken)
		if err != nil {
			continue // skip single failed message
		}

		var m struct {
			Snippet string `json:"snippet"`
			Payload struct {
				Headers []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"headers"`
			} `json:"payload"`
			InternalDate string `json:"internalDate"` // millis since epoch
		}
		if err := json.Unmarshal(body, &m); err != nil {
			continue
		}

		headers := make(map[string]string)
		for _, h := range m.Payload.Headers {
			headers[strings.ToLower(h.Name)] = h.Value
		}

		occurred, _ := time.Parse(time.RFC1123Z, headers["date"])
		if occurred.IsZero() {
			occurred = time.Now()
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Communication",
			Title:       headers["subject"],
			Body:        base64.StdEncoding.EncodeToString([]byte(m.Snippet)),
			Metadata: map[string]string{
				"from":       headers["from"],
				"message_id": msg.ID,
			},
			OccurredAt: occurred,
		})
	}
	return signals, nil
}
