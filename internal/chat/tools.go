package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/harvest"
	"github.com/egokernel/ek1/internal/notifications"
)

// buildLiveContext queries the stores directly and returns a plain-text data
// block to append to the system prompt, plus a boolean that is true only when
// at least one section contains real (non-zero) data.
//
// Plain text is used deliberately — local LLMs ignore JSON "no data" markers
// and hallucinate numbers. Explicit English assertions are much harder to override.
func (h *Handler) buildLiveContext(ctx context.Context) (section string, hasData bool) {
	var wg sync.WaitGroup
	var (
		gains         []activities.GainSummary
		events        []activities.Event
		notifs        []notifications.Notification
		harvestResult *harvest.HarvestResult
	)

	wg.Add(4)
	go func() { defer wg.Done(); gains, _ = h.events.SumGains(time.Time{}) }()
	go func() { defer wg.Done(); events, _ = h.events.List() }()
	go func() { defer wg.Done(); notifs, _ = h.notifs.ListUnread() }()
	go func() { defer wg.Done(); harvestResult, _ = h.harvest.Latest() }()
	wg.Wait()

	var sb strings.Builder
	sb.WriteString("\n\n⚡ LIVE DATA SNAPSHOT — queried right now from the database\n")
	sb.WriteString("════════════════════════════════════════════════════════\n")
	sb.WriteString("USE ONLY THE NUMBERS BELOW. DO NOT ADD, INVENT, OR RECALL ANY OTHER FIGURES.\n\n")

	// ── Gains ─────────────────────────────────────────────────────────────────
	sb.WriteString("GAINS (all time across all kernel decisions):\n")
	gainsByKind := map[activities.GainKind]*activities.GainSummary{}
	for i := range gains {
		gainsByKind[gains[i].Kind] = &gains[i]
	}
	if m := gainsByKind[activities.Money]; m != nil && m.Count > 0 {
		hasData = true
		fmt.Fprintf(&sb, "  Money: %s%.2f across %d events\n", m.Symbol, m.TotalValue, m.Count)
	} else {
		sb.WriteString("  Money: NO DATA — Plaid or Stripe not connected or no sync run yet\n")
	}
	if t := gainsByKind[activities.Time]; t != nil && t.Count > 0 {
		hasData = true
		fmt.Fprintf(&sb, "  Time:  %.2f%s across %d events\n", t.TotalValue, t.Symbol, t.Count)
	} else {
		sb.WriteString("  Time:  NO DATA — no events with time gains yet\n")
	}

	// ── Recent events ─────────────────────────────────────────────────────────
	sb.WriteString("\nRECENT DECISIONS (last 10):\n")
	shown := 0
	for _, e := range events {
		if shown >= 10 {
			break
		}
		hasData = true
		gainStr := ""
		if e.Gain.Value != 0 {
			gainStr = fmt.Sprintf(" [gain: %s%.2f]", e.Gain.Symbol, e.Gain.Value)
		}
		fmt.Fprintf(&sb, "  %s — %s — %s%s\n",
			e.CreatedAt.Format("Jan 2"), eventTypeName(e.EventType), e.Narrative, gainStr)
		shown++
	}
	if shown == 0 {
		sb.WriteString("  NONE — brain pipeline has not processed any signals yet\n")
	}

	// ── Notifications ─────────────────────────────────────────────────────────
	if len(notifs) > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nUNREAD NOTIFICATIONS (%d):\n", len(notifs))
		for _, n := range notifs {
			fmt.Fprintf(&sb, "  [%s] %s — %s\n", n.Type, n.Title, n.Body)
		}
	} else {
		sb.WriteString("\nUNREAD NOTIFICATIONS: none\n")
	}

	// ── Harvest ───────────────────────────────────────────────────────────────
	if harvestResult != nil && harvestResult.TotalValue > 0 {
		hasData = true
		fmt.Fprintf(&sb, "\nSOCIAL DEBT SCAN: $%.0f unreciprocated across %d contacts\n",
			harvestResult.TotalValue, harvestResult.ContactsFound)
	} else {
		sb.WriteString("\nSOCIAL DEBT SCAN: NO DATA — run a harvest scan first\n")
	}

	sb.WriteString("\n════════════════════════════════════════════════════════\n")
	if !hasData {
		sb.WriteString("⚠️  ALL SECTIONS SHOW NO DATA.\n")
		sb.WriteString("You MUST respond with ONLY this: \"No data yet — connect your integrations at Connectors and trigger a sync.\"\n")
		sb.WriteString("Do NOT mention any dollar amounts, hours, or percentages.\n")
	} else {
		sb.WriteString("Answer using ONLY the figures above. Any number not listed above is FORBIDDEN.\n")
	}
	sb.WriteString("END LIVE DATA SNAPSHOT\n")

	return sb.String(), hasData
}

// buildTools returns Ollama tool definitions wired to the handler's live stores.
// Each tool's Execute func queries the DB directly and returns a JSON string.
func (h *Handler) buildTools() []ai.Tool {
	return []ai.Tool{
		{
			Name: "get_gain_summary",
			Description: "Return the total time saved or money earned/saved across all kernel decisions. " +
				"Use this to answer questions like 'How much time have you saved me?', " +
				"'How much money has the kernel saved?' or similar aggregate questions.",
			Parameters: ai.ToolParameters{
				Properties: map[string]ai.ToolParam{
					"kind": {
						Type:        "string",
						Description: "Which gain kind to summarise: 'time' (hours), 'money' (currency), or 'all'.",
						Enum:        []string{"time", "money", "all"},
					},
					"since_days": {
						Type:        "integer",
						Description: "Only count gains from the last N days. 0 or omit for all time.",
					},
				},
				Required: []string{"kind"},
			},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				sinceDays := intArg(args, "since_days", 0)
				since := time.Time{}
				if sinceDays > 0 {
					since = time.Now().Add(-time.Duration(sinceDays) * 24 * time.Hour)
				}
				gains, err := h.events.SumGains(since)
				if err != nil {
					return "", err
				}
				kind := stringArg(args, "kind", "all")
				var filtered []activities.GainSummary
				for _, g := range gains {
					switch kind {
					case "time":
						if g.Kind == activities.Time {
							filtered = append(filtered, g)
						}
					case "money":
						if g.Kind == activities.Money {
							filtered = append(filtered, g)
						}
					default:
						filtered = append(filtered, g)
					}
				}
				if filtered == nil {
					return `{"total_value":0,"count":0,"note":"No gain data recorded yet."}`, nil
				}
				b, _ := json.Marshal(filtered)
				return string(b), nil
			},
		},
		{
			Name: "get_recent_events",
			Description: "Return recent brain events (decisions, signals processed). " +
				"Use this to answer questions about what the kernel has accepted, declined, or automated, " +
				"or to list activity by category.",
			Parameters: ai.ToolParameters{
				Properties: map[string]ai.ToolParam{
					"limit": {
						Type:        "integer",
						Description: "Max events to return (default 10, max 50).",
					},
					"event_type": {
						Type:        "string",
						Description: "Filter by category. Omit for all.",
						Enum:        []string{"finance", "calendar", "communication", "billing", "health", "other"},
					},
				},
				Required: []string{},
			},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				all, err := h.events.List()
				if err != nil {
					return "", err
				}
				limit := intArg(args, "limit", 10)
				if limit > 50 {
					limit = 50
				}
				typeFilter := strings.ToLower(stringArg(args, "event_type", ""))

				var out []map[string]any
				for _, e := range all {
					if typeFilter != "" && strings.ToLower(eventTypeName(e.EventType)) != typeFilter {
						continue
					}
					out = append(out, map[string]any{
						"id":             e.ID,
						"event_type":     eventTypeName(e.EventType),
						"decision":       decisionName(e.Decision),
						"importance":     e.Importance,
						"narrative":      e.Narrative,
						"gain_kind":      gainKindName(e.Gain.Kind),
						"gain_value":     e.Gain.Value,
						"gain_symbol":    e.Gain.Symbol,
						"source_service": e.SourceService,
						"created_at":     e.CreatedAt.Format(time.RFC3339),
					})
					if len(out) >= limit {
						break
					}
				}
				b, _ := json.Marshal(out)
				return string(b), nil
			},
		},
		{
			Name:        "get_brain_status",
			Description: "Return kernel stats: total decisions made, accepted/declined/automated counts, reputation score and tier, biometric shield status.",
			Parameters:  ai.ToolParameters{Properties: map[string]ai.ToolParam{}, Required: []string{}},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				snap := h.brainSvc.Kernel().Snapshot()
				score := h.ledger.Score(h.uid)
				tier := h.ledger.Tier(h.uid)
				ci, _ := h.bio.Get()
				shieldActive := ci != nil && (ci.StressLevel > 7 || ci.Sleep < 5)
				result := map[string]any{
					"status":           snap.Status,
					"decisions_made":   snap.DecisionCount,
					"identity_entropy": snap.IdentityEntropy,
					"reputation_score": score,
					"tier":             string(tier),
					"biometric_shield": shieldActive,
				}
				b, _ := json.Marshal(result)
				return string(b), nil
			},
		},
		{
			Name:        "get_biometrics",
			Description: "Return the current health check-in: mood, stress level, sleep hours, energy. Use for wellbeing or shield status questions.",
			Parameters:  ai.ToolParameters{Properties: map[string]ai.ToolParam{}, Required: []string{}},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				ci, err := h.bio.Get()
				if err != nil || ci == nil {
					return `{"message":"No check-in recorded yet."}`, nil
				}
				b, _ := json.Marshal(ci)
				return string(b), nil
			},
		},
		{
			Name:        "get_notifications",
			Description: "Return unread kernel notifications: H2HI alerts, opportunities, harvest findings.",
			Parameters:  ai.ToolParameters{Properties: map[string]ai.ToolParam{}, Required: []string{}},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				notifs, err := h.notifs.ListUnread()
				if err != nil {
					return "", err
				}
				if notifs == nil {
					return `[]`, nil
				}
				b, _ := json.Marshal(notifs)
				return string(b), nil
			},
		},
		{
			Name:        "get_harvest_results",
			Description: "Return the latest social debt harvest scan: contacts who owe favours and estimated values.",
			Parameters:  ai.ToolParameters{Properties: map[string]ai.ToolParam{}, Required: []string{}},
			Execute: func(ctx context.Context, args map[string]any) (string, error) {
				result, err := h.harvest.Latest()
				if err != nil || result == nil {
					return `{"message":"No harvest scan has been run yet."}`, nil
				}
				b, _ := json.Marshal(result)
				return string(b), nil
			},
		},
	}
}

// gainKindName maps GainKind to a human-readable string.
func gainKindName(k activities.GainKind) string {
	if k == activities.Time {
		return "time"
	}
	return "money"
}

// intArg extracts an integer argument from the args map, with a default.
// Ollama passes JSON numbers as float64.
func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return def
}

// stringArg extracts a string argument from the args map, with a default.
func stringArg(args map[string]any, key string, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}
