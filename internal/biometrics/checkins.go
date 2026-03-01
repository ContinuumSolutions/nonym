package biometrics

import (
	"database/sql"
	"errors"
	"time"
)

type CheckIn struct {
	Mood         int       `json:"mood"`
	StressLevel  int       `json:"stress_level"`
	Sleep        float64   `json:"sleep"`
	Energy       int       `json:"energy"`
	ExtraContext string    `json:"extra_context"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("no check-in recorded yet")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS check_ins (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			mood          INTEGER NOT NULL,
			stress_level  INTEGER NOT NULL,
			sleep         REAL    NOT NULL,
			energy        INTEGER NOT NULL,
			extra_context TEXT    NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at    INTEGER NOT NULL DEFAULT (unixepoch())
		);
		CREATE TABLE IF NOT EXISTS check_in_history (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			mood          INTEGER NOT NULL,
			stress_level  INTEGER NOT NULL,
			sleep         REAL    NOT NULL,
			energy        INTEGER NOT NULL,
			extra_context TEXT    NOT NULL DEFAULT '',
			recorded_at   INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

func (s *Store) Get() (*CheckIn, error) {
	row := s.db.QueryRow(`
		SELECT mood, stress_level, sleep, energy, extra_context, created_at, updated_at
		FROM check_ins WHERE id = 1
	`)
	var c CheckIn
	var createdAt, updatedAt int64
	err := row.Scan(&c.Mood, &c.StressLevel, &c.Sleep, &c.Energy, &c.ExtraContext, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = time.Unix(createdAt, 0).UTC()
	c.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &c, nil
}

func (s *Store) Upsert(in *CheckIn) (*CheckIn, error) {
	now := time.Now().UTC().Unix()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(`
		INSERT INTO check_ins (id, mood, stress_level, sleep, energy, extra_context, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			mood          = excluded.mood,
			stress_level  = excluded.stress_level,
			sleep         = excluded.sleep,
			energy        = excluded.energy,
			extra_context = excluded.extra_context,
			updated_at    = excluded.updated_at
	`, in.Mood, in.StressLevel, in.Sleep, in.Energy, in.ExtraContext, now, now)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		INSERT INTO check_in_history (mood, stress_level, sleep, energy, extra_context, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, in.Mood, in.StressLevel, in.Sleep, in.Energy, in.ExtraContext, now)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get()
}

// History returns the last limit check-ins, newest first.
// limit is capped at 90.
func (s *Store) History(limit int) ([]CheckIn, error) {
	if limit <= 0 {
		limit = 7
	}
	if limit > 90 {
		limit = 90
	}
	rows, err := s.db.Query(`
		SELECT mood, stress_level, sleep, energy, extra_context, recorded_at
		FROM check_in_history
		ORDER BY recorded_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CheckIn
	for rows.Next() {
		var c CheckIn
		var recordedAt int64
		if err := rows.Scan(&c.Mood, &c.StressLevel, &c.Sleep, &c.Energy, &c.ExtraContext, &recordedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(recordedAt, 0).UTC()
		c.UpdatedAt = c.CreatedAt
		result = append(result, c)
	}
	return result, rows.Err()
}
