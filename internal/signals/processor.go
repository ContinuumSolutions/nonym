package signals

import (
	"context"
	"log"
	"strings"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/datasync"
)

// Processor handles the simplified signal analysis pipeline.
// Replaces the complex brain pipeline with focused signal processing.
// Uses biometrics to influence signal prioritization and scheduling decisions.
type Processor struct {
	store      *Store
	ai         *ai.Client
	biometrics *biometrics.Store
}

// NewProcessor creates a new signal processor with biometrics integration.
func NewProcessor(store *Store, aiClient *ai.Client, biometricsStore *biometrics.Store) *Processor {
	return &Processor{
		store:      store,
		ai:         aiClient,
		biometrics: biometricsStore,
	}
}

// ProcessResult summarizes a signal processing run.
type ProcessResult struct {
	TotalSignals    int      `json:"total_signals"`
	ProcessedOK     int      `json:"processed_ok"`
	AIErrors        int      `json:"ai_errors"`
	StoreErrors     int      `json:"store_errors"`
	RelevantSignals int      `json:"relevant_signals"`
	RepliesGenerated int     `json:"replies_generated"`
	Errors          []string `json:"errors,omitempty"`
}

// Process takes raw signals from datasync, analyzes them with AI,
// and stores them in the signals database.
//
// This replaces the complex brain pipeline with simple signal analysis.
func (p *Processor) Process(ctx context.Context, rawSignals []datasync.RawSignal) (ProcessResult, error) {
	result := ProcessResult{
		TotalSignals: len(rawSignals),
	}

	if len(rawSignals) == 0 {
		return result, nil
	}

	// Step 1: Analyze signals with AI (batch processing for efficiency)
	analysedSignals, aiErrors := p.ai.AnalyseBatch(ctx, rawSignals)

	// Step 2: Store each analyzed signal
	for i, analysed := range analysedSignals {
		if analysed == nil {
			result.AIErrors++
			if aiErrors[i] != nil {
				log.Printf("signals/processor: AI analysis failed for signal %d: %v", i, aiErrors[i])
				result.Errors = append(result.Errors, aiErrors[i].Error())
			}
			continue
		}

		// Apply biometrics-based prioritization
		p.applyBiometricsPrioritization(analysed)

		// Create signal record
		signal, err := p.store.Create(rawSignals[i], *analysed)
		if err != nil {
			result.StoreErrors++
			log.Printf("signals/processor: failed to store signal %d: %v", i, err)
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.ProcessedOK++

		// Count relevant signals and replies for summary
		if analysed.Category == "relevant" {
			result.RelevantSignals++
		}

		if analysed.NeedsReply && analysed.ReplyDraft != "" {
			// Create draft reply if AI generated one
			_, err := p.store.CreateDraftReply(
				signal.ID,
				analysed.ReplyDraft,
				analysed.ReplyTone,
				extractRecipients(rawSignals[i]),
				generateReplySubject(rawSignals[i]),
			)
			if err != nil {
				log.Printf("signals/processor: failed to create draft reply for signal %d: %v", signal.ID, err)
			} else {
				result.RepliesGenerated++
			}
		}
	}

	log.Printf("signals/processor: processed %d/%d signals, %d relevant, %d replies",
		result.ProcessedOK, result.TotalSignals, result.RelevantSignals, result.RepliesGenerated)

	return result, nil
}

// extractRecipients gets email addresses from the raw signal.
func extractRecipients(rawSignal datasync.RawSignal) []string {
	// For emails, extract from field in the metadata
	if fromEmail, exists := rawSignal.Metadata["from_email"]; exists && fromEmail != "" {
		return []string{fromEmail}
	}
	if from, exists := rawSignal.Metadata["from"]; exists && from != "" {
		return []string{from}
	}
	if sender, exists := rawSignal.Metadata["sender"]; exists && sender != "" {
		return []string{sender}
	}
	return []string{}
}

// generateReplySubject creates a reply subject line.
func generateReplySubject(rawSignal datasync.RawSignal) string {
	if subject, exists := rawSignal.Metadata["subject"]; exists && subject != "" {
		// Add "Re: " prefix if not already present
		if len(subject) < 3 || subject[:3] != "Re:" {
			return "Re: " + subject
		}
		return subject
	}
	return "Re: " + rawSignal.Title
}

// applyBiometricsPrioritization adjusts signal relevance and priority based on user's biometric state.
// High stress/low sleep = focus on urgent items, defer non-critical meetings.
// Good state = can handle more complex tasks and social interactions.
func (p *Processor) applyBiometricsPrioritization(signal *ai.AnalysedSignal) {
	// Get current biometrics
	checkIn, err := p.biometrics.Get()
	if err != nil {
		// No biometrics available, use defaults
		return
	}

	// Biometric thresholds
	highStress := checkIn.StressLevel > 7
	lowSleep := checkIn.Sleep < 5
	lowEnergy := checkIn.Energy < 4

	// Under stress/tired: focus on urgent, defer meetings
	if highStress || lowSleep || lowEnergy {
		// Elevate priority of urgent emails/notifications
		if signal.Category == "relevant" && signal.Priority == "medium" {
			// Check for urgency keywords in the signal
			if containsUrgentKeywords(signal.ReplyDraft) {
				signal.Priority = "high"
				log.Printf("signals/processor: elevated priority due to stress/fatigue: %s", signal.Category)
			}
		}

		// Suggest postponing non-urgent meetings
		if signal.Category == "relevant" && contains(signal.ReplyDraft, "meeting") && signal.Priority == "low" {
			signal.SuggestedAction = "suggest_reschedule"
			signal.Reasoning = "Current stress/energy levels suggest rescheduling non-urgent meetings"
		}
	}

	// Good state: can handle more complex interactions
	if !highStress && checkIn.Sleep >= 7 && checkIn.Energy >= 7 {
		// Suggest proactive engagement for opportunities
		if signal.Category == "relevant" && contains(signal.ReplyDraft, "opportunity") {
			signal.Priority = "high"
			signal.SuggestedAction = "engage_proactively"
		}
	}
}

// containsUrgentKeywords checks for urgency indicators
func containsUrgentKeywords(text string) bool {
	urgentWords := []string{"urgent", "asap", "deadline", "emergency", "critical", "expires", "due"}
	lowerText := strings.ToLower(text)
	for _, word := range urgentWords {
		if strings.Contains(lowerText, word) {
			return true
		}
	}
	return false
}

// contains checks if text contains a substring (case-insensitive)
func contains(text, substr string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(substr))
}