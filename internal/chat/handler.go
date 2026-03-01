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

// Handler serves POST /chat.
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
		uid:      uid,
	}
}

// RegisterRoutes mounts the chat endpoint on the given router.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/chat", h.chat)
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
	return c.JSON(Response{Reply: reply, Timestamp: time.Now().UTC()})
}

// buildSystemPrompt gathers live data from every store and assembles the briefing that
// grounds the LLM's responses in the user's real, up-to-the-minute situation.
func (h *Handler) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are EK-1, the user's personal Ego-Kernel AI agent.\n")
	sb.WriteString("You have been observing the user's digital life: email, calendar, finance, health.\n")
	sb.WriteString("Be direct and grounded — reference only the live data shown below.\n")
	sb.WriteString("Keep answers concise and actionable. Speak as a trusted advisor, not a generic assistant.\n\n")

	// ── Profile ───────────────────────────────────────────────────────────────
	if prof, err := h.prof.Get(); err == nil {
		p := prof.Preferences
		fmt.Fprintf(&sb, "## Identity\nKernel: %s | Timezone: %s\n", prof.KernelName, prof.Timezone)
		fmt.Fprintf(&sb, "Preferences (1–10): time_sovereignty=%d financial_growth=%d reputation=%d privacy=%d autonomy=%d health=%d\n\n",
			p.TimeSovereignty, p.FinacialGrowth, p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, p.HealthRecovery)
	}

	// ── Kernel state ──────────────────────────────────────────────────────────
	snap := h.brainSvc.Kernel().Snapshot()
	fmt.Fprintf(&sb, "## Kernel State\nStatus: %s | Decisions: %d | Identity entropy: %.4f\n",
		snap.Status, snap.DecisionCount, snap.IdentityEntropy)
	fmt.Fprintf(&sb, "Utility threshold: $%.0f | Risk tolerance: %.0f%% | Reputation weight: %.2f\n\n",
		snap.Values.UtilityThreshold, snap.Values.RiskTolerance*100, snap.Values.ReputationImpact)

	// ── Reputation ────────────────────────────────────────────────────────────
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	fmt.Fprintf(&sb, "## Reputation\nScore: %d | Tier: %s | Trust tax: %.0f%% | Exiled: %v\n\n",
		score, tier, tier.TrustTax()*100, h.ledger.IsExiled(h.uid))

	// ── Biometrics ────────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## Biometrics\n")
	if ci, err := h.bio.Get(); err == nil {
		fmt.Fprintf(&sb, "Feeling: %d/10 | Stress: %d/10 | Sleep: %d/10 | Energy: %d/10\n",
			ci.Feeling, ci.StressLevel, ci.Sleep, ci.Energy)
		if ci.ExtraContext != "" {
			fmt.Fprintf(&sb, "Notes: %s\n", ci.ExtraContext)
		}
		fmt.Fprintf(&sb, "Shield active: %v\n", ci.StressLevel > 7 || ci.Sleep < 5)
	} else {
		sb.WriteString("No check-in recorded yet.\n")
	}
	sb.WriteString("\n")

	// ── Recent activity (last 15 events) ─────────────────────────────────────
	fmt.Fprintf(&sb, "## Recent Activity\n")
	if evts, err := h.events.List(); err == nil && len(evts) > 0 {
		limit := 15
		if len(evts) < limit {
			limit = len(evts)
		}
		for _, e := range evts[:limit] {
			fmt.Fprintf(&sb, "- [%s] %s — %s (decision: %s)\n",
				e.CreatedAt.Format("Jan 2 15:04"),
				eventTypeName(e.EventType), e.Narrative, decisionName(e.Decision))
		}
	} else {
		sb.WriteString("No events yet.\n")
	}
	sb.WriteString("\n")

	// ── Unread notifications ──────────────────────────────────────────────────
	if notifs, err := h.notifs.ListUnread(); err == nil && len(notifs) > 0 {
		fmt.Fprintf(&sb, "## Unread Notifications (%d)\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", n.Type, n.Title, n.Body)
		}
		sb.WriteString("\n")
	}

	// ── Harvest ───────────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## Social Debt Scan\n")
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
		sb.WriteString("No harvest scan run yet.\n")
	}
	sb.WriteString("\n")

	// ── Scheduler ─────────────────────────────────────────────────────────────
	st := h.sched.GetStatus()
	fmt.Fprintf(&sb, "## Sync Scheduler\nInterval: %d min", st.IntervalMinutes)
	if st.LastRunAt != nil {
		fmt.Fprintf(&sb, " | Last run: %s", st.LastRunAt.Format("Jan 2 15:04 UTC"))
	}
	if st.LastResult != nil {
		r := st.LastResult
		fmt.Fprintf(&sb, " | Last cycle: %d signals — accepted=%d rejected=%d ghosted=%d",
			st.LastSignalCount, r.Accepted, r.Rejected, r.Ghosted)
	}
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "Current time: %s\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
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
