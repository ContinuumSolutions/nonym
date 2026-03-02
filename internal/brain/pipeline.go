package brain

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/datasync"
)

const shieldNote = "Note: kernel operating in reduced-load mode due to elevated stress."

// PipelineResult summarises one full sync cycle through the brain.
type PipelineResult struct {
	Total        int    `json:"total"`
	Accepted     int    `json:"accepted"`     // passed Triage + Decide → Decision: Automated
	Rejected     int    `json:"rejected"`     // failed Triage or Decide → Decision: Declined
	Ghosted      int    `json:"ghosted"`      // manipulation detected → Decision: Declined (ghost)
	Shielded     bool   `json:"shielded"`     // biometrics gate was active during this run
	ShieldReason string `json:"shield_reason,omitempty"` // why shield was active
}

// Analyser is satisfied by *ai.Client and can be stubbed in tests.
type Analyser interface {
	AnalyseBatch(ctx context.Context, signals []datasync.RawSignal) ([]*ai.AnalysedSignal, []error)
}

// Pipeline is the full RawSignal → LLM → Triage → Decide → Event flow.
// It is constructed once at startup and called by the scheduler (step 9).
type Pipeline struct {
	svc        *Service
	ai         Analyser
	events     *activities.Store
	biometrics *biometrics.Store
}

// NewPipeline wires the AI client, brain service, activities store, and biometrics store.
func NewPipeline(svc *Service, aiClient Analyser, events *activities.Store, bio *biometrics.Store) *Pipeline {
	return &Pipeline{svc: svc, ai: aiClient, events: events, biometrics: bio}
}

// Run processes a batch of raw signals end-to-end:
//
//	Biometrics gate → RawSignal → LLM analysis → Triage → (Accept → Decide) → Write Event
//
// Every signal produces exactly one Event row regardless of outcome.
// In Stage 1 (Shadow), Decision: Automated means "would have acted" — no real action is taken.
func (p *Pipeline) Run(ctx context.Context, signals []datasync.RawSignal) (PipelineResult, error) {
	result := PipelineResult{Total: len(signals)}

	// ── Step 0: Biometrics gate ──────────────────────────────────────────────
	checkIn, err := p.biometrics.Get()
	if err != nil && !errors.Is(err, biometrics.ErrNotFound) {
		log.Printf("brain/pipeline: read biometrics: %v", err)
	}
	if errors.Is(err, biometrics.ErrNotFound) {
		checkIn = nil
	}

	shielded := p.svc.ApplyBiometricsGate(checkIn)
	if shielded {
		result.Shielded = true
		result.ShieldReason = fmt.Sprintf(
			"stress=%d sleep=%.1f — UtilityThreshold raised %.0f%%",
			checkIn.StressLevel, checkIn.Sleep, (shieldMultiplier-1)*100,
		)
	}

	if len(signals) == 0 {
		return result, nil
	}

	// ── Step 1: LLM analysis — concurrent, capped at 3 goroutines ───────────
	analysed, errs := p.ai.AnalyseBatch(ctx, signals)

	// ── Step 2: Triage + Decide + Write ─────────────────────────────────────
	for i, as := range analysed {
		if as == nil {
			log.Printf("brain/pipeline: signal[%d] LLM error: %v — skipping", i, errs[i])
			continue
		}

		req := IncomingRequest{
			ID:              as.Request.ID,
			SenderID:        as.Request.SenderID,
			Description:     as.Request.Description,
			EstimatedROI:    as.Request.EstimatedROI,
			TimeCommitment:  as.Request.TimeCommitment,
			ManipulationPct: as.Request.ManipulationPct,
		}
		action, reason := p.svc.kernel.Triage(req)

		var (
			decision  activities.Decision
			narrative string
		)

		switch action {
		case "GHOST":
			decision = activities.Declined
			narrative = fmt.Sprintf("Ghosted: %s", reason)
			result.Ghosted++

		case "REJECT":
			decision = activities.Declined
			narrative = fmt.Sprintf("Rejected: %s", reason)
			result.Rejected++

		case "ACCEPT":
			op := TradeOpportunity{
				Name:           fmt.Sprintf("%s — %s", as.Signal.ServiceSlug, as.Signal.Title),
				ExpectedROI:    as.Request.EstimatedROI,
				TimeCommitment: as.Request.TimeCommitment,
				ReputationRisk: as.Request.ManipulationPct,
			}
			eval := p.svc.kernel.Decide(op)
			if eval.Execute {
				decision = activities.Automated
				narrative = fmt.Sprintf("%s | %s", as.Narrative, eval.Reason)
				p.svc.ledger.LogSuccess(p.svc.uid, int64(eval.Utility))
				result.Accepted++
			} else {
				decision = activities.Declined
				narrative = fmt.Sprintf("Rejected post-decide: %s", eval.Reason)
				result.Rejected++
			}

		default:
			decision = activities.Declined
			narrative = fmt.Sprintf("Unknown triage action %q: %s", action, reason)
			result.Rejected++
		}

		// Prepend the shield annotation when the biometrics gate is active.
		if shielded {
			narrative = fmt.Sprintf("[SHIELDED] %s | %s", shieldNote, narrative)
		}

		event := activities.Event{
			EventType:     as.EventType,
			Decision:      decision,
			Importance:    as.Importance,
			Narrative:     narrative,
			Gain:          as.Gain,
			SourceService: as.Signal.ServiceSlug,
		}
		if _, err := p.events.Create(event); err != nil {
			log.Printf("brain/pipeline: write event for signal[%d]: %v", i, err)
		}
	}

	return result, nil
}
