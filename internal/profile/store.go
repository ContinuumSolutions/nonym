package profile

import (
	"database/sql"
	"time"
)

// Profile is the combined view returned by the API.
type Profile struct {
	KernelName  string             `json:"kernel_name"`
	APIEndpoint string             `json:"api_endpoint"`
	Timezone    string             `json:"timezone"`
	Preferences DecisionPreference `json:"preferences"`
	Progress    EKProgress         `json:"progress"`
	HasPIN      bool               `json:"has_pin"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS profile (
			id                  INTEGER PRIMARY KEY CHECK (id = 1),
			kernel_name         TEXT    NOT NULL DEFAULT 'EK-1',
			api_endpoint        TEXT    NOT NULL DEFAULT '',
			timezone            TEXT    NOT NULL DEFAULT 'UTC',
			time_sovereignty    INTEGER NOT NULL DEFAULT 5,
			financial_growth    INTEGER NOT NULL DEFAULT 5,
			health_recovery     INTEGER NOT NULL DEFAULT 5,
			reputation_building INTEGER NOT NULL DEFAULT 5,
			privacy_protection  INTEGER NOT NULL DEFAULT 5,
			autonomy            INTEGER NOT NULL DEFAULT 5,
			created_at          INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at          INTEGER NOT NULL DEFAULT (unixepoch())
		);
		INSERT INTO profile (id) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM profile WHERE id = 1);
	`)
	if err != nil {
		return err
	}
	// Idempotent: add columns for schemas that predate them.
	_, _ = s.db.Exec(`ALTER TABLE profile ADD COLUMN pin_hash TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`ALTER TABLE profile ADD COLUMN base_hourly_rate REAL NOT NULL DEFAULT 85.0`)
	return nil
}

func (s *Store) Get() (*Profile, error) {
	row := s.db.QueryRow(`
		SELECT kernel_name, api_endpoint, timezone,
		       time_sovereignty, financial_growth, health_recovery,
		       reputation_building, privacy_protection, autonomy,
		       pin_hash, created_at, updated_at, base_hourly_rate
		FROM profile WHERE id = 1
	`)
	return scan(row)
}

// GetPINHash returns the stored SHA-256 hash; empty string means no PIN is set.
func (s *Store) GetPINHash() (string, error) {
	var hash string
	err := s.db.QueryRow(`SELECT pin_hash FROM profile WHERE id = 1`).Scan(&hash)
	return hash, err
}

// SetPIN stores a SHA-256 hex hash of the user's PIN.
func (s *Store) SetPIN(hash string) (time.Time, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE profile SET pin_hash = ?, updated_at = ? WHERE id = 1`,
		hash, now.Unix(),
	)
	return now, err
}

// RemovePIN clears the stored PIN hash.
func (s *Store) RemovePIN() (time.Time, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE profile SET pin_hash = '', updated_at = ? WHERE id = 1`,
		now.Unix(),
	)
	return now, err
}

func (s *Store) UpdatePreferences(p DecisionPreference) (*Profile, error) {
	now := time.Now().UTC().Unix()
	_, err := s.db.Exec(`
		UPDATE profile SET
			time_sovereignty    = ?,
			financial_growth    = ?,
			health_recovery     = ?,
			reputation_building = ?,
			privacy_protection  = ?,
			autonomy            = ?,
			base_hourly_rate    = ?,
			updated_at          = ?
		WHERE id = 1
	`, p.TimeSovereignty, p.FinacialGrowth, p.HealthRecovery,
		p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, p.BaseHourlyRate, now,
	)
	if err != nil {
		return nil, err
	}
	return s.Get()
}

func (s *Store) UpdateConnection(c ConnectionSetting) (*Profile, error) {
	now := time.Now().UTC().Unix()
	_, err := s.db.Exec(`
		UPDATE profile SET
			kernel_name  = ?,
			api_endpoint = ?,
			timezone     = ?,
			updated_at   = ?
		WHERE id = 1
	`, c.KernelName, c.APIEndpoint, c.Timezone, now,
	)
	if err != nil {
		return nil, err
	}
	return s.Get()
}

func scan(row *sql.Row) (*Profile, error) {
	var p Profile
	var pinHash string
	var createdAt, updatedAt int64
	err := row.Scan(
		&p.KernelName, &p.APIEndpoint, &p.Timezone,
		&p.Preferences.TimeSovereignty, &p.Preferences.FinacialGrowth, &p.Preferences.HealthRecovery,
		&p.Preferences.ReputationBuilding, &p.Preferences.PrivacyProtection, &p.Preferences.Autonomy,
		&pinHash, &createdAt, &updatedAt, &p.Preferences.BaseHourlyRate,
	)
	if err != nil {
		return nil, err
	}
	p.HasPIN = pinHash != ""
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	p.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	// Progress reflects current stage. Updated dynamically in later steps.
	p.Progress = EKProgress{Shadow: true, Hand: false, Voice: false}
	return &p, nil
}
