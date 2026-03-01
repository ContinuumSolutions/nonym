package biometrics

import (
	"database/sql"
	"errors"
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

func TestGet_NotFoundBeforeAnyUpsert(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUpsert_CreatesRow(t *testing.T) {
	s := newTestStore(t)
	in := &CheckIn{Feeling: 8, StressLevel: 3, Sleep: 7, Energy: 9, ExtraContext: "feeling good"}
	got, err := s.Upsert(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Feeling != 8 || got.StressLevel != 3 || got.Sleep != 7 || got.Energy != 9 {
		t.Errorf("unexpected check-in values: %+v", got)
	}
	if got.ExtraContext != "feeling good" {
		t.Errorf("ExtraContext: want %q, got %q", "feeling good", got.ExtraContext)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestUpsert_UpdatesExistingRow(t *testing.T) {
	s := newTestStore(t)
	s.Upsert(&CheckIn{Feeling: 5, StressLevel: 5, Sleep: 5, Energy: 5})

	updated, err := s.Upsert(&CheckIn{Feeling: 9, StressLevel: 2, Sleep: 8, Energy: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Feeling != 9 {
		t.Errorf("want updated Feeling=9, got %d", updated.Feeling)
	}
	if updated.StressLevel != 2 {
		t.Errorf("want updated StressLevel=2, got %d", updated.StressLevel)
	}
}

func TestGet_AfterUpsert(t *testing.T) {
	s := newTestStore(t)
	s.Upsert(&CheckIn{Feeling: 7, StressLevel: 4, Sleep: 6, Energy: 8})
	got, err := s.Get()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Feeling != 7 {
		t.Errorf("want Feeling=7, got %d", got.Feeling)
	}
}

// ── History ───────────────────────────────────────────────────────────────────

func TestHistory_EmptyBeforeAnyUpsert(t *testing.T) {
	s := newTestStore(t)
	entries, err := s.History(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

func TestHistory_RecordsEachUpsert(t *testing.T) {
	s := newTestStore(t)
	s.Upsert(&CheckIn{Feeling: 5, StressLevel: 5, Sleep: 5, Energy: 5})
	s.Upsert(&CheckIn{Feeling: 7, StressLevel: 3, Sleep: 8, Energy: 9})

	entries, err := s.History(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 history entries, got %d", len(entries))
	}
}

func TestHistory_NewestFirst(t *testing.T) {
	s := newTestStore(t)
	s.Upsert(&CheckIn{Feeling: 3, StressLevel: 8, Sleep: 4, Energy: 3})
	s.Upsert(&CheckIn{Feeling: 9, StressLevel: 1, Sleep: 9, Energy: 9})

	entries, err := s.History(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].Feeling != 9 {
		t.Errorf("want newest entry first (Feeling=9), got Feeling=%d", entries[0].Feeling)
	}
}

func TestHistory_LimitRespected(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		s.Upsert(&CheckIn{Feeling: i + 1, StressLevel: 5, Sleep: 5, Energy: 5})
	}
	entries, err := s.History(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("want 3 entries with limit=3, got %d", len(entries))
	}
}

func TestHistory_LimitCappedAt90(t *testing.T) {
	s := newTestStore(t)
	entries, err := s.History(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No rows, but the cap should not error
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}
}

func TestHistory_DefaultLimitAppliedForZero(t *testing.T) {
	s := newTestStore(t)
	// History(0) should use the default of 7, not error
	_, err := s.History(0)
	if err != nil {
		t.Fatalf("History(0) should not error, got %v", err)
	}
}

func TestHistory_CreatedAtPopulated(t *testing.T) {
	s := newTestStore(t)
	s.Upsert(&CheckIn{Feeling: 6, StressLevel: 4, Sleep: 7, Energy: 8})

	entries, err := s.History(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries[0].CreatedAt.IsZero() {
		t.Error("want non-zero CreatedAt in history entry")
	}
}
