package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ZohoMailAdapter pulls unread messages from Zoho Mail using the Zoho Mail API v1.
// OAuth scopes required: ZohoMail.accounts.READ, ZohoMail.messages.READ
type ZohoMailAdapter struct{}

func (a *ZohoMailAdapter) Slug() string { return "zoho-mail" }

func (a *ZohoMailAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Resolve the regional Zoho Mail API base URL.
	// Priority: explicit api_endpoint stored at OAuth callback time (new connections)
	// → derived from token URL override (existing connections without re-auth)
	// → global default.
	apiBase := zohoMailAPIBase(creds)

	// Step 1: resolve the user's primary Zoho account ID.
	accountID, err := zohoAccountID(ctx, creds.OAuthAccessToken, apiBase)
	if err != nil {
		return nil, fmt.Errorf("zoho-mail: get account id: %w", err)
	}

	// Step 2: list recent messages (newest first, up to 50).
	// Zoho uses `limit` (not `count`) and sortorder=true for descending.
	// No status filter — read/unread both surface as signals; the `since` window
	// in the calling engine handles deduplication across sync cycles.
	url := fmt.Sprintf(
		"%s/api/accounts/%s/messages/view?limit=50&sortorder=true",
		apiBase, accountID,
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

// zohoMailAPIBase returns the correct regional Zoho Mail API base URL.
// Priority:
//  1. Explicit api_endpoint stored at OAuth callback time.
//  2. Derived from the token URL override (for connections made before api_endpoint was stored).
//  3. Global default "https://mail.zoho.com".
func zohoMailAPIBase(creds Credentials) string {
	if creds.APIEndpoint != "" {
		return creds.APIEndpoint
	}
	if creds.TokenURLOverride != "" {
		// e.g. "https://accounts.zoho.eu/oauth/v2/token" → "https://mail.zoho.eu"
		if u, err := url.Parse(creds.TokenURLOverride); err == nil {
			if mailHost := strings.Replace(u.Hostname(), "accounts.", "mail.", 1); mailHost != u.Hostname() {
				return "https://" + mailHost
			}
		}
	}
	return "https://mail.zoho.com"
}

// zohoAccountID fetches the user's primary Zoho Mail account ID.
func zohoAccountID(ctx context.Context, token, apiBase string) (string, error) {
	body, err := authGet(ctx, apiBase+"/api/accounts", token)
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
