package brain

import (
	"fmt"

	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/profile"
)

// shieldMultiplier is the factor by which UtilityThreshold is raised when the
// biometrics gate detects elevated stress or poor sleep.
const shieldMultiplier = 1.5

// Service is the runtime brain — an EgoKernel paired with its Ledger.
// Initialised once at startup from the stored profile; values can be
// hot-reloaded via UpdateValues when the user updates their preferences.
type Service struct {
	uid    string
	kernel *EgoKernel
	ledger ledger.Ledger

	// baseThreshold stores the unshielded UtilityThreshold so it can be
	// restored when the biometrics gate lifts the shield.
	baseThreshold float64
}

// NewService initialises the kernel from the user's stored preferences.
// The ledger is injected so it can be swapped between implementations
// (SQLiteLedger for Phase 1/2, Solana RPC for Phase 3).
func NewService(uid string, prefs profile.DecisionPreference, l ledger.Ledger) *Service {
	vm := MatrixFromPreferences(prefs)
	k := NewKernel(uid, vm)
	return &Service{uid: uid, kernel: k, ledger: l}
}

// Kernel returns the underlying EgoKernel for use by other packages (e.g. scheduler).
func (s *Service) Kernel() *EgoKernel { return s.kernel }

// Ledger returns the reputation ledger for use by other packages.
func (s *Service) Ledger() ledger.Ledger { return s.ledger }

// UpdateValues rebuilds the ValueMatrix from updated preferences and applies it
// to the running kernel. Safe to call at any time.
func (s *Service) UpdateValues(prefs profile.DecisionPreference) {
	vm := MatrixFromPreferences(prefs)
	s.kernel.mu.Lock()
	s.kernel.Values = vm
	s.baseThreshold = vm.UtilityThreshold
	s.kernel.mu.Unlock()
}

// ApplyBiometricsGate checks today's biometrics and updates the kernel status.
//
// Shield conditions (from PLAN.md step 8):
//   - StressLevel > 7 OR Sleep < 5 → StatusShielded, UtilityThreshold × shieldMultiplier
//   - Otherwise                    → StatusOnline (if was Shielded), restore threshold
//
// Returns true if the shield is active after the call.
// Safe to call on every pipeline run; idempotent for unchanged conditions.
func (s *Service) ApplyBiometricsGate(checkIn *biometrics.CheckIn) bool {
	shielded := checkIn != nil && (checkIn.StressLevel > 7 || checkIn.Sleep < 5)

	s.kernel.mu.Lock()
	defer s.kernel.mu.Unlock()

	if shielded && s.kernel.Status != StatusShielded {
		// Save the current threshold before boosting.
		s.baseThreshold = s.kernel.Values.UtilityThreshold
		boosted := s.baseThreshold * shieldMultiplier
		s.kernel.Values.UtilityThreshold = boosted
		s.kernel.Status = StatusShielded
		s.kernel.emit(fmt.Sprintf(
			"BIOMETRICS GATE: StatusShielded activated — stress=%d sleep=%d. "+
				"UtilityThreshold raised %.0f → %.0f.",
			checkIn.StressLevel, checkIn.Sleep, s.baseThreshold, boosted,
		))
	} else if !shielded && s.kernel.Status == StatusShielded {
		if s.baseThreshold > 0 {
			s.kernel.Values.UtilityThreshold = s.baseThreshold
		}
		s.kernel.Status = StatusOnline
		s.kernel.emit("BIOMETRICS GATE: Shield lifted — kernel returning to ONLINE mode.")
	}

	return shielded
}

// MatrixFromPreferences translates a 1–10 DecisionPreference into the float
// ranges the ValueMatrix expects. HealthRecovery is handled in step 8
// (biometrics gate) and is intentionally excluded here.
//
// Mappings (verified against DefaultMatrix values at scale=5):
//
//	TimeSovereignty  (1–10) → TemporalSovereignty  (0.1–1.0)
//	FinancialGrowth  (1–10) → UtilityThreshold     (2000→200, inverse)
//	ReputationBuilding(1–10)→ ReputationImpact      (0.1–1.0)
//	PrivacyProtection(1–10) → RiskTolerance         (0.32→0.05, inverse)
//	Autonomy         (1–10) → SocialEntropy         (0.14→0.05, inverse)
func MatrixFromPreferences(p profile.DecisionPreference) *ValueMatrix {
	return &ValueMatrix{
		// Direct mappings
		TemporalSovereignty: float64(p.TimeSovereignty) / 10.0,
		ReputationImpact:    float64(p.ReputationBuilding) / 10.0,

		// Inverse mappings (higher preference = lower value)
		// FG=5 → 1200, close to DefaultMatrix(1000); full range 200–2000
		UtilityThreshold: 2200.0 - float64(p.FinacialGrowth)*200.0,
		// PP=5 → 0.20, matches DefaultMatrix; range 0.05–0.32
		RiskTolerance: 0.05 + (1.0-float64(p.PrivacyProtection)/10.0)*0.30,
		// A=5 → 0.10, matches DefaultMatrix; range 0.05–0.14
		SocialEntropy: 0.05 + (1.0-float64(p.Autonomy)/10.0)*0.10,

		// Fixed defaults — extended in future steps
		BaseHourlyRate:      500.0,
		PresentBiasDiscount: 0.05,
	}
}
