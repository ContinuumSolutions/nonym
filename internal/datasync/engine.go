// Package datasync pulls raw data from installed integrations and normalises
// it into RawSignal structs for the brain pipeline (step 6).
package datasync

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egokernel/ek1/internal/integrations"
)

// RawSignal is the normalised output of every service adapter.
// It carries just enough information for the LLM (step 5) to classify
// and score it before the brain (step 6) turns it into an Event.
type RawSignal struct {
	ServiceSlug    string
	ServicePurpose string            // what this service is and what it's used for (injected by engine)
	Category       string            // "Calendar", "Communication", "Finance", "Health", "Billing"
	Title          string
	Body           string
	Metadata       map[string]string // service-specific fields (sender, amount, channel, etc.)
	OccurredAt     time.Time
}

// Credentials holds decrypted auth material passed to each adapter.
type Credentials struct {
	APIKey            string
	APIEndpoint       string // regional API base URL, if stored (e.g. "https://mail.zoho.eu")
	TokenURLOverride  string // regional token endpoint URL, if stored (e.g. Zoho EU/India)
	OAuthAccessToken  string
	OAuthRefreshToken string
}

// Adapter is the interface every service integration must implement.
// Pull fetches new signals that occurred after `since`.
type Adapter interface {
	Slug() string
	Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error)
}

// Engine orchestrates the sync cycle across all installed services.
type Engine struct {
	services *integrations.Store
	adapters map[string]Adapter

	mu           sync.Mutex
	lastSync     map[string]time.Time
	lastCount    map[string]int    // signals pulled per slug in last cycle
	lastError    map[string]string // last error message per slug (empty = no error)
	currentSlug  string            // slug of the adapter currently being pulled ("" = idle)
}

// NewEngine creates a sync engine with the given adapter registry.
func NewEngine(services *integrations.Store, adapters []Adapter) *Engine {
	m := make(map[string]Adapter, len(adapters))
	for _, a := range adapters {
		m[a.Slug()] = a
	}
	return &Engine{
		services:  services,
		adapters:  m,
		lastSync:  make(map[string]time.Time),
		lastCount: make(map[string]int),
		lastError: make(map[string]string),
	}
}

// Run pulls fresh signals from every connected service that has a registered
// adapter. Services without adapters (custom) are skipped silently.
// Adapter errors are logged but do not abort the run.
func (e *Engine) Run(ctx context.Context) ([]RawSignal, error) {
	connected, err := e.services.ListConnected()
	if err != nil {
		return nil, fmt.Errorf("datasync: list connected: %w", err)
	}

	var all []RawSignal
	for _, svc := range connected {
		adapter, ok := e.adapters[svc.Slug]
		if !ok {
			continue // custom service — no built-in adapter yet
		}

		e.mu.Lock()
		since := e.lastSync[svc.Slug]
		if since.IsZero() {
			since = time.Now().Add(-24 * time.Hour)
		}
		e.mu.Unlock()

		creds := Credentials{
			APIKey:            svc.APIKey,
			APIEndpoint:       svc.APIEndpoint,
			TokenURLOverride:  svc.OAuthTokenURLOverride,
			OAuthAccessToken:  svc.OAuthAccessToken,
			OAuthRefreshToken: svc.OAuthRefreshToken,
		}

		e.mu.Lock()
		e.currentSlug = svc.Slug
		e.mu.Unlock()

		signals, err := adapter.Pull(ctx, creds, since)

		e.mu.Lock()
		e.currentSlug = ""
		e.mu.Unlock()

		// On 401 from an OAuth service: attempt a token refresh and retry once.
		// If the refresh fails, TryRefresh marks the service Disconnected automatically.
		if err != nil && strings.Contains(err.Error(), "HTTP 401") && svc.AuthMethod == integrations.OAuth2Auth {
			log.Printf("datasync: [%s] 401 — attempting token refresh", svc.Slug)
			newToken, refreshErr := e.services.TryRefresh(svc.ID, svc.Slug)
			if refreshErr != nil {
				log.Printf("datasync: [%s] token refresh failed: %v — service marked disconnected", svc.Slug, refreshErr)
			} else {
				creds.OAuthAccessToken = newToken
				signals, err = adapter.Pull(ctx, creds, since)
			}
		}

		// On 403 from an OAuth service: could be missing scopes, disabled API, or access policy.
		// A refresh won't help — mark the service as NeedsReauth so the user is prompted to re-authorize.
		if err != nil && strings.Contains(err.Error(), "HTTP 403") && svc.AuthMethod == integrations.OAuth2Auth {
			log.Printf("datasync: [%s] 403 — %v", svc.Slug, err)
			if reauthErr := e.services.MarkNeedsReauth(svc.ID); reauthErr != nil {
				log.Printf("datasync: [%s] failed to mark needs-reauth: %v", svc.Slug, reauthErr)
			}
		}

		if err != nil {
			log.Printf("datasync: [%s] pull error: %v", svc.Slug, err)
			e.mu.Lock()
			e.lastError[svc.Slug] = err.Error()
			e.mu.Unlock()
			continue
		}

		// Stamp each signal with the service purpose so the LLM has full context.
		if svc.Description != "" {
			for i := range signals {
				signals[i].ServicePurpose = svc.Description
			}
		}

		e.mu.Lock()
		e.lastSync[svc.Slug] = time.Now()
		e.lastCount[svc.Slug] = len(signals)
		e.lastError[svc.Slug] = ""
		e.mu.Unlock()

		all = append(all, signals...)
	}

	return all, nil
}

// LastSync returns the last successful sync time for a service slug.
func (e *Engine) LastSync(slug string) time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastSync[slug]
}

// ServiceStatus is a point-in-time snapshot of one adapter's sync state.
type ServiceStatus struct {
	Slug        string     `json:"slug"`
	Active      bool       `json:"active"`                 // true while this adapter is being pulled right now
	LastSyncAt  *time.Time `json:"last_sync_at"`           // nil = never synced
	SignalCount int        `json:"signal_count"`           // signals pulled in the last completed cycle
	LastError   string     `json:"last_error,omitempty"`   // non-empty = last pull failed
}

// ServiceStatuses returns a live snapshot of every registered adapter's sync state.
// Active reflects whether that adapter is being pulled at the moment of the call.
func (e *Engine) ServiceStatuses() []ServiceStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]ServiceStatus, 0, len(e.adapters))
	for slug := range e.adapters {
		ss := ServiceStatus{
			Slug:        slug,
			Active:      slug == e.currentSlug,
			SignalCount: e.lastCount[slug],
			LastError:   e.lastError[slug],
		}
		if t, ok := e.lastSync[slug]; ok {
			ts := t
			ss.LastSyncAt = &ts
		}
		out = append(out, ss)
	}
	return out
}

// debugHTTP is true when EK_DEBUG_HTTP=1 is set. Logs full request/response details.
var debugHTTP = os.Getenv("EK_DEBUG_HTTP") == "1"

// maskToken returns the last 6 chars of a token for safe log output.
func maskToken(t string) string {
	if len(t) <= 6 {
		return "******"
	}
	return "......" + t[len(t)-6:]
}

// authGet performs a GET request with a Bearer token and returns the body bytes.
func authGet(ctx context.Context, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	if debugHTTP {
		log.Printf("[DEBUG] GET %s  token=%s", url, maskToken(token))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)

	if debugHTTP {
		preview := string(body)
		if len(preview) > 400 {
			preview = preview[:400] + "…"
		}
		log.Printf("[DEBUG] → %d  body=%s", resp.StatusCode, preview)
	}

	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("HTTP 401 from %s — access token was revoked or expired; re-authorize to restore sync", url)
	}
	if resp.StatusCode == 403 {
		// Include the provider's error message verbatim — it often contains actionable detail
		// (e.g. "Gmail API has not been enabled", "insufficient scopes") that the generic
		// message hides.
		errBody := strings.TrimSpace(string(body))
		if len(errBody) > 300 {
			errBody = errBody[:300] + "…"
		}
		return nil, fmt.Errorf("HTTP 403 from %s — %s", url, errBody)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return body, nil
}
