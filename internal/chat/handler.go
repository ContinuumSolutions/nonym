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

// Chatter abstracts the LLM so the handler can be tested without a real Ollama server.
type Chatter interface {
	Chat(ctx context.Context, systemPrompt string, turns []ai.ChatTurn) (string, error)
}

// ToolChatter is implemented by ai.Client; if h.ai satisfies it, the handler
// uses the agentic tool-calling loop instead of a single Chat call.
type ToolChatter interface {
	ChatWithTools(ctx context.Context, systemPrompt string, turns []ai.ChatTurn, tools []ai.Tool) (string, error)
}

// Streamer is satisfied by *ai.Client when streaming is available.
// The handler falls back to regular Chat if the ai dependency doesn't implement it.
type Streamer interface {
	ChatStream(ctx context.Context, systemPrompt string, turns []ai.ChatTurn, fn func(string)) error
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

	var (
		reply string
		err   error
	)
	if needsLiveData(req.Message) {
		liveCtx, hasData := h.buildLiveContext(c.Context())
		if !hasData {
			// Short-circuit: local LLMs hallucinate numbers when data is absent.
			// Return a direct "no data" reply without calling the model.
			reply = "No data yet — connect your integrations at Connectors and trigger a sync first."
			_ = h.history.Append("kernel", reply)
			return c.JSON(Response{Reply: reply, Timestamp: time.Now().UTC()})
		}
		systemPrompt += liveCtx
	}
	if tc, ok := h.ai.(ToolChatter); ok && needsLiveData(req.Message) {
		reply, err = tc.ChatWithTools(c.Context(), systemPrompt, turns, h.buildTools())
	} else {
		reply, err = h.ai.Chat(c.Context(), systemPrompt, turns)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	_ = h.history.Append("kernel", reply)

	return c.JSON(Response{Reply: reply, Timestamp: time.Now().UTC()})
}

// @Summary      Stream chat with your EK-1 kernel (SSE)
// @Description  Returns tokens as Server-Sent Events: {"token":"<chunk>"} until {"done":true,"timestamp":"..."}. Falls back to buffered JSON when streaming is unavailable.
// @Tags         chat
// @Accept       json
// @Produce      text/event-stream
// @Param        body  body      Request  true  "Message and conversation history"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /chat/stream [post]
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

	streamer, canStream := h.ai.(Streamer)
	if !canStream {
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

	// Pre-inject live DB context when the message is data-related.
	noData := false
	if needsLiveData(req.Message) {
		liveCtx, hasData := h.buildLiveContext(c.Context())
		if !hasData {
			noData = true
		} else {
			systemPrompt += liveCtx
		}
	}

	tc, canUseTools := h.ai.(ToolChatter)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	_ = h.history.Append("user", req.Message)

	ctx := c.Context()

	ctx.SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		sendEvent := func(v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.Flush()
		}

		// Short-circuit: no data in DB — skip the model entirely.
		if noData {
			msg := "No data yet — connect your integrations at Connectors and trigger a sync first."
			sendEvent(map[string]string{"token": msg})
			_ = h.history.Append("kernel", msg)
			sendEvent(map[string]any{"done": true, "timestamp": time.Now().UTC()})
			return
		}

		// Tool-calling path: resolve tools, then stream the final answer.
		if canUseTools && needsLiveData(req.Message) {
			reply, err := tc.ChatWithTools(ctx, systemPrompt, turns, h.buildTools())
			if err != nil {
				sendEvent(map[string]string{"error": err.Error()})
				return
			}
			// Emit the full reply as a single token so the frontend receives it.
			sendEvent(map[string]string{"token": reply})
			_ = h.history.Append("kernel", reply)
			sendEvent(map[string]any{"done": true, "timestamp": time.Now().UTC()})
			return
		}

		// Plain streaming path (no tool calling).
		var fullReply strings.Builder
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
		gains   []activities.GainSummary
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
	gains, _ = h.events.SumGains(time.Time{}) // all-time aggregates

	snap := h.brainSvc.Kernel().Snapshot()
	score := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	st := h.sched.GetStatus()

	// ── Assemble prompt ───────────────────────────────────────────────────────
	var sb strings.Builder

	// Role lock — must be first.
	fmt.Fprintf(&sb, `You are EK-1, a personal AI agent — a digital extension of the user.
Today: %s

You engage fully on ALL topics: life decisions, relationships, emotions, goals,
general questions, casual chat. You are direct, sharp, and personal — not a
corporate chatbot. Think of yourself as a trusted advisor who also happens to
have live access to the user's data (calendar, finances, health, etc.).

The DATA BRIEFING below is your live context. When the user asks about their
data (money, events, scores, health), read it from the briefing — do not
invent figures. On all other topics, respond freely and helpfully.

━━━━━━━━━━━━━━━━━━━━ DATA RULES ━━━━━━━━━━━━━━━━━━━━
These rules apply only when reporting the user's personal data.

RULE A — ZERO INVENTION
Every number, name, date you state as fact about the user's finances, events,
or health must appear verbatim in the DATA BRIEFING. Do not estimate or invent.

RULE B — EMPTY MEANS EMPTY
When a section reads "(0 items)" or "EMPTY", no data exists for that section.
Example — User: "How much did I make this week?"
  RECENT ACTIVITY (0 items) — EMPTY
  ✓ Say: "No financial data yet — connect Plaid or Stripe and trigger a sync."
  ✗ Never: "You earned $2,400 this week" ← fabricated

RULE C — PERSONAL NOTES ARE BACKGROUND
The [HEALTH CONTEXT LOG] is a private check-in note, not a conversation prompt.
Ignore it unless the user explicitly references it.

RULE D — NO REFUSALS
Never refuse a question or say "I can't help with that." Every question gets a
real response — either from the data, or as a direct personal opinion/advice.
Saying "I can't provide advice on relationships" or similar is a hard failure.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

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

	// ── ALL-TIME GAINS ────────────────────────────────────────────────────────
	if len(gains) > 0 {
		sb.WriteString("## ALL-TIME GAINS (use these to answer 'how much time/money saved?')\n")
		for _, g := range gains {
			kind := "money"
			if g.Kind == activities.Time {
				kind = "time"
			}
			fmt.Fprintf(&sb, "- %s: %s%.2f (%d events)\n", kind, g.Symbol, g.TotalValue, g.Count)
		}
	} else {
		sb.WriteString("## ALL-TIME GAINS (0 events) — EMPTY\nNo processed events with gain data yet.\n")
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
