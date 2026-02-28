package activities

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

func sampleEvent() Event {
	return Event{
		EventType:  Finance,
		Decision:   Pending,
		Importance: High,
		Narrative:  "Test narrative",
		Gain:       Gain{Type: Positive, Value: 500.0, Symbol: "$", Details: "Test gain"},
	}
}

func TestCreate_ReturnsEventWithID(t *testing.T) {
	s := newTestStore(t)
	ev, err := s.Create(sampleEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.ID == 0 {
		t.Error("expected non-zero ID after create")
	}
	if ev.Narrative != "Test narrative" {
		t.Errorf("want narrative %q, got %q", "Test narrative", ev.Narrative)
	}
	if ev.Read {
		t.Error("read should default to false")
	}
}

func TestList_EmptyStoreReturnsNil(t *testing.T) {
	s := newTestStore(t)
	events, err := s.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want 0 events, got %d", len(events))
	}
}

func TestList_ReturnsAllEvents(t *testing.T) {
	s := newTestStore(t)
	s.Create(Event{EventType: Finance, Decision: Pending, Narrative: "first"})
	s.Create(Event{EventType: Calendar, Decision: Automated, Narrative: "second"})

	events, err := s.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("want 2 events, got %d", len(events))
	}
}

func TestGet_ExistingEvent(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.Create(sampleEvent())
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("want ID %d, got %d", created.ID, got.ID)
	}
}

func TestGet_NonExistentReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get(99999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestToggleRead_TogglesFromFalseToTrue(t *testing.T) {
	s := newTestStore(t)
	ev, _ := s.Create(sampleEvent())
	if ev.Read {
		t.Fatal("expected Read=false initially")
	}

	toggled, err := s.ToggleRead(ev.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !toggled.Read {
		t.Error("expected Read=true after first toggle")
	}
}

func TestToggleRead_TogglesBackToFalse(t *testing.T) {
	s := newTestStore(t)
	ev, _ := s.Create(sampleEvent())
	s.ToggleRead(ev.ID) // → true
	ev2, _ := s.ToggleRead(ev.ID) // → false
	if ev2.Read {
		t.Error("expected Read=false after second toggle")
	}
}

func TestToggleRead_NonExistentReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.ToggleRead(99999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}
