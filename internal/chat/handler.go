package chat

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/valyala/fasthttp"
)

// Streamer is satisfied by *ai.Client when streaming is available.
// The handler falls back to regular Chat if the ai dependency doesn't implement it.
type Streamer interface {
	ChatStream(ctx context.Context, systemPrompt string, turns []ai.ChatTurn, fn func(string)) error
}

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
	r.Post("/chat/stream", h.chatStream)
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

	// Persist the user message before calling the LLM so its timestamp is
	// always earlier than the kernel reply, even when both land in the same second.
	_ = h.history.Append("user", req.Message)

	reply, err := h.ai.Chat(c.Context(), systemPrompt, turns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	_ = h.history.Append("kernel", reply)

	return c.JSON(Response{Reply: reply, Timestamp: time.Now().UTC()})
}

// chatStream handles POST /chat/stream, returning tokens as Server-Sent Events.
// Each SSE event is {"token":"<chunk>"} until the final {"done":true,"timestamp":"..."}.
// Falls back to buffered JSON (same as POST /chat) when the ai dependency does not
// implement the Streamer interface.
func (h *Handler) chatStream(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message is required"})
	}

	streamer, ok := h.ai.(Streamer)
	if !ok {
		// ai doesn't support streaming — fall back transparently.
		return h.chat(c)
	}

	systemPrompt := h.buildSystemPrompt()

	turns := make([]ai.ChatTurn, 0, len(req.History)+1)
	for _, m := range req.History {
		role := m.Role
		if role == "kernel" {
			role = "assistant"
		}
		turns = append(turns, ai.ChatTurn{Role: role, Content: m.Content})
	}
	turns = append(turns, ai.ChatTurn{Role: "user", Content: req.Message})

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	// Persist the user message now, before streaming begins, so its timestamp
	// is always earlier than the kernel reply.
	_ = h.history.Append("user", req.Message)

	ctx := c.Context()

	ctx.SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		var fullReply strings.Builder

		sendEvent := func(v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.Flush()
		}

		err := streamer.ChatStream(ctx, systemPrompt, turns, func(token string) {
			fullReply.WriteString(token)
			sendEvent(map[string]string{"token": token})
		})
		if err != nil {
			sendEvent(map[string]string{"error": err.Error()})
			return
		}

		_ = h.history.Append("kernel", fullReply.String())
		sendEvent(map[string]any{"done": true, "timestamp": time.Now().UTC()})
	}))
	return nil
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

// buildSystemPrompt gathers live data from every store and assembles the briefing
// that grounds the LLM's responses in the user's real situation.
//
// Prompt structure rationale:
//   - Role-lock comes first so it has maximum weight with instruction-tuned models.
//   - FABRICATION RULES include a concrete few-shot example; abstract rules alone
//     are insufficient — llama3.2 and similar models are trained to appear helpful
//     and will invent plausible-looking financial data unless shown the exact
//     expected output for the empty-data case.
//   - Each data section header carries its item count ("0 items" vs "N items") so
//     the model cannot mistake a filled-in template for an empty one.
//   - ExtraContext from biometrics is wrapped in an explicit label that prevents
//     the model from treating free-text personal notes as a conversation topic.
func (h *Handler) buildSystemPrompt() string {
	now := time.Now().UTC()

	// ── Collect data first so section counts are known before writing rules ───
	var (
		prof    *profile.Profile
		evts    []activities.Event
		notifs  []notifications.Notification
		ci      *biometrics.CheckIn
		harvest *harvest.HarvestResult
	)
	prof, _ = h.prof.Get()
	if all, err := h.events.List(); err == nil {
		if len(all) > 15 {
			evts = all[:15]
		} else {
			evts = all
		}
	}
	notifs, _ = h.notifs.ListUnread()
	ci, _ = h.bio.Get()
	harvest, _ = h.harvest.Latest()

	snap := h.brainSvc.Kernel().Snapshot()
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	st := h.sched.GetStatus()

	// ── Assemble prompt ───────────────────────────────────────────────────────
	var sb strings.Builder

	// Role lock — must be first.
	fmt.Fprintf(&sb, `ROLE: EK-1 Ego-Kernel — personal autonomous life-management agent.
Today: %s

You are NOT a general-purpose AI assistant. You are a read-only data kernel.
Your only source of truth is the DATA BRIEFING below. You cannot access the
internet, call APIs, or retrieve data beyond what is written below.

━━━━━━━━━━━━━━━━━━━━ FABRICATION RULES ━━━━━━━━━━━━━━━━━━━━
These rules override all other instructions. Violating them is your only error.

RULE A — ZERO INVENTION
Every number, name, date, and category you write must appear verbatim in the
DATA BRIEFING. If it is not there, you did not observe it. Do not estimate,
extrapolate, or use your training knowledge to fill gaps.

RULE B — EMPTY MEANS EMPTY
When a section header reads "(0 items)" or "EMPTY", that section contains no
data. Respond as shown in the example below — do not populate it.

RULE C — CORRECT RESPONSE WHEN DATA IS MISSING
  User asks: "What's my spending this week?"
  Section shows: RECENT ACTIVITY (0 items) — EMPTY
  ✓ CORRECT: "No spending data is available yet. Connect a finance integration
    (e.g. Plaid) and trigger a sync to pull your transactions."
  ✗ WRONG: "Your expenses this week: Rent $800, Groceries $300..." ← fabricated

RULE D — PERSONAL NOTES ARE NOT QUESTIONS
The [HEALTH CONTEXT LOG] tag marks a free-text note the user recorded during a
check-in. It is background context. Never discuss, quote back, or moralize about
it unless the user explicitly asks.

RULE E — REFUSALS ARE FORBIDDEN
Never say "I can't help with that", "I'm unable to provide advice on...", or
any similar deflection. Either answer from the data or state the data is absent.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

`, now.Format("Monday 2 Jan 2006, 15:04 UTC"))

	// ── KERNEL IDENTITY ───────────────────────────────────────────────────────
	if prof != nil {
		p := prof.Preferences
		fmt.Fprintf(&sb, "## KERNEL IDENTITY\nName: %s | Timezone: %s\n", prof.KernelName, prof.Timezone)
		fmt.Fprintf(&sb, "Value weights (1–10): time=%d money=%d reputation=%d privacy=%d autonomy=%d health=%d\n\n",
			p.TimeSovereignty, p.FinacialGrowth, p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, p.HealthRecovery)
	}

	// ── KERNEL STATE ──────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## KERNEL STATE\nStatus: %s | Decisions made: %d | Identity entropy: %.4f\n",
		snap.Status, snap.DecisionCount, snap.IdentityEntropy)
	fmt.Fprintf(&sb, "Utility threshold: $%.0f | Risk tolerance: %.0f%% | Reputation weight: %.2f\n\n",
		snap.Values.UtilityThreshold, snap.Values.RiskTolerance*100, snap.Values.ReputationImpact)

	// ── REPUTATION ────────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## REPUTATION LEDGER\nScore: %d | Tier: %s | Trust tax: %.0f%% | Exiled: %v\n\n",
		score, tier, tier.TrustTax()*100, h.ledger.IsExiled(h.uid))

	// ── HEALTH CHECK-IN ───────────────────────────────────────────────────────
	if ci != nil {
		fmt.Fprintf(&sb, "## HEALTH CHECK-IN\n")
		fmt.Fprintf(&sb, "Mood: %d/10 | Stress: %d/10 | Sleep: %.1fh | Energy: %d/10\n",
			ci.Mood, ci.StressLevel, ci.Sleep, ci.Energy)
		fmt.Fprintf(&sb, "Decision shield active: %v\n", ci.StressLevel > 7 || ci.Sleep < 5)
		if ci.ExtraContext != "" {
			fmt.Fprintf(&sb, "[HEALTH CONTEXT LOG — background note, not a question]: %s\n", ci.ExtraContext)
		}
	} else {
		sb.WriteString("## HEALTH CHECK-IN\nEMPTY — no check-in recorded yet.\n")
	}
	sb.WriteString("\n")

	// ── RECENT ACTIVITY ───────────────────────────────────────────────────────
	// The item count in the header is the anti-hallucination anchor: the model
	// can see at a glance whether there is anything to report.
	if len(evts) > 0 {
		fmt.Fprintf(&sb, "## RECENT ACTIVITY (%d items)\n", len(evts))
		for _, e := range evts {
			gainStr := ""
			if e.Gain.Value != 0 {
				gainStr = fmt.Sprintf(" | gain: %s%.2f (%s)", e.Gain.Symbol, e.Gain.Value, e.Gain.Details)
			}
			fmt.Fprintf(&sb, "- [%s] %s — %s (decision: %s%s)\n",
				e.CreatedAt.Format("Jan 2 15:04"),
				eventTypeName(e.EventType), e.Narrative, decisionName(e.Decision), gainStr)
		}
	} else {
		sb.WriteString("## RECENT ACTIVITY (0 items) — EMPTY\n" +
			"No events have been processed by the pipeline yet. The brain pipeline\n" +
			"runs after a sync cycle. Connect integrations and trigger a sync.\n")
	}
	sb.WriteString("\n")

	// ── UNREAD NOTIFICATIONS ──────────────────────────────────────────────────
	if len(notifs) > 0 {
		fmt.Fprintf(&sb, "## UNREAD NOTIFICATIONS (%d items)\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", n.Type, n.Title, n.Body)
		}
		sb.WriteString("\n")
	}

	// ── SOCIAL DEBT SCAN ──────────────────────────────────────────────────────
	if harvest != nil {
		fmt.Fprintf(&sb, "## SOCIAL DEBT SCAN\n")
		fmt.Fprintf(&sb, "Scanned: %s | Contacts: %d | Total unreciprocated value: $%.0f\n",
			harvest.ScannedAt.Format("Jan 2 15:04 UTC"), harvest.ContactsFound, harvest.TotalValue)
		for _, d := range harvest.Debts {
			fmt.Fprintf(&sb, "  - %s: $%.0f (%d net favours) → %s\n",
				d.Contact.Name, d.EstimatedValue, d.NetFavors, d.Action)
		}
		for _, o := range harvest.Opportunities {
			fmt.Fprintf(&sb, "  Opportunity: %s\n", o)
		}
	} else {
		sb.WriteString("## SOCIAL DEBT SCAN (0 items) — EMPTY\nNo harvest scan has been run yet.\n")
	}
	sb.WriteString("\n")

	// ── SYNC SCHEDULER ────────────────────────────────────────────────────────
	fmt.Fprintf(&sb, "## SYNC SCHEDULER\nInterval: %d min", st.IntervalMinutes)
	if st.LastRunAt != nil {
		fmt.Fprintf(&sb, " | Last sync: %s", st.LastRunAt.Format("Jan 2 15:04 UTC"))
	} else {
		sb.WriteString(" | No sync has run yet")
	}
	if st.LastResult != nil {
		r := st.LastResult
		fmt.Fprintf(&sb, " | Signals: %d (accepted=%d rejected=%d ghosted=%d)",
			st.LastSignalCount, r.Accepted, r.Rejected, r.Ghosted)
	}
	sb.WriteString("\n\n")

	sb.WriteString("━━━ END OF DATA BRIEFING ━━━\n" +
		"Use only the data above. Every figure you write must come from this briefing.\n")
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
