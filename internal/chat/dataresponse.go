package chat

// dataresponse.go formats structured DB data directly as text, bypassing the LLM.
//
// Local LLMs (llama3.2, Mistral, etc.) ignore injected context and hallucinate
// plausible-sounding financial data even when explicitly told not to. For
// factual data queries the only reliable solution is to never call the model
// and instead render the real data ourselves.
//
// The LLM is still called for everything else: advice, general chat,
// questions that can't be answered by a DB lookup, and data questions where
// the DB genuinely has matching records (those go through buildLiveContext).

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egokernel/ek1/internal/activities"
)

// directDataResponse checks whether the message is a structured data query
// that can be answered entirely from the DB. If so it returns (reply, true)
// without touching the LLM. Otherwise it returns ("", false) so the caller
// falls through to the normal chat path.
func (h *Handler) directDataResponse(ctx context.Context, message string) (string, bool) {
	msg := strings.ToLower(strings.TrimSpace(message))

	switch {
	case isDecisionsQuery(msg):
		return h.replyDecisions(ctx)
	case isGainsQuery(msg, "time"):
		return h.replyGains(ctx, activities.Time)
	case isGainsQuery(msg, "money"):
		return h.replyGains(ctx, activities.Money)
	case isAllGainsQuery(msg):
		return h.replyAllGains(ctx)
	case isNotificationsQuery(msg):
		return h.replyNotifications(ctx)
	}
	return "", false
}

// ── Query classifiers ─────────────────────────────────────────────────────────

func isDecisionsQuery(msg string) bool {
	triggers := []string{
		"what decisions", "which decisions",
		"what did you decide", "what have you decided",
		"what did you do", "what have you done",
		"show decisions", "list decisions",
		"recent decisions", "recent activity",
		"what actions", "what have you handled",
		"show me what you", "what did the kernel",
	}
	return matchesAny(msg, triggers)
}

func isGainsQuery(msg, kind string) bool {
	switch kind {
	case "time":
		return matchesAny(msg, []string{
			"time saved", "time have you saved", "hours saved",
			"how much time", "time did you save",
		})
	case "money":
		return matchesAny(msg, []string{
			"money saved", "money have you saved", "how much money",
			"spending", "spent", "expenses", "what did i spend",
			"how much did i spend", "spend this", "financial summary",
			"money did you save", "how much have you saved",
		})
	}
	return false
}

func isAllGainsQuery(msg string) bool {
	return matchesAny(msg, []string{
		"what have you saved", "total savings", "overall savings",
		"what's my roi", "what is my roi",
	})
}

func isNotificationsQuery(msg string) bool {
	return matchesAny(msg, []string{
		"notifications", "alerts", "unread", "any alerts",
		"what alerts", "show alerts", "what notifications",
	})
}

func matchesAny(msg string, phrases []string) bool {
	for _, p := range phrases {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// ── Formatters ────────────────────────────────────────────────────────────────

func (h *Handler) replyDecisions(ctx context.Context) (string, bool) {
	events, err := h.events.List()
	if err != nil {
		return "", false // let LLM handle DB errors gracefully
	}
	if len(events) == 0 {
		return "No decisions recorded yet. Connect your integrations and trigger a sync to start.", true
	}

	limit := 10
	if len(events) < limit {
		limit = len(events)
	}
	shown := events[:limit]

	var sb strings.Builder
	fmt.Fprintf(&sb, "Here are the last %d decisions the kernel made", len(shown))
	if len(events) > limit {
		fmt.Fprintf(&sb, " (out of %d total)", len(events))
	}
	sb.WriteString(":\n\n")

	for i, e := range shown {
		fmt.Fprintf(&sb, "%d. [%s] %s — %s — %s",
			i+1,
			e.CreatedAt.Format("Jan 2, 15:04"),
			eventTypeName(e.EventType),
			decisionName(e.Decision),
			e.Narrative,
		)
		if e.Gain.Value != 0 {
			fmt.Fprintf(&sb, " (%s%.2f %s)", e.Gain.Symbol, e.Gain.Value, e.Gain.Details)
		}
		sb.WriteString("\n")
	}

	if len(events) > limit {
		fmt.Fprintf(&sb, "\n...and %d more. Ask for a specific date range or category to filter.", len(events)-limit)
	}
	return sb.String(), true
}

func (h *Handler) replyGains(ctx context.Context, kind activities.GainKind) (string, bool) {
	gains, err := h.events.SumGains(time.Time{})
	if err != nil {
		return "", false
	}

	for _, g := range gains {
		if g.Kind == kind {
			if g.Count == 0 {
				break
			}
			label := "money"
			unit := g.Symbol
			if kind == activities.Time {
				label = "time"
			}
			return fmt.Sprintf("All-time %s gains: %s%.2f across %d decisions.",
				label, unit, g.TotalValue, g.Count), true
		}
	}

	if kind == activities.Time {
		return "No time savings recorded yet. Gains are tracked as decisions are processed.", true
	}
	return "No financial gains recorded yet. Connect Plaid or Stripe and trigger a sync.", true
}

func (h *Handler) replyAllGains(ctx context.Context) (string, bool) {
	gains, err := h.events.SumGains(time.Time{})
	if err != nil {
		return "", false
	}
	if len(gains) == 0 {
		return "No gains recorded yet. Connect your integrations and trigger a sync.", true
	}

	var sb strings.Builder
	sb.WriteString("All-time gains:\n\n")
	for _, g := range gains {
		if g.Kind == activities.Time {
			fmt.Fprintf(&sb, "• Time saved: %.2f%s across %d decisions\n", g.TotalValue, g.Symbol, g.Count)
		} else {
			fmt.Fprintf(&sb, "• Money: %s%.2f across %d decisions\n", g.Symbol, g.TotalValue, g.Count)
		}
	}
	return sb.String(), true
}

func (h *Handler) replyNotifications(ctx context.Context) (string, bool) {
	notifs, err := h.notifs.ListUnread()
	if err != nil {
		return "", false
	}
	if len(notifs) == 0 {
		return "No unread notifications.", true
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d unread notification(s):\n\n", len(notifs))
	for i, n := range notifs {
		fmt.Fprintf(&sb, "%d. [%s] %s — %s\n", i+1, n.Type, n.Title, n.Body)
	}
	return sb.String(), true
}
