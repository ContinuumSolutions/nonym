// Package ledger provides the Reputation Ledger interface and a local in-memory
// implementation for Phase 1 (Shadow) and Phase 2 (Hand) development.
//
// In Phase 3 (The Voice), this is replaced by the on-chain Anchor program
// deployed to Solana Devnet — the `programs/ek-logic/src/lib.rs` contract.
//
// Reputation formula (temporal decay):
//
//	R(t) = ∫[−∞,t] [ ΣSᵢ(τ) − ΣBⱼ(τ)·Φ ] · e^(−λ(t−τ)) dτ
//
// Tier mapping:
//
//	≥ 980  → Sovereign  (98th percentile): zero-deposit contracts
//	500–979 → Stable    (50th–97th):       standard rates
//	100–499 → Volatile  (10th–49th):       20% trust tax
//	< 100  → Exiled    (< 10th):          automated blacklisting
package ledger

import (
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	BaselineScore       = 1000
	BetrayalMultiplier  = 5    // betrayal costs 5× a success
	ExileThreshold      = 100
	DecayConstantLambda = 0.01 // reputation from 10 years ago decays naturally
)

// ReputationTier categorizes a Kernel's standing in the network.
type ReputationTier string

const (
	TierSovereign ReputationTier = "SOVEREIGN"
	TierStable    ReputationTier = "STABLE"
	TierVolatile  ReputationTier = "VOLATILE"
	TierExiled    ReputationTier = "EXILED"
)

// TrustTax returns the additional cost multiplier for a given tier.
func (t ReputationTier) TrustTax() float64 {
	switch t {
	case TierSovereign:
		return 0.0
	case TierStable:
		return 0.0
	case TierVolatile:
		return 0.20
	case TierExiled:
		return math.Inf(1) // no transactions allowed
	default:
		return 0.0
	}
}

// InteractionRecord is a single event on the reputation ledger.
type InteractionRecord struct {
	Timestamp time.Time
	Success   bool
	Impact    int64 // base impact before multiplier
}

// LocalLedger is a thread-safe in-memory reputation store for development.
// Replace with Solana RPC calls in Phase 3.
type LocalLedger struct {
	mu      sync.RWMutex
	records map[string][]InteractionRecord
	exiled  map[string]bool
}

// NewLocalLedger creates a new in-memory ledger.
func NewLocalLedger() *LocalLedger {
	return &LocalLedger{
		records: make(map[string][]InteractionRecord),
		exiled:  make(map[string]bool),
	}
}

// Initialize seeds a new Kernel with the baseline reputation score.
func (l *LocalLedger) Initialize(uid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.records[uid]; !exists {
		l.records[uid] = []InteractionRecord{
			{Timestamp: time.Now(), Success: true, Impact: BaselineScore},
		}
	}
}

// LogSuccess records a successful interaction and increases the score.
func (l *LocalLedger) LogSuccess(uid string, impact int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records[uid] = append(l.records[uid], InteractionRecord{
		Timestamp: time.Now(),
		Success:   true,
		Impact:    impact,
	})
}

// LogBetrayal records a failed/dishonest interaction.
// The penalty is impact × BetrayalMultiplier; the record is permanent.
func (l *LocalLedger) LogBetrayal(uid string, impact int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records[uid] = append(l.records[uid], InteractionRecord{
		Timestamp: time.Now(),
		Success:   false,
		Impact:    impact * BetrayalMultiplier,
	})
}

// Score computes the current reputation score for a Kernel using temporal decay.
func (l *LocalLedger) Score(uid string) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	var score float64

	for _, rec := range l.records[uid] {
		age := now.Sub(rec.Timestamp).Hours() / (24 * 365) // age in years
		decayFactor := math.Exp(-DecayConstantLambda * age)

		if rec.Success {
			score += float64(rec.Impact) * decayFactor
		} else {
			score -= float64(rec.Impact) * decayFactor
		}
	}

	if score < 0 {
		score = 0
	}

	result := int64(math.Round(score))
	// Check exile.
	if result < ExileThreshold {
		l.exiled[uid] = true
	}
	return result
}

// Tier returns the reputation tier for a given Kernel.
func (l *LocalLedger) Tier(uid string) ReputationTier {
	s := l.Score(uid)
	switch {
	case s >= 980:
		return TierSovereign
	case s >= 500:
		return TierStable
	case s >= ExileThreshold:
		return TierVolatile
	default:
		return TierExiled
	}
}

// IsExiled returns true if the Kernel is in the exile state.
func (l *LocalLedger) IsExiled(uid string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.exiled[uid]
}

// Summary prints a formatted reputation summary for a Kernel.
func (l *LocalLedger) Summary(uid string) string {
	score := l.Score(uid)
	tier := l.Tier(uid)
	tax := tier.TrustTax()

	if math.IsInf(tax, 1) {
		return fmt.Sprintf("%-30s | Score: EXILED | Tier: %s | Trust Tax: ∞ (blacklisted)", uid, tier)
	}
	return fmt.Sprintf("%-30s | Score: %5d | Tier: %-9s | Trust Tax: %.0f%%",
		uid, score, tier, tax*100)
}
