package ledger

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

func newTestLedger(t *testing.T) *SQLiteLedger {
	t.Helper()
	l := NewSQLiteLedger(newTestDB(t))
	if err := l.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return l
}

func TestSQLiteInitialize_SeedsBaseline(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k1")
	score := l.Score("k1")
	if score < BaselineScore-10 || score > BaselineScore+10 {
		t.Errorf("expected score near %d, got %d", BaselineScore, score)
	}
}

func TestSQLiteInitialize_Idempotent(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k1")
	l.Initialize("k1") // second call should not add another baseline
	score := l.Score("k1")
	// Score should still be near baseline, not doubled
	if score > BaselineScore+50 {
		t.Errorf("double initialize inflated score to %d", score)
	}
}

func TestSQLiteLogSuccess_IncreasesScore(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k1")
	before := l.Score("k1")
	l.LogSuccess("k1", 100)
	after := l.Score("k1")
	if after <= before {
		t.Errorf("score should increase after success: before=%d after=%d", before, after)
	}
}

func TestSQLiteLogBetrayal_DecreasesScore(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k2")
	before := l.Score("k2")
	l.LogBetrayal("k2", 50)
	after := l.Score("k2")
	if after >= before {
		t.Errorf("score should decrease after betrayal: before=%d after=%d", before, after)
	}
}

func TestSQLiteTier_Sovereign(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k3")
	l.LogSuccess("k3", 500) // push score well above 980
	tier := l.Tier("k3")
	if tier != TierSovereign {
		t.Errorf("want SOVEREIGN, got %s", tier)
	}
}

func TestSQLiteIsExiled_TrueAfterHeavyBetrayals(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("exile")
	for i := 0; i < 50; i++ {
		l.LogBetrayal("exile", 100)
	}
	if !l.IsExiled("exile") {
		t.Error("want IsExiled=true after repeated betrayals")
	}
}

func TestSQLiteIsExiled_FalseForFreshKernel(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("fresh")
	if l.IsExiled("fresh") {
		t.Error("fresh kernel should not be exiled")
	}
}

func TestSQLiteScore_UnknownUIDReturnsZero(t *testing.T) {
	l := newTestLedger(t)
	if s := l.Score("nobody"); s != 0 {
		t.Errorf("unknown uid: want 0, got %d", s)
	}
}

func TestSQLiteHistory_ReturnsPaginatedEntries(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k4")
	l.LogSuccess("k4", 10)
	l.LogSuccess("k4", 20)
	l.LogBetrayal("k4", 5)

	all, err := l.History("k4", 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Initialize + 2 successes + 1 betrayal = 4 entries
	if len(all) != 4 {
		t.Errorf("want 4 entries, got %d", len(all))
	}
}

func TestSQLiteHistory_LimitAndOffset(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("k5")
	l.LogSuccess("k5", 10)
	l.LogSuccess("k5", 20)

	page1, _ := l.History("k5", 2, 0)
	page2, _ := l.History("k5", 2, 2)

	if len(page1) != 2 {
		t.Errorf("page1: want 2, got %d", len(page1))
	}
	if len(page2) != 1 {
		t.Errorf("page2: want 1, got %d", len(page2))
	}
}

func TestSQLiteSummary_NonEmpty(t *testing.T) {
	l := newTestLedger(t)
	l.Initialize("sum")
	s := l.Summary("sum")
	if s == "" {
		t.Error("Summary should return a non-empty string")
	}
}

func TestSQLiteTrustTax(t *testing.T) {
	cases := []struct {
		tier ReputationTier
		want float64
	}{
		{TierSovereign, 0.0},
		{TierStable, 0.0},
		{TierVolatile, 0.20},
	}
	for _, tc := range cases {
		got := tc.tier.TrustTax()
		if got != tc.want {
			t.Errorf("TrustTax(%s): want %.2f, got %.2f", tc.tier, tc.want, got)
		}
	}
}
