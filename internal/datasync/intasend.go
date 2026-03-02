package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IntaSendAdapter pulls wallet transactions from the IntaSend API.
// Auth: Bearer token (APIKey field).
// Docs: https://developers.intasend.com/reference/api_v1_transactions_list
type IntaSendAdapter struct{}

func (a *IntaSendAdapter) Slug() string { return "intasend" }

func (a *IntaSendAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// IntaSend paginates via ?page=N. We walk pages until next is null or
	// all results predate `since`.
	var signals []RawSignal
	page := 1

	for {
		url := fmt.Sprintf("https://api.intasend.com/api/v1/transactions/?page=%d", page)
		body, err := authGet(ctx, url, creds.APIKey)
		if err != nil {
			return nil, fmt.Errorf("intasend: transactions page %d: %w", page, err)
		}

		var resp struct {
			Count   int    `json:"count"`
			Next    string `json:"next"`
			Results []struct {
				ID        string  `json:"id"`
				Value     float64 `json:"value"`    // numeric e.g. 1500.00
				Currency  string  `json:"currency"`
				TransType string  `json:"trans_type"`
				Status    string  `json:"status"`
				Narrative string  `json:"narrative"`
				Account   string  `json:"account"`   // counter-party identifier
				CreatedAt string  `json:"created_at"` // ISO-8601
				UpdatedAt string  `json:"updated_at"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("intasend: decode page %d: %w", page, err)
		}

		done := false
		for _, tx := range resp.Results {
			occurred, err := time.Parse(time.RFC3339, tx.CreatedAt)
			if err != nil {
				// Try without timezone suffix
				occurred, _ = time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(tx.CreatedAt, "Z"))
			}

			// Stop walking pages once we reach transactions older than `since`.
			if !occurred.IsZero() && occurred.Before(since) {
				done = true
				break
			}

			title := tx.Narrative
			if title == "" {
				title = fmt.Sprintf("%s transaction %s", tx.TransType, tx.ID)
			}

			valueStr := fmt.Sprintf("%.2f", tx.Value)
			signals = append(signals, RawSignal{
				ServiceSlug: a.Slug(),
				Category:    "Finance",
				Title:       title,
				Body: fmt.Sprintf(
					"%s %s — %s (%s)",
					valueStr, tx.Currency, tx.Status, tx.TransType,
				),
				Metadata: map[string]string{
					"transaction_id": tx.ID,
					"amount":         valueStr,
					"currency":       tx.Currency,
					"trans_type":     tx.TransType,
					"status":         tx.Status,
					"account":        tx.Account,
				},
				OccurredAt: occurred,
			})
		}

		if done || resp.Next == "" {
			break
		}
		page++
	}

	return signals, nil
}
