package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidPIN         = errors.New("invalid_pin")
	ErrCooldown           = errors.New("cooldown")
	ErrSessionInvalidated = errors.New("session_invalidated")
)

type session struct {
	Token           string
	ResumeToken     string
	ResumeExpiresAt time.Time
	Locked          bool
	PINAttempts     int
	LockedUntil     time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id                INTEGER PRIMARY KEY CHECK (id = 1),
			token             TEXT    NOT NULL DEFAULT '',
			resume_token      TEXT    NOT NULL DEFAULT '',
			resume_expires_at INTEGER NOT NULL DEFAULT 0,
			locked            INTEGER NOT NULL DEFAULT 0,
			pin_attempts      INTEGER NOT NULL DEFAULT 0,
			locked_until      INTEGER NOT NULL DEFAULT 0,
			created_at        INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at        INTEGER NOT NULL DEFAULT (unixepoch())
		);
		INSERT INTO sessions (id) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM sessions WHERE id = 1);
	`)
	return err
}

func (s *Store) get() (*session, error) {
	row := s.db.QueryRow(`
		SELECT token, resume_token, resume_expires_at, locked, pin_attempts, locked_until
		FROM sessions WHERE id = 1
	`)
	var sess session
	var resumeExp, lockedUntil int64
	var locked int
	if err := row.Scan(&sess.Token, &sess.ResumeToken, &resumeExp, &locked,
		&sess.PINAttempts, &lockedUntil); err != nil {
		return nil, err
	}
	sess.Locked = locked == 1
	sess.ResumeExpiresAt = time.Unix(resumeExp, 0).UTC()
	sess.LockedUntil = time.Unix(lockedUntil, 0).UTC()
	return &sess, nil
}

// Lock invalidates the current token and returns a short-lived resume token (1h TTL).
func (s *Store) Lock() (resumeToken string, lockedAt time.Time, err error) {
	resumeToken, err = genToken()
	if err != nil {
		return
	}
	resumeExp := time.Now().UTC().Add(time.Hour).Unix()
	lockedAt = time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE sessions SET
			token             = '',
			resume_token      = ?,
			resume_expires_at = ?,
			locked            = 1,
			pin_attempts      = 0,
			locked_until      = 0,
			updated_at        = unixepoch()
		WHERE id = 1
	`, resumeToken, resumeExp)
	return
}

// Unlock verifies pinHash against storedHash and issues a fresh API token.
// Returns ErrInvalidPIN, ErrCooldown, or ErrSessionInvalidated on failure.
func (s *Store) Unlock(pinHash, storedHash string) (token string, unlockedAt time.Time, err error) {
	sess, err := s.get()
	if err != nil {
		return
	}

	// Hard lockout: 5+ failed attempts require a page refresh
	if sess.PINAttempts >= 5 {
		_ = s.forceInvalidate()
		err = ErrSessionInvalidated
		return
	}

	// Cooldown after 3 failed attempts
	if time.Now().UTC().Before(sess.LockedUntil) {
		err = ErrCooldown
		return
	}

	if pinHash != storedHash {
		newAttempts := sess.PINAttempts + 1
		var cooldown int64
		if newAttempts >= 3 {
			cooldown = time.Now().UTC().Add(30 * time.Second).Unix()
		}
		_, _ = s.db.Exec(`
			UPDATE sessions SET pin_attempts = ?, locked_until = ?, updated_at = unixepoch()
			WHERE id = 1
		`, newAttempts, cooldown)
		if newAttempts >= 5 {
			_ = s.forceInvalidate()
			err = ErrSessionInvalidated
			return
		}
		err = ErrInvalidPIN
		return
	}

	// Correct PIN — issue a fresh token
	token, err = genToken()
	if err != nil {
		return
	}
	unlockedAt = time.Now().UTC()
	_, err = s.db.Exec(`
		UPDATE sessions SET
			token             = ?,
			resume_token      = '',
			resume_expires_at = 0,
			locked            = 0,
			pin_attempts      = 0,
			locked_until      = 0,
			updated_at        = unixepoch()
		WHERE id = 1
	`, token)
	return
}

// forceInvalidate wipes the session when brute-force threshold is reached.
func (s *Store) forceInvalidate() error {
	_, err := s.db.Exec(`
		UPDATE sessions SET
			token             = '',
			resume_token      = '',
			resume_expires_at = 0,
			locked            = 1,
			pin_attempts      = 0,
			locked_until      = 0,
			updated_at        = unixepoch()
		WHERE id = 1
	`)
	return err
}

func genToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
