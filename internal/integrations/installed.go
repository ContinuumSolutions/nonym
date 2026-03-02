package integrations

import (
	"context"
	"log"
	"time"
)

// ConnectedService holds a fully decrypted service row.
// For internal use by the sync engine only — never returned by the HTTP API.
type ConnectedService struct {
	ID                int
	Slug              string
	Name              string
	Category          string
	AuthMethod        AuthMethod
	APIKey            string
	APIEndpoint       string
	OAuthAccessToken  string
	OAuthRefreshToken string
	OAuthTokenExpiry  int64 // Unix timestamp; 0 means unknown/not set
}

// ListConnected returns every service with status = Connected, with credentials
// fully decrypted. For OAuth services whose access token is expiring within the
// next 5 minutes, a background refresh is attempted using the stored refresh_token.
// If the refresh fails, the service is marked Disconnected and excluded from the result.
// The caller must treat all returned values as secrets.
func (s *Store) ListConnected() ([]ConnectedService, error) {
	rows, err := s.db.Query(`
		SELECT id, slug, name, category, auth_method,
		       api_key, api_endpoint,
		       oauth_access_token, oauth_refresh_token, oauth_token_expiry,
		       oauth_client_id, oauth_client_secret
		FROM services WHERE status = ?
	`, Connected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []ConnectedService
	for rows.Next() {
		var svc ConnectedService
		var apiKeyEnc, accessEnc, refreshEnc, clientIDEnc, clientSecEnc string

		if err := rows.Scan(
			&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.AuthMethod,
			&apiKeyEnc, &svc.APIEndpoint,
			&accessEnc, &refreshEnc, &svc.OAuthTokenExpiry,
			&clientIDEnc, &clientSecEnc,
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

		// Step 9d: for OAuth services, proactively refresh tokens expiring within 5 minutes.
		if svc.AuthMethod == OAuth2Auth {
			refreshDeadline := time.Now().Add(5 * time.Minute).Unix()
			if svc.OAuthTokenExpiry != 0 && svc.OAuthTokenExpiry <= refreshDeadline && svc.OAuthRefreshToken != "" {
				clientID, _ := decrypt(s.key, clientIDEnc)
				clientSecret, _ := decrypt(s.key, clientSecEnc)
				def := lookupCatalog(svc.Slug)
				if def != nil && clientID != "" {
					newAccess, newExpiry, refreshErr := refreshToken(context.Background(), def, clientID, clientSecret, svc.OAuthRefreshToken)
					if refreshErr != nil {
						log.Printf("integrations: token refresh for %s failed: %v — marking disconnected", svc.Slug, refreshErr)
						s.DisconnectOAuth(svc.ID) //nolint:errcheck
						continue                  // skip this service from the result
					}
					if updateErr := s.UpdateOAuthTokens(svc.ID, newAccess, newExpiry.Unix()); updateErr != nil {
						log.Printf("integrations: store refreshed token for %s: %v", svc.Slug, updateErr)
					}
					svc.OAuthAccessToken = newAccess
					svc.OAuthTokenExpiry = newExpiry.Unix()
				}
			}
		}

		services = append(services, svc)
	}
	return services, rows.Err()
}
