package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"io"
	"strconv"
	"time"
)

type StripeAdapter struct{}

func (a *StripeAdapter) Slug() string { return "stripe" }

func (a *StripeAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// Stripe uses secret key as Basic auth (username=key, password empty).
	url := fmt.Sprintf(
		"https://api.stripe.com/v1/charges?created[gte]=%d&limit=50",
		since.Unix(),
	)

	body, err := stripeGet(ctx, url, creds.APIKey)
	if err != nil {
		return nil, fmt.Errorf("stripe: list charges: %w", err)
	}

	var resp struct {
		Data []struct {
			ID          string `json:"id"`
			Amount      int64  `json:"amount"` // cents
			Currency    string `json:"currency"`
			Description string `json:"description"`
			Status      string `json:"status"`
			Created     int64  `json:"created"`
			Customer    string `json:"customer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("stripe: decode: %w", err)
	}

	var signals []RawSignal
	for _, charge := range resp.Data {
		occurred := time.Unix(charge.Created, 0)
		amountStr := strconv.FormatFloat(float64(charge.Amount)/100.0, 'f', 2, 64)

		title := charge.Description
		if title == "" {
			title = fmt.Sprintf("Charge %s", charge.ID)
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Billing",
			Title:       title,
			Body:        fmt.Sprintf("%s %s — %s", amountStr, charge.Currency, charge.Status),
			Metadata: map[string]string{
				"charge_id": charge.ID,
				"amount":    amountStr,
				"currency":  charge.Currency,
				"status":    charge.Status,
				"customer":  charge.Customer,
			},
			OccurredAt: occurred,
		})
	}
	return signals, nil
}

// stripeGet performs a GET with Stripe secret-key Basic auth.
func stripeGet(ctx context.Context, url, secretKey string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(secretKey, "")
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
