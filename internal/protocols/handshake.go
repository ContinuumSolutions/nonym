// Package protocols implements the "Titan Handshake" — the P2P Recursive
// Value-Alignment protocol used when two Ego-Kernels meet and must negotiate
// over a shared resource.
//
// The Titan Handshake runs in the time it takes a human to blink:
//
//	Tier 1 — Zero-Knowledge Negotiation (Ghost Phase): compare Proof of Intent.
//	Tier 2 — Nash-Equilibrium Auction: micro-bidding war in compute credits.
//	Tier 3 — Escalation Ladder: Mirroring → Reputational Poisoning → Resource Starvation.
//	MAS    — Mutually Assured Sanity: deadlock breaker; force H2HI if cost > 10% net worth.
package protocols

import (
	"fmt"
	"math"
	"time"

	"github.com/egokernel/ek1/internal/ledger"
)

// HandshakeParams describes the context for a negotiation.
type HandshakeParams struct {
	ResourceName  string  // What is being negotiated
	UserDesire    float64 // 0–1: how much User's Kernel wants this
	RivalDesire   float64 // 0–1: how much the Rival-AE wants this
	UserRepScore  float64 // User's current reputation score
	RivalRepScore float64 // Rival's current reputation score
	MarketRate    float64 // USD baseline market value of the resource
}

// HandshakeOutcome is the result returned after the full Titan Handshake sequence.
type HandshakeOutcome struct {
	Outcome    string        // "YIELD", "ACCEPTED", "ESCALATED", "DEADLOCK_RESOLVED"
	FinalPrice float64       // agreed contract price in USD
	Duration   time.Duration // simulated time for the handshake
	Log        []string
}

// Handshake manages a single Titan Handshake session between two Kernels.
type Handshake struct {
	userID  string
	rivalID string
	ledger  ledger.Ledger
	log     []string
}

// NewHandshake creates a new Titan Handshake session.
func NewHandshake(userID, rivalID string, rep ledger.Ledger) *Handshake {
	return &Handshake{
		userID:  userID,
		rivalID: rivalID,
		ledger:  rep,
	}
}

// Execute runs the full Titan Handshake protocol and returns the outcome.
func (h *Handshake) Execute(p HandshakeParams) HandshakeOutcome {
	start := time.Now()
	h.emit(fmt.Sprintf("TITAN HANDSHAKE initiated: %s ↔ %s | Resource: %s",
		h.userID, h.rivalID, p.ResourceName))

	// --- Tier 1: Zero-Knowledge Negotiation (Ghost Phase) ---
	h.emit("Tier 1: ZK Negotiation — comparing Proof of Intent...")
	outcome := h.tier1ZeroKnowledge(p)
	if outcome != "" {
		return h.resolve(outcome, p.MarketRate, start)
	}

	// --- Tier 2: Nash-Equilibrium Auction ---
	h.emit("Tier 2: Nash-Equilibrium Auction — entering micro-bidding war...")
	price, nashOutcome := h.tier2NashAuction(p)
	if nashOutcome != "" {
		return h.resolveWithPrice(nashOutcome, price, start)
	}

	// --- Tier 3: Escalation ---
	h.emit("Tier 3: Escalation — Rival-AE refuses fair terms.")
	escalateOutcome, escalatePrice := h.tier3Escalation(p)
	return h.resolveWithPrice(escalateOutcome, escalatePrice, start)
}

// tier1ZeroKnowledge compares desire thresholds and reputation pools.
// If the Rival clearly has higher desire and a stronger favor pool, yield instantly
// to conserve mental energy for winnable battles.
func (h *Handshake) tier1ZeroKnowledge(p HandshakeParams) string {
	h.emit(fmt.Sprintf("  User desire=%.2f | Rival desire=%.2f", p.UserDesire, p.RivalDesire))
	h.emit(fmt.Sprintf("  User rep=%.0f | Rival rep=%.0f", p.UserRepScore, p.RivalRepScore))

	// Yield if Rival has meaningfully higher desire AND reputation (they'll win anyway).
	if p.RivalDesire > p.UserDesire*1.2 && p.RivalRepScore > p.UserRepScore*1.1 {
		h.emit("  Ghost Phase: Rival advantage is clear. Yielding to preserve energy.")
		return "YIELD"
	}

	// If Rival has a bad reputation, flag it and apply a Sovereignty Premium.
	if p.RivalRepScore < 700 {
		h.emit(fmt.Sprintf(
			"  Rival Trust-Weight R=%.0f is low. Applying Sovereignty Premium.",
			p.RivalRepScore,
		))
	}

	return "" // proceed to Tier 2
}

// tier2NashAuction solves for the Nash Equilibrium contract price.
//
//	Contract_Price = MarketRate × 2.5 + ReputationDelta × 10,000
func (h *Handshake) tier2NashAuction(p HandshakeParams) (float64, string) {
	repDelta := math.Max(0, 1000-p.RivalRepScore) / 1000.0
	price := p.MarketRate*2.5 + repDelta*10000

	h.emit(fmt.Sprintf("  Nash price computed: $%.2f (market=%.2f × 2.5 + repDelta=%.4f × 10k)",
		price, p.MarketRate, repDelta))

	// Simulate rival attempting to stall with a "meeting" request.
	h.emit("  Rival-AE: 'We need a meeting to discuss vision.'")
	meetingFee := 5000.0
	h.emit(fmt.Sprintf(
		"  EK-1 counter: 'Meeting fee: $%.0f non-refundable. Or accept the 2.5× multiplier and sign in 3ms.'",
		meetingFee,
	))

	// Rival accepts if they can't afford a reputation hit.
	if p.RivalRepScore < 800 {
		h.emit("  Rival-AE accepted the Sovereignty Premium to avoid Reputation penalty.")
		h.ledger.LogSuccess(h.userID, 100)
		return price, "ACCEPTED"
	}

	return price, "" // escalate
}

// tier3Escalation activates the Aggression Modes.
func (h *Handshake) tier3Escalation(p HandshakeParams) (string, float64) {
	// Mode 1: Mirroring — replicate hostility (Tit-for-Tat).
	h.emit("  Escalation Mode: MIRRORING — signaling we are not a soft target.")

	// Mode 2: Reputational Poisoning — broadcast Dishonesty Hash.
	h.emit("  Escalation Mode: REPUTATIONAL POISONING — broadcasting signed Dishonesty Hash.")
	h.ledger.LogBetrayal(h.rivalID, 1)
	newRivalScore := h.ledger.Score(h.rivalID)
	h.emit(fmt.Sprintf("  Rival-AE Trust Score after flag: %d (transactions now 40%% more expensive)",
		newRivalScore))

	// MAS Protocol: check if deadlock is too costly.
	conflictCost := p.MarketRate * 0.15
	h.emit(fmt.Sprintf("  MAS Protocol: conflict cost estimate $%.2f — forcing H2HI reset.", conflictCost))
	h.emit(fmt.Sprintf(
		"  Notification sent: 'Deadlock with %s. Irrational/highly leveraged. Recommend physical reset.'",
		h.rivalID,
	))

	return "DEADLOCK_RESOLVED", p.MarketRate // settle at market rate after MAS
}

func (h *Handshake) resolve(outcome string, price float64, start time.Time) HandshakeOutcome {
	return h.resolveWithPrice(outcome, price, start)
}

func (h *Handshake) resolveWithPrice(outcome string, price float64, start time.Time) HandshakeOutcome {
	elapsed := time.Since(start)
	h.emit(fmt.Sprintf("HANDSHAKE COMPLETE: %s | price=$%.2f | duration=%s", outcome, price, elapsed))
	return HandshakeOutcome{
		Outcome:    outcome,
		FinalPrice: price,
		Duration:   elapsed,
		Log:        h.log,
	}
}

func (h *Handshake) emit(msg string) {
	h.log = append(h.log, msg)
}
