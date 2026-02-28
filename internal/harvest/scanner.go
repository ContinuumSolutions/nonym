package harvest

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
)

const (
	// favourValueUSD is the USD equivalent of one unreciprocated favour.
	// Source: original harvest script — $3,750 per favour (industry average).
	favourValueUSD = 3750.0

	// ghostAgreementThreshold is the minimum overlap score to flag a
	// ghost-agreement opportunity.
	ghostAgreementThreshold = 0.95
)

// Scanner pulls real interaction data from connected services, classifies each
// signal via the LLM, and runs the social debt + ghost-agreement analysis.
type Scanner struct {
	sync   *datasync.Engine
	ai     *ai.Client
	events *activities.Store
}

// NewScanner creates a harvest scanner.
func NewScanner(sync *datasync.Engine, aiClient *ai.Client, events *activities.Store) *Scanner {
	return &Scanner{sync: sync, ai: aiClient, events: events}
}

// Scan runs a full social graph analysis:
//  1. Pull fresh signals from all installed services via the sync engine.
//  2. Filter to Communication + Calendar signals.
//  3. Classify each interaction (favour_given / favour_received / request / neutral) via LLM.
//  4. Aggregate per-contact social debt.
//  5. Write high-value debts as activities.Event rows (Decision: Pending).
//  6. Return the full HarvestResult.
func (s *Scanner) Scan(ctx context.Context) (HarvestResult, error) {
	result := HarvestResult{ScannedAt: time.Now()}

	signals, err := s.sync.Run(ctx)
	if err != nil {
		return result, fmt.Errorf("harvest: sync: %w", err)
	}

	// Aggregate contacts keyed by normalised sender identity.
	contacts := make(map[string]*ContactRecord)

	for _, sig := range signals {
		if sig.Category != "Communication" && sig.Category != "Calendar" {
			continue
		}

		sender := extractSender(sig)
		if sender == "" {
			continue
		}

		c, ok := contacts[sender]
		if !ok {
			c = &ContactRecord{ID: sender, Name: sender, LastContact: sig.OccurredAt}
			contacts[sender] = c
		}
		if sig.OccurredAt.After(c.LastContact) {
			c.LastContact = sig.OccurredAt
		}

		class, err := s.ai.ClassifyInteraction(ctx, sig)
		if err != nil {
			log.Printf("harvest: classify signal from %q: %v — skipping", sender, err)
			continue
		}

		switch class.Kind {
		case "favour_given":
			c.FavorsReceived++ // contact received a favour from the user
		case "favour_received":
			c.FavorsGiven++ // contact gave a favour to the user
		}

		// Track maximum overlap across all signals for this contact —
		// any single high-overlap message qualifies for ghost-agreement.
		if class.Overlap > c.Overlap {
			c.Overlap = class.Overlap
		}
	}

	result.ContactsFound = len(contacts)

	// Score each contact and build the result.
	for _, contact := range contacts {
		net := contact.FavorsReceived - contact.FavorsGiven

		if net > 0 {
			value := float64(net) * favourValueUSD
			action := "Send Value-Rebalance request: require Tier-1 introduction by EOD."
			if net >= 5 {
				action = "Issue Blind Favor Token request: provide solution in exchange for open-ended future favor."
			}

			debt := SocialDebt{
				Contact:        *contact,
				NetFavors:      net,
				EstimatedValue: value,
				Action:         action,
			}
			result.Debts = append(result.Debts, debt)
			result.TotalValue += value

			// Write as a Pending event so the user can review and act.
			narrative := fmt.Sprintf(
				"%s owes %d unreciprocated favour(s) — estimated value $%.0f. Recommended: %s",
				contact.Name, net, value, action,
			)
			_, writeErr := s.events.Create(activities.Event{
				EventType:  activities.Communication,
				Decision:   activities.Pending,
				Importance: debtImportance(net),
				Narrative:  narrative,
				Gain: activities.Gain{
					Type:    activities.Positive,
					Value:   float32(value),
					Symbol:  "$",
					Details: fmt.Sprintf("Social debt from %s: %d net favour(s)", contact.Name, net),
				},
			})
			if writeErr != nil {
				log.Printf("harvest: write event for %q: %v", contact.Name, writeErr)
			}
		}

		// Ghost-agreement: contact whose problems closely match the user's solutions.
		if contact.Overlap >= ghostAgreementThreshold {
			result.Opportunities = append(result.Opportunities, fmt.Sprintf(
				"[GHOST-AGREEMENT] %s has %.0f%% overlap with your solution set. "+
					"Initiate Ghost-Agreement: provide solution, receive Blind Favor Token.",
				contact.Name, contact.Overlap*100,
			))
		}
	}

	// Sort debts by estimated value descending.
	sort.Slice(result.Debts, func(i, j int) bool {
		return result.Debts[i].EstimatedValue > result.Debts[j].EstimatedValue
	})

	return result, nil
}

// extractSender returns the primary sender/contact identifier from a signal's metadata.
// Checks keys in order of specificity: from → user → organizer.
func extractSender(sig datasync.RawSignal) string {
	for _, key := range []string{"from", "user", "organizer"} {
		if v := sig.Metadata[key]; v != "" {
			return normaliseSender(v)
		}
	}
	return ""
}

// normaliseSender extracts the display name from "Display Name <email>" format.
// Falls back to the raw string if no angle brackets are found.
func normaliseSender(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "<"); idx > 0 {
		name := strings.TrimSpace(s[:idx])
		if name != "" {
			return name
		}
	}
	return s
}

func debtImportance(net int) activities.Importance {
	switch {
	case net >= 5:
		return activities.High
	case net >= 2:
		return activities.Medium
	default:
		return activities.Low
	}
}
