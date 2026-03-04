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
	"github.com/egokernel/ek1/internal/execution"
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
	exec       *execution.Engine // Stage 2: execution engine (nil = shadow mode)
}

// NewPipeline wires the AI client, brain service, activities store, biometrics store,
// and optional execution engine (nil disables Stage 2 actions).
func NewPipeline(svc *Service, aiClient Analyser, events *activities.Store, bio *biometrics.Store, exec *execution.Engine) *Pipeline {
	return &Pipeline{svc: svc, ai: aiClient, events: events, biometrics: bio, exec: exec}
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

	// Snapshot the kernel matrix once (stable for this run — biometrics gate already applied).
	p.svc.kernel.mu.RLock()
	vals := *p.svc.kernel.Values
	p.svc.kernel.mu.RUnlock()

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

		// ROI threshold mirrors Triage Gate 1 so we can surface it in analysis.
		roiThreshold := vals.BaseHourlyRate * req.TimeCommitment * vals.TemporalSovereignty * MinROIMultiplier

		analysis := activities.SignalAnalysis{
			ServiceSlug:     as.Signal.ServiceSlug,
			SignalTitle:     as.Signal.Title,
			EstimatedROI:    req.EstimatedROI,
			TimeCommitment:  req.TimeCommitment,
			ManipulationPct: req.ManipulationPct,
			ROIThreshold:    roiThreshold,
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
			analysis.TriageGate = "manipulation"
			result.Ghosted++
			log.Printf("brain/pipeline: [%s] %q → GHOST (manipulation=%.0f%%) %s",
				as.Signal.ServiceSlug, as.Signal.Title, req.ManipulationPct*100, reason)

		case "REJECT":
			decision = activities.Declined
			narrative = fmt.Sprintf("Rejected: %s", reason)
			analysis.TriageGate = "financial_insignificance"
			result.Rejected++
			log.Printf("brain/pipeline: [%s] %q → REJECT (financial_insignificance) roi=%.2f roi_threshold=%.2f time=%.2fh manip=%.0f%%",
				as.Signal.ServiceSlug, as.Signal.Title,
				req.EstimatedROI, roiThreshold, req.TimeCommitment, req.ManipulationPct*100)

		case "ACCEPT":
			op := TradeOpportunity{
				Name:           fmt.Sprintf("%s — %s", as.Signal.ServiceSlug, as.Signal.Title),
				ExpectedROI:    as.Request.EstimatedROI,
				TimeCommitment: as.Request.TimeCommitment,
				ReputationRisk: as.Request.ManipulationPct,
			}
			eval := p.svc.kernel.Decide(op)
			analysis.DecideUtility = eval.Utility
			analysis.DecideThreshold = vals.UtilityThreshold
			if eval.Execute {
				narrative = fmt.Sprintf("%s | %s", as.Narrative, eval.Reason)
				analysis.TriageGate = "accepted"
				p.svc.ledger.LogSuccess(p.svc.uid, int64(eval.Utility))
				result.Accepted++
				log.Printf("brain/pipeline: [%s] %q → ACCEPT utility=%.2f roi=%.2f time=%.2fh",
					as.Signal.ServiceSlug, as.Signal.Title, eval.Utility, req.EstimatedROI, req.TimeCommitment)

				// Stage 2: attempt real execution via the execution engine.
				if p.exec != nil {
					action := execution.ClassifyAction(as.Signal, as)
					if action.Type != execution.ActionNone {
						if shielded {
							narrative = fmt.Sprintf("[SHIELDED] %s | %s", shieldNote, narrative)
						}
						event := activities.Event{
							EventType:     as.EventType,
							Decision:      activities.Pending,
							Importance:    as.Importance,
							Narrative:     narrative,
							Analysis:      analysis,
							Gain:          as.Gain,
							SourceService: as.Signal.ServiceSlug,
						}
						created, createErr := p.events.Create(event)
						if createErr != nil {
							log.Printf("brain/pipeline: write event for signal[%d]: %v", i, createErr)
							continue
						}
						queued, execErr := p.exec.Process(ctx, action, created.ID)
						if execErr != nil {
							log.Printf("brain/pipeline: execution error for signal[%d]: %v", i, execErr)
						} else if !queued {
							p.events.UpdateDecision(created.ID, activities.Automated) //nolint:errcheck
						}
						// Event already written — skip the normal Create below.
						continue
					}
				}
				decision = activities.Automated
			} else {
				decision = activities.Declined
				narrative = fmt.Sprintf("Rejected post-decide: %s", eval.Reason)
				if eval.Utility <= vals.UtilityThreshold {
					analysis.TriageGate = "decide_utility"
					log.Printf("brain/pipeline: [%s] %q → REJECT (decide_utility) utility=%.2f threshold=%.2f roi=%.2f time=%.2fh",
						as.Signal.ServiceSlug, as.Signal.Title,
						eval.Utility, vals.UtilityThreshold, req.EstimatedROI, req.TimeCommitment)
				} else {
					analysis.TriageGate = "decide_risk"
					log.Printf("brain/pipeline: [%s] %q → REJECT (decide_risk) manip=%.0f%% risk_tolerance=%.2f utility=%.2f",
						as.Signal.ServiceSlug, as.Signal.Title,
						req.ManipulationPct*100, vals.RiskTolerance, eval.Utility)
				}
				result.Rejected++
			}

		default:
			decision = activities.Declined
			narrative = fmt.Sprintf("Unknown triage action %q: %s", action, reason)
			analysis.TriageGate = "unknown"
			result.Rejected++
			log.Printf("brain/pipeline: [%s] %q → REJECT (unknown action %q): %s",
				as.Signal.ServiceSlug, as.Signal.Title, action, reason)
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
			Analysis:      analysis,
			Gain:          as.Gain,
			SourceService: as.Signal.ServiceSlug,
		}
		if _, err := p.events.Create(event); err != nil {
			log.Printf("brain/pipeline: write event for signal[%d]: %v", i, err)
		}
	}

	return result, nil
}
