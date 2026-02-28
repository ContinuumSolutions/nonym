package datasync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type PlaidAdapter struct{}

func (a *PlaidAdapter) Slug() string { return "plaid" }

func (a *PlaidAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Plaid uses API key auth via JSON body (not Bearer token).
	// Endpoint: /transactions/get
	reqBody := map[string]interface{}{
		"client_id":    "", // set via creds.APIKey (format: "client_id:secret")
		"secret":       "",
		"access_token": creds.OAuthAccessToken,
		"start_date":   since.Format("2006-01-02"),
		"end_date":     time.Now().Format("2006-01-02"),
		"options": map[string]interface{}{
			"count":  100,
			"offset": 0,
		},
	}
	// APIKey stored as "client_id|secret"
	parsed := splitPipe(creds.APIKey)
	reqBody["client_id"] = parsed[0]
	if len(parsed) > 1 {
		reqBody["secret"] = parsed[1]
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("plaid: marshal request: %w", err)
	}

	url := "https://production.plaid.com/transactions/get"

	body, err := postJSON(ctx, url, payload)
	if err != nil {
		return nil, fmt.Errorf("plaid: transactions/get: %w", err)
	}

	var resp struct {
		Transactions []struct {
			TransactionID string  `json:"transaction_id"`
			Name          string  `json:"name"`
			Amount        float64 `json:"amount"`
			Date          string  `json:"date"`
			Category      []string `json:"category"`
			MerchantName  string  `json:"merchant_name"`
			PaymentChannel string `json:"payment_channel"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("plaid: decode: %w", err)
	}

	var signals []RawSignal
	for _, tx := range resp.Transactions {
		occurred, _ := time.Parse("2006-01-02", tx.Date)

		category := ""
		if len(tx.Category) > 0 {
			category = tx.Category[len(tx.Category)-1]
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Finance",
			Title:       tx.Name,
			Body:        fmt.Sprintf("%.2f via %s", tx.Amount, tx.PaymentChannel),
			Metadata: map[string]string{
				"transaction_id": tx.TransactionID,
				"amount":         fmt.Sprintf("%.2f", tx.Amount),
				"category":       category,
				"merchant":       tx.MerchantName,
				"channel":        tx.PaymentChannel,
			},
			OccurredAt: occurred,
		})
	}
	return signals, nil
}

// splitPipe splits "a|b" into ["a","b"]. Returns [""] on empty input.
func splitPipe(s string) []string {
	for i, ch := range s {
		if ch == '|' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// postJSON sends a JSON POST and returns the response body.
func postJSON(ctx context.Context, url string, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
