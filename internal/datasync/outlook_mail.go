package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type OutlookMailAdapter struct{}

func (a *OutlookMailAdapter) Slug() string { return "outlook-mail" }

func (a *OutlookMailAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Microsoft Graph API: list unread messages received after `since`
	filter := fmt.Sprintf("isRead eq false and receivedDateTime ge %s", since.UTC().Format(time.RFC3339))
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages?$filter=%s&$select=subject,from,receivedDateTime,bodyPreview&$top=25&$orderby=receivedDateTime desc",
		filter,
	)

	body, err := authGet(ctx, url, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("outlook-mail: list messages: %w", err)
	}

	var resp struct {
		Value []struct {
			Subject          string `json:"subject"`
			BodyPreview      string `json:"bodyPreview"`
			ReceivedDateTime string `json:"receivedDateTime"`
			From             struct {
				EmailAddress struct {
					Name    string `json:"name"`
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"from"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("outlook-mail: decode: %w", err)
	}

	var signals []RawSignal
	for _, msg := range resp.Value {
		occurred, _ := time.Parse(time.RFC3339, msg.ReceivedDateTime)
		if occurred.IsZero() {
			occurred = time.Now()
		}

		from := msg.From.EmailAddress.Name
		if from == "" {
			from = msg.From.EmailAddress.Address
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Communication",
			Title:       msg.Subject,
			Body:        msg.BodyPreview,
			Metadata: map[string]string{
				"from": from,
			},
			OccurredAt: occurred,
		})
	}
	return signals, nil
}
