package integrations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// basicAuthHeader encodes client_id:client_secret as an HTTP Basic auth value.
func basicAuthHeader(clientID, clientSecret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret))
}

// generateState creates a cryptographically random CSRF state token (32 bytes, base64url).
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generatePKCE returns a (verifier, challenge) pair per RFC 7636, method S256.
// verifier = 32 random bytes, base64url-encoded (no padding).
// challenge = BASE64URL(SHA-256(verifier)).
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate pkce: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// lookupCatalog returns the ServiceDef for slug, or nil if not found.
func lookupCatalog(slug string) *ServiceDef {
	for i := range registry {
		if registry[i].Slug == slug {
			return &registry[i]
		}
	}
	return nil
}

// buildAuthURL constructs the full OAuth2 authorization URL including CSRF params.
// PKCE (code_challenge/code_challenge_method) is omitted when def.NoPKCE is true (e.g. Notion).
func buildAuthURL(def *ServiceDef, clientID, redirectURI, state, challenge string) string {
	params := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"state":         {state},
	}
	if len(def.Scopes) > 0 {
		params.Set("scope", strings.Join(def.Scopes, " "))
	}
	if !def.NoPKCE && challenge != "" {
		params.Set("code_challenge", challenge)
		params.Set("code_challenge_method", "S256")
	}
	// Merge service-specific extra query params (e.g. access_type=offline for Google).
	for k, v := range def.ExtraParams {
		params.Set(k, v)
	}
	return def.AuthURL + "?" + params.Encode()
}

// tokenResponse is the common token endpoint JSON response.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
	// Slack OAuth v2 returns the user token nested inside authed_user when
	// user_scope is requested. Top-level access_token is the bot token (empty
	// when no bot scopes are requested).
	AuthedUser struct {
		AccessToken string `json:"access_token"`
	} `json:"authed_user"`
}

// exchangeCode exchanges an authorization code for access/refresh tokens.
// When def.UseBasicAuth is true, client credentials go in the Authorization header (e.g. Notion).
// When def.NoPKCE is true, code_verifier is omitted from the body.
// Returns a zero expiry (time.Time{}) when the provider omits expires_in and NoPKCE is true
// (Notion tokens don't expire and have no refresh_token).
func exchangeCode(ctx context.Context, def *ServiceDef, clientID, clientSecret, code, redirectURI, codeVerifier string) (access, refresh string, expiry time.Time, err error) {
	body := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}
	if !def.UseBasicAuth {
		body.Set("client_id", clientID)
		if clientSecret != "" {
			body.Set("client_secret", clientSecret)
		}
	}
	if !def.NoPKCE && codeVerifier != "" {
		body.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.TokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("exchange code: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if def.UseBasicAuth {
		req.Header.Set("Authorization", basicAuthHeader(clientID, clientSecret))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", "", time.Time{}, fmt.Errorf("exchange code: decode: %w", err)
	}
	if tr.Error != "" {
		return "", "", time.Time{}, fmt.Errorf("exchange code: %s: %s", tr.Error, tr.ErrorDesc)
	}
	// Slack OAuth v2: when only user_scope is requested, the user token lives in
	// authed_user.access_token and the top-level access_token is absent.
	if tr.AccessToken == "" && tr.AuthedUser.AccessToken != "" {
		tr.AccessToken = tr.AuthedUser.AccessToken
	}
	if tr.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("exchange code: empty access_token in response")
	}

	var exp time.Time
	if tr.ExpiresIn > 0 {
		exp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else if !def.NoPKCE {
		// Provider omitted expires_in but uses standard refresh flow — default 1h.
		exp = time.Now().Add(time.Hour)
	}
	// NoPKCE services (e.g. Notion) return non-expiring tokens; exp stays zero.
	return tr.AccessToken, tr.RefreshToken, exp, nil
}

// refreshToken uses the refresh_token grant to obtain a new access_token.
// newRefresh is non-empty only when the provider returns a rotated refresh token (e.g. Zoho).
// Callers must persist newRefresh when non-empty, otherwise the old token becomes invalid.
func refreshToken(ctx context.Context, def *ServiceDef, clientID, clientSecret, refresh string) (access, newRefresh string, expiry time.Time, err error) {
	body := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {clientID},
	}
	if clientSecret != "" {
		body.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.TokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("refresh token: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("refresh token: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", "", time.Time{}, fmt.Errorf("refresh token: decode: %w", err)
	}
	if tr.Error != "" {
		return "", "", time.Time{}, fmt.Errorf("refresh token: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("refresh token: empty access_token in response")
	}

	exp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.ExpiresIn == 0 {
		exp = time.Now().Add(time.Hour)
	}
	return tr.AccessToken, tr.RefreshToken, exp, nil
}

// revokeToken calls the service's revocation endpoint. Best-effort — errors are logged,
// not returned, so disconnect always succeeds even if revocation fails.
func revokeToken(ctx context.Context, def *ServiceDef, accessToken string) error {
	if def.RevokeURL == "" || accessToken == "" {
		return nil
	}
	body := url.Values{
		"token":           {accessToken},
		"token_type_hint": {"access_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.RevokeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("revoke token: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	resp.Body.Close()
	return nil
}
