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
	return err
}

func (s *Store) Get() (*Profile, error) {
	row := s.db.QueryRow(`
		SELECT kernel_name, api_endpoint, timezone,
		       time_sovereignty, financial_growth, health_recovery,
		       reputation_building, privacy_protection, autonomy,
		       created_at, updated_at
		FROM profile WHERE id = 1
	`)
	return scan(row)
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
			updated_at          = ?
		WHERE id = 1
	`, p.TimeSovereignty, p.FinacialGrowth, p.HealthRecovery,
		p.ReputationBuilding, p.PrivacyProtection, p.Autonomy, now,
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
	var createdAt, updatedAt int64
	err := row.Scan(
		&p.KernelName, &p.APIEndpoint, &p.Timezone,
		&p.Preferences.TimeSovereignty, &p.Preferences.FinacialGrowth, &p.Preferences.HealthRecovery,
		&p.Preferences.ReputationBuilding, &p.Preferences.PrivacyProtection, &p.Preferences.Autonomy,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(createdAt, 0).UTC()
	p.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	// Progress reflects current stage. Updated dynamically in later steps.
	p.Progress = EKProgress{Shadow: true, Hand: false, Voice: false}
	return &p, nil
}
