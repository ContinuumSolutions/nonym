package execution

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/egokernel/ek1/internal/datasync"
)

// StripeExecutor issues a refund for a charge via the Stripe API.
// Uses Basic auth with the secret key (same credential stored for the stripe integration).
// Step 12 — Micro-Wallet (Stripe side).
type StripeExecutor struct{}

func (e *StripeExecutor) Slug() string { return "stripe" }

func (e *StripeExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	if action.ResourceID == "" {
		return fmt.Errorf("stripe: refund: missing charge_id")
	}

	form := url.Values{}
	form.Set("charge", action.ResourceID)
	form.Set("reason", "requested_by_customer")

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://api.stripe.com/v1/refunds",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return err
	}
	req.SetBasicAuth(creds.APIKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("stripe: refund %s: %w", action.ResourceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stripe: refund %s: HTTP %d: %s", action.ResourceID, resp.StatusCode, body)
	}
	return nil
}
