package chat

// focused.go builds data-grounded user messages following the pattern:
//   "Based on this data: [exact DB records], [user question]"
//
// Embedding data in the user turn (rather than only the system prompt) is
// significantly more reliable with local LLMs like llama3.2 — the model is
// forced to read the data as part of the question it must answer, making it
// much harder to ignore and hallucinate plausible-looking figures.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/signals"
)

// buildFocusedUserMessage replaces the last user turn with a version that
// embeds exactly the data relevant to the detected intent.
// Returns (enrichedMessage, hasData).
// If hasData=false for a data intent the caller should short-circuit with a
// "no data" reply instead of calling the LLM.
func (h *Handler) buildFocusedUserMessage(ctx context.Context, intent Intent, originalMessage string) (string, bool) {
	switch intent {
	case IntentFocusToday:
		return h.focusTodayMessage(ctx, originalMessage)
	case IntentDecisions:
		return h.decisionsMessage(ctx, originalMessage)
	case IntentFinancial:
		return h.financialMessage(ctx, originalMessage)
	case IntentHealth:
		return h.healthMessage(ctx, originalMessage)
	case IntentSocialDebt:
		return h.socialDebtMessage(ctx, originalMessage)
	case IntentReputation:
		return h.reputationMessage(ctx, originalMessage)
	case IntentNotifications:
		return h.notificationsMessage(ctx, originalMessage)
	case IntentKernelStatus:
		return h.kernelStatusMessage(ctx, originalMessage)
	}
	// IntentGeneral — no data injection, pass through unchanged.
	return originalMessage, true
}

// header writes the standard preamble for every focused message.
func focusedHeader(sb *strings.Builder, original string) {
	fmt.Fprintf(sb, "Based on my actual data below, please answer: %q\n\n", original)
	sb.WriteString("DATA (use ONLY what is listed here — do not invent or generalise):\n")
	sb.WriteString("────────────────────────────────────────────────────────────\n")
}

func focusedFooter(sb *strings.Builder) {
	sb.WriteString("────────────────────────────────────────────────────────────\n")
	sb.WriteString("Answer using the data above. If a section is EMPTY, say so explicitly.\n")
}

// ── Per-intent builders ───────────────────────────────────────────────────────

func (h *Handler) focusTodayMessage(ctx context.Context, original string) (string, bool) {
	// Get relevant signals that need attention
	relevantFilter := signals.FilterCriteria{Category: "relevant"}
	pending := signals.StatusPending
	relevantFilter.Status = &pending
	relevantSignals, _ := h.signals.List(relevantFilter, 20)

	// Get high priority signals (for counting/display)

	// Get signals needing replies
	needsReply := true
	replyFilter := signals.FilterCriteria{NeedsReply: &needsReply}
	replyFilter.Status = &pending
	replySignals, _ := h.signals.List(replyFilter, 10)

	notifs, _ := h.notifs.ListUnread()
	ci, _ := h.bio.Get()
	hasData := false

	var sb strings.Builder
	focusedHeader(&sb, original)

	if len(relevantSignals) > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nRELEVANT ITEMS — need your attention (%d):\n", len(relevantSignals))
		for _, s := range relevantSignals {
			priority := ""
			if s.Analysis.Priority == "high" {
				priority = " 🔴"
			} else if s.Analysis.Priority == "medium" {
				priority = " 🟡"
			}
			fmt.Fprintf(&sb, "  • [%s] %s%s — %s\n",
				s.ProcessedAt.Format("Jan 2 15:04"),
				s.OriginalSignal.Title,
				priority,
				s.Analysis.Summary,
			)
			if s.Analysis.SuggestedAction != "" {
				fmt.Fprintf(&sb, "    → %s\n", s.Analysis.SuggestedAction)
			}
		}
	} else {
		sb.WriteString("\nRELEVANT ITEMS: EMPTY — no signals need your attention right now.\n")
	}

	if len(replySignals) > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nNEED REPLIES (%d):\n", len(replySignals))
		for _, s := range replySignals {
			fmt.Fprintf(&sb, "  • %s — %s\n",
				s.OriginalSignal.Title,
				s.Analysis.Summary,
			)
		}
	}

	if len(notifs) > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nUNREAD NOTIFICATIONS (%d):\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "  • [%s] %s — %s\n", n.Type, n.Title, n.Body)
		}
	} else {
		sb.WriteString("\nNOTIFICATIONS: none\n")
	}

	if ci != nil {
		hasData = true
		fmt.Fprintf(&sb, "\nHEALTH: mood=%d/10 stress=%d/10 sleep=%.1fh energy=%d/10",
			ci.Mood, ci.StressLevel, ci.Sleep, ci.Energy)
		if ci.StressLevel > 7 || ci.Sleep < 5 {
			sb.WriteString(" | DECISION SHIELD ACTIVE")
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("\nHEALTH: no check-in recorded\n")
	}

	focusedFooter(&sb)
	return sb.String(), hasData
}

func (h *Handler) decisionsMessage(ctx context.Context, original string) (string, bool) {
	events, _ := h.events.List()
	if len(events) == 0 {
		return original, false
	}

	limit := 15
	if len(events) < limit {
		limit = len(events)
	}

	var sb strings.Builder
	focusedHeader(&sb, original)
	fmt.Fprintf(&sb, "\nRECENT DECISIONS (%d of %d total):\n", limit, len(events))
	for i, e := range events[:limit] {
		gainStr := ""
		if e.Gain.Value != 0 {
			gainStr = fmt.Sprintf(" | gain: %s%.2f", e.Gain.Symbol, e.Gain.Value)
		}
		fmt.Fprintf(&sb, "  %d. [%s] %s — %s — %s%s\n",
			i+1,
			e.CreatedAt.Format("Jan 2 15:04"),
			eventTypeName(e.EventType),
			decisionName(e.Decision),
			e.Narrative,
			gainStr,
		)
	}
	focusedFooter(&sb)
	return sb.String(), true
}

func (h *Handler) financialMessage(ctx context.Context, original string) (string, bool) {
	gains, _ := h.events.SumGains(time.Time{})
	events, _ := h.events.List()
	hasData := false

	var sb strings.Builder
	focusedHeader(&sb, original)

	sb.WriteString("\nGAINS (all time):\n")
	if len(gains) == 0 {
		sb.WriteString("  EMPTY — no gain data recorded. Connect Plaid or Stripe and sync.\n")
	} else {
		for _, g := range gains {
			hasData = true
			if g.Kind == activities.Time {
				fmt.Fprintf(&sb, "  Time saved: %.2f%s across %d decisions\n", g.TotalValue, g.Symbol, g.Count)
			} else {
				fmt.Fprintf(&sb, "  Money: %s%.2f across %d decisions\n", g.Symbol, g.TotalValue, g.Count)
			}
		}
	}

	// Finance/billing events
	var finEvents []activities.Event
	for _, e := range events {
		if e.EventType == activities.Finance || e.EventType == activities.Billing {
			finEvents = append(finEvents, e)
			if len(finEvents) >= 10 {
				break
			}
		}
	}
	if len(finEvents) > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nRECENT FINANCE/BILLING EVENTS (%d):\n", len(finEvents))
		for _, e := range finEvents {
			gainStr := ""
			if e.Gain.Value != 0 {
				gainStr = fmt.Sprintf(" | %s%.2f", e.Gain.Symbol, e.Gain.Value)
			}
			fmt.Fprintf(&sb, "  • [%s] %s — %s%s\n",
				e.CreatedAt.Format("Jan 2"),
				decisionName(e.Decision),
				e.Narrative,
				gainStr,
			)
		}
	} else {
		sb.WriteString("\nRECENT FINANCE/BILLING EVENTS: EMPTY\n")
	}

	focusedFooter(&sb)
	return sb.String(), hasData
}

func (h *Handler) healthMessage(ctx context.Context, original string) (string, bool) {
	ci, _ := h.bio.Get()

	var sb strings.Builder
	focusedHeader(&sb, original)

	if ci == nil {
		sb.WriteString("\nHEALTH CHECK-IN: EMPTY — no check-in recorded yet.\n")
		focusedFooter(&sb)
		return sb.String(), false
	}

	fmt.Fprintf(&sb, "\nHEALTH CHECK-IN:\n")
	fmt.Fprintf(&sb, "  Mood:        %d / 10\n", ci.Mood)
	fmt.Fprintf(&sb, "  Stress:      %d / 10\n", ci.StressLevel)
	fmt.Fprintf(&sb, "  Sleep:       %.1f hours\n", ci.Sleep)
	fmt.Fprintf(&sb, "  Energy:      %d / 10\n", ci.Energy)
	shielded := ci.StressLevel > 7 || ci.Sleep < 5
	fmt.Fprintf(&sb, "  Shield:      %v (stress>7 or sleep<5)\n", shielded)
	if ci.ExtraContext != "" {
		fmt.Fprintf(&sb, "  Notes:       %s\n", ci.ExtraContext)
	}

	focusedFooter(&sb)
	return sb.String(), true
}

func (h *Handler) socialDebtMessage(ctx context.Context, original string) (string, bool) {
	result, _ := h.harvest.Latest()

	var sb strings.Builder
	focusedHeader(&sb, original)

	if result == nil || result.TotalValue == 0 {
		sb.WriteString("\nSOCIAL DEBT SCAN: EMPTY — no harvest scan has been run yet.\n")
		focusedFooter(&sb)
		return sb.String(), false
	}

	fmt.Fprintf(&sb, "\nSOCIAL DEBT SCAN (as of %s):\n", result.ScannedAt.Format("Jan 2 15:04 UTC"))
	fmt.Fprintf(&sb, "  Contacts scanned: %d\n", result.ContactsFound)
	fmt.Fprintf(&sb, "  Total unreciprocated value: $%.0f\n", result.TotalValue)
	if len(result.Debts) > 0 {
		sb.WriteString("\nDEBTS:\n")
		for _, d := range result.Debts {
			fmt.Fprintf(&sb, "  • %s — $%.0f (%d net favours) → suggested action: %s\n",
				d.Contact.Name, d.EstimatedValue, d.NetFavors, d.Action)
		}
	}
	if len(result.Opportunities) > 0 {
		sb.WriteString("\nOPPORTUNITIES:\n")
		for _, o := range result.Opportunities {
			fmt.Fprintf(&sb, "  • %s\n", o)
		}
	}

	focusedFooter(&sb)
	return sb.String(), true
}

func (h *Handler) reputationMessage(ctx context.Context, original string) (string, bool) {
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	exiled := h.ledger.IsExiled(h.uid)

	var sb strings.Builder
	focusedHeader(&sb, original)

	fmt.Fprintf(&sb, "\nREPUTATION LEDGER:\n")
	fmt.Fprintf(&sb, "  Score:      %d\n", score)
	fmt.Fprintf(&sb, "  Tier:       %s\n", tier)
	fmt.Fprintf(&sb, "  Trust tax:  %.0f%%\n", tier.TrustTax()*100)
	fmt.Fprintf(&sb, "  Exiled:     %v\n", exiled)

	focusedFooter(&sb)
	return sb.String(), true
}

func (h *Handler) notificationsMessage(ctx context.Context, original string) (string, bool) {
	notifs, _ := h.notifs.ListUnread()

	var sb strings.Builder
	focusedHeader(&sb, original)

	if len(notifs) == 0 {
		sb.WriteString("\nNOTIFICATIONS: EMPTY — no unread notifications.\n")
		focusedFooter(&sb)
		return sb.String(), false
	}

	fmt.Fprintf(&sb, "\nUNREAD NOTIFICATIONS (%d):\n", len(notifs))
	for i, n := range notifs {
		fmt.Fprintf(&sb, "  %d. [%s] %s — %s\n", i+1, n.Type, n.Title, n.Body)
	}
	focusedFooter(&sb)
	return sb.String(), true
}

func (h *Handler) kernelStatusMessage(ctx context.Context, original string) (string, bool) {
	snap := h.brainSvc.Kernel().Snapshot()
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	st := h.sched.GetStatus()

	var sb strings.Builder
	focusedHeader(&sb, original)

	fmt.Fprintf(&sb, "\nKERNEL STATE:\n")
	fmt.Fprintf(&sb, "  Status:            %s\n", snap.Status)
	fmt.Fprintf(&sb, "  Decisions made:    %d\n", snap.DecisionCount)
	fmt.Fprintf(&sb, "  Identity entropy:  %.4f\n", snap.IdentityEntropy)
	fmt.Fprintf(&sb, "  Utility threshold: $%.0f\n", snap.Values.UtilityThreshold)
	fmt.Fprintf(&sb, "  Reputation score:  %d (%s)\n", score, tier)

	fmt.Fprintf(&sb, "\nSCHEDULER:\n")
	fmt.Fprintf(&sb, "  Interval: %d minutes\n", st.IntervalMinutes)
	if st.LastRunAt != nil {
		fmt.Fprintf(&sb, "  Last run: %s\n", st.LastRunAt.Format("Jan 2 15:04 UTC"))
	} else {
		sb.WriteString("  Last run: never\n")
	}
	if st.LastResult != nil {
		r := st.LastResult
		fmt.Fprintf(&sb, "  Last cycle: %d signals (accepted=%d declined=%d ghosted=%d)\n",
			st.LastSignalCount, r.Accepted, r.Rejected, r.Ghosted)
	}

	focusedFooter(&sb)
	return sb.String(), true
}
