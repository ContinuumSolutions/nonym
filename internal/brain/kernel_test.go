package brain

import "testing"

func TestKernelStatus_String(t *testing.T) {
	cases := []struct {
		s    KernelStatus
		want string
	}{
		{StatusOnline, "ONLINE"},
		{StatusShielded, "SHIELDED"},
		{StatusH2HI, "H2HI — MANUAL SYNC REQUIRED"},
		{StatusExiled, "EXILED"},
		{KernelStatus(99), "UNKNOWN"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("KernelStatus(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestTriage_ExiledKernelRejectsAll(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	k.Status = StatusExiled
	action, _ := k.Triage(IncomingRequest{EstimatedROI: 1e9, TimeCommitment: 0})
	if action != "REJECT" {
		t.Errorf("exiled kernel: want REJECT, got %q", action)
	}
}

func TestTriage_RejectsLowROI(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	// attention cost = 500 * 1h * 0.8 = 400; threshold = 400 * 1.5 = 600; ROI=100 < 600
	action, _ := k.Triage(IncomingRequest{EstimatedROI: 100, TimeCommitment: 1.0})
	if action != "REJECT" {
		t.Errorf("low-ROI: want REJECT, got %q", action)
	}
}

func TestTriage_GhostsManipulativeRequest(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	// ManipulationPct > ManipulationThreshold (0.15)
	action, _ := k.Triage(IncomingRequest{
		EstimatedROI:    1e9,
		TimeCommitment:  0,
		ManipulationPct: 0.20,
	})
	if action != "GHOST" {
		t.Errorf("manipulative: want GHOST, got %q", action)
	}
}

func TestTriage_AcceptsCleanHighROI(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	action, _ := k.Triage(IncomingRequest{
		EstimatedROI:    5000,
		TimeCommitment:  0.5,
		ManipulationPct: 0.0,
	})
	if action != "ACCEPT" {
		t.Errorf("clean high-ROI: want ACCEPT, got %q", action)
	}
}

func TestDecide_AppendsAlignmentEntry(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	op := TradeOpportunity{
		Name:           "deal",
		ExpectedROI:    5000,
		TimeCommitment: 0.1,
		ReputationRisk: 0.01,
	}
	k.Decide(op)
	k.mu.RLock()
	defer k.mu.RUnlock()
	if len(k.alignmentHistory) != 1 {
		t.Errorf("want 1 history entry, got %d", len(k.alignmentHistory))
	}
}

func TestDecide_IncrementDecisionCount(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	if k.decisionCount.Load() != 0 {
		t.Fatal("expected initial count 0")
	}
	op := TradeOpportunity{Name: "t", ExpectedROI: 5000, TimeCommitment: 0.1}
	k.Decide(op)
	if k.decisionCount.Load() != 1 {
		t.Errorf("want count=1 after Decide, got %d", k.decisionCount.Load())
	}
}

func TestAcknowledgeManualSync_ClearsH2HIAndHistory(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	k.mu.Lock()
	k.Status = StatusH2HI
	k.alignmentHistory = []float64{0.5, 0.5, 0.5}
	k.mu.Unlock()

	k.AcknowledgeManualSync()

	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Status != StatusOnline {
		t.Errorf("want ONLINE after ack, got %v", k.Status)
	}
	if len(k.alignmentHistory) != 0 {
		t.Errorf("want alignment history cleared, got len=%d", len(k.alignmentHistory))
	}
}

func TestAcknowledgeManualSync_NoopWhenNotH2HI(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	k.AcknowledgeManualSync() // should not panic or change anything
	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Status != StatusOnline {
		t.Errorf("want ONLINE (unchanged), got %v", k.Status)
	}
}

func TestCheckEntropy_TriggersH2HIAt50Percent(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	// 50/50 split → maximum entropy → exceeds IdentityEntropyLimit
	k.mu.Lock()
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			k.alignmentHistory = append(k.alignmentHistory, 1.0)
		} else {
			k.alignmentHistory = append(k.alignmentHistory, 0.0)
		}
	}
	k.mu.Unlock()

	k.checkEntropy()

	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Status != StatusH2HI {
		t.Errorf("want H2HI after high entropy, got %v", k.Status)
	}
}

func TestCheckEntropy_SkipsWhenHistoryShort(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	k.mu.Lock()
	// Only 9 entries — below the minimum of 10
	for i := 0; i < 9; i++ {
		k.alignmentHistory = append(k.alignmentHistory, 0.0)
	}
	k.mu.Unlock()

	k.checkEntropy()

	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Status != StatusOnline {
		t.Errorf("want ONLINE for <10 entries, got %v", k.Status)
	}
}

func TestCheckEntropy_NoTriggerWhenAligned(t *testing.T) {
	k := NewKernel("x", DefaultMatrix())
	k.mu.Lock()
	// 100% aligned — near-zero entropy
	for i := 0; i < 20; i++ {
		k.alignmentHistory = append(k.alignmentHistory, 1.0)
	}
	k.mu.Unlock()

	k.checkEntropy()

	k.mu.RLock()
	defer k.mu.RUnlock()
	if k.Status != StatusOnline {
		t.Errorf("want ONLINE for aligned history, got %v", k.Status)
	}
}
