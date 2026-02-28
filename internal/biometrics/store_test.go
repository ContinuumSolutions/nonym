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
