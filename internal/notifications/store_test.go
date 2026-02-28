package notifications

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

func TestCreate_AssignsID(t *testing.T) {
	s := newTestStore(t)
	n, err := s.Create(Notification{Type: TypeH2HI, Title: "Alert", Body: "Details"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if n.Type != TypeH2HI {
		t.Errorf("Type: want H2HI, got %q", n.Type)
	}
	if n.Read {
		t.Error("new notification should not be read")
	}
}

func TestListUnread_EmptyInitially(t *testing.T) {
	s := newTestStore(t)
	items, err := s.ListUnread()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("want 0 items, got %d", len(items))
	}
}

func TestListUnread_ExcludesRead(t *testing.T) {
	s := newTestStore(t)
	n1, _ := s.Create(Notification{Type: TypeOpportunity, Title: "A", Body: "a"})
	n2, _ := s.Create(Notification{Type: TypeHarvest, Title: "B", Body: "b"})
	s.MarkRead(n1.ID)

	items, err := s.ListUnread()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("want 1 unread, got %d", len(items))
	}
	if items[0].ID != n2.ID {
		t.Errorf("wrong notification returned: want ID %d, got %d", n2.ID, items[0].ID)
	}
}

func TestMarkRead_SetsReadFlag(t *testing.T) {
	s := newTestStore(t)
	n, _ := s.Create(Notification{Type: TypeH2HI, Title: "X", Body: "y"})
	if err := s.MarkRead(n.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items, _ := s.ListUnread()
	if len(items) != 0 {
		t.Error("notification should be marked as read")
	}
}

func TestMarkRead_NotFoundReturnsErr(t *testing.T) {
	s := newTestStore(t)
	if err := s.MarkRead(99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestMarkAllRead_ClearsAllUnread(t *testing.T) {
	s := newTestStore(t)
	s.Create(Notification{Type: TypeH2HI, Title: "1", Body: "a"})
	s.Create(Notification{Type: TypeH2HI, Title: "2", Body: "b"})
	s.Create(Notification{Type: TypeH2HI, Title: "3", Body: "c"})

	if err := s.MarkAllRead(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items, _ := s.ListUnread()
	if len(items) != 0 {
		t.Errorf("want 0 unread after MarkAllRead, got %d", len(items))
	}
}

func TestList_IncludesReadAndUnread(t *testing.T) {
	s := newTestStore(t)
	n1, _ := s.Create(Notification{Type: TypeH2HI, Title: "1", Body: "a"})
	s.Create(Notification{Type: TypeH2HI, Title: "2", Body: "b"})
	s.MarkRead(n1.ID)

	all, err := s.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("want 2 total, got %d", len(all))
	}
}
