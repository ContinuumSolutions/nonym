package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/signals"
	"github.com/gofiber/fiber/v2"
)

// SimpleHandler serves chat endpoints using signals data instead of brain/activities
type SimpleHandler struct {
	ai      *ai.Client
	prof    *profile.Store
	bio     *biometrics.Store
	notifs  *notifications.Store
	signals *signals.Store
	history *HistoryStore
}

// NewSimpleHandler creates a chat handler with simplified dependencies
func NewSimpleHandler(
	ai *ai.Client,
	prof *profile.Store,
	bio *biometrics.Store,
	notifs *notifications.Store,
	signalsStore *signals.Store,
	history *HistoryStore,
) *SimpleHandler {
	return &SimpleHandler{
		ai:      ai,
		prof:    prof,
		bio:     bio,
		notifs:  notifs,
		signals: signalsStore,
		history: history,
	}
}

// RegisterRoutes mounts the chat endpoints
func (h *SimpleHandler) RegisterRoutes(r fiber.Router) {
	r.Post("/chat", h.chat)
	r.Get("/chat/history", h.getHistory)
}

// chat handles POST /chat
func (h *SimpleHandler) chat(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message is required"})
	}

	// Convert history: map "kernel" → "assistant" for LLM
	turns := make([]ai.ChatTurn, 0, len(req.History)+1)
	for _, m := range req.History {
		role := m.Role
		if role == "kernel" {
			role = "assistant"
		}
		turns = append(turns, ai.ChatTurn{Role: role, Content: m.Content})
	}
	turns = append(turns, ai.ChatTurn{Role: "user", Content: req.Message})

	// Save user message to history
	_ = h.history.Append("user", req.Message)

	// Build system prompt with available data
	systemPrompt := h.buildSystemPrompt()

	// Get reply from LLM
	reply, err := h.ai.Chat(c.Context(), systemPrompt, turns)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Save kernel reply to history
	_ = h.history.Append("kernel", reply)

	return c.JSON(Response{
		Reply:     reply,
		Timestamp: time.Now().UTC(),
	})
}

// getHistory handles GET /chat/history
func (h *SimpleHandler) getHistory(c *fiber.Ctx) error {
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

// buildSystemPrompt creates a system prompt using available data
func (h *SimpleHandler) buildSystemPrompt() string {
	now := time.Now().UTC()
	var sb strings.Builder

	// Get profile data
	prof, _ := h.prof.Get()
	name := "EK-1"
	timezone := "UTC"
	if prof != nil {
		name = prof.KernelName
		timezone = prof.Timezone
	}

	// Role definition
	fmt.Fprintf(&sb, `You are %s, a personal AI agent — a digital extension of the user.
Today: %s | Timezone: %s

You engage fully on ALL topics: life decisions, relationships, emotions, goals, general questions, casual chat. You are direct, sharp, and personal — not a corporate chatbot.

When asked about specific data, use only the information provided below. For general questions, respond freely and helpfully.

`, name, now.Format("Monday 2 Jan 2006, 15:04 UTC"), timezone)

	// Add profile preferences if available
	if prof != nil {
		p := prof.Preferences
		fmt.Fprintf(&sb, "## USER PREFERENCES\n")
		fmt.Fprintf(&sb, "Value priorities (1–10): time=%d, money=%d, reputation=%d, privacy=%d, autonomy=%d, health=%d\n\n",
			p.TimeSovereignty, p.FinacialGrowth, p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, p.HealthRecovery)
	}

	// Add health check-in data
	if ci, err := h.bio.Get(); err == nil && ci != nil {
		fmt.Fprintf(&sb, "## HEALTH CHECK-IN\n")
		fmt.Fprintf(&sb, "Mood: %d/10 | Stress: %d/10 | Sleep: %.1fh | Energy: %d/10\n",
			ci.Mood, ci.StressLevel, ci.Sleep, ci.Energy)
		if ci.ExtraContext != "" {
			fmt.Fprintf(&sb, "Notes: %s\n", ci.ExtraContext)
		}
		sb.WriteString("\n")
	}

	// Add recent signals summary
	if summary, err := h.signals.GetSummary(); err == nil {
		fmt.Fprintf(&sb, "## SIGNALS SUMMARY\n")
		fmt.Fprintf(&sb, "Pending signals: %d | Total unread: %d\n\n", summary.TotalPending, summary.TotalPending)
	}

	// Add unread notifications
	if notifs, err := h.notifs.ListUnread(); err == nil && len(notifs) > 0 {
		fmt.Fprintf(&sb, "## UNREAD NOTIFICATIONS (%d items)\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "- [%s] %s: %s\n", n.Type, n.Title, n.Body)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Use the data above when answering questions about the user's current state. For general questions, respond as a helpful personal AI assistant.\n")

	return sb.String()
}
