package datasync

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/integrations"
	_ "modernc.org/sqlite"
)

// stubAdapter implements Adapter for testing without network calls.
type stubAdapter struct {
	slug    string
	signals []RawSignal
	err     error
}

func (a *stubAdapter) Slug() string { return a.slug }

func (a *stubAdapter) Pull(_ context.Context, _ Credentials, _ time.Time) ([]RawSignal, error) {
	return a.signals, a.err
}

func newTestIntegrationStore(t *testing.T) *integrations.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	key := make([]byte, 32)
	s := integrations.NewStore(db, key)
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return s
}

// insertConnectedService directly inserts a service row with status=Connected (2)
// so the engine can find it via ListConnected().
func insertConnectedService(t *testing.T, s *integrations.Store, slug string) {
	t.Helper()
	// Use CreateCustom with a dummy key, then patch the slug directly via the store's List+Get pattern.
	// Since Store doesn't expose a raw insert, we use the public API:
	// CreateCustom sets slug=''; we work around by creating and relying on the engine
	// matching adapters by slug from ListConnected.
	// Instead, insert a connected service via the integration store's Seed path for real slugs,
	// or use StartConnect+CompleteConnect on a seeded row.
	s.Seed()
	svcs, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Find the seeded service with the matching slug and install it.
	for _, svc := range svcs {
		if svc.Slug == slug {
			s.StartConnect(svc.ID)
			s.CompleteConnect(svc.ID, integrations.ConnectInput{APIKey: "testkey1234"})
			return
		}
	}
	t.Fatalf("seeded service with slug %q not found", slug)
}

func TestEngineRun_NoConnectedServices(t *testing.T) {
	store := newTestIntegrationStore(t)
	engine := NewEngine(store, nil)

	signals, err := engine.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("want 0 signals with no connected services, got %d", len(signals))
	}
}

func TestEngineRun_WithStubAdapter_CollectsSignals(t *testing.T) {
	store := newTestIntegrationStore(t)
	// Install the google-calendar service (known slug from catalog)
	insertConnectedService(t, store, "google-calendar")

	expected := []RawSignal{
		{ServiceSlug: "google-calendar", Category: "Calendar", Title: "Test event"},
	}
	stub := &stubAdapter{slug: "google-calendar", signals: expected}
	engine := NewEngine(store, []Adapter{stub})

	signals, err := engine.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(signals) != 1 {
		t.Errorf("want 1 signal, got %d", len(signals))
	}
	if signals[0].Title != "Test event" {
		t.Errorf("Title: want %q, got %q", "Test event", signals[0].Title)
	}
}

func TestEngineRun_AdapterError_ContinuesGracefully(t *testing.T) {
	store := newTestIntegrationStore(t)
	// Install two services
	insertConnectedService(t, store, "google-calendar")
	insertConnectedService(t, store, "gmail")

	calStub := &stubAdapter{
		slug:    "google-calendar",
		signals: []RawSignal{{ServiceSlug: "google-calendar", Title: "OK"}},
	}
	gmailStub := &stubAdapter{
		slug: "gmail",
		err:  errors.New("simulated adapter error"),
	}
	engine := NewEngine(store, []Adapter{calStub, gmailStub})

	signals, err := engine.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	// gmail errored but google-calendar succeeded — should get 1 signal
	if len(signals) != 1 {
		t.Errorf("want 1 signal (gmail skipped on error), got %d", len(signals))
	}
}

func TestEngineRun_UpdatesLastSync(t *testing.T) {
	store := newTestIntegrationStore(t)
	insertConnectedService(t, store, "google-calendar")

	stub := &stubAdapter{slug: "google-calendar", signals: []RawSignal{
		{ServiceSlug: "google-calendar", Title: "Meeting"},
	}}
	engine := NewEngine(store, []Adapter{stub})

	before := time.Now()
	engine.Run(context.Background())
	after := time.Now()

	last := engine.LastSync("google-calendar")
	if last.Before(before) || last.After(after) {
		t.Errorf("LastSync %v not in expected range [%v, %v]", last, before, after)
	}
}

func TestEngineRun_NoAdapterForConnectedService_Skipped(t *testing.T) {
	store := newTestIntegrationStore(t)
	// Create a custom service (no slug, so no adapter matches)
	store.CreateCustom(&integrations.Service{
		Name: "Custom", Category: "Finance", APIEndpoint: "https://x.com", APIKey: "key1234",
	})
	engine := NewEngine(store, nil) // no adapters registered

	signals, err := engine.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("want 0 signals when no adapter matches, got %d", len(signals))
	}
}
