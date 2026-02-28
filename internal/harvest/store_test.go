package harvest

import (
	"database/sql"
	"testing"
	"time"

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

func TestLatest_NilWhenNothingSaved(t *testing.T) {
	s := newTestStore(t)
	result, err := s.Latest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("want nil, got %+v", result)
	}
}

func TestSaveAndLatest_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	hr := HarvestResult{
		ScannedAt:     time.Now().UTC().Truncate(0),
		ContactsFound: 3,
		TotalValue:    11250.0,
		Debts: []SocialDebt{
			{
				Contact:        ContactRecord{ID: "alice", Name: "Alice"},
				NetFavors:      3,
				EstimatedValue: 11250.0,
				Action:         "send request",
			},
		},
		Opportunities: []string{"ghost-deal"},
	}

	if err := s.Save(hr); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.Latest()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.ContactsFound != 3 {
		t.Errorf("ContactsFound: want 3, got %d", got.ContactsFound)
	}
	if got.TotalValue != 11250.0 {
		t.Errorf("TotalValue: want 11250.0, got %.1f", got.TotalValue)
	}
	if len(got.Debts) != 1 {
		t.Errorf("Debts: want 1, got %d", len(got.Debts))
	}
	if len(got.Opportunities) != 1 {
		t.Errorf("Opportunities: want 1, got %d", len(got.Opportunities))
	}
}

func TestLatest_ReturnsNewestScan(t *testing.T) {
	s := newTestStore(t)
	s.Save(HarvestResult{ContactsFound: 1})
	s.Save(HarvestResult{ContactsFound: 5}) // newer

	got, _ := s.Latest()
	if got.ContactsFound != 5 {
		t.Errorf("want ContactsFound=5 (newest), got %d", got.ContactsFound)
	}
}
