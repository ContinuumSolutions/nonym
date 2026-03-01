package chat

import (
	"database/sql"
	"time"
)

// HistoryStore persists conversation turns to SQLite.
type HistoryStore struct {
	db *sql.DB
}

func NewHistoryStore(db *sql.DB) *HistoryStore { return &HistoryStore{db: db} }

func (s *HistoryStore) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_history (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			role       TEXT    NOT NULL,
			content    TEXT    NOT NULL,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

// Append saves a single message turn. Role should be "user" or "kernel".
func (s *HistoryStore) Append(role, content string) error {
	_, err := s.db.Exec(`INSERT INTO chat_history (role, content) VALUES (?, ?)`, role, content)
	return err
}

// List returns the last limit turns, oldest first (chronological order for display).
// limit is capped between 1 and 200; zero means default of 50.
func (s *HistoryStore) List(limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT role, content, created_at
		FROM chat_history
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		var ts int64
		if err := rows.Scan(&m.Role, &m.Content, &ts); err != nil {
			return nil, err
		}
		m.Timestamp = time.Unix(ts, 0).UTC()
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Reverse DESC result to chronological order (oldest first).
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
