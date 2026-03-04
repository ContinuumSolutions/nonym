// ek1-mcp is a Model Context Protocol server that exposes EK-1 data as tools.
// Connect it to Claude Desktop (or any MCP client) so Claude can answer
// questions like "How much time have you saved?" using real database data.
//
// Usage:
//
//	./ek1-mcp --db ./ek1.db
//
// Claude Desktop config (~/.config/claude/claude_desktop_config.json):
//
//	{
//	  "mcpServers": {
//	    "ek1": {
//	      "command": "/path/to/ek1-mcp",
//	      "args": ["--db", "/path/to/ek1.db"]
//	    }
//	  }
//	}
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "./ek1.db", "path to ek1.db SQLite file")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath+"?mode=ro")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		log.Fatalf("pragma: %v", err)
	}

	q := &querier{db: db}

	s := server.NewMCPServer("EK-1", "1.0.0",
		server.WithToolCapabilities(false),
	)

	// ── tool: get_recent_events ───────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_recent_events",
			mcp.WithDescription("Return the most recent brain events (decisions, signals processed). "+
				"Use this to answer questions about what the kernel has been doing, "+
				"what was accepted/declined, or activity by category."),
			mcp.WithNumber("limit",
				mcp.Description("Max events to return (1–100, default 20)"),
			),
			mcp.WithString("event_type",
				mcp.Description("Filter by type: finance, calendar, communication, billing, health, other. Omit for all."),
				mcp.Enum("finance", "calendar", "communication", "billing", "health", "other"),
			),
			mcp.WithNumber("since_days",
				mcp.Description("Only events from the last N days. 0 or omit = all time."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			limit := mcp.ParseInt(req, "limit", 20)
			if limit < 1 {
				limit = 1
			}
			if limit > 100 {
				limit = 100
			}
			sinceDays := mcp.ParseInt(req, "since_days", 0)
			evtType := mcp.ParseString(req, "event_type", "")

			events, err := q.recentEvents(limit, evtType, sinceDays)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(events, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_gain_summary ────────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_gain_summary",
			mcp.WithDescription("Return aggregated gains (time saved or money earned/saved) "+
				"across all processed events. Use this to answer 'How much time have you saved?', "+
				"'How much money has the kernel saved me?', or similar totals questions."),
			mcp.WithString("kind",
				mcp.Description("Which gain kind to summarise: 'time' (hours), 'money' (currency), or 'all'."),
				mcp.Enum("time", "money", "all"),
				mcp.Required(),
			),
			mcp.WithNumber("since_days",
				mcp.Description("Only count gains from the last N days. 0 or omit = all time."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			kind := mcp.ParseString(req, "kind", "all")
			sinceDays := mcp.ParseInt(req, "since_days", 0)

			summaries, err := q.gainSummary(kind, sinceDays)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(summaries, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_brain_status ────────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_brain_status",
			mcp.WithDescription("Return the current kernel state: decision count, utility threshold, "+
				"reputation score and tier, last sync info, and biometric shield status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			status, err := q.brainStatus()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(status, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_biometrics ──────────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_biometrics",
			mcp.WithDescription("Return the current health check-in: mood, stress, sleep, energy. "+
				"Use this to answer questions about the user's wellbeing or shield status."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			bio, err := q.biometrics()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(bio, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_notifications ───────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_notifications",
			mcp.WithDescription("Return kernel notifications (H2HI alerts, opportunities, harvest findings). "+
				"Use this to answer questions about alerts or things requiring attention."),
			mcp.WithBoolean("unread_only",
				mcp.Description("If true (default), return only unread notifications."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			unreadOnly := mcp.ParseBoolean(req, "unread_only", true)
			notifs, err := q.notifications(unreadOnly)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(notifs, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_reputation_history ─────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_reputation_history",
			mcp.WithDescription("Return reputation ledger entries (successes and betrayals). "+
				"Use this for questions about trust score, reputation trajectory, or specific ledger events."),
			mcp.WithNumber("limit",
				mcp.Description("Max entries to return (1–50, default 20)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			limit := mcp.ParseInt(req, "limit", 20)
			if limit < 1 {
				limit = 1
			}
			if limit > 50 {
				limit = 50
			}
			hist, err := q.reputationHistory(limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(hist, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	// ── tool: get_harvest_results ─────────────────────────────────────────────
	s.AddTool(
		mcp.NewTool("get_harvest_results",
			mcp.WithDescription("Return the latest social debt harvest scan. "+
				"Use this to answer questions about unreciprocated favours, social debts, or contacts who owe."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := q.latestHarvest()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(b)), nil
		},
	)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp serve: %v", err)
	}
}

// ── querier ───────────────────────────────────────────────────────────────────

type querier struct{ db *sql.DB }

// sinceUnix converts a "last N days" param to a Unix timestamp.
// Returns 0 (= all time) when days is 0.
func sinceUnix(days int) int64 {
	if days <= 0 {
		return 0
	}
	return time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
}

// eventTypeInt maps a human name to the integer stored in the DB.
func eventTypeInt(name string) (int, bool) {
	m := map[string]int{
		"finance": 0, "calendar": 1, "communication": 2,
		"billing": 3, "health": 4, "other": 5,
	}
	v, ok := m[name]
	return v, ok
}

func eventTypeName(v int) string {
	names := []string{"Finance", "Calendar", "Communication", "Billing", "Health", "Other"}
	if v >= 0 && v < len(names) {
		return names[v]
	}
	return "Unknown"
}

func decisionName(v int) string {
	names := []string{"Pending", "Accepted", "Declined", "Negotiated", "Automated", "Cancelled"}
	if v >= 0 && v < len(names) {
		return names[v]
	}
	return "Unknown"
}

type eventRow struct {
	ID            int     `json:"id"`
	EventType     string  `json:"event_type"`
	Decision      string  `json:"decision"`
	Importance    int     `json:"importance"`
	Narrative     string  `json:"narrative"`
	GainKind      string  `json:"gain_kind,omitempty"`
	GainValue     float64 `json:"gain_value,omitempty"`
	GainSymbol    string  `json:"gain_symbol,omitempty"`
	GainDetails   string  `json:"gain_details,omitempty"`
	SourceService string  `json:"source_service,omitempty"`
	CreatedAt     string  `json:"created_at"`
}


func (q *querier) recentEvents(limit int, evtType string, sinceDays int) ([]eventRow, error) {
	since := sinceUnix(sinceDays)
	var rows *sql.Rows
	var err error
	base := `SELECT id, event_type, decision, importance, narrative,
	                gain_kind, gain_value, gain_symbol, gain_details, source_service, created_at
	         FROM events WHERE created_at >= ?`
	if typeInt, ok := eventTypeInt(evtType); ok {
		rows, err = q.db.Query(base+` AND event_type = ? ORDER BY created_at DESC LIMIT ?`, since, typeInt, limit)
	} else {
		rows, err = q.db.Query(base+` ORDER BY created_at DESC LIMIT ?`, since, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []eventRow
	for rows.Next() {
		var r eventRow
		var evtTypeInt, decisionInt, gainKind int
		var createdAt int64
		if err := rows.Scan(&r.ID, &evtTypeInt, &decisionInt, &r.Importance, &r.Narrative,
			&gainKind, &r.GainValue, &r.GainSymbol, &r.GainDetails, &r.SourceService, &createdAt); err != nil {
			return nil, err
		}
		r.EventType = eventTypeName(evtTypeInt)
		r.Decision = decisionName(decisionInt)
		if gainKind == 0 {
			r.GainKind = "money"
		} else {
			r.GainKind = "time"
		}
		r.CreatedAt = time.Unix(createdAt, 0).UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

type gainSummaryRow struct {
	Kind       string  `json:"kind"`
	TotalValue float64 `json:"total_value"`
	Count      int     `json:"count"`
	Symbol     string  `json:"symbol"`
	Note       string  `json:"note"`
}

func (q *querier) gainSummary(kind string, sinceDays int) ([]gainSummaryRow, error) {
	since := sinceUnix(sinceDays)
	var kindFilter string
	switch kind {
	case "money":
		kindFilter = " AND gain_kind = 0"
	case "time":
		kindFilter = " AND gain_kind = 1"
	}

	rows, err := q.db.Query(`
		SELECT gain_kind,
		       SUM(CASE gain_type WHEN 0 THEN gain_value ELSE -gain_value END),
		       COUNT(*),
		       MAX(gain_symbol)
		FROM events
		WHERE gain_value != 0 AND created_at >= ?`+kindFilter+`
		GROUP BY gain_kind
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []gainSummaryRow
	for rows.Next() {
		var g gainSummaryRow
		var gainKindInt int
		if err := rows.Scan(&gainKindInt, &g.TotalValue, &g.Count, &g.Symbol); err != nil {
			return nil, err
		}
		if gainKindInt == 0 {
			g.Kind = "money"
			g.Note = fmt.Sprintf("Total net money gained: %s%.2f across %d events", g.Symbol, g.TotalValue, g.Count)
		} else {
			g.Kind = "time"
			g.Note = fmt.Sprintf("Total time saved: %.2f%s across %d events", g.TotalValue, g.Symbol, g.Count)
		}
		out = append(out, g)
	}
	if len(out) == 0 {
		out = []gainSummaryRow{{Kind: kind, TotalValue: 0, Count: 0, Note: "No gain data recorded yet."}}
	}
	return out, rows.Err()
}

type brainStatusRow struct {
	TotalEvents    int     `json:"total_events"`
	AcceptedEvents int     `json:"accepted_events"`
	DeclinedEvents int     `json:"declined_events"`
	AutomatedEvents int    `json:"automated_events"`
	ReputationScore float64 `json:"reputation_score"`
	KernelName     string  `json:"kernel_name"`
	Timezone       string  `json:"timezone"`
	LastSyncAt     string  `json:"last_sync_at,omitempty"`
	BiometricShield bool   `json:"biometric_shield_active"`
}

func (q *querier) brainStatus() (*brainStatusRow, error) {
	r := &brainStatusRow{}

	_ = q.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&r.TotalEvents)
	_ = q.db.QueryRow(`SELECT COUNT(*) FROM events WHERE decision = 1`).Scan(&r.AcceptedEvents)
	_ = q.db.QueryRow(`SELECT COUNT(*) FROM events WHERE decision = 2`).Scan(&r.DeclinedEvents)
	_ = q.db.QueryRow(`SELECT COUNT(*) FROM events WHERE decision = 4`).Scan(&r.AutomatedEvents)

	// Reputation: exponential decay sum
	_ = q.db.QueryRow(`
		SELECT COALESCE(SUM(CAST(impact AS REAL) * EXP(-0.01 * ((unixepoch() - created_at) / 86400.0))), 0)
		FROM reputation_events
	`).Scan(&r.ReputationScore)

	_ = q.db.QueryRow(`SELECT kernel_name, timezone FROM profile WHERE id = 1`).Scan(&r.KernelName, &r.Timezone)

	// Biometric shield: stress > 7 OR sleep < 5
	var stress, sleep float64
	if err := q.db.QueryRow(`SELECT stress_level, sleep FROM check_ins WHERE id = 1`).Scan(&stress, &sleep); err == nil {
		r.BiometricShield = stress > 7 || sleep < 5
	}

	return r, nil
}

type bioRow struct {
	Mood        int     `json:"mood_1_10"`
	StressLevel int     `json:"stress_1_10"`
	Sleep       float64 `json:"sleep_hours"`
	Energy      int     `json:"energy_1_10"`
	ShieldActive bool   `json:"decision_shield_active"`
	ExtraContext string `json:"extra_context,omitempty"`
}

func (q *querier) biometrics() (*bioRow, error) {
	r := &bioRow{}
	err := q.db.QueryRow(`SELECT mood, stress_level, sleep, energy, COALESCE(extra_context,'') FROM check_ins WHERE id = 1`).
		Scan(&r.Mood, &r.StressLevel, &r.Sleep, &r.Energy, &r.ExtraContext)
	if err == sql.ErrNoRows {
		return &bioRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	r.ShieldActive = r.StressLevel > 7 || r.Sleep < 5
	return r, nil
}

type notifRow struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

func (q *querier) notifications(unreadOnly bool) ([]notifRow, error) {
	query := `SELECT id, type, title, body, read, created_at FROM notifications`
	if unreadOnly {
		query += ` WHERE read = 0`
	}
	query += ` ORDER BY created_at DESC LIMIT 50`

	rows, err := q.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []notifRow
	for rows.Next() {
		var r notifRow
		var createdAt int64
		var readInt int
		if err := rows.Scan(&r.ID, &r.Type, &r.Title, &r.Body, &readInt, &createdAt); err != nil {
			return nil, err
		}
		r.Read = readInt == 1
		r.CreatedAt = time.Unix(createdAt, 0).UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

type repHistRow struct {
	ID        string  `json:"id"`
	Success   bool    `json:"success"`
	Impact    float64 `json:"impact"`
	CreatedAt string  `json:"created_at"`
}

func (q *querier) reputationHistory(limit int) ([]repHistRow, error) {
	rows, err := q.db.Query(`
		SELECT uid, success, impact, created_at
		FROM reputation_events
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repHistRow
	for rows.Next() {
		var r repHistRow
		var successInt int
		var createdAt int64
		if err := rows.Scan(&r.ID, &successInt, &r.Impact, &createdAt); err != nil {
			return nil, err
		}
		r.Success = successInt == 1
		r.CreatedAt = time.Unix(createdAt, 0).UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *querier) latestHarvest() (map[string]any, error) {
	var blob string
	err := q.db.QueryRow(`SELECT result_json FROM harvest_results ORDER BY scanned_at DESC LIMIT 1`).Scan(&blob)
	if err == sql.ErrNoRows {
		return map[string]any{"message": "No harvest scan has been run yet."}, nil
	}
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(blob), &out); err != nil {
		return nil, err
	}
	return out, nil
}
