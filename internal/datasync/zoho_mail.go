package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ZohoMailAdapter pulls unread messages from Zoho Mail using the Zoho Mail API v1.
// OAuth scope required: ZohoMail.messages.READ
type ZohoMailAdapter struct{}

func (a *ZohoMailAdapter) Slug() string { return "zoho-mail" }

func (a *ZohoMailAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Step 1: resolve the user's primary Zoho account ID.
	accountID, err := zohoAccountID(ctx, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("zoho-mail: get account id: %w", err)
	}

	// Step 2: list unread messages (newest first, up to 25).
	url := fmt.Sprintf(
		"https://mail.zoho.com/api/accounts/%s/messages/view?status=unread&count=25&sortorder=false",
		accountID,
	)
	body, err := authGet(ctx, url, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("zoho-mail: list messages: %w", err)
	}

	var resp struct {
		Data []struct {
			MessageID    string `json:"messageId"`
			Subject      string `json:"subject"`
			FromAddress  string `json:"fromAddress"`
			ReceivedTime string `json:"receivedTime"` // milliseconds since epoch as a string
			Summary      string `json:"summary"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("zoho-mail: decode messages: %w", err)
	}

	var signals []RawSignal
	for _, msg := range resp.Data {
		occurred := zohoParseTime(msg.ReceivedTime)
		if occurred.Before(since) {
			continue // already processed
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Communication",
			Title:       msg.Subject,
			Body:        msg.Summary,
			Metadata: map[string]string{
				"from":       msg.FromAddress,
				"message_id": msg.MessageID,
			},
			OccurredAt: occurred,
		})
	}
	return signals, nil
}

// zohoAccountID fetches the user's primary Zoho Mail account ID.
func zohoAccountID(ctx context.Context, token string) (string, error) {
	body, err := authGet(ctx, "https://mail.zoho.com/api/accounts", token)
	if err != nil {
		return "", err
	}
	var resp struct {
		Data []struct {
			AccountID string `json:"accountId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode accounts: %w", err)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("no Zoho Mail accounts found")
	}
	return resp.Data[0].AccountID, nil
}

// zohoParseTime converts Zoho's millisecond-epoch string to time.Time.
func zohoParseTime(ms string) time.Time {
	millis, err := strconv.ParseInt(ms, 10, 64)
	if err != nil || millis == 0 {
		return time.Now()
	}
	return time.Unix(millis/1000, (millis%1000)*int64(time.Millisecond)).UTC()
}
