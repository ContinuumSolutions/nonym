package signals

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS signals (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			service_slug    TEXT    NOT NULL,
			raw_signal      TEXT    NOT NULL,  -- JSON blob of datasync.RawSignal
			analysis        TEXT    NOT NULL,  -- JSON blob of ai.AnalysedSignal
			status          INTEGER NOT NULL DEFAULT 0,
			reply_status    INTEGER NOT NULL DEFAULT 0,
			user_notes      TEXT    NOT NULL DEFAULT '',
			processed_at    INTEGER NOT NULL DEFAULT (unixepoch()),
			last_updated    INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	if err != nil {
		return err
	}

	// Indexes for common queries
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_signals_status ON signals(status)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_signals_service ON signals(service_slug)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_signals_processed_at ON signals(processed_at)`)

	// Draft replies table
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS draft_replies (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			signal_id     INTEGER NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
			original_text TEXT    NOT NULL,
			edited_text   TEXT    NOT NULL DEFAULT '',
			tone          TEXT    NOT NULL DEFAULT 'professional',
			recipients    TEXT    NOT NULL DEFAULT '',  -- JSON array
			subject       TEXT    NOT NULL DEFAULT '',
			status        INTEGER NOT NULL DEFAULT 1,  -- drafted
			created_at    INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at    INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

// Create stores a new processed signal.
func (s *Store) Create(rawSignal datasync.RawSignal, analysis ai.AnalysedSignal) (*Signal, error) {
	rawJSON, err := json.Marshal(rawSignal)
	if err != nil {
		return nil, err
	}
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		INSERT INTO signals (service_slug, raw_signal, analysis, processed_at, last_updated)
		VALUES (?, ?, ?, ?, ?)
	`, rawSignal.ServiceSlug, string(rawJSON), string(analysisJSON), now, now)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.Get(int(id))
}

// Get retrieves a signal by ID.
func (s *Store) Get(id int) (*Signal, error) {
	var sig Signal
	var rawSignalJSON, analysisJSON string
	var processedAt, lastUpdated int64

	err := s.db.QueryRow(`
		SELECT id, service_slug, raw_signal, analysis, status, reply_status,
		       user_notes, processed_at, last_updated
		FROM signals WHERE id = ?
	`, id).Scan(
		&sig.ID, &sig.ServiceSlug, &rawSignalJSON, &analysisJSON,
		&sig.Status, &sig.ReplyStatus, &sig.UserNotes, &processedAt, &lastUpdated,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(rawSignalJSON), &sig.OriginalSignal); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(analysisJSON), &sig.Analysis); err != nil {
		return nil, err
	}

	sig.ProcessedAt = time.Unix(processedAt, 0).UTC()
	sig.LastUpdated = time.Unix(lastUpdated, 0).UTC()

	return &sig, nil
}

// List retrieves signals based on filter criteria.
func (s *Store) List(filter FilterCriteria, limit int) ([]*Signal, error) {
	if limit <= 0 || limit > 100 {
		limit = 50 // sensible default
	}

	query := `
		SELECT id, service_slug, raw_signal, analysis, status, reply_status,
		       user_notes, processed_at, last_updated
		FROM signals
		WHERE 1=1
	`
	args := []interface{}{}

	// Build WHERE clause based on filter
	if filter.ServiceSlug != "" {
		query += " AND service_slug = ?"
		args = append(args, filter.ServiceSlug)
	}
	if filter.Status != nil {
		query += " AND status = ?"
		args = append(args, int(*filter.Status))
	}
	if !filter.Since.IsZero() {
		query += " AND processed_at >= ?"
		args = append(args, filter.Since.Unix())
	}

	query += " ORDER BY processed_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []*Signal
	for rows.Next() {
		var sig Signal
		var rawSignalJSON, analysisJSON string
		var processedAt, lastUpdated int64

		err := rows.Scan(
			&sig.ID, &sig.ServiceSlug, &rawSignalJSON, &analysisJSON,
			&sig.Status, &sig.ReplyStatus, &sig.UserNotes, &processedAt, &lastUpdated,
		)
		if err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(rawSignalJSON), &sig.OriginalSignal); err != nil {
			continue
		}
		if err := json.Unmarshal([]byte(analysisJSON), &sig.Analysis); err != nil {
			continue
		}

		sig.ProcessedAt = time.Unix(processedAt, 0).UTC()
		sig.LastUpdated = time.Unix(lastUpdated, 0).UTC()

		// Apply additional filters that require analyzing the JSON
		if filter.Category != "" && sig.Analysis.Category != filter.Category {
			continue
		}
		if filter.Priority != "" && sig.Analysis.Priority != filter.Priority {
			continue
		}
		if filter.NeedsReply != nil && sig.Analysis.NeedsReply != *filter.NeedsReply {
			continue
		}
		if filter.IsRelevant != nil && sig.Analysis.IsRelevant != *filter.IsRelevant {
			continue
		}

		signals = append(signals, &sig)
	}

	return signals, rows.Err()
}

// UpdateStatus changes the status of a signal.
func (s *Store) UpdateStatus(id int, status Status, notes string) error {
	now := time.Now().UTC().Unix()
	_, err := s.db.Exec(`
		UPDATE signals
		SET status = ?, user_notes = ?, last_updated = ?
		WHERE id = ?
	`, int(status), notes, now, id)
	return err
}

// GetSummary returns counts for dashboard display.
func (s *Store) GetSummary() (*SignalSummary, error) {
	summary := &SignalSummary{}

	// Count pending signals
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM signals WHERE status = ?
	`, int(StatusPending)).Scan(&summary.TotalPending)
	if err != nil {
		return nil, err
	}

	// Count high priority signals (need to parse JSON)
	rows, err := s.db.Query(`
		SELECT analysis FROM signals WHERE status = ?
	`, int(StatusPending))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var analysisJSON string
		if err := rows.Scan(&analysisJSON); err != nil {
			continue
		}

		var analysis ai.AnalysedSignal
		if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
			continue
		}

		if analysis.Priority == "high" {
			summary.HighPriority++
		}
		if analysis.NeedsReply {
			summary.NeedingReplies++
		}
		if analysis.Category == "relevant" {
			summary.RelevantToday++
		}
		if analysis.Category == "newsletter" {
			summary.NewslettersToday++
		}
	}

	return summary, rows.Err()
}

// CreateDraftReply stores a generated reply draft.
func (s *Store) CreateDraftReply(signalID int, originalText, tone string, recipients []string, subject string) (*DraftReply, error) {
	recipientsJSON, err := json.Marshal(recipients)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		INSERT INTO draft_replies (signal_id, original_text, tone, recipients, subject, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, signalID, originalText, tone, string(recipientsJSON), subject, now, now)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetDraftReply(int(id))
}

// GetDraftReply retrieves a draft reply by ID.
func (s *Store) GetDraftReply(id int) (*DraftReply, error) {
	var draft DraftReply
	var recipientsJSON string
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, signal_id, original_text, edited_text, tone, recipients,
		       subject, status, created_at, updated_at
		FROM draft_replies WHERE id = ?
	`, id).Scan(
		&draft.ID, &draft.SignalID, &draft.OriginalText, &draft.EditedText,
		&draft.Tone, &recipientsJSON, &draft.Subject, &draft.Status, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(recipientsJSON), &draft.Recipients); err != nil {
		return nil, err
	}

	draft.CreatedAt = time.Unix(createdAt, 0).UTC()
	draft.UpdatedAt = time.Unix(updatedAt, 0).UTC()

	return &draft, nil
}

// UpdateDraftReply modifies an existing draft reply.
func (s *Store) UpdateDraftReply(id int, editedText string, status ReplyStatus) error {
	now := time.Now().UTC().Unix()
	_, err := s.db.Exec(`
		UPDATE draft_replies
		SET edited_text = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, editedText, int(status), now, id)
	return err
}

// GetPendingDrafts returns all draft replies waiting for user action.
func (s *Store) GetPendingDrafts() ([]*DraftReply, error) {
	rows, err := s.db.Query(`
		SELECT id, signal_id, original_text, edited_text, tone, recipients,
		       subject, status, created_at, updated_at
		FROM draft_replies
		WHERE status IN (?, ?)
		ORDER BY created_at DESC
	`, int(ReplyDrafted), int(ReplyEdited))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var drafts []*DraftReply
	for rows.Next() {
		var draft DraftReply
		var recipientsJSON string
		var createdAt, updatedAt int64

		err := rows.Scan(
			&draft.ID, &draft.SignalID, &draft.OriginalText, &draft.EditedText,
			&draft.Tone, &recipientsJSON, &draft.Subject, &draft.Status, &createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		if err := json.Unmarshal([]byte(recipientsJSON), &draft.Recipients); err != nil {
			continue
		}

		draft.CreatedAt = time.Unix(createdAt, 0).UTC()
		draft.UpdatedAt = time.Unix(updatedAt, 0).UTC()

		drafts = append(drafts, &draft)
	}

	return drafts, rows.Err()
}