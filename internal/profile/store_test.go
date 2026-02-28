package profile

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s := NewStore(newTestDB(t))
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func TestGet_DefaultsAfterMigrate(t *testing.T) {
	s := newTestStore(t)
	p, err := s.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.KernelName != "EK-1" {
		t.Errorf("want KernelName=EK-1, got %q", p.KernelName)
	}
	if p.Timezone != "UTC" {
		t.Errorf("want Timezone=UTC, got %q", p.Timezone)
	}
	if p.Preferences.TimeSovereignty != 5 {
		t.Errorf("want TimeSovereignty=5, got %d", p.Preferences.TimeSovereignty)
	}
	if !p.Progress.Shadow {
		t.Error("Shadow progress should be true")
	}
}

func TestUpdatePreferences_PersistsValues(t *testing.T) {
	s := newTestStore(t)
	prefs := DecisionPreference{
		TimeSovereignty:    8,
		FinacialGrowth:     7,
		HealthRecovery:     6,
		ReputationBuilding: 9,
		PrivacyProtection:  4,
		Autonomy:           3,
	}
	updated, err := s.UpdatePreferences(prefs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Preferences.TimeSovereignty != 8 {
		t.Errorf("TimeSovereignty: want 8, got %d", updated.Preferences.TimeSovereignty)
	}
	if updated.Preferences.Autonomy != 3 {
		t.Errorf("Autonomy: want 3, got %d", updated.Preferences.Autonomy)
	}
}

func TestUpdatePreferences_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	prefs := DecisionPreference{
		TimeSovereignty: 10, FinacialGrowth: 1, HealthRecovery: 10,
		ReputationBuilding: 10, PrivacyProtection: 1, Autonomy: 1,
	}
	s.UpdatePreferences(prefs)

	got, err := s.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Preferences.TimeSovereignty != 10 {
		t.Errorf("round-trip TimeSovereignty: want 10, got %d", got.Preferences.TimeSovereignty)
	}
}

func TestUpdateConnection_PersistsValues(t *testing.T) {
	s := newTestStore(t)
	conn := ConnectionSetting{
		KernelName:  "my-kernel",
		APIEndpoint: "https://example.com/api",
		Timezone:    "America/New_York",
	}
	updated, err := s.UpdateConnection(conn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.KernelName != "my-kernel" {
		t.Errorf("KernelName: want %q, got %q", "my-kernel", updated.KernelName)
	}
	if updated.Timezone != "America/New_York" {
		t.Errorf("Timezone: want %q, got %q", "America/New_York", updated.Timezone)
	}
}
