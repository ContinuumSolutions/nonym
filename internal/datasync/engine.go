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

	mu       sync.Mutex
	lastSync map[string]time.Time
}

// NewEngine creates a sync engine with the given adapter registry.
func NewEngine(services *integrations.Store, adapters []Adapter) *Engine {
	m := make(map[string]Adapter, len(adapters))
	for _, a := range adapters {
		m[a.Slug()] = a
	}
	return &Engine{
		services: services,
		adapters: m,
		lastSync: make(map[string]time.Time),
	}
}

// Run pulls fresh signals from every installed service that has a registered
// adapter. Services without adapters (custom) are skipped silently.
// Adapter errors are logged but do not abort the run.
func (e *Engine) Run(ctx context.Context) ([]RawSignal, error) {
	installed, err := e.services.ListInstalled()
	if err != nil {
		return nil, fmt.Errorf("datasync: list installed: %w", err)
	}

	var all []RawSignal
	for _, svc := range installed {
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
			continue
		}

		e.mu.Lock()
		e.lastSync[svc.Slug] = time.Now()
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
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
