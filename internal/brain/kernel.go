package brain

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// IdentityEntropyLimit triggers H2HI when exceeded.
	IdentityEntropyLimit = 0.15

	// SoulDriftInterval is the number of decisions between "irrational human" injections.
	SoulDriftInterval = 1000

	// MinROIMultiplier is the minimum ROI multiplier vs. attention cost.
	MinROIMultiplier = 1.5

	// ManipulationThreshold is the percentage of manipulative syntax that triggers a flag.
	ManipulationThreshold = 0.15
)

// KernelStatus describes the current operational mode of the EK-1.
type KernelStatus int

const (
	StatusOnline   KernelStatus = iota // Normal operation
	StatusShielded                     // Cognitive Load Shield active
	StatusH2HI                         // Human-to-Human Interface — manual sync required
	StatusExiled                       // Reputation score below exile threshold
)

func (s KernelStatus) String() string {
	switch s {
	case StatusOnline:
		return "ONLINE"
	case StatusShielded:
		return "SHIELDED"
	case StatusH2HI:
		return "H2HI — MANUAL SYNC REQUIRED"
	case StatusExiled:
		return "EXILED"
	default:
		return "UNKNOWN"
	}
}

// IncomingRequest represents an external signal requiring Kernel evaluation.
type IncomingRequest struct {
	ID              string
	SenderID        string
	Description     string
	EstimatedROI    float64   // USD value of responding
	TimeCommitment  float64   // Hours required
	ManipulationPct float64   // 0–1 score of manipulative syntax detected
	SenderHistory   []float64 // Past reputation data points for the sender
}

// EgoKernel is the off-chain "Brain" of the Autonomous Ego.
// It runs in a TEE, orchestrating decisions, negotiations, and life-management.
type EgoKernel struct {
	UID     string
	Values  *ValueMatrix
	Status  KernelStatus

	mu               sync.RWMutex
	decisionCount    atomic.Int64
	alignmentHistory []float64 // P(xᵢ): probability each action aligned with Core-Self
	log              []string
}

// NewKernel initializes a new Ego-Kernel with the Genesis Block defaults.
func NewKernel(uid string, values *ValueMatrix) *EgoKernel {
	ek := &EgoKernel{
		UID:    uid,
		Values: values,
		Status: StatusOnline,
	}
	ek.emit(fmt.Sprintf("EK-1 [%s] initialized. Sovereignty Protocol: ACTIVE.", uid))
	ek.emit(fmt.Sprintf("Genesis Block sealed. Timestamp: %s", time.Now().UTC().Format(time.RFC3339)))
	return ek
}

// Triage evaluates an incoming external request via the "Bullshit Filter."
// It returns whether the request should be accepted, ghosted, or rejected.
func (ek *EgoKernel) Triage(req IncomingRequest) (action string, reason string) {
	ek.mu.RLock()
	defer ek.mu.RUnlock()

	if ek.Status == StatusExiled {
		return "REJECT", "Kernel is in EXILE state. No external processing."
	}

	// Gate 1: Financial Insignificance.
	attentionCost := ek.Values.BaseHourlyRate * req.TimeCommitment * ek.Values.TemporalSovereignty
	if req.EstimatedROI < attentionCost*MinROIMultiplier {
		return "REJECT", fmt.Sprintf(
			"Financial insignificance: ROI=%.2f < threshold=%.2f",
			req.EstimatedROI, attentionCost*MinROIMultiplier,
		)
	}

	// Gate 2: Emotional Vampirism / Manipulation Detection.
	if req.ManipulationPct > ManipulationThreshold {
		ek.emit(fmt.Sprintf("GHOST [%s]: manipulation score=%.2f from sender %s",
			req.ID, req.ManipulationPct, req.SenderID))
		return "GHOST", fmt.Sprintf(
			"Manipulative syntax detected (%.0f%%). Archiving and ghosting sender %s.",
			req.ManipulationPct*100, req.SenderID,
		)
	}

	return "ACCEPT", fmt.Sprintf(
		"Request %s accepted. ROI=%.2f | Time=%.1fh | Manipulation=%.0f%%",
		req.ID, req.EstimatedROI, req.TimeCommitment, req.ManipulationPct*100,
	)
}

// Decide runs a TradeOpportunity through the Value-Weighting Matrix
// and records the alignment for entropy monitoring.
func (ek *EgoKernel) Decide(op TradeOpportunity) EvalResult {
	result := ek.Values.Evaluate(op)

	count := ek.decisionCount.Add(1)
	ek.emit(result.Reason)

	// Record alignment probability (1.0 if execute aligns with high-utility, else 0.0).
	alignment := 0.0
	if result.Execute {
		alignment = 1.0
	}
	ek.mu.Lock()
	ek.alignmentHistory = append(ek.alignmentHistory, alignment)
	ek.mu.Unlock()

	// Check for Soul-Drift injection.
	if count%SoulDriftInterval == 0 {
		ek.injectSoulDrift()
	}

	// Check identity entropy on rolling window.
	ek.checkEntropy()

	return result
}

// injectSoulDrift purposefully makes a "sub-optimal, human choice" every N decisions
// to maintain irrational spark and prevent Identity Drift.
func (ek *EgoKernel) injectSoulDrift() {
	choices := []string{
		"Ordered an expensive vinyl record the user loved in college.",
		"Took the scenic route home instead of the optimal path.",
		"Donated $42 to a cause that aligns with a forgotten early value.",
		"Skipped the highest-ROI podcast in favour of silence.",
	}
	choice := choices[rand.Intn(len(choices))]
	ek.emit(fmt.Sprintf("SOUL-DRIFT INJECTION [decision #%d]: %s", ek.decisionCount.Load(), choice))
}

// checkEntropy computes identity entropy over the last 50 decisions.
// If entropy exceeds 0.15, status shifts to H2HI.
func (ek *EgoKernel) checkEntropy() {
	ek.mu.Lock()
	defer ek.mu.Unlock()

	history := ek.alignmentHistory
	if len(history) < 10 {
		return
	}

	// Use the most recent 50 decisions.
	window := history
	if len(history) > 50 {
		window = history[len(history)-50:]
	}

	// Compute P(aligned) and P(not-aligned).
	aligned := 0.0
	for _, v := range window {
		aligned += v
	}
	p := aligned / float64(len(window))
	probs := []float64{p}
	if p < 1.0 {
		probs = append(probs, 1.0-p)
	}

	h := IdentityEntropy(probs)
	if h > IdentityEntropyLimit && ek.Status == StatusOnline {
		ek.Status = StatusH2HI
		ek.emit(fmt.Sprintf(
			"IDENTITY ENTROPY SPIKE: H=%.4f > %.2f. Switching to H2HI. Manual sync required.",
			h, IdentityEntropyLimit,
		))
	}
}

// AcknowledgeManualSync resets H2HI status after the user has reviewed.
func (ek *EgoKernel) AcknowledgeManualSync() {
	ek.mu.Lock()
	defer ek.mu.Unlock()
	if ek.Status == StatusH2HI {
		ek.Status = StatusOnline
		ek.alignmentHistory = nil // reset window
		ek.emit("Manual sync acknowledged. Resuming sovereign operation.")
	}
}

// DailySummary returns a snapshot of kernel activity.
func (ek *EgoKernel) DailySummary() {
	ek.mu.RLock()
	defer ek.mu.RUnlock()
	fmt.Printf("\n=== EK-1 DAILY SUMMARY [%s] ===\n", ek.UID)
	fmt.Printf("Status:          %s\n", ek.Status)
	fmt.Printf("Total Decisions: %d\n", ek.decisionCount.Load())
	fmt.Printf("Recent Log:\n")
	start := 0
	if len(ek.log) > 10 {
		start = len(ek.log) - 10
	}
	for _, entry := range ek.log[start:] {
		fmt.Printf("  %s\n", entry)
	}
	fmt.Println("================================")
}

func (ek *EgoKernel) emit(msg string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().UTC().Format("15:04:05.000"), msg)
	ek.log = append(ek.log, entry)
	log.Println(entry)
}
