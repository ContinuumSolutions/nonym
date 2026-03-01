package integrations

// ConnectedService holds a fully decrypted service row.
// For internal use by the sync engine only — never returned by the HTTP API.
type ConnectedService struct {
	ID                int
	Slug              string
	Name              string
	Category          string
	APIKey            string
	APIEndpoint       string
	OAuthAccessToken  string
	OAuthRefreshToken string
}

// ListConnected returns every service with status = Connected, with credentials
// fully decrypted. The caller must treat these values as secrets.
func (s *Store) ListConnected() ([]ConnectedService, error) {
	rows, err := s.db.Query(`
		SELECT id, slug, name, category, api_key, api_endpoint,
		       oauth_access_token, oauth_refresh_token
		FROM services WHERE status = ?
	`, Connected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []ConnectedService
	for rows.Next() {
		var svc ConnectedService
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
