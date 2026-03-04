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
)

// buildLiveContext runs all tools concurrently and returns a formatted string
// that is appended to the system prompt. This guarantees fresh data is always
// in-context without relying on the model to call tools itself.
func (h *Handler) buildLiveContext(ctx context.Context) string {
	tools := h.buildTools()

	type result struct {
		name string
		data string
	}
	results := make([]result, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func(i int, t ai.Tool) {
			defer wg.Done()
			data, err := t.Execute(ctx, map[string]any{"kind": "all"})
			if err != nil {
				data = `{"error":"` + err.Error() + `"}`
			} else if data == "" || data == "null" || data == "[]" {
				data = `{"message":"no data recorded yet"}`
			}
			results[i] = result{name: t.Name, data: data}
		}(i, t)
	}
	wg.Wait()

	var sb strings.Builder
	sb.WriteString("\n\n━━━ LIVE QUERY RESULTS — answer data questions from these ━━━\n")
	sb.WriteString("These were fetched RIGHT NOW from the database. They override the briefing above.\n")
	for _, r := range results {
		fmt.Fprintf(&sb, "\n[%s]\n%s\n", r.name, r.data)
	}
	sb.WriteString("\n━━━ END LIVE QUERY RESULTS ━━━\n")
	sb.WriteString("If a section shows no data, tell the user to connect the relevant integration and trigger a sync.\n")
	return sb.String()
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
