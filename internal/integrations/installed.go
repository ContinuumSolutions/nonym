package integrations

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"
)

// ConnectedService holds a fully decrypted service row.
// For internal use by the sync engine only — never returned by the HTTP API.
type ConnectedService struct {
	ID                    int
	Slug                  string
	Name                  string
	Category              string
	Description           string // what this service is and what it's used for
	AuthMethod            AuthMethod
	APIKey                string
	APIEndpoint           string
	OAuthAccessToken      string
	OAuthRefreshToken     string
	OAuthTokenExpiry      int64  // Unix timestamp; 0 means unknown/not set
	OAuthTokenURLOverride string // non-empty for regional providers (e.g. Zoho EU)
}

// ListConnected returns every service with status = Connected, with credentials
// fully decrypted. For OAuth services whose access token is expiring within the
// next 5 minutes, a background refresh is attempted using the stored refresh_token.
// If the refresh fails, the service is marked Disconnected and excluded from the result.
// The caller must treat all returned values as secrets.
func (s *Store) ListConnected() ([]ConnectedService, error) {
	rows, err := s.db.Query(`
		SELECT id, slug, name, category, description, auth_method,
		       api_key, api_endpoint,
		       oauth_access_token, oauth_refresh_token, oauth_token_expiry,
		       oauth_client_id, oauth_client_secret,
		       oauth_token_url_override
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
			&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Description, &svc.AuthMethod,
			&apiKeyEnc, &svc.APIEndpoint,
			&accessEnc, &refreshEnc, &svc.OAuthTokenExpiry,
			&clientIDEnc, &clientSecEnc,
			&svc.OAuthTokenURLOverride,
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
					// Apply regional token URL override (e.g. Zoho EU/India).
					effectiveDef := *def
					if svc.OAuthTokenURLOverride != "" {
						effectiveDef.TokenURL = svc.OAuthTokenURLOverride
					}
					newAccess, newRefresh, newExpiry, refreshErr := refreshToken(context.Background(), &effectiveDef, clientID, clientSecret, svc.OAuthRefreshToken)
					if refreshErr != nil {
						log.Printf("integrations: token refresh for %s failed: %v — marking disconnected", svc.Slug, refreshErr)
						s.DisconnectOAuth(svc.ID) //nolint:errcheck
						continue                  // skip this service from the result
					}
					if updateErr := s.UpdateOAuthTokens(svc.ID, newAccess, newRefresh, newExpiry.Unix()); updateErr != nil {
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

// GetConnectedBySlug returns the decrypted credentials for a single connected service.
// Returns ErrNotFound if the service is not in status=Connected.
// Used by the execution engine to fetch credentials at action time.
func (s *Store) GetConnectedBySlug(slug string) (*ConnectedService, error) {
	var svc ConnectedService
	var apiKeyEnc, accessEnc, refreshEnc, clientIDEnc, clientSecEnc string

	err := s.db.QueryRow(`
		SELECT id, slug, name, category, description, auth_method,
		       api_key, api_endpoint,
		       oauth_access_token, oauth_refresh_token, oauth_token_expiry,
		       oauth_client_id, oauth_client_secret,
		       oauth_token_url_override
		FROM services WHERE slug = ? AND status = ?
	`, slug, Connected).Scan(
		&svc.ID, &svc.Slug, &svc.Name, &svc.Category, &svc.Description, &svc.AuthMethod,
		&apiKeyEnc, &svc.APIEndpoint,
		&accessEnc, &refreshEnc, &svc.OAuthTokenExpiry,
		&clientIDEnc, &clientSecEnc,
		&svc.OAuthTokenURLOverride,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var decErr error
	if svc.APIKey, decErr = decrypt(s.key, apiKeyEnc); decErr != nil {
		return nil, decErr
	}
	if svc.OAuthAccessToken, decErr = decrypt(s.key, accessEnc); decErr != nil {
		return nil, decErr
	}
	if svc.OAuthRefreshToken, decErr = decrypt(s.key, refreshEnc); decErr != nil {
		return nil, decErr
	}

	// Proactively refresh expiring OAuth tokens (same logic as ListConnected).
	if svc.AuthMethod == OAuth2Auth {
		refreshDeadline := time.Now().Add(5 * time.Minute).Unix()
		if svc.OAuthTokenExpiry != 0 && svc.OAuthTokenExpiry <= refreshDeadline && svc.OAuthRefreshToken != "" {
			clientID, _ := decrypt(s.key, clientIDEnc)
			clientSecret, _ := decrypt(s.key, clientSecEnc)
			def := lookupCatalog(svc.Slug)
			if def != nil && clientID != "" {
				effectiveDef := *def
				if svc.OAuthTokenURLOverride != "" {
					effectiveDef.TokenURL = svc.OAuthTokenURLOverride
				}
				newAccess, newRefresh, newExpiry, refreshErr := refreshToken(context.Background(), &effectiveDef, clientID, clientSecret, svc.OAuthRefreshToken)
				if refreshErr != nil {
					log.Printf("integrations: token refresh for %s failed: %v — marking disconnected", svc.Slug, refreshErr)
					s.DisconnectOAuth(svc.ID) //nolint:errcheck
					return nil, ErrNotFound
				}
				if updateErr := s.UpdateOAuthTokens(svc.ID, newAccess, newRefresh, newExpiry.Unix()); updateErr != nil {
					log.Printf("integrations: store refreshed token for %s: %v", svc.Slug, updateErr)
				}
				svc.OAuthAccessToken = newAccess
				svc.OAuthTokenExpiry = newExpiry.Unix()
			}
		}
	}

	return &svc, nil
}
