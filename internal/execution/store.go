package execution

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

var ErrNotFound = errors.New("queue entry not found")

// QueueEntry represents a pending action awaiting human approval.
type QueueEntry struct {
	ID             int               `json:"id"`
	EventID        int               `json:"event_id"`
	ActionType     string            `json:"action_type"`
	ServiceSlug    string            `json:"service_slug"`
	ResourceID     string            `json:"resource_id"`
	ResourceMeta   map[string]string `json:"resource_meta"`
	Reason         string            `json:"reason"`
	EstimatedCost  float64           `json:"estimated_cost"`
	ReputationRisk float64           `json:"reputation_risk"`
	Status         string            `json:"status"` // pending|approved|rejected|executed|failed
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Store persists approval queue entries to SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS approval_queue (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id        INTEGER NOT NULL,
			action_type     TEXT    NOT NULL,
			service_slug    TEXT    NOT NULL,
			resource_id     TEXT    NOT NULL DEFAULT '',
			resource_meta   TEXT    NOT NULL DEFAULT '{}',
			reason          TEXT    NOT NULL DEFAULT '',
			estimated_cost  REAL    NOT NULL DEFAULT 0,
			reputation_risk REAL    NOT NULL DEFAULT 0,
			status          TEXT    NOT NULL DEFAULT 'pending',
			created_at      INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at      INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

// Enqueue inserts a new queue entry and returns it with its assigned ID.
func (s *Store) Enqueue(entry QueueEntry) (*QueueEntry, error) {
	metaJSON, err := json.Marshal(entry.ResourceMeta)
	if err != nil {
		return nil, err
	}

	res, err := s.db.Exec(`
		INSERT INTO approval_queue
			(event_id, action_type, service_slug, resource_id, resource_meta,
			 reason, estimated_cost, reputation_risk, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending')
	`,
		entry.EventID, entry.ActionType, entry.ServiceSlug, entry.ResourceID,
		string(metaJSON), entry.Reason, entry.EstimatedCost, entry.ReputationRisk,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.Get(int(id))
}

// ListPending returns all entries with status=pending, newest first.
func (s *Store) ListPending() ([]QueueEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, event_id, action_type, service_slug, resource_id, resource_meta,
		       reason, estimated_cost, reputation_risk, status, created_at, updated_at
		FROM approval_queue
		WHERE status = 'pending'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// Get returns a single queue entry by ID.
func (s *Store) Get(id int) (*QueueEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, event_id, action_type, service_slug, resource_id, resource_meta,
		       reason, estimated_cost, reputation_risk, status, created_at, updated_at
		FROM approval_queue WHERE id = ?
	`, id)
	return scanRow(row)
}

// SetStatus updates the status and updated_at of a queue entry.
func (s *Store) SetStatus(id int, status string) error {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(
		`UPDATE approval_queue SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateResourceMeta stores the updated resource_meta (e.g. after IntaSend initiate stores tracking_id).
func (s *Store) UpdateResourceMeta(id int, meta map[string]string) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	_, err = s.db.Exec(
		`UPDATE approval_queue SET resource_meta = ?, updated_at = ? WHERE id = ?`,
		string(metaJSON), now, id,
	)
	return err
}

func scanRow(row *sql.Row) (*QueueEntry, error) {
	var e QueueEntry
	var createdAt, updatedAt int64
	var metaJSON string

	if err := row.Scan(
		&e.ID, &e.EventID, &e.ActionType, &e.ServiceSlug, &e.ResourceID, &metaJSON,
		&e.Reason, &e.EstimatedCost, &e.ReputationRisk, &e.Status,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	e.CreatedAt = time.Unix(createdAt, 0).UTC()
	e.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	if metaJSON != "" && metaJSON != "{}" {
		_ = json.Unmarshal([]byte(metaJSON), &e.ResourceMeta)
	}
	if e.ResourceMeta == nil {
		e.ResourceMeta = map[string]string{}
	}
	return &e, nil
}

func scanRows(rows *sql.Rows) ([]QueueEntry, error) {
	var out []QueueEntry
	for rows.Next() {
		var e QueueEntry
		var createdAt, updatedAt int64
		var metaJSON string

		if err := rows.Scan(
			&e.ID, &e.EventID, &e.ActionType, &e.ServiceSlug, &e.ResourceID, &metaJSON,
			&e.Reason, &e.EstimatedCost, &e.ReputationRisk, &e.Status,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		e.CreatedAt = time.Unix(createdAt, 0).UTC()
		e.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		if metaJSON != "" && metaJSON != "{}" {
			_ = json.Unmarshal([]byte(metaJSON), &e.ResourceMeta)
		}
		if e.ResourceMeta == nil {
			e.ResourceMeta = map[string]string{}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
