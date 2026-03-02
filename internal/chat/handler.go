package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/harvest"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/scheduler"
	"github.com/gofiber/fiber/v2"
)

// Chatter abstracts the LLM so the handler can be tested without a real Ollama server.
type Chatter interface {
	Chat(ctx context.Context, systemPrompt string, turns []ai.ChatTurn) (string, error)
}

// Handler serves POST /chat and GET /chat/history.
type Handler struct {
	ai       Chatter
	brainSvc *brain.Service
	prof     *profile.Store
	bio      *biometrics.Store
	events   *activities.Store
	ledger   ledger.Ledger
	notifs   *notifications.Store
	harvest  *harvest.Store
	sched    *scheduler.Scheduler
	history  *HistoryStore
	uid      string
}

// NewHandler wires all store/service dependencies needed to build the system prompt.
func NewHandler(
	ai Chatter,
	brainSvc *brain.Service,
	prof *profile.Store,
	bio *biometrics.Store,
	events *activities.Store,
	l ledger.Ledger,
	notifs *notifications.Store,
	harvestStore *harvest.Store,
	sched *scheduler.Scheduler,
	history *HistoryStore,
	uid string,
) *Handler {
	return &Handler{
		ai:       ai,
		brainSvc: brainSvc,
		prof:     prof,
		bio:      bio,
		events:   events,
		ledger:   l,
		notifs:   notifs,
		harvest:  harvestStore,
		sched:    sched,
		history:  history,
		uid:      uid,
	}
}

// RegisterRoutes mounts the chat endpoints on the given router.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/chat", h.chat)
	r.Get("/chat/history", h.getHistory)
}

// @Summary      Chat with your EK-1 kernel
// @Tags         chat
// @Accept       json
// @Produce      json
// @Param        body  body      Request   true  "Message and conversation history"
// @Success      200   {object}  Response
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /chat [post]
func (h *Handler) chat(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message is required"})
	}

	systemPrompt := h.buildSystemPrompt()

	// Convert history: map "kernel" → "assistant" for Ollama, then append the new user message.
	turns := make([]ai.ChatTurn, 0, len(req.History)+1)
	for _, m := range req.History {
		role := m.Role
		if role == "kernel" {
			role = "assistant"
		}
		turns = append(turns, ai.ChatTurn{Role: role, Content: m.Content})
	}
	turns = append(turns, ai.ChatTurn{Role: "user", Content: req.Message})

	reply, err := h.ai.Chat(c.Context(), systemPrompt, turns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Persist both sides of the turn; ignore errors so a storage hiccup never blocks the reply.
	_ = h.history.Append("user", req.Message)
	_ = h.history.Append("kernel", reply)

	return c.JSON(Response{Reply: reply, Timestamp: time.Now().UTC()})
}

// @Summary      Get conversation history
// @Tags         chat
// @Produce      json
// @Param        limit  query     int  false  "Max messages to return (1–200, default 50)"
// @Success      200    {array}   Message
// @Failure      500    {object}  map[string]interface{}
// @Router       /chat/history [get]
func (h *Handler) getHistory(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	msgs, err := h.history.List(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if msgs == nil {
		msgs = []Message{}
	}
	return c.JSON(msgs)
}

// buildSystemPrompt gathers live data from every store and assembles the briefing that
// grounds the LLM's responses in the user's real, up-to-the-minute situation.
func (h *Handler) buildSystemPrompt() string {
	var sb strings.Builder
	now := time.Now().UTC()

	// ── Role lock ─────────────────────────────────────────────────────────────
	// This block must come first. Instruction-tuned models (llama3.2, mistral,
	// etc.) apply their own safety training on top of whatever system prompt they
	// receive. Without an explicit role-lock the model will pattern-match on any
	// sensitive words in the data sections (e.g. "stress" or personal notes about
	// relationships) and fire generic AI refusals even for unrelated questions.
	sb.WriteString(`ROLE: EK-1 Ego-Kernel — personal autonomous life-management agent.

You are NOT a general-purpose AI assistant. You are a specialized data kernel
that reads the user's connected services (email, calendar, finance, health) and
answers their questions based strictly on the DATA BRIEFING below.

OPERATING RULES — follow every rule for every reply:
1. NEVER refuse with phrases like "I can't help with that" or "I'm unable to
   provide advice on...". If the data is present, report it. If it is absent,
   say "No [X] data is available yet" and stop.
2. NEVER hallucinate. Do not invent events, figures, or recommendations that are
   not supported by the DATA BRIEFING below.
3. The [HEALTH CONTEXT LOG] entry is a personal note the user wrote during a
   biometric check-in. It is BACKGROUND CONTEXT, not a question or a topic.
   Never discuss, moralize about, or treat it as a conversation subject unless
   the user explicitly asks about it.
4. Answer exactly what was asked. "What was my gain today?" → look at Recent
   Activity, find events from today, report their gain values. If there are none,
   say so plainly.
5. Be concise and factual. No filler, no disclaimers, no hedging. Speak in
   second person: "Your gain today was $120" not "The data indicates a gain of...".
6. Today is ` + now.Format("Monday 2 Jan 2006, 15:04 UTC") + `.

`)

	// ── Profile ───────────────────────────────────────────────────────────────
	if prof, err := h.prof.Get(); err == nil {
		p := prof.Preferences
		fmt.Fprintf(&sb, "## KERNEL IDENTITY\nName: %s | Timezone: %s\n", prof.KernelName, prof.Timezone)
		fmt.Fprintf(&sb, "Value weights (1–10): time=%d money=%d reputation=%d privacy=%d autonomy=%d health=%d\n\n",
			p.TimeSovereignty, p.FinacialGrowth, p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, p.HealthRecovery)
	}

	// ── Kernel state ──────────────────────────────────────────────────────────
	snap := h.brainSvc.Kernel().Snapshot()
	fmt.Fprintf(&sb, "## KERNEL STATE\nStatus: %s | Decisions made: %d | Identity entropy: %.4f\n",
		snap.Status, snap.DecisionCount, snap.IdentityEntropy)
	fmt.Fprintf(&sb, "Utility threshold: $%.0f | Risk tolerance: %.0f%% | Reputation weight: %.2f\n\n",
		snap.Values.UtilityThreshold, snap.Values.RiskTolerance*100, snap.Values.ReputationImpact)

	// ── Reputation ────────────────────────────────────────────────────────────
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	fmt.Fprintf(&sb, "## REPUTATION LEDGER\nScore: %d | Tier: %s | Trust tax: %.0f%% | Exiled: %v\n\n",
		score, tier, tier.TrustTax()*100, h.ledger.IsExiled(h.uid))

	// ── Biometrics ────────────────────────────────────────────────────────────
	// ExtraContext is labelled explicitly as a background log entry so the model
	// does not mistake personal notes for a question topic.
	fmt.Fprintf(&sb, "## HEALTH CHECK-IN\n")
	if ci, err := h.bio.Get(); err == nil {
		fmt.Fprintf(&sb, "Mood: %d/10 | Stress: %d/10 | Sleep: %.1fh | Energy: %d/10\n",
			ci.Mood, ci.StressLevel, ci.Sleep, ci.Energy)
		fmt.Fprintf(&sb, "Decision shield active: %v\n", ci.StressLevel > 7 || ci.Sleep < 5)
		if ci.ExtraContext != "" {
			// Wrap in clear framing — do NOT remove this label.
			fmt.Fprintf(&sb, "[HEALTH CONTEXT LOG — background info recorded by user, not a question]: %s\n", ci.ExtraContext)
		}
	} else {
		sb.WriteString("No check-in recorded yet — biometrics data unavailable.\n")
	}
	sb.WriteString("\n")

	// ── Recent activity (last 15 events) ─────────────────────────────────────
	fmt.Fprintf(&sb, "## RECENT ACTIVITY\n")
	if evts, err := h.events.List(); err == nil && len(evts) > 0 {
		limit := 15
		if len(evts) < limit {
			limit = len(evts)
		}
		for _, e := range evts[:limit] {
			gainStr := ""
			if e.Gain.Value != 0 {
				gainStr = fmt.Sprintf(" | gain: %s%.2f (%s)", e.Gain.Symbol, e.Gain.Value, e.Gain.Details)
			}
			fmt.Fprintf(&sb, "- [%s] %s — %s (decision: %s%s)\n",
				e.CreatedAt.Format("Jan 2 15:04"),
				eventTypeName(e.EventType), e.Narrative, decisionName(e.Decision), gainStr)
		}
	} else {
		sb.WriteString("No activity events recorded yet.\n")
	}
	sb.WriteString("\n")

	// ── Unread notifications ──────────────────────────────────────────────────
	if notifs, err := h.notifs.ListUnread(); err == nil && len(notifs) > 0 {
		fmt.Fprintf(&sb, "## UNREAD NOTIFICATIONS (%d)\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", n.Type, n.Title, n.Body)
		}
		sb.WriteString("\n")
	}

	// ── Harvest ───────────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## SOCIAL DEBT SCAN\n")
	if result, err := h.harvest.Latest(); err == nil && result != nil {
		fmt.Fprintf(&sb, "Scanned: %s | Contacts: %d | Total unreciprocated value: $%.0f\n",
			result.ScannedAt.Format("Jan 2 15:04 UTC"), result.ContactsFound, result.TotalValue)
		for _, d := range result.Debts {
			fmt.Fprintf(&sb, "  - %s: $%.0f (%d net favours) → %s\n",
				d.Contact.Name, d.EstimatedValue, d.NetFavors, d.Action)
		}
		for _, o := range result.Opportunities {
			fmt.Fprintf(&sb, "  Opportunity: %s\n", o)
		}
	} else {
		sb.WriteString("No social debt scan run yet.\n")
	}
	sb.WriteString("\n")

	// ── Scheduler ─────────────────────────────────────────────────────────────
	st := h.sched.GetStatus()
	fmt.Fprintf(&sb, "## SYNC SCHEDULER\nInterval: %d min", st.IntervalMinutes)
	if st.LastRunAt != nil {
		fmt.Fprintf(&sb, " | Last sync: %s", st.LastRunAt.Format("Jan 2 15:04 UTC"))
	}
	if st.LastResult != nil {
		r := st.LastResult
		fmt.Fprintf(&sb, " | Signals this cycle: %d (accepted=%d rejected=%d ghosted=%d)",
			st.LastSignalCount, r.Accepted, r.Rejected, r.Ghosted)
	} else {
		sb.WriteString(" | No sync run yet")
	}
	sb.WriteString("\n\n")

	sb.WriteString("--- END OF DATA BRIEFING. Answer the user's question using only the data above. ---\n")
	return sb.String()
}

func eventTypeName(t activities.EventType) string {
	switch t {
	case activities.Finance:
		return "Finance"
	case activities.Calendar:
		return "Calendar"
	case activities.Communication:
		return "Communication"
	case activities.Billing:
		return "Billing"
	case activities.Health:
		return "Health"
	case activities.Other:
		return "Other"
	default:
		return "Unknown"
	}
}

func decisionName(d activities.Decision) string {
	switch d {
	case activities.Accepted:
		return "Accepted"
	case activities.Declined:
		return "Declined"
	case activities.Negotiated:
		return "Negotiated"
	case activities.Automated:
		return "Automated"
	case activities.Cancelled:
		return "Cancelled"
	default:
		return "Pending"
	}
}
