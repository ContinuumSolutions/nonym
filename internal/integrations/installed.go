package integrations

// InstalledService holds a fully decrypted service row.
// For internal use by the sync engine only — never returned by the HTTP API.
type InstalledService struct {
	ID                int
	Slug              string
	Name              string
	Category          string
	APIKey            string
	APIEndpoint       string
	OAuthAccessToken  string
	OAuthRefreshToken string
}

// ListInstalled returns every service with status = Installed, with credentials
// fully decrypted. The caller must treat these values as secrets.
func (s *Store) ListInstalled() ([]InstalledService, error) {
	rows, err := s.db.Query(`
		SELECT id, slug, name, category, api_key, api_endpoint,
		       oauth_access_token, oauth_refresh_token
		FROM services WHERE status = ?
	`, Installed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []InstalledService
	for rows.Next() {
		var svc InstalledService
		var apiKeyEnc, accessEnc, refreshEnc string

		if err := rows.Scan(
			&svc.ID, &svc.Slug, &svc.Name, &svc.Category,
			&apiKeyEnc, &svc.APIEndpoint, &accessEnc, &refreshEnc,
		); err != nil {
			return nil, err
		}

		if svc.APIKey, err = decrypt(s.key, apiKeyEnc); err != nil {
			return nil, err
		}
		if svc.OAuthAccessToken, err = decrypt(s.key, accessEnc); err != nil {
			return nil, err
		}
		if svc.OAuthRefreshToken, err = decrypt(s.key, refreshEnc); err != nil {
			return nil, err
		}

		services = append(services, svc)
	}
	return services, rows.Err()
}
