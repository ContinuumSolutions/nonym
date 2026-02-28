package harvest

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Store persists harvest results to SQLite so the last scan
// is available without re-running.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS harvest_results (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			result_json TEXT    NOT NULL,
			created_at  INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

// Save persists a HarvestResult as a JSON blob.
func (s *Store) Save(result HarvestResult) error {
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("harvest: marshal result: %w", err)
	}
	_, err = s.db.Exec(`INSERT INTO harvest_results (result_json) VALUES (?)`, string(b))
	return err
}

// Latest returns the most recent stored result, or nil if no scan has run yet.
func (s *Store) Latest() (*HarvestResult, error) {
	row := s.db.QueryRow(`
		SELECT result_json FROM harvest_results
		ORDER BY created_at DESC LIMIT 1
	`)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var result HarvestResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("harvest: unmarshal result: %w", err)
	}
	return &result, nil
}
