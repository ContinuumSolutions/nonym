// Package datasync pulls raw data from installed integrations and normalises
// it into RawSignal structs for the brain pipeline (step 6).
package datasync

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/egokernel/ek1/internal/integrations"
)

// RawSignal is the normalised output of every service adapter.
// It carries just enough information for the LLM (step 5) to classify
// and score it before the brain (step 6) turns it into an Event.
type RawSignal struct {
	ServiceSlug string
	Category    string            // "Calendar", "Communication", "Finance", "Health", "Billing"
	Title       string
	Body        string
	Metadata    map[string]string // service-specific fields (sender, amount, channel, etc.)
	OccurredAt  time.Time
}

// Credentials holds decrypted auth material passed to each adapter.
type Credentials struct {
	APIKey            string
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

	mu        sync.Mutex
	lastSync  map[string]time.Time
	lastCount map[string]int    // signals pulled per slug in last cycle
	lastError map[string]string // last error message per slug (empty = no error)
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
			OAuthAccessToken:  svc.OAuthAccessToken,
			OAuthRefreshToken: svc.OAuthRefreshToken,
		}

		signals, err := adapter.Pull(ctx, creds, since)
		if err != nil {
			log.Printf("datasync: [%s] pull error: %v", svc.Slug, err)
			e.mu.Lock()
			e.lastError[svc.Slug] = err.Error()
			e.mu.Unlock()
			continue
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
	LastSyncAt  *time.Time `json:"last_sync_at"`           // nil = never synced
	SignalCount int        `json:"signal_count"`
	LastError   string     `json:"last_error,omitempty"`
}

// ServiceStatuses returns a snapshot of every registered adapter's sync state.
func (e *Engine) ServiceStatuses() []ServiceStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]ServiceStatus, 0, len(e.adapters))
	for slug := range e.adapters {
		ss := ServiceStatus{
			Slug:        slug,
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

// authGet performs a GET request with a Bearer token and returns the body bytes.
func authGet(ctx context.Context, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("HTTP 401 from %s — access token was revoked or expired; re-authorize to restore sync", url)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
