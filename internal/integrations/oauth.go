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

// buildAuthURL constructs the full OAuth2 authorization URL including PKCE and CSRF params.
func buildAuthURL(def *ServiceDef, clientID, redirectURI, state, challenge string) string {
	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(def.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
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
}

// exchangeCode exchanges an authorization code for access/refresh tokens.
func exchangeCode(ctx context.Context, def *ServiceDef, clientID, clientSecret, code, redirectURI, codeVerifier string) (access, refresh string, expiry time.Time, err error) {
	body := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}
	if clientSecret != "" {
		body.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, def.TokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("exchange code: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

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
	if tr.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("exchange code: empty access_token in response")
	}

	exp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.ExpiresIn == 0 {
		exp = time.Now().Add(time.Hour) // default 1h when provider omits expires_in
	}
	return tr.AccessToken, tr.RefreshToken, exp, nil
}

// refreshToken uses the refresh_token grant to obtain a new access_token.
func refreshToken(ctx context.Context, def *ServiceDef, clientID, clientSecret, refresh string) (access string, expiry time.Time, err error) {
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
		return "", time.Time{}, fmt.Errorf("refresh token: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh token: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", time.Time{}, fmt.Errorf("refresh token: decode: %w", err)
	}
	if tr.Error != "" {
		return "", time.Time{}, fmt.Errorf("refresh token: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("refresh token: empty access_token in response")
	}

	exp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.ExpiresIn == 0 {
		exp = time.Now().Add(time.Hour)
	}
	return tr.AccessToken, exp, nil
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
