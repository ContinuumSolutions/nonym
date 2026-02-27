package brain

import (
	"fmt"
	"math"
)

// ValuePriority defines the enforcement level of a value module.
type ValuePriority int

const (
	PriorityCritical ValuePriority = iota
	PriorityHigh
	PriorityMedium
	PriorityLow
)

func (p ValuePriority) String() string {
	switch p {
	case PriorityCritical:
		return "CRITICAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityMedium:
		return "MEDIUM"
	case PriorityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// ValueMatrix encodes the biological and philosophical constraints of the user.
// These weights define the "gut feeling" in binary bedrock.
type ValueMatrix struct {
	// TemporalSovereignty: how much the user values unstructured free time (0–1).
	// CRITICAL priority. Any interaction yielding < 1.5× ROI on attention is culled.
	TemporalSovereignty float64

	// RiskTolerance: 0.05 = very conservative, 0.90 = degen.
	// MEDIUM priority. Mapped to a configurable σ.
	RiskTolerance float64

	// ReputationImpact: how much the user cares about ledger cleanliness (0–1).
	// HIGH priority. Predatory trades incur a reputation cost.
	ReputationImpact float64

	// SocialEntropy: desire to maintain human-like friction (prevents appearing robotic).
	// LOW priority. Set > 0 to inject periodic "irrational" human choices.
	SocialEntropy float64

	// BaseHourlyRate is the user's baseline value of one hour (in USD).
	// Used to compute the Cognitive Tax on time-consuming opportunities.
	BaseHourlyRate float64

	// UtilityThreshold is the minimum computed utility required to execute an action.
	// Actions below this are automatically culled.
	UtilityThreshold float64

	// PresentBiasDiscount (ρ) is the user's personal discount rate for future rewards.
	// Higher ρ = more present-biased (prefers immediate gains).
	PresentBiasDiscount float64
}

// DefaultMatrix returns a sensible sovereign default.
func DefaultMatrix() *ValueMatrix {
	return &ValueMatrix{
		TemporalSovereignty: 0.80, // values time highly
		RiskTolerance:       0.20, // steady wins; no gambles
		ReputationImpact:    0.90, // wants a clean ledger
		SocialEntropy:       0.10, // minimal human-friction injection
		BaseHourlyRate:      500.0,
		UtilityThreshold:    1000.0,
		PresentBiasDiscount: 0.05,
	}
}

// TradeOpportunity describes a potential action the Kernel is evaluating.
type TradeOpportunity struct {
	// Name is a human-readable label for logging.
	Name string

	// ExpectedROI is the gross expected return in USD.
	ExpectedROI float64

	// TimeCommitment is the number of hours of active cognitive attention required.
	TimeCommitment float64

	// ReputationRisk is the probability (0–1) of a negative ledger hit if executed.
	ReputationRisk float64
}

// EvalResult is the output of the Value-Weighting Matrix evaluation.
type EvalResult struct {
	Execute      bool
	AdjustedROI  float64
	Utility      float64
	Reason       string
}

// Evaluate runs the Value-Integrated Utility calculation and returns a decision.
//
// U = adjustedROI × exp(−reputationRisk / (1 − reputationImpact))
// adjustedROI = expectedROI − (timeCommitment × baseHourlyRate × temporalSovereignty)
func (vm *ValueMatrix) Evaluate(op TradeOpportunity) EvalResult {
	// 1. Cognitive Tax: subtract the cost of time from the gross ROI.
	cognitiveTax := op.TimeCommitment * vm.BaseHourlyRate * vm.TemporalSovereignty
	adjustedROI := op.ExpectedROI - cognitiveTax

	// 2. Reputation-Adjusted Utility: drop exponentially when reputation risk is high
	//    relative to the user's tolerance for reputational damage.
	safetyFactor := 1.0 - vm.ReputationImpact
	if safetyFactor < 1e-9 {
		safetyFactor = 1e-9 // prevent division by zero for 100% reputation-conscious users
	}
	utility := adjustedROI * math.Exp(-op.ReputationRisk/safetyFactor)

	// 3. Sovereign Threshold gate.
	if utility <= vm.UtilityThreshold {
		return EvalResult{
			Execute:     false,
			AdjustedROI: adjustedROI,
			Utility:     utility,
			Reason: fmt.Sprintf(
				"REJECT [%s]: utility=%.2f below threshold=%.2f",
				op.Name, utility, vm.UtilityThreshold,
			),
		}
	}

	// 4. Risk gate.
	if op.ReputationRisk >= vm.RiskTolerance {
		return EvalResult{
			Execute:     false,
			AdjustedROI: adjustedROI,
			Utility:     utility,
			Reason: fmt.Sprintf(
				"REJECT [%s]: reputation risk=%.2f exceeds tolerance=%.2f",
				op.Name, op.ReputationRisk, vm.RiskTolerance,
			),
		}
	}

	return EvalResult{
		Execute:     true,
		AdjustedROI: adjustedROI,
		Utility:     utility,
		Reason: fmt.Sprintf(
			"EXECUTE [%s]: utility=%.2f | adjROI=%.2f | risk=%.2f",
			op.Name, utility, adjustedROI, op.ReputationRisk,
		),
	}
}

// ValueIntegratedUtility computes the 10-year discounted utility of an action.
// This is the Monte Carlo Moral Simulation used for high-stakes decisions.
//
//	U_total = ∫[0,T] e^(−ρt) · [Σ ωᵢ(t) · ψᵢ(a,t)] dt
func ValueIntegratedUtility(weights []float64, satisfactions []float64, rho float64, horizon int) float64 {
	if len(weights) != len(satisfactions) {
		return 0
	}
	total := 0.0
	dt := 1.0 // discrete time steps (years)
	for t := 0; t < horizon; t++ {
		discountFactor := math.Exp(-rho * float64(t))
		weightedSum := 0.0
		for i := range weights {
			weightedSum += weights[i] * satisfactions[i]
		}
		total += discountFactor * weightedSum * dt
	}
	return total
}

// IdentityEntropy computes Shannon entropy over the action-alignment distribution.
// If H(P) > 0.15, the Kernel halts and triggers H2HI (Human-to-Human Interface).
//
//	H(P) = −Σ P(xᵢ) · log(P(xᵢ))
func IdentityEntropy(probabilities []float64) float64 {
	h := 0.0
	for _, p := range probabilities {
		if p > 0 {
			h -= p * math.Log(p)
		}
	}
	return h
}
