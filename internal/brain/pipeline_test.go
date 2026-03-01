package brain

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/profile"
	_ "modernc.org/sqlite"
)

// stubAnalyser satisfies the Analyser interface for pipeline tests.
type stubAnalyser struct {
	results []*ai.AnalysedSignal
	errs    []error
}

func (s *stubAnalyser) AnalyseBatch(_ context.Context, signals []datasync.RawSignal) ([]*ai.AnalysedSignal, []error) {
	if s.results == nil {
		// Default: return nil per signal (LLM error path).
		return make([]*ai.AnalysedSignal, len(signals)), make([]error, len(signals))
	}
	return s.results, s.errs
}

// newTestPipeline wires a minimal Pipeline with in-memory SQLite stores.
func newTestPipeline(t *testing.T, analyser Analyser) *Pipeline {
	t.Helper()

	l := ledger.NewLocalLedger()
	prefs := profile.DecisionPreference{
		TimeSovereignty:    5,
		FinacialGrowth:     5,
		HealthRecovery:     5,
		ReputationBuilding: 5,
		PrivacyProtection:  5,
		Autonomy:           5,
	}
	svc := NewService("test-uid", prefs, l)

	actDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open activities db: %v", err)
	}
	t.Cleanup(func() { actDB.Close() })
	actStore := activities.NewStore(actDB)
	if err := actStore.Migrate(); err != nil {
		t.Fatalf("activities Migrate: %v", err)
	}

	bioDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open biometrics db: %v", err)
	}
	t.Cleanup(func() { bioDB.Close() })
	bioStore := biometrics.NewStore(bioDB)
	if err := bioStore.Migrate(); err != nil {
		t.Fatalf("biometrics Migrate: %v", err)
	}

	return NewPipeline(svc, analyser, actStore, bioStore)
}

func TestPipelineRun_EmptySignals(t *testing.T) {
	p := newTestPipeline(t, &stubAnalyser{})
	result, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("want Total=0, got %d", result.Total)
	}
	if result.Accepted != 0 || result.Rejected != 0 || result.Ghosted != 0 {
		t.Errorf("want all zero counts, got accepted=%d rejected=%d ghosted=%d",
			result.Accepted, result.Rejected, result.Ghosted)
	}
}

func TestPipelineRun_NilLLMResult_Skipped(t *testing.T) {
	signal := datasync.RawSignal{ServiceSlug: "test", Title: "Test signal", Category: "Finance"}
	analyser := &stubAnalyser{
		results: []*ai.AnalysedSignal{nil},
		errs:    []error{fmt.Errorf("llm unavailable")},
	}
	p := newTestPipeline(t, analyser)
	result, err := p.Run(context.Background(), []datasync.RawSignal{signal})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("want Total=1, got %d", result.Total)
	}
	// Nil LLM result → signal skipped; no classification counted
	if result.Accepted+result.Rejected+result.Ghosted != 0 {
		t.Errorf("want 0 classified when LLM returns nil, got accepted=%d rejected=%d ghosted=%d",
			result.Accepted, result.Rejected, result.Ghosted)
	}
}

func TestPipelineRun_AcceptPath(t *testing.T) {
	// With prefs all=5:
	//   UtilityThreshold = 2200 - 5*200 = 1200
	//   TemporalSovereignty = 0.5, BaseHourlyRate = 500
	//   adjustedROI = 5000 - 1.0*500*0.5 = 4750 > 1200 → EXECUTE=true
	signal := datasync.RawSignal{ServiceSlug: "test", Title: "High ROI deal", Category: "Finance"}
	analysed := &ai.AnalysedSignal{
		Signal:     signal,
		EventType:  activities.Finance,
		Importance: activities.High,
		Narrative:  "High-value opportunity",
		Request: ai.SignalRequest{
			ID:              "req-1",
			SenderID:        "sender-1",
			Description:     "High ROI deal",
			EstimatedROI:    5000,
			TimeCommitment:  1.0,
			ManipulationPct: 0.0,
		},
		Gain: activities.Gain{Type: activities.Positive, Value: 5000, Symbol: "$"},
	}
	analyser := &stubAnalyser{
		results: []*ai.AnalysedSignal{analysed},
		errs:    []error{nil},
	}
	p := newTestPipeline(t, analyser)
	result, err := p.Run(context.Background(), []datasync.RawSignal{signal})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("want Total=1, got %d", result.Total)
	}
	if result.Accepted != 1 {
		t.Errorf("want Accepted=1, got %d (rejected=%d ghosted=%d)",
			result.Accepted, result.Rejected, result.Ghosted)
	}
}

func TestPipelineRun_RejectPath_LowROI(t *testing.T) {
	// EstimatedROI=100 < UtilityThreshold=1200 → Triage returns REJECT
	signal := datasync.RawSignal{ServiceSlug: "test", Title: "Low value", Category: "Finance"}
	analysed := &ai.AnalysedSignal{
		Signal:     signal,
		EventType:  activities.Finance,
		Importance: activities.Low,
		Narrative:  "Not worth it",
		Request: ai.SignalRequest{
			ID:              "req-2",
			SenderID:        "sender-2",
			EstimatedROI:    100, // below UtilityThreshold=1200
			TimeCommitment:  0.5,
			ManipulationPct: 0.0,
		},
	}
	analyser := &stubAnalyser{
		results: []*ai.AnalysedSignal{analysed},
		errs:    []error{nil},
	}
	p := newTestPipeline(t, analyser)
	result, err := p.Run(context.Background(), []datasync.RawSignal{signal})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Rejected != 1 {
		t.Errorf("want Rejected=1, got %d (accepted=%d ghosted=%d)",
			result.Rejected, result.Accepted, result.Ghosted)
	}
}

func TestPipelineRun_GhostPath_HighManipulation(t *testing.T) {
	// ManipulationPct=1.0 > RiskTolerance=0.20 → Triage returns GHOST
	signal := datasync.RawSignal{ServiceSlug: "test", Title: "Suspicious offer", Category: "Finance"}
	analysed := &ai.AnalysedSignal{
		Signal:     signal,
		EventType:  activities.Finance,
		Importance: activities.Low,
		Narrative:  "Highly manipulative",
		Request: ai.SignalRequest{
			ID:              "req-3",
			SenderID:        "sender-3",
			EstimatedROI:    10000, // high ROI but manipulative
			TimeCommitment:  1.0,
			ManipulationPct: 1.0, // 100% — above RiskTolerance=0.20
		},
	}
	analyser := &stubAnalyser{
		results: []*ai.AnalysedSignal{analysed},
		errs:    []error{nil},
	}
	p := newTestPipeline(t, analyser)
	result, err := p.Run(context.Background(), []datasync.RawSignal{signal})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Ghosted != 1 {
		t.Errorf("want Ghosted=1, got %d (accepted=%d rejected=%d)",
			result.Ghosted, result.Accepted, result.Rejected)
	}
}

func TestPipelineRun_ShieldedFlagSetWhenBiometricsActive(t *testing.T) {
	// Insert a high-stress check-in so the biometrics gate fires.
	l := ledger.NewLocalLedger()
	prefs := profile.DecisionPreference{
		TimeSovereignty: 5, FinacialGrowth: 5, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	}
	svc := NewService("test-uid", prefs, l)

	actDB, _ := sql.Open("sqlite", ":memory:")
	t.Cleanup(func() { actDB.Close() })
	actStore := activities.NewStore(actDB)
	actStore.Migrate()

	bioDB, _ := sql.Open("sqlite", ":memory:")
	t.Cleanup(func() { bioDB.Close() })
	bioStore := biometrics.NewStore(bioDB)
	bioStore.Migrate()
	// Stress=8 > 7 → shield active
	bioStore.Upsert(&biometrics.CheckIn{Mood: 5, StressLevel: 8, Sleep: 6, Energy: 5})

	p := NewPipeline(svc, &stubAnalyser{}, actStore, bioStore)
	result, err := p.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Shielded {
		t.Error("want Shielded=true when stress>7")
	}
	if result.ShieldReason == "" {
		t.Error("want non-empty ShieldReason")
	}
}
