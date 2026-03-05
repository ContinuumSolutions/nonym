package integrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	// Idempotent ALTER TABLE additions — "duplicate column" errors are swallowed intentionally.
	for _, col := range []string{
		`ALTER TABLE services ADD COLUMN color               TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE services ADD COLUMN oauth_client_id     TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE services ADD COLUMN oauth_client_secret TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE services ADD COLUMN oauth_token_expiry  INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE services ADD COLUMN oauth_state         TEXT    NOT NULL DEFAULT ''`,
		`ALTER TABLE services ADD COLUMN oauth_state_expiry  INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE services ADD COLUMN oauth_code_verifier    TEXT    NOT NULL DEFAULT ''`,
		// Stores the full token-endpoint URL override (e.g. Zoho regional: accounts.zoho.eu).
		// Empty means use the catalog default. Written at callback time, read on every refresh.
		`ALTER TABLE services ADD COLUMN oauth_token_url_override TEXT NOT NULL DEFAULT ''`,
	} {
		s.db.Exec(col) //nolint:errcheck
	}
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
		// Always sync color and description so existing rows gain updates on upgrade.
		_, err = s.db.Exec(
			`UPDATE services SET color = ?, description = ? WHERE slug = ? AND custom = 0`,
			def.Color, def.Description, def.Slug,
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
	       CASE WHEN oauth_access_token  != '' THEN 1 ELSE 0 END,
	       CASE WHEN oauth_client_id     != '' THEN 1 ELSE 0 END,
	       created_at, updated_at
	FROM services
`

// scanRow scans a *sql.Row, decrypts credentials, and masks the API key.
func (s *Store) scanRow(row *sql.Row) (*Service, error) {
	var svc Service
	var apiKeyEnc string
	var custom, oauthConnected, appConfigured int
	var createdAt, updatedAt int64

	err := row.Scan(
		&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Icon, &svc.Color, &svc.Description,
		&svc.AuthMethod, &svc.Status, &custom,
		&apiKeyEnc, &svc.APIEndpoint, &oauthConnected, &appConfigured,
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
	svc.AppConfigured = appConfigured != 0
	svc.APIKey = maskKey(apiKey)
	svc.CreatedAt = time.Unix(createdAt, 0).UTC()
	svc.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return &svc, nil
}

// scanRows scans a *sql.Rows row, decrypts credentials, and masks the API key.
func (s *Store) scanRows(rows *sql.Rows) (*Service, error) {
	var svc Service
	var apiKeyEnc string
	var custom, oauthConnected, appConfigured int
	var createdAt, updatedAt int64

	err := rows.Scan(
		&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Icon, &svc.Color, &svc.Description,
		&svc.AuthMethod, &svc.Status, &custom,
		&apiKeyEnc, &svc.APIEndpoint, &oauthConnected, &appConfigured,
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
	svc.AppConfigured = appConfigured != 0
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

// CreateCustom registers a custom service with credentials and sets it to Connected immediately.
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
	`, svc.Name, svc.Category, svc.Description, APIKeyAuth, Connected,
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

// StartConnect marks the service as Pending — the user has begun connecting.
func (s *Store) StartConnect(id int) (*Service, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(
		`UPDATE services SET status = ?, updated_at = ? WHERE id = ?`,
		Pending, now, id,
	)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

// CompleteConnect encrypts and saves credentials, then marks the service as Connected.
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
	`, Connected,
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

// Uninstall resets the service to Disconnected and clears all credentials except
// OAuth client_id/client_secret — those are kept so the user can re-authorise
// without re-entering their app credentials.
func (s *Store) Uninstall(id int) (*Service, error) {
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		UPDATE services SET
			status              = ?,
			api_key             = '',
			oauth_access_token  = '',
			oauth_refresh_token = '',
			oauth_token_expiry  = 0,
			oauth_state         = '',
			oauth_state_expiry  = 0,
			oauth_code_verifier = '',
			updated_at          = ?
		WHERE id = ?
	`, Disconnected, now, id)
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

// ── OAuth BYOA methods ────────────────────────────────────────────────────────

// SaveOAuthApp encrypts and stores the user's OAuth client_id/client_secret (step 9a).
// Clears any existing tokens so oauth_connected resets to false until re-authorised.
func (s *Store) SaveOAuthApp(id int, clientID, clientSecret string) (*Service, error) {
	encID, err := encrypt(s.key, clientID)
	if err != nil {
		return nil, err
	}
	encSecret, err := encrypt(s.key, clientSecret)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		UPDATE services SET
			oauth_client_id     = ?,
			oauth_client_secret = ?,
			oauth_access_token  = '',
			oauth_refresh_token = '',
			oauth_token_expiry  = 0,
			status              = ?,
			updated_at          = ?
		WHERE id = ?
	`, encID, encSecret, Disconnected, now, id)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

// SetOAuthState persists the CSRF state token and PKCE code_verifier for an in-progress flow.
// expiry is a Unix timestamp; the state is invalid after this time.
func (s *Store) SetOAuthState(id int, state, codeVerifier string, expiry int64) error {
	_, err := s.db.Exec(`
		UPDATE services SET oauth_state = ?, oauth_code_verifier = ?, oauth_state_expiry = ?
		WHERE id = ?
	`, state, codeVerifier, expiry, id)
	return err
}

// GetByState looks up a service by its CSRF state token, validating that it has not expired.
// Returns (serviceID, slug, codeVerifier) or ErrNotFound if not found/expired.
func (s *Store) GetByState(state string) (serviceID int, slug, codeVerifier string, err error) {
	now := time.Now().Unix()
	row := s.db.QueryRow(`
		SELECT id, slug, oauth_code_verifier
		FROM services
		WHERE oauth_state = ? AND oauth_state_expiry > ?
	`, state, now)
	err = row.Scan(&serviceID, &slug, &codeVerifier)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", "", ErrNotFound
	}
	return
}

// GetOAuthCreds returns fully decrypted OAuth credentials for internal use only.
// Never expose these values in API responses.
// tokenURLOverride is non-empty when a regional token endpoint is stored (e.g. Zoho EU).
func (s *Store) GetOAuthCreds(id int) (clientID, clientSecret, accessToken, refreshToken, tokenURLOverride string, tokenExpiry int64, err error) {
	var cIDEnc, cSecEnc, accEnc, refEnc string
	err = s.db.QueryRow(`
		SELECT oauth_client_id, oauth_client_secret,
		       oauth_access_token, oauth_refresh_token, oauth_token_expiry,
		       oauth_token_url_override
		FROM services WHERE id = ?
	`, id).Scan(&cIDEnc, &cSecEnc, &accEnc, &refEnc, &tokenExpiry, &tokenURLOverride)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
		return
	}
	if err != nil {
		return
	}
	if clientID, err = decrypt(s.key, cIDEnc); err != nil {
		return
	}
	if clientSecret, err = decrypt(s.key, cSecEnc); err != nil {
		return
	}
	if accessToken, err = decrypt(s.key, accEnc); err != nil {
		return
	}
	refreshToken, err = decrypt(s.key, refEnc)
	return
}

// SetAPIEndpoint stores a regional API base URL for OAuth services (e.g. "https://mail.zoho.eu").
// Used when the OAuth callback reveals the user's regional datacenter.
func (s *Store) SetAPIEndpoint(id int, endpoint string) error {
	_, err := s.db.Exec(
		`UPDATE services SET api_endpoint = ?, updated_at = ? WHERE id = ?`,
		endpoint, time.Now().UTC().Unix(), id,
	)
	return err
}

// SetOAuthTokenURLOverride stores a regional token endpoint URL override.
// Used for services like Zoho that return an accounts-server in the callback.
func (s *Store) SetOAuthTokenURLOverride(id int, tokenURL string) error {
	_, err := s.db.Exec(
		`UPDATE services SET oauth_token_url_override = ?, updated_at = ? WHERE id = ?`,
		tokenURL, time.Now().UTC().Unix(), id,
	)
	return err
}

// CompleteOAuth encrypts and stores access/refresh tokens from the callback, marks Connected (step 9c).
// Clears the ephemeral state and code_verifier.
func (s *Store) CompleteOAuth(id int, accessToken, refreshToken string, expiry int64) (*Service, error) {
	encAccess, err := encrypt(s.key, accessToken)
	if err != nil {
		return nil, err
	}
	encRefresh, err := encrypt(s.key, refreshToken)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Unix()
	res, err := s.db.Exec(`
		UPDATE services SET
			status              = ?,
			oauth_access_token  = ?,
			oauth_refresh_token = ?,
			oauth_token_expiry  = ?,
			oauth_state         = '',
			oauth_state_expiry  = 0,
			oauth_code_verifier = '',
			updated_at          = ?
		WHERE id = ?
	`, Connected, encAccess, encRefresh, expiry, now, id)
	if err != nil {
		return nil, err
	}
	return s.requireAffected(res, id)
}

// UpdateOAuthTokens replaces the access token and expiry after a background refresh (step 9d).
func (s *Store) UpdateOAuthTokens(id int, accessToken string, expiry int64) error {
	encAccess, err := encrypt(s.key, accessToken)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE services SET oauth_access_token = ?, oauth_token_expiry = ?, updated_at = ?
		WHERE id = ?
	`, encAccess, expiry, time.Now().UTC().Unix(), id)
	return err
}

// TryRefresh attempts to exchange the stored refresh token for a new access token.
// On success it persists the new token and returns it, ready to be passed to the adapter.
// On any failure it marks the service as Disconnected (so the user sees "re-authorize")
// and returns an error. Safe to call from any goroutine.
func (s *Store) TryRefresh(id int, slug string) (string, error) {
	clientID, clientSecret, _, refreshTok, tokenURLOverride, _, err := s.GetOAuthCreds(id)
	if err != nil {
		s.DisconnectOAuth(id) //nolint:errcheck
		return "", fmt.Errorf("read creds: %w", err)
	}
	if refreshTok == "" {
		s.DisconnectOAuth(id) //nolint:errcheck
		return "", fmt.Errorf("no refresh token stored — re-authorization required")
	}
	def := lookupCatalog(slug)
	if def == nil {
		s.DisconnectOAuth(id) //nolint:errcheck
		return "", fmt.Errorf("no catalog entry for slug %q", slug)
	}
	effectiveDef := *def
	if tokenURLOverride != "" {
		effectiveDef.TokenURL = tokenURLOverride
	}
	newToken, newExpiry, err := refreshToken(context.Background(), &effectiveDef, clientID, clientSecret, refreshTok)
	if err != nil {
		s.DisconnectOAuth(id) //nolint:errcheck
		return "", fmt.Errorf("refresh token exchange failed: %w", err)
	}
	if err := s.UpdateOAuthTokens(id, newToken, newExpiry.Unix()); err != nil {
		log.Printf("integrations: persist refreshed token for %s (id=%d): %v", slug, id, err)
	}
	log.Printf("integrations: token refreshed for %s (id=%d) — new expiry %s", slug, id, newExpiry.Format("2006-01-02 15:04 UTC"))
	return newToken, nil
}

// MarkNeedsReauth marks an OAuth service as NeedsReauth — the token is missing required
// scopes and cannot be fixed by a refresh. Client credentials (client_id/secret) are kept
// so the user can re-authorize without re-entering their app credentials.
func (s *Store) MarkNeedsReauth(id int) error {
	_, err := s.db.Exec(`
		UPDATE services SET
			status              = ?,
			oauth_access_token  = '',
			oauth_refresh_token = '',
			oauth_token_expiry  = 0,
			updated_at          = ?
		WHERE id = ?
	`, NeedsReauth, time.Now().UTC().Unix(), id)
	return err
}

// DisconnectOAuth marks an OAuth service as Disconnected without clearing client credentials.
// Used when a token refresh fails — the user can re-authorise without re-entering app creds.
func (s *Store) DisconnectOAuth(id int) error {
	_, err := s.db.Exec(`
		UPDATE services SET
			status             = ?,
			oauth_access_token  = '',
			oauth_refresh_token = '',
			oauth_token_expiry  = 0,
			updated_at          = ?
		WHERE id = ?
	`, Disconnected, time.Now().UTC().Unix(), id)
	return err
}
