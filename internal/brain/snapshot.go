package brain

// KernelSnapshot is the point-in-time state of the kernel returned by the API.
type KernelSnapshot struct {
	Status          string      `json:"status"`
	DecisionCount   int64       `json:"decision_count"`
	IdentityEntropy float64     `json:"identity_entropy"`
	Values          ValueMatrix `json:"values"`
}

// Snapshot captures the current kernel state without modifying it.
func (ek *EgoKernel) Snapshot() KernelSnapshot {
	ek.mu.RLock()
	defer ek.mu.RUnlock()

	var entropy float64
	if len(ek.alignmentHistory) >= 2 {
		window := ek.alignmentHistory
		if len(window) > 50 {
			window = window[len(window)-50:]
		}
		aligned := 0.0
		for _, v := range window {
			aligned += v
		}
		p := aligned / float64(len(window))
		probs := []float64{p}
		if p < 1.0 {
			probs = append(probs, 1.0-p)
		}
		entropy = IdentityEntropy(probs)
	}

	return KernelSnapshot{
		Status:          ek.Status.String(),
		DecisionCount:   ek.decisionCount.Load(),
		IdentityEntropy: entropy,
		Values:          *ek.Values,
	}
}
