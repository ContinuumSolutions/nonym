// unghost is a management CLI for reviewing and overriding ghosted signals.
//
// A signal is ghosted when the brain pipeline detects manipulation_pct > 0.15.
// This script lets you inspect ghosted events and manually accept them when
// the ghost decision was wrong (e.g. a payment gateway misread as manipulation).
//
// Usage:
//
//	go run ./scripts/unghost                        # list all ghosted events
//	go run ./scripts/unghost --service=intasend     # list ghosted events for one service
//	go run ./scripts/unghost --id=42                # unghost a single event
//	go run ./scripts/unghost --service=intasend --apply  # unghost all for a service
//	go run ./scripts/unghost --all --apply          # unghost everything
//	go run ./scripts/unghost --all --apply --db=/path/to/ek1.db
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	_ "modernc.org/sqlite"
)

// signalAnalysis mirrors activities.SignalAnalysis for JSON round-tripping.
type signalAnalysis struct {
	ServiceSlug     string  `json:"service_slug"`
	SignalTitle      string  `json:"signal_title"`
	EstimatedROI    float64 `json:"estimated_roi"`
	TimeCommitment  float64 `json:"time_commitment"`
	ManipulationPct float64 `json:"manipulation_pct"`
	ROIThreshold    float64 `json:"roi_threshold"`
	TriageGate      string  `json:"triage_gate"`
	DecideUtility   float64 `json:"decide_utility,omitempty"`
	DecideThreshold float64 `json:"decide_threshold,omitempty"`
}

type ghostedEvent struct {
	ID           int
	EventType    int
	Narrative    string
	SourceService string
	Analysis     signalAnalysis
	CreatedAt    time.Time
}

func main() {
	dbPath  := flag.String("db", envOr("EK1_DB", "./ek1.db"), "Path to ek1.db")
	service := flag.String("service", "", "Filter by service slug (e.g. intasend)")
	id      := flag.Int("id", 0, "Unghost a single event by ID")
	all     := flag.Bool("all", false, "Target all ghosted events")
	apply   := flag.Bool("apply", false, "Actually apply the unghost (default: list only)")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatalf("connect to %s: %v\nIs EK-1 running? Make sure the DB path is correct.", *dbPath, err)
	}
	if err := checkSchema(db); err != nil {
		fatalf("%v", err)
	}

	// ── Single event by ID ───────────────────────────────────────────────────
	if *id != 0 {
		ev, err := loadEvent(db, *id)
		if err != nil {
			fatalf("event %d not found: %v", *id, err)
		}
		if ev.Analysis.TriageGate != "manipulation" {
			fatalf("event %d triage_gate=%q — not a ghosted event", *id, ev.Analysis.TriageGate)
		}
		printEvents([]ghostedEvent{*ev})
		fmt.Println()
		if !*apply {
			fmt.Println("Re-run with --apply to unghost this event.")
			return
		}
		n, err := unghost(db, "WHERE id = ?", *id)
		if err != nil {
			fatalf("unghost: %v", err)
		}
		fmt.Printf("Unghosted %d event(s).\n", n)
		return
	}

	// ── List / bulk unghost ──────────────────────────────────────────────────
	if !*all && *service == "" {
		// Default: list all ghosted events grouped by service.
		events, err := loadGhosted(db, "")
		if err != nil {
			fatalf("query: %v", err)
		}
		if len(events) == 0 {
			fmt.Println("No ghosted events found.")
			return
		}
		printEvents(events)
		fmt.Printf("\nTotal: %d ghosted event(s)\n", len(events))
		fmt.Println("\nTo unghost:")
		fmt.Println("  by service:  go run ./scripts/unghost --service=<slug> --apply")
		fmt.Println("  single:      go run ./scripts/unghost --id=<N> --apply")
		fmt.Println("  everything:  go run ./scripts/unghost --all --apply")
		return
	}

	var (
		events []ghostedEvent
		where  string
		args   []interface{}
	)

	if *service != "" {
		events, err = loadGhosted(db, *service)
		where = "WHERE source_service = ? AND decision = 2 AND analysis LIKE '%\"triage_gate\":\"manipulation\"%'"
		args = []interface{}{*service}
	} else {
		events, err = loadGhosted(db, "")
		where = "WHERE decision = 2 AND analysis LIKE '%\"triage_gate\":\"manipulation\"%'"
	}
	if err != nil {
		fatalf("query: %v", err)
	}

	if len(events) == 0 {
		if *service != "" {
			fmt.Printf("No ghosted events for service %q.\n", *service)
		} else {
			fmt.Println("No ghosted events found.")
		}
		return
	}

	printEvents(events)
	fmt.Printf("\nTotal: %d ghosted event(s)\n", len(events))

	if !*apply {
		fmt.Println("\nRe-run with --apply to unghost these events.")
		return
	}

	fmt.Println()
	n, err := unghost(db, where, args...)
	if err != nil {
		fatalf("unghost: %v", err)
	}
	fmt.Printf("Unghosted %d event(s).\n", n)
}

// loadGhosted returns ghosted events (triage_gate=manipulation) optionally filtered by service.
func loadGhosted(db *sql.DB, service string) ([]ghostedEvent, error) {
	q := `
		SELECT id, event_type, narrative, source_service, analysis, created_at
		FROM events
		WHERE decision = 2
		  AND analysis LIKE '%"triage_gate":"manipulation"%'
	`
	var args []interface{}
	if service != "" {
		q += ` AND source_service = ?`
		args = append(args, service)
	}
	q += ` ORDER BY created_at DESC`

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// loadEvent returns a single event by ID.
func loadEvent(db *sql.DB, id int) (*ghostedEvent, error) {
	rows, err := db.Query(`
		SELECT id, event_type, narrative, source_service, analysis, created_at
		FROM events WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	evs, err := scanEvents(rows)
	if err != nil {
		return nil, err
	}
	if len(evs) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return &evs[0], nil
}

func scanEvents(rows *sql.Rows) ([]ghostedEvent, error) {
	var out []ghostedEvent
	for rows.Next() {
		var ev ghostedEvent
		var analysisJSON string
		var createdAt int64
		if err := rows.Scan(&ev.ID, &ev.EventType, &ev.Narrative, &ev.SourceService, &analysisJSON, &createdAt); err != nil {
			return nil, err
		}
		ev.CreatedAt = time.Unix(createdAt, 0).UTC()
		_ = json.Unmarshal([]byte(analysisJSON), &ev.Analysis)
		out = append(out, ev)
	}
	return out, rows.Err()
}

// unghost updates matching events: sets decision=1 (Accepted) and triage_gate="unghosted_manual".
// where + args are appended to the event selection predicate.
func unghost(db *sql.DB, where string, args ...interface{}) (int64, error) {
	now := time.Now().UTC().Unix()

	// Load matching events so we can rewrite the analysis JSON.
	q := `SELECT id, analysis FROM events ` + where
	rows, err := db.Query(q, args...)
	if err != nil {
		return 0, fmt.Errorf("select: %w", err)
	}

	type row struct {
		id       int
		analysis signalAnalysis
	}
	var targets []row
	for rows.Next() {
		var r row
		var aj string
		if err := rows.Scan(&r.id, &aj); err != nil {
			rows.Close()
			return 0, err
		}
		_ = json.Unmarshal([]byte(aj), &r.analysis)
		targets = append(targets, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var updated int64
	for _, t := range targets {
		t.analysis.TriageGate = "unghosted_manual"
		aj, _ := json.Marshal(t.analysis)
		res, err := db.Exec(`
			UPDATE events SET decision = 1, analysis = ?, updated_at = ? WHERE id = ?
		`, string(aj), now, t.id)
		if err != nil {
			return updated, fmt.Errorf("update event %d: %w", t.id, err)
		}
		n, _ := res.RowsAffected()
		updated += n
	}
	return updated, nil
}

// printEvents renders a compact table of ghosted events.
func printEvents(events []ghostedEvent) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSERVICE\tMANIP%\tROI\tDATE\tTITLE")
	fmt.Fprintln(w, strings.Repeat("-", 90))
	for _, ev := range events {
		title := ev.Analysis.SignalTitle
		if title == "" {
			// Fall back to narrative (truncated).
			title = ev.Narrative
			if len(title) > 60 {
				title = title[:57] + "..."
			}
		}
		fmt.Fprintf(w, "%d\t%s\t%.0f%%\t$%.2f\t%s\t%s\n",
			ev.ID,
			ev.SourceService,
			ev.Analysis.ManipulationPct*100,
			ev.Analysis.EstimatedROI,
			ev.CreatedAt.Format("2006-01-02 15:04"),
			title,
		)
	}
	w.Flush()
}

// checkSchema verifies the events table has the columns this script requires.
// They are added by activities.Store.Migrate() on first server start.
func checkSchema(db *sql.DB) error {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('events')
		WHERE name IN ('source_service', 'analysis')
	`).Scan(&count)
	if err != nil || count < 2 {
		return fmt.Errorf(
			"events table is missing required columns (source_service, analysis).\n" +
				"Start EK-1 at least once so it can run its database migrations, then retry.",
		)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
