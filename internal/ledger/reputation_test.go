package ledger

import (
	"testing"
)

func TestInitializeAndScore(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("test-kernel-001")
	score := l.Score("test-kernel-001")
	if score < BaselineScore-10 || score > BaselineScore+10 {
		t.Errorf("expected baseline score ~%d, got %d", BaselineScore, score)
	}
}

func TestLogSuccess_IncreasesScore(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("k1")
	before := l.Score("k1")
	l.LogSuccess("k1", 100)
	after := l.Score("k1")
	if after <= before {
		t.Errorf("expected score to increase after success: before=%d after=%d", before, after)
	}
}

func TestLogBetrayal_DecreasesScore(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("k2")
	before := l.Score("k2")
	l.LogBetrayal("k2", 50)
	after := l.Score("k2")
	if after >= before {
		t.Errorf("expected score to decrease after betrayal: before=%d after=%d", before, after)
	}
}

func TestExile_TriggeredWhenScoreBelowThreshold(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("exile-test")
	// Hammer the score with large betrayals to drop below threshold.
	for i := 0; i < 100; i++ {
		l.LogBetrayal("exile-test", 100)
	}
	tier := l.Tier("exile-test")
	if tier != TierExiled {
		t.Errorf("expected EXILED tier after repeated betrayals, got %s", tier)
	}
}

func TestTier_Sovereign(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("sovereign")
	l.LogSuccess("sovereign", 500) // push above 980
	tier := l.Tier("sovereign")
	if tier != TierSovereign {
		t.Errorf("expected SOVEREIGN tier, got %s", tier)
	}
}

func TestSummary_Format(t *testing.T) {
	l := NewLocalLedger()
	l.Initialize("summary-test")
	summary := l.Summary("summary-test")
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}
