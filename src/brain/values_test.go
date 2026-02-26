package brain

import (
	"math"
	"testing"
)

func TestDefaultMatrix(t *testing.T) {
	vm := DefaultMatrix()
	if vm.TemporalSovereignty != 0.80 {
		t.Errorf("expected TemporalSovereignty=0.80, got %.2f", vm.TemporalSovereignty)
	}
	if vm.RiskTolerance != 0.20 {
		t.Errorf("expected RiskTolerance=0.20, got %.2f", vm.RiskTolerance)
	}
}

func TestEvaluate_RejectsLowROI(t *testing.T) {
	vm := DefaultMatrix()
	op := TradeOpportunity{
		Name:           "low-roi-meeting",
		ExpectedROI:    50,
		TimeCommitment: 1.0, // 1 hour × $500 × 0.8 = $400 cognitive tax → ROI too low
		ReputationRisk: 0.0,
	}
	result := vm.Evaluate(op)
	if result.Execute {
		t.Errorf("expected REJECT for low-ROI opportunity, got EXECUTE")
	}
}

func TestEvaluate_AcceptsAutoArbitrage(t *testing.T) {
	vm := DefaultMatrix()
	op := TradeOpportunity{
		Name:           "auto-arbitrage",
		ExpectedROI:    1200,
		TimeCommitment: 0.01, // near-zero oversight
		ReputationRisk: 0.01,
	}
	result := vm.Evaluate(op)
	if !result.Execute {
		t.Errorf("expected EXECUTE for low-time, low-risk opportunity: %s", result.Reason)
	}
}

func TestEvaluate_RejectsHighRisk(t *testing.T) {
	vm := DefaultMatrix()
	op := TradeOpportunity{
		Name:           "risky-deal",
		ExpectedROI:    50000,
		TimeCommitment: 0.0,
		ReputationRisk: 0.5, // exceeds RiskTolerance=0.20
	}
	result := vm.Evaluate(op)
	if result.Execute {
		t.Errorf("expected REJECT for high-risk opportunity")
	}
}

func TestEvaluate_HighRoiHighTime_Rejected(t *testing.T) {
	vm := DefaultMatrix()
	// $5000 ROI but 10 hours × $500 × 0.8 = $4000 cognitive tax → adj = $1000 → utility too low
	op := TradeOpportunity{
		Name:           "vc-prestige-project",
		ExpectedROI:    5000,
		TimeCommitment: 10.0,
		ReputationRisk: 0.10,
	}
	result := vm.Evaluate(op)
	if result.Execute {
		t.Errorf("expected REJECT for time-heavy deal with insufficient adj ROI")
	}
}

func TestIdentityEntropy_LowForAligned(t *testing.T) {
	probs := []float64{0.99, 0.01}
	h := IdentityEntropy(probs)
	if h > IdentityEntropyLimit {
		t.Errorf("expected low entropy for aligned distribution, got H=%.4f", h)
	}
}

func TestIdentityEntropy_HighForUniform(t *testing.T) {
	probs := []float64{0.50, 0.50}
	h := IdentityEntropy(probs)
	expected := math.Log(2) // max entropy for binary
	if math.Abs(h-expected) > 0.001 {
		t.Errorf("expected H≈%.4f for uniform binary, got H=%.4f", expected, h)
	}
}

func TestValueIntegratedUtility_PositiveHorizon(t *testing.T) {
	weights := []float64{0.8, 0.2}
	satisfactions := []float64{100.0, 50.0}
	u := ValueIntegratedUtility(weights, satisfactions, 0.05, 10)
	if u <= 0 {
		t.Errorf("expected positive utility, got %.4f", u)
	}
}
