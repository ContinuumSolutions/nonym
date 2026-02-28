package brain

import (
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/profile"
)

// Service is the runtime brain — an EgoKernel paired with its Ledger.
// Initialised once at startup from the stored profile; values can be
// hot-reloaded via UpdateValues when the user updates their preferences.
type Service struct {
	uid    string
	kernel *EgoKernel
	ledger *ledger.LocalLedger
}

// NewService initialises the kernel and ledger from the user's stored preferences.
func NewService(uid string, prefs profile.DecisionPreference) *Service {
	l := ledger.NewLocalLedger()
	l.Initialize(uid)

	vm := MatrixFromPreferences(prefs)
	k := NewKernel(uid, vm)

	return &Service{uid: uid, kernel: k, ledger: l}
}

// Kernel returns the underlying EgoKernel for use by other packages (e.g. scheduler).
func (s *Service) Kernel() *EgoKernel { return s.kernel }

// Ledger returns the reputation ledger for use by other packages.
func (s *Service) Ledger() *ledger.LocalLedger { return s.ledger }

// UpdateValues rebuilds the ValueMatrix from updated preferences and applies it
// to the running kernel. Safe to call at any time.
func (s *Service) UpdateValues(prefs profile.DecisionPreference) {
	vm := MatrixFromPreferences(prefs)
	s.kernel.mu.Lock()
	s.kernel.Values = vm
	s.kernel.mu.Unlock()
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
