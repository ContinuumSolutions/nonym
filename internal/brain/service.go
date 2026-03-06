package brain

import (
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/profile"
)

// Service provides simplified preference and biometrics management.
// Replaces the complex EgoKernel with focus on biometric-driven decisions.
type Service struct {
	uid         string
	preferences profile.DecisionPreference
}

// NewService creates a simplified brain service focused on preferences.
func NewService(uid string, prefs profile.DecisionPreference) *Service {
	return &Service{
		uid:         uid,
		preferences: prefs,
	}
}

// ApplyBiometricsGate checks if the user's biometric state suggests
// they should operate in a reduced-load mode (high stress, low sleep).
// Returns true if shielding is active.
func (s *Service) ApplyBiometricsGate(checkIn *biometrics.CheckIn) bool {
	if checkIn == nil {
		return false // No biometrics data, no shielding
	}

	// Shield if stress is high OR sleep is poor
	highStress := checkIn.StressLevel > 7
	lowSleep := checkIn.Sleep < 5

	return highStress || lowSleep
}

// GetPreferences returns the current user preferences.
func (s *Service) GetPreferences() profile.DecisionPreference {
	return s.preferences
}

// UpdatePreferences updates the stored preferences.
func (s *Service) UpdatePreferences(prefs profile.DecisionPreference) {
	s.preferences = prefs
}