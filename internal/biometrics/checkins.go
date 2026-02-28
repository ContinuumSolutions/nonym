package biometrics

import (
	"database/sql"
	"errors"
	"time"
)

type CheckIn struct {
	Feeling      int       `json:"feeling"`
	StressLevel  int       `json:"stress_level"`
	Sleep        int       `json:"sleep"`
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
			feeling       INTEGER NOT NULL,
			stress_level  INTEGER NOT NULL,
			sleep         INTEGER NOT NULL,
			energy        INTEGER NOT NULL,
			extra_context TEXT    NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at    INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	return err
}

func (s *Store) Get() (*CheckIn, error) {
	row := s.db.QueryRow(`
		SELECT feeling, stress_level, sleep, energy, extra_context, created_at, updated_at
		FROM check_ins WHERE id = 1
	`)
	var c CheckIn
	var createdAt, updatedAt int64
	err := row.Scan(&c.Feeling, &c.StressLevel, &c.Sleep, &c.Energy, &c.ExtraContext, &createdAt, &updatedAt)
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
	_, err := s.db.Exec(`
		INSERT INTO check_ins (id, feeling, stress_level, sleep, energy, extra_context, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			feeling       = excluded.feeling,
			stress_level  = excluded.stress_level,
			sleep         = excluded.sleep,
			energy        = excluded.energy,
			extra_context = excluded.extra_context,
			updated_at    = excluded.updated_at
	`, in.Feeling, in.StressLevel, in.Sleep, in.Energy, in.ExtraContext, now, now)
	if err != nil {
		return nil, err
	}
	return s.Get()
}
