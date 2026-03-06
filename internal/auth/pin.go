package auth

import (
	"database/sql"
	"errors"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPINAlreadySet = errors.New("pin already configured")
	ErrPINInvalid    = errors.New("pin invalid")
	ErrPINNotSet     = errors.New("pin not set")
)

// PINStore manages PIN storage with bcrypt hashing
type PINStore struct {
	db *sql.DB
}

// NewPINStore creates a new PIN store
func NewPINStore(db *sql.DB) *PINStore {
	return &PINStore{db: db}
}

// Migrate creates the PIN storage table
func (p *PINStore) Migrate() error {
	_, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS pin_auth (
			id         INTEGER PRIMARY KEY CHECK (id = 1),
			pin_hash   TEXT    NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at INTEGER NOT NULL DEFAULT (unixepoch())
		);
		INSERT OR IGNORE INTO pin_auth (id) VALUES (1);
	`)
	return err
}

// IsConfigured returns whether a PIN has been set
func (p *PINStore) IsConfigured() (bool, error) {
	var pinHash string
	err := p.db.QueryRow("SELECT pin_hash FROM pin_auth WHERE id = 1").Scan(&pinHash)
	if err != nil {
		return false, err
	}
	return pinHash != "", nil
}

// SetupPIN sets the initial PIN (only if none is configured)
func (p *PINStore) SetupPIN(pin string) error {
	if !isValidPIN(pin) {
		return ErrPINInvalid
	}

	// Check if PIN is already set
	configured, err := p.IsConfigured()
	if err != nil {
		return err
	}
	if configured {
		return ErrPINAlreadySet
	}

	// Hash the PIN with bcrypt cost 12
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), 12)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = p.db.Exec(`
		UPDATE pin_auth
		SET pin_hash = ?, updated_at = ?
		WHERE id = 1
	`, string(hash), now)

	return err
}

// VerifyPIN checks if the provided PIN matches the stored hash
func (p *PINStore) VerifyPIN(pin string) error {
	if !isValidPIN(pin) {
		return ErrPINInvalid
	}

	var storedHash string
	err := p.db.QueryRow("SELECT pin_hash FROM pin_auth WHERE id = 1").Scan(&storedHash)
	if err != nil {
		return err
	}

	if storedHash == "" {
		return ErrPINNotSet
	}

	// Compare with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(pin)); err != nil {
		return ErrPINInvalid
	}

	return nil
}

// ChangePIN updates the PIN after verifying the current one
func (p *PINStore) ChangePIN(currentPIN, newPIN string) error {
	// Verify current PIN first
	if err := p.VerifyPIN(currentPIN); err != nil {
		return err
	}

	if !isValidPIN(newPIN) {
		return ErrPINInvalid
	}

	// Hash the new PIN
	hash, err := bcrypt.GenerateFromPassword([]byte(newPIN), 12)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err = p.db.Exec(`
		UPDATE pin_auth
		SET pin_hash = ?, updated_at = ?
		WHERE id = 1
	`, string(hash), now)

	return err
}

// RemovePIN deletes the PIN after verification
func (p *PINStore) RemovePIN(currentPIN string) error {
	// Verify current PIN first
	if err := p.VerifyPIN(currentPIN); err != nil {
		return err
	}

	now := time.Now().Unix()
	_, err := p.db.Exec(`
		UPDATE pin_auth
		SET pin_hash = '', updated_at = ?
		WHERE id = 1
	`, now)

	return err
}

// ResetPIN clears the PIN without verification (admin function)
func (p *PINStore) ResetPIN() error {
	now := time.Now().Unix()
	_, err := p.db.Exec(`
		UPDATE pin_auth
		SET pin_hash = '', updated_at = ?
		WHERE id = 1
	`, now)
	return err
}

// isValidPIN validates that PIN is exactly 4 digits
func isValidPIN(pin string) bool {
	matched, _ := regexp.MatchString("^[0-9]{4}$", pin)
	return matched
}