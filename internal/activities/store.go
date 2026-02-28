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
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type   INTEGER NOT NULL,
			decision     INTEGER NOT NULL DEFAULT 0,
			importance   INTEGER NOT NULL DEFAULT 0,
			narrative    TEXT    NOT NULL DEFAULT '',
			gain_type    INTEGER NOT NULL DEFAULT 0,
			gain_value   REAL    NOT NULL DEFAULT 0,
			gain_symbol  TEXT    NOT NULL DEFAULT '',
			gain_details TEXT    NOT NULL DEFAULT '',
			read         INTEGER NOT NULL DEFAULT 0,
			created_at   INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at   INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

const selectFields = `
	SELECT id, event_type, decision, importance, narrative,
	       gain_type, gain_value, gain_symbol, gain_details,
	       read, created_at, updated_at
	FROM events
`

func scanRow(row *sql.Row) (*Event, error) {
	var e Event
	var createdAt, updatedAt int64
	err := row.Scan(
		&e.ID, &e.EventType, &e.Decision, &e.Importance, &e.Narrative,
		&e.Gain.Type, &e.Gain.Value, &e.Gain.Symbol, &e.Gain.Details,
		&e.Read, &createdAt, &updatedAt,
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
			&e.Gain.Type, &e.Gain.Value, &e.Gain.Symbol, &e.Gain.Details,
			&e.Read, &createdAt, &updatedAt,
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
