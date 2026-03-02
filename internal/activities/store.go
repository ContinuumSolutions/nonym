package activities

import (
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("event not found")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type     INTEGER NOT NULL,
			decision       INTEGER NOT NULL DEFAULT 0,
			importance     INTEGER NOT NULL DEFAULT 0,
			narrative      TEXT    NOT NULL DEFAULT '',
			gain_type      INTEGER NOT NULL DEFAULT 0,
			gain_kind      INTEGER NOT NULL DEFAULT 0,
			gain_value     REAL    NOT NULL DEFAULT 0,
			gain_symbol    TEXT    NOT NULL DEFAULT '',
			gain_details   TEXT    NOT NULL DEFAULT '',
			source_service TEXT    NOT NULL DEFAULT '',
			read           INTEGER NOT NULL DEFAULT 0,
			created_at     INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at     INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	if err != nil {
		return err
	}
	// Add gain_kind to existing databases that predate this column.
	_, _ = s.db.Exec(`ALTER TABLE events ADD COLUMN gain_kind INTEGER NOT NULL DEFAULT 0`)
	// Add source_service to existing databases that predate this column.
	_, _ = s.db.Exec(`ALTER TABLE events ADD COLUMN source_service TEXT NOT NULL DEFAULT ''`)
	return nil
}

const selectFields = `
	SELECT id, event_type, decision, importance, narrative,
	       gain_type, gain_kind, gain_value, gain_symbol, gain_details,
	       source_service, read, created_at, updated_at
	FROM events
`

func scanRow(row *sql.Row) (*Event, error) {
	var e Event
	var createdAt, updatedAt int64
	err := row.Scan(
		&e.ID, &e.EventType, &e.Decision, &e.Importance, &e.Narrative,
		&e.Gain.Type, &e.Gain.Kind, &e.Gain.Value, &e.Gain.Symbol, &e.Gain.Details,
		&e.SourceService, &e.Read, &createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.CreatedAt = time.Unix(createdAt, 0).UTC()
	e.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &e, nil
}

func (s *Store) List() ([]Event, error) {
	rows, err := s.db.Query(selectFields + `ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var createdAt, updatedAt int64
		err := rows.Scan(
			&e.ID, &e.EventType, &e.Decision, &e.Importance, &e.Narrative,
			&e.Gain.Type, &e.Gain.Kind, &e.Gain.Value, &e.Gain.Symbol, &e.Gain.Details,
			&e.SourceService, &e.Read, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		e.CreatedAt = time.Unix(createdAt, 0).UTC()
		e.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) Get(id int) (*Event, error) {
	row := s.db.QueryRow(selectFields+`WHERE id = ?`, id)
	return scanRow(row)
}

// Create inserts a new event written by the brain pipeline.
// The read flag is always initialised to false.
func (s *Store) Create(e Event) (*Event, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		INSERT INTO events
			(event_type, decision, importance, narrative,
			 gain_type, gain_kind, gain_value, gain_symbol, gain_details,
			 source_service, read, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, e.EventType, e.Decision, e.Importance, e.Narrative,
		e.Gain.Type, e.Gain.Kind, e.Gain.Value, e.Gain.Symbol, e.Gain.Details,
		e.SourceService, now, now,
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

// CountHandledToday returns the number of events created since midnight UTC
// that have a terminal decision (Accepted, Declined, Negotiated, or Automated).
// Pending (0) and Cancelled (5) are excluded as they represent no kernel action.
func (s *Store) CountHandledToday() (int, error) {
	midnight := time.Now().UTC().Truncate(24 * time.Hour).Unix()
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM events
		WHERE created_at >= ? AND decision NOT IN (0, 5)
	`, midnight).Scan(&count)
	return count, err
}

func (s *Store) ToggleRead(id int) (*Event, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		UPDATE events SET read = NOT read, updated_at = ? WHERE id = ?
	`, now, id)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.Get(id)
}
