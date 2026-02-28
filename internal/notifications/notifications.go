// Package notifications manages in-app alerts surfaced to the user.
// In Stage 1 (Shadow) these are read-only alerts; Stage 2 will add action buttons.
package notifications

import (
	"database/sql"
	"errors"
	"time"
)

// Type classifies why a notification was created.
type Type string

const (
	TypeH2HI        Type = "H2HI"        // identity entropy spike — manual sync required
	TypeOpportunity Type = "OPPORTUNITY"  // high-value social debt detected by harvest engine
	TypeHarvest     Type = "HARVEST"      // ghost-agreement opportunity detected
	TypeSoulDrift   Type = "SOUL_DRIFT"   // periodic irrational-human injection event
)

// Notification is a single in-app alert.
type Notification struct {
	ID        int       `json:"id"`
	Type      Type      `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

var ErrNotFound = errors.New("notification not found")

// Store persists notifications to SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			type       TEXT    NOT NULL,
			title      TEXT    NOT NULL,
			body       TEXT    NOT NULL,
			read       INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

// Create inserts a new notification and returns it with its assigned ID.
func (s *Store) Create(n Notification) (*Notification, error) {
	res, err := s.db.Exec(
		`INSERT INTO notifications (type, title, body) VALUES (?, ?, ?)`,
		string(n.Type), n.Title, n.Body,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.get(int(id))
}

// ListUnread returns all unread notifications, newest first.
func (s *Store) ListUnread() ([]Notification, error) {
	rows, err := s.db.Query(`
		SELECT id, type, title, body, read, created_at
		FROM notifications
		WHERE read = 0
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// List returns all notifications (read and unread), newest first.
func (s *Store) List() ([]Notification, error) {
	rows, err := s.db.Query(`
		SELECT id, type, title, body, read, created_at
		FROM notifications
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// MarkRead marks a single notification as read.
func (s *Store) MarkRead(id int) error {
	res, err := s.db.Exec(`UPDATE notifications SET read = 1 WHERE id = ?`, id)
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

// MarkAllRead marks every unread notification as read.
func (s *Store) MarkAllRead() error {
	_, err := s.db.Exec(`UPDATE notifications SET read = 1 WHERE read = 0`)
	return err
}

func (s *Store) get(id int) (*Notification, error) {
	row := s.db.QueryRow(
		`SELECT id, type, title, body, read, created_at FROM notifications WHERE id = ?`, id,
	)
	var n Notification
	var ts int64
	var readInt int
	if err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &readInt, &ts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	n.Read = readInt == 1
	n.CreatedAt = time.Unix(ts, 0).UTC()
	return &n, nil
}

func scanRows(rows *sql.Rows) ([]Notification, error) {
	var out []Notification
	for rows.Next() {
		var n Notification
		var ts int64
		var readInt int
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &readInt, &ts); err != nil {
			return nil, err
		}
		n.Read = readInt == 1
		n.CreatedAt = time.Unix(ts, 0).UTC()
		out = append(out, n)
	}
	return out, rows.Err()
}
