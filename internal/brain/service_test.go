package brain

import (
	"testing"

	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/profile"
)

func defaultPrefs() profile.DecisionPreference {
	return profile.DecisionPreference{
		TimeSovereignty:    5,
		FinacialGrowth:     5,
		HealthRecovery:     5,
		ReputationBuilding: 5,
		PrivacyProtection:  5,
		Autonomy:           5,
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	l := ledger.NewLocalLedger()
	l.Initialize("test")
	return NewService("test", defaultPrefs(), l)
}

// ── MatrixFromPreferences ─────────────────────────────────────────────────────

func TestMatrixFromPreferences_MidScale(t *testing.T) {
	vm := MatrixFromPreferences(defaultPrefs())
	// At scale=5: TemporalSovereignty = 5/10 = 0.5
	if vm.TemporalSovereignty != 0.5 {
		t.Errorf("TemporalSovereignty: want 0.5, got %.2f", vm.TemporalSovereignty)
	}
	// UtilityThreshold = 2200 - 5*200 = 1200
	if vm.UtilityThreshold != 1200.0 {
		t.Errorf("UtilityThreshold: want 1200.0, got %.1f", vm.UtilityThreshold)
	}
	// ReputationImpact = 5/10 = 0.5
	if vm.ReputationImpact != 0.5 {
		t.Errorf("ReputationImpact: want 0.5, got %.2f", vm.ReputationImpact)
	}
}

func TestMatrixFromPreferences_TimeSovereigntyScalesDirectly(t *testing.T) {
	lo := MatrixFromPreferences(profile.DecisionPreference{
		TimeSovereignty: 1, FinacialGrowth: 5, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	})
	hi := MatrixFromPreferences(profile.DecisionPreference{
		TimeSovereignty: 10, FinacialGrowth: 5, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	})
	if lo.TemporalSovereignty >= hi.TemporalSovereignty {
		t.Error("TemporalSovereignty should increase with TimeSovereignty")
	}
}

func TestMatrixFromPreferences_FinancialGrowthIsInverse(t *testing.T) {
	// Higher FinancialGrowth → lower UtilityThreshold (less conservative)
	loFG := MatrixFromPreferences(profile.DecisionPreference{
		TimeSovereignty: 5, FinacialGrowth: 1, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	})
	hiFG := MatrixFromPreferences(profile.DecisionPreference{
		TimeSovereignty: 5, FinacialGrowth: 10, HealthRecovery: 5,
		ReputationBuilding: 5, PrivacyProtection: 5, Autonomy: 5,
	})
	if loFG.UtilityThreshold <= hiFG.UtilityThreshold {
		t.Error("UtilityThreshold should decrease as FinancialGrowth increases (inverse)")
	}
}

// ── IsH2HI ────────────────────────────────────────────────────────────────────

func TestIsH2HI_FalseOnStartup(t *testing.T) {
	svc := newTestService(t)
	if svc.IsH2HI() {
		t.Error("fresh kernel should not be H2HI")
	}
}

func TestIsH2HI_TrueWhenStatusH2HI(t *testing.T) {
	svc := newTestService(t)
	svc.kernel.mu.Lock()
	svc.kernel.Status = StatusH2HI
	svc.kernel.mu.Unlock()
	if !svc.IsH2HI() {
		t.Error("want IsH2HI()=true when kernel.Status is H2HI")
	}
}

// ── ApplyBiometricsGate ───────────────────────────────────────────────────────

func TestApplyBiometricsGate_NilCheckInDoesNotShield(t *testing.T) {
	svc := newTestService(t)
	if svc.ApplyBiometricsGate(nil) {
		t.Error("nil check-in must not trigger shield")
	}
	svc.kernel.mu.RLock()
	defer svc.kernel.mu.RUnlock()
	if svc.kernel.Status != StatusOnline {
		t.Errorf("want ONLINE, got %v", svc.kernel.Status)
	}
}

func TestApplyBiometricsGate_HighStressShields(t *testing.T) {
	svc := newTestService(t)
	ci := &biometrics.CheckIn{StressLevel: 8, Sleep: 7}
	if !svc.ApplyBiometricsGate(ci) {
		t.Error("stress>7 should trigger shield")
	}
	svc.kernel.mu.RLock()
	defer svc.kernel.mu.RUnlock()
	if svc.kernel.Status != StatusShielded {
		t.Errorf("want SHIELDED, got %v", svc.kernel.Status)
	}
	// Threshold must be raised
	if svc.kernel.Values.UtilityThreshold <= svc.baseThreshold {
		t.Error("UtilityThreshold should be boosted when shielded")
	}
}

func TestApplyBiometricsGate_LowSleepShields(t *testing.T) {
	svc := newTestService(t)
	ci := &biometrics.CheckIn{StressLevel: 3, Sleep: 4}
	if !svc.ApplyBiometricsGate(ci) {
		t.Error("sleep<5 should trigger shield")
	}
}

func TestApplyBiometricsGate_BoundaryStress7DoesNotShield(t *testing.T) {
	svc := newTestService(t)
	ci := &biometrics.CheckIn{StressLevel: 7, Sleep: 6}
	if svc.ApplyBiometricsGate(ci) {
		t.Error("stress=7 (not >7) should NOT shield")
	}
}

func TestApplyBiometricsGate_BoundarySleep5DoesNotShield(t *testing.T) {
	svc := newTestService(t)
	ci := &biometrics.CheckIn{StressLevel: 3, Sleep: 5}
	if svc.ApplyBiometricsGate(ci) {
		t.Error("sleep=5 (not <5) should NOT shield")
	}
}

func TestApplyBiometricsGate_LiftShieldRestoresThreshold(t *testing.T) {
	svc := newTestService(t)
	// Activate shield
	svc.ApplyBiometricsGate(&biometrics.CheckIn{StressLevel: 9, Sleep: 3})
	base := svc.baseThreshold
	// Lift shield
	if svc.ApplyBiometricsGate(&biometrics.CheckIn{StressLevel: 3, Sleep: 7}) {
		t.Error("normal check-in should lift shield")
	}
	svc.kernel.mu.RLock()
	defer svc.kernel.mu.RUnlock()
	if svc.kernel.Status != StatusOnline {
		t.Errorf("want ONLINE after shield lifted, got %v", svc.kernel.Status)
	}
	if svc.kernel.Values.UtilityThreshold != base {
		t.Errorf("want threshold restored to %.1f, got %.1f", base, svc.kernel.Values.UtilityThreshold)
	}
}

// ── UpdateValues ──────────────────────────────────────────────────────────────

func TestUpdateValues_RebuildMatrix(t *testing.T) {
	svc := newTestService(t)
	newPrefs := profile.DecisionPreference{
		TimeSovereignty:    10,
		FinacialGrowth:     1,
		HealthRecovery:     5,
		ReputationBuilding: 10,
		PrivacyProtection:  1,
		Autonomy:           1,
	}
	svc.UpdateValues(newPrefs)
	svc.kernel.mu.RLock()
	defer svc.kernel.mu.RUnlock()
	if svc.kernel.Values.TemporalSovereignty != 1.0 {
		t.Errorf("want TemporalSovereignty=1.0, got %.2f", svc.kernel.Values.TemporalSovereignty)
	}
}
