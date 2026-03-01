package integrations

import (
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("service not found")

type Store struct {
	db  *sql.DB
	key []byte
}

func NewStore(db *sql.DB, key []byte) *Store {
	return &Store{db: db, key: key}
}

func (s *Store) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS services (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			slug                TEXT    NOT NULL DEFAULT '',
			name                TEXT    NOT NULL,
			category            TEXT    NOT NULL,
			icon                TEXT    NOT NULL DEFAULT '',
			color               TEXT    NOT NULL DEFAULT '',
			description         TEXT    NOT NULL DEFAULT '',
			auth_method         INTEGER NOT NULL DEFAULT 0,
			status              INTEGER NOT NULL DEFAULT 0,
			custom              INTEGER NOT NULL DEFAULT 0,
			api_key             TEXT    NOT NULL DEFAULT '',
			api_endpoint        TEXT    NOT NULL DEFAULT '',
			oauth_access_token  TEXT    NOT NULL DEFAULT '',
			oauth_refresh_token TEXT    NOT NULL DEFAULT '',
			created_at          INTEGER NOT NULL DEFAULT (unixepoch()),
			updated_at          INTEGER NOT NULL DEFAULT (unixepoch())
		);
	`)
	if err != nil {
		return err
	}
	// Add color column to databases created before this field was introduced.
	// SQLite ignores the error when the column already exists via the IF NOT EXISTS workaround,
	// but since ALTER TABLE has no IF NOT EXISTS, we swallow the "duplicate column" error.
	s.db.Exec(`ALTER TABLE services ADD COLUMN color TEXT NOT NULL DEFAULT ''`) //nolint:errcheck
	return nil
}

// Seed inserts built-in services that don't yet exist in the DB, and always
// syncs the color column so upgrades from pre-color schema are handled.
func (s *Store) Seed() error {
	for _, def := range registry {
		_, err := s.db.Exec(`
			INSERT INTO services (slug, name, category, icon, color, description, auth_method, status, custom)
			SELECT ?, ?, ?, ?, ?, ?, ?, 0, 0
			WHERE NOT EXISTS (SELECT 1 FROM services WHERE slug = ? AND custom = 0)
		`, def.Slug, def.Name, def.Category, def.Icon, def.Color, def.Description, def.AuthMethod, def.Slug)
		if err != nil {
			return err
		}
		// Always update color so existing rows gain the field on upgrade.
		_, err = s.db.Exec(
			`UPDATE services SET color = ? WHERE slug = ? AND custom = 0`,
			def.Color, def.Slug,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// maskKey returns the last 4 characters of a key preceded by bullets.
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}

const selectFields = `
	SELECT id, slug, name, category, icon, color, description, auth_method, status, custom,
	       api_key, api_endpoint,
	       CASE WHEN oauth_access_token != '' THEN 1 ELSE 0 END,
	       created_at, updated_at
	FROM services
`

// scanRow scans a *sql.Row, decrypts credentials, and masks the API key.
func (s *Store) scanRow(row *sql.Row) (*Service, error) {
	var svc Service
	var apiKeyEnc string
	var custom, oauthConnected int
	var createdAt, updatedAt int64

	err := row.Scan(
		&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Icon, &svc.Color, &svc.Description,
		&svc.AuthMethod, &svc.Status, &custom,
		&apiKeyEnc, &svc.APIEndpoint, &oauthConnected,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	apiKey, err := decrypt(s.key, apiKeyEnc)
	if err != nil {
		return nil, err
	}
	svc.Custom = custom != 0
	svc.OAuthConnected = oauthConnected != 0
	svc.APIKey = maskKey(apiKey)
	svc.CreatedAt = time.Unix(createdAt, 0).UTC()
	svc.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &svc, nil
}

// scanRows scans a *sql.Rows row, decrypts credentials, and masks the API key.
func (s *Store) scanRows(rows *sql.Rows) (*Service, error) {
	var svc Service
	var apiKeyEnc string
	var custom, oauthConnected int
	var createdAt, updatedAt int64

	err := rows.Scan(
		&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Icon, &svc.Color, &svc.Description,
		&svc.AuthMethod, &svc.Status, &custom,
		&apiKeyEnc, &svc.APIEndpoint, &oauthConnected,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	apiKey, err := decrypt(s.key, apiKeyEnc)
	if err != nil {
		return nil, err
	}
	svc.Custom = custom != 0
	svc.OAuthConnected = oauthConnected != 0
	svc.APIKey = maskKey(apiKey)
	svc.CreatedAt = time.Unix(createdAt, 0).UTC()
	svc.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &svc, nil
}

func (s *Store) List() ([]Service, error) {
	rows, err := s.db.Query(selectFields + `ORDER BY category, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		svc, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, *svc)
	}
	return services, rows.Err()
}

func (s *Store) Get(id int) (*Service, error) {
	row := s.db.QueryRow(selectFields+`WHERE id = ?`, id)
	return s.scanRow(row)
}

// CreateCustom registers a custom service with credentials and sets it to Installed immediately.
func (s *Store) CreateCustom(svc *Service) (*Service, error) {
	encKey, err := encrypt(s.key, svc.APIKey)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		INSERT INTO services
			(slug, name, category, icon, color, description, auth_method, status, custom, api_key, api_endpoint, created_at, updated_at)
		VALUES ('', ?, ?, '', '', ?, ?, ?, 1, ?, ?, ?, ?)
	`, svc.Name, svc.Category, svc.Description, APIKeyAuth, Installed,
		encKey, svc.APIEndpoint, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.Get(int(id))
}

// ConnectInput holds credentials supplied during the connect flow.
type ConnectInput struct {
	APIKey            string `json:"api_key"`
	OAuthAccessToken  string `json:"oauth_access_token"`
	OAuthRefreshToken string `json:"oauth_refresh_token"`
}

// StartConnect marks the service as InProgress — the user has begun connecting.
func (s *Store) StartConnect(id int) (*Service, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(
		`UPDATE services SET status = ?, updated_at = ? WHERE id = ?`,
		InProgress, now, id,
	)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

// CompleteConnect encrypts and saves credentials, then marks the service as Installed.
func (s *Store) CompleteConnect(id int, input ConnectInput) (*Service, error) {
	encKey, err := encrypt(s.key, input.APIKey)
	if err != nil {
		return nil, err
	}
	encAccess, err := encrypt(s.key, input.OAuthAccessToken)
	if err != nil {
		return nil, err
	}
	encRefresh, err := encrypt(s.key, input.OAuthRefreshToken)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Unix()
	// The unencrypted input values drive the CASE WHEN check (update only if provided).
	// The encrypted values are what actually gets stored.
	res, err := s.db.Exec(`
		UPDATE services SET
			status              = ?,
			api_key             = CASE WHEN ? != '' THEN ? ELSE api_key END,
			oauth_access_token  = CASE WHEN ? != '' THEN ? ELSE oauth_access_token END,
			oauth_refresh_token = CASE WHEN ? != '' THEN ? ELSE oauth_refresh_token END,
			updated_at          = ?
		WHERE id = ?
	`, Installed,
		input.APIKey, encKey,
		input.OAuthAccessToken, encAccess,
		input.OAuthRefreshToken, encRefresh,
		now, id,
	)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

// Uninstall clears all credentials and resets the service to Pending.
func (s *Store) Uninstall(id int) (*Service, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		UPDATE services SET
			status              = ?,
			api_key             = '',
			oauth_access_token  = '',
			oauth_refresh_token = '',
			updated_at          = ?
		WHERE id = ?
	`, Pending, now, id)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

func (s *Store) requireAffected(res sql.Result, id int) (*Service, error) {
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.Get(id)
}
