package ledger

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

// HistoryEntry is a single ledger row returned by the history API.
type HistoryEntry struct {
	ID        int64     `json:"id"`
	Success   bool      `json:"success"`
	Impact    int64     `json:"impact"`
	CreatedAt time.Time `json:"created_at"`
}

// SQLiteLedger is a persistent reputation store backed by SQLite.
// It satisfies the Ledger interface and uses the identical scoring formula
// as LocalLedger so behaviour is unchanged after migration.
type SQLiteLedger struct {
	db *sql.DB
}

func NewSQLiteLedger(db *sql.DB) *SQLiteLedger {
	return &SQLiteLedger{db: db}
}

func (l *SQLiteLedger) Migrate() error {
	_, err := l.db.Exec(`
		CREATE TABLE IF NOT EXISTS reputation_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			uid        TEXT    NOT NULL,
			success    INTEGER NOT NULL,
			impact     INTEGER NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
		CREATE INDEX IF NOT EXISTS idx_reputation_events_uid ON reputation_events(uid);
	`)
	return err
}

// Initialize seeds the baseline score for a uid if no records exist yet.
func (l *SQLiteLedger) Initialize(uid string) {
	var count int
	if err := l.db.QueryRow(
		`SELECT COUNT(*) FROM reputation_events WHERE uid = ?`, uid,
	).Scan(&count); err != nil {
		log.Printf("ledger: initialize scan error: %v", err)
		return
	}
	if count == 0 {
		if _, err := l.db.Exec(
			`INSERT INTO reputation_events (uid, success, impact) VALUES (?, 1, ?)`,
			uid, BaselineScore,
		); err != nil {
			log.Printf("ledger: initialize insert error: %v", err)
		}
	}
}

// LogSuccess records a positive interaction.
func (l *SQLiteLedger) LogSuccess(uid string, impact int64) {
	if _, err := l.db.Exec(
		`INSERT INTO reputation_events (uid, success, impact) VALUES (?, 1, ?)`,
		uid, impact,
	); err != nil {
		log.Printf("ledger: log success error: %v", err)
	}
}

// LogBetrayal records a dishonest interaction.
// The stored impact is already multiplied by BetrayalMultiplier, matching LocalLedger.
func (l *SQLiteLedger) LogBetrayal(uid string, impact int64) {
	if _, err := l.db.Exec(
		`INSERT INTO reputation_events (uid, success, impact) VALUES (?, 0, ?)`,
		uid, impact*BetrayalMultiplier,
	); err != nil {
		log.Printf("ledger: log betrayal error: %v", err)
	}
}

// Score computes the current reputation score using the same temporal-decay
// formula as LocalLedger:
//
//	R(t) = Σ impact_i · e^(−λ·age_i)   (positive for success, negative for betrayal)
func (l *SQLiteLedger) Score(uid string) int64 {
	rows, err := l.db.Query(
		`SELECT success, impact, created_at FROM reputation_events WHERE uid = ?`, uid,
	)
	if err != nil {
		log.Printf("ledger: score query error: %v", err)
		return 0
	}
	defer rows.Close()

	now := time.Now()
	var score float64

	for rows.Next() {
		var success int
		var impact, createdAt int64
		if err := rows.Scan(&success, &impact, &createdAt); err != nil {
			log.Printf("ledger: score scan error: %v", err)
			continue
		}
		age := now.Sub(time.Unix(createdAt, 0)).Hours() / (24 * 365)
		decay := math.Exp(-DecayConstantLambda * age)
		if success == 1 {
			score += float64(impact) * decay
		} else {
			score -= float64(impact) * decay
		}
	}

	if score < 0 {
		score = 0
	}
	return int64(math.Round(score))
}

// Tier returns the reputation tier derived from the current score.
func (l *SQLiteLedger) Tier(uid string) ReputationTier {
	s := l.Score(uid)
	switch {
	case s >= 980:
		return TierSovereign
	case s >= 500:
		return TierStable
	case s >= ExileThreshold:
		return TierVolatile
	default:
		return TierExiled
	}
}

// IsExiled returns true when the score is below the exile threshold.
func (l *SQLiteLedger) IsExiled(uid string) bool {
	return l.Score(uid) < ExileThreshold
}

// Summary returns a formatted one-line reputation summary.
func (l *SQLiteLedger) Summary(uid string) string {
	score := l.Score(uid)
	tier := l.Tier(uid)
	tax := tier.TrustTax()
	if math.IsInf(tax, 1) {
		return fmt.Sprintf("%-30s | Score: EXILED | Tier: %s | Trust Tax: ∞ (blacklisted)", uid, tier)
	}
	return fmt.Sprintf("%-30s | Score: %5d | Tier: %-9s | Trust Tax: %.0f%%",
		uid, score, tier, tax*100)
}

// History returns paginated ledger events for a uid, newest first.
func (l *SQLiteLedger) History(uid string, limit, offset int) ([]HistoryEntry, error) {
	rows, err := l.db.Query(`
		SELECT id, success, impact, created_at
		FROM reputation_events
		WHERE uid = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, uid, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		var success int
		var createdAt int64
		if err := rows.Scan(&e.ID, &success, &e.Impact, &createdAt); err != nil {
			return nil, err
		}
		e.Success = success == 1
		e.CreatedAt = time.Unix(createdAt, 0).UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
