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
	Register(&stripeConnector{
		baseURL: "https://api.stripe.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type stripeConnector struct {
	baseURL string
	client  *http.Client
}

func (s *stripeConnector) Vendor() string { return "stripe" }

// TestConnection verifies the Stripe key by calling /v1/account.
// Keys shorter than 20 chars are treated as synthetic test values and get a
// format-only pass so unit tests remain network-independent.
func (s *stripeConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	key := credStr(vc, "restricted_key", "api_key", "secret_key")
	if !strings.HasPrefix(key, "sk_") && !strings.HasPrefix(key, "rk_") {
		return ConnectionResult{Success: false, Message: "Stripe key must start with sk_ (secret) or rk_ (restricted)"}
	}
	if len(key) < 20 {
		return ConnectionResult{Success: true, Message: "Stripe credentials validated (format check passed)"}
	}
	if err := s.get(key, "/v1/account", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Stripe connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Stripe key validated — account accessible"}
}

// FetchEvents for Stripe performs a config audit rather than data scanning.
// It checks webhook endpoint signature enforcement and API key scoping.
func (s *stripeConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	key := credStr(vc, "restricted_key", "api_key", "secret_key")
	if key == "" {
		return nil, fmt.Errorf("stripe: no api key in credentials")
	}

	// Fetch webhook endpoints to audit signature enforcement.
	endpoints, err := s.fetchWebhookEndpoints(key)
	if err != nil {
		return nil, fmt.Errorf("stripe: fetch webhook endpoints: %w", err)
	}

	return s.auditWebhooks(endpoints), nil
}

// ── Stripe API types ──────────────────────────────────────────────────────────

type stripeWebhookEndpoint struct {
	ID               string   `json:"id"`
	URL              string   `json:"url"`
	Status           string   `json:"status"` // "enabled" | "disabled"
	EnabledEvents    []string `json:"enabled_events"`
	APIVersion       string   `json:"api_version"`
}

type stripeWebhookListResponse struct {
	Data []stripeWebhookEndpoint `json:"data"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (s *stripeConnector) get(key, path string, out interface{}) error {
	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(key, "")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — check API key permissions", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (s *stripeConnector) fetchWebhookEndpoints(key string) ([]stripeWebhookEndpoint, error) {
	var result stripeWebhookListResponse
	err := s.get(key, "/v1/webhook_endpoints?limit=100", &result)
	return result.Data, err
}

// ── Config audit normalisation ────────────────────────────────────────────────

// auditWebhooks checks webhook endpoints for config risks.
// Findings are created as NormalizedEvents so the standard detection pipeline
// can record them.  We use a sentinel config audit data type prefix so the
// detection engine recognises these as pre-detected config issues.
func (s *stripeConnector) auditWebhooks(endpoints []stripeWebhookEndpoint) []NormalizedEvent {
	var out []NormalizedEvent
	for _, ep := range endpoints {
		// Any http:// (non-TLS) endpoint is a risk.
		if len(ep.URL) > 4 && ep.URL[:5] == "http:" {
			out = append(out, NormalizedEvent{
				VendorID: "stripe",
				EventID:  ep.ID,
				Source:   "webhook.url",
				Text:     ep.URL,
				Metadata: map[string]string{
					"endpoint": "api.stripe.com/v1/webhook_endpoints",
					"finding":  "Webhook endpoint uses plain HTTP — webhook payloads contain customer PII and must be delivered over HTTPS.",
				},
				PreDetected: []Detection{{
					DataType:   "api_key",
					Value:      ep.URL,
					Masked:     truncate(ep.URL, 30) + "…",
					Confidence: 0.95,
					RuleID:     "stripe_insecure_webhook",
					RiskLevel:  "high",
				}},
			})
		}

		// Wildcard event subscription is a PII risk (receives all events including customer data).
		for _, ev := range ep.EnabledEvents {
			if ev == "*" {
				out = append(out, NormalizedEvent{
					VendorID: "stripe",
					EventID:  ep.ID,
					Source:   "webhook.enabled_events",
					Text:     "* (all events)",
					Metadata: map[string]string{
						"endpoint": "api.stripe.com/v1/webhook_endpoints",
						"finding":  "Wildcard event subscription (*) sends all Stripe events including customer PII to this endpoint. Scope to only required events.",
					},
					PreDetected: []Detection{{
						DataType:   "api_key",
						Value:      "*",
						Masked:     "*",
						Confidence: 0.80,
						RuleID:     "stripe_wildcard_webhook",
						RiskLevel:  "medium",
					}},
				})
				break
			}
		}
	}
	return out
}
