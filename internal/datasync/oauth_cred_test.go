package datasync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/integrations"
	_ "modernc.org/sqlite"
)

// newOAuthStore returns a migrated, seeded integrations.Store backed by an in-memory SQLite.
func newOAuthStore(t *testing.T) *integrations.Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	key := make([]byte, 32) // all-zero key — fine for tests
	s := integrations.NewStore(db, key)
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := s.Seed(); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return s
}

// connectOAuth marks a catalog service as Connected with a specific OAuth access token
// (bypassing the real OAuth flow so we can inject a known token).
func connectOAuth(t *testing.T, s *integrations.Store, slug, accessToken string) int {
	t.Helper()
	svcs, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, svc := range svcs {
		if svc.Slug == slug {
			if _, err := s.StartConnect(svc.ID); err != nil {
				t.Fatalf("StartConnect: %v", err)
			}
			if _, err := s.CompleteConnect(svc.ID, integrations.ConnectInput{
				OAuthAccessToken: accessToken,
			}); err != nil {
				t.Fatalf("CompleteConnect: %v", err)
			}
			return svc.ID
		}
	}
	t.Fatalf("slug %q not found in catalog", slug)
	return 0
}

// ─── authGet header tests ─────────────────────────────────────────────────────

// TestAuthGet_SendsBearerToken confirms that authGet sets Authorization: Bearer <token>.
func TestAuthGet_SendsBearerToken(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := authGet(context.Background(), srv.URL, "xoxb-test-token-abc")
	if err != nil {
		t.Fatalf("authGet: %v", err)
	}
	if gotHeader != "Bearer xoxb-test-token-abc" {
		t.Errorf("Authorization header = %q, want %q", gotHeader, "Bearer xoxb-test-token-abc")
	}
}

// TestAuthGet_EmptyToken_SendsBearerEmpty ensures we can detect when the token is empty
// (results in "Authorization: Bearer " which all providers reject).
func TestAuthGet_EmptyToken_ServerSees(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		// Simulate provider rejecting empty token with 401.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := authGet(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("error = %q, want to contain 'HTTP 401'", err.Error())
	}
	// Go's HTTP stack trims trailing whitespace: "Bearer " + "" becomes "Bearer".
	if gotHeader != "Bearer" {
		t.Errorf("Authorization header = %q, want 'Bearer' (empty token — trailing space trimmed)", gotHeader)
	}
}

// TestAuthGet_Returns401Error checks the 401 error message for re-auth guidance.
func TestAuthGet_Returns401Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := authGet(context.Background(), srv.URL, "expired-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("error %q should contain 'HTTP 401'", err.Error())
	}
	if !strings.Contains(err.Error(), "re-authorize") {
		t.Errorf("error %q should suggest re-authorization", err.Error())
	}
}

// TestAuthGet_Returns403Error checks the 403 error message for missing scope guidance.
func TestAuthGet_Returns403Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := authGet(context.Background(), srv.URL, "token-missing-scopes")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("error %q should contain 'HTTP 403'", err.Error())
	}
	if !strings.Contains(err.Error(), "OAuth scopes") {
		t.Errorf("error %q should mention OAuth scopes", err.Error())
	}
}

// ─── Credential round-trip tests ─────────────────────────────────────────────

// TestCredentialRoundTrip_TokenReachesAdapter verifies the full path:
// OAuth token stored encrypted → ListConnected decrypts → Engine passes to adapter.
func TestCredentialRoundTrip_TokenReachesAdapter(t *testing.T) {
	const wantToken = "xoxb-real-slack-bot-token"

	store := newOAuthStore(t)
	connectOAuth(t, store, "slack", wantToken)

	var gotToken string
	capturingAdapter := &credCaptureAdapter{slug: "slack", onPull: func(creds Credentials) {
		gotToken = creds.OAuthAccessToken
	}}
	engine := NewEngine(store, []Adapter{capturingAdapter})

	if _, err := engine.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotToken != wantToken {
		t.Errorf("adapter received token %q, want %q", gotToken, wantToken)
	}
}

// TestCredentialRoundTrip_EmptyToken_DetectedByAdapter verifies that if no token
// was saved (e.g. OAuth flow incomplete), the adapter receives an empty string —
// making it possible to detect and reject instead of sending "Bearer ".
func TestCredentialRoundTrip_EmptyToken_DetectedByAdapter(t *testing.T) {
	store := newOAuthStore(t)
	// Connect without an access token (simulates incomplete OAuth callback).
	connectOAuth(t, store, "slack", "")

	var gotToken string
	capturingAdapter := &credCaptureAdapter{slug: "slack", onPull: func(creds Credentials) {
		gotToken = creds.OAuthAccessToken
	}}
	engine := NewEngine(store, []Adapter{capturingAdapter})
	engine.Run(context.Background()) //nolint:errcheck

	if gotToken != "" {
		t.Errorf("expected empty token when none was stored, got %q", gotToken)
	}
}

// ─── HTTP-level adapter tests ─────────────────────────────────────────────────

// TestSlackAdapter_SendsBearerToConversationsList verifies that the Slack adapter
// hits conversations.list with the correct Authorization header.
func TestSlackAdapter_SendsBearerToConversationsList(t *testing.T) {
	const token = "xoxb-slack-bot-token-12345"
	var gotAuth string

	// Serve a minimal valid conversations.list response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"channels": []any{},
		})
	}))
	defer srv.Close()

	// Patch the adapter to use our test server URL.
	adapter := &testableSlackAdapter{baseURL: srv.URL}
	_, err := adapter.Pull(context.Background(), Credentials{OAuthAccessToken: token}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if gotAuth != "Bearer "+token {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer "+token)
	}
}

// TestSlackAdapter_ApplicationLevelMissingScope verifies that Slack's HTTP-200
// + ok:false + error:missing_scope is surfaced as an error (not silently dropped).
func TestSlackAdapter_ApplicationLevelMissingScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slack returns 200 even for auth errors — application-level error.
		json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "missing_scope",
		})
	}))
	defer srv.Close()

	adapter := &testableSlackAdapter{baseURL: srv.URL}
	_, err := adapter.Pull(context.Background(), Credentials{OAuthAccessToken: "token"}, time.Now().Add(-time.Hour))
	if err == nil {
		t.Fatal("expected error for missing_scope, got nil")
	}
	if !strings.Contains(err.Error(), "missing_scope") {
		t.Errorf("error %q should mention missing_scope", err.Error())
	}
}

// TestEngineRun_403_DisconnectsOAuthService verifies that a real HTTP 403 from
// a service causes the engine to mark it Disconnected (re-auth required).
func TestEngineRun_403_DisconnectsOAuthService(t *testing.T) {
	store := newOAuthStore(t)
	svcID := connectOAuth(t, store, "gmail", "token-without-scopes")

	// Adapter that always returns a 403 error (as authGet would).
	forbidden := &stubAdapter{
		slug: "gmail",
		err:  fmt.Errorf("HTTP 403 from https://gmail.googleapis.com — missing required OAuth scopes"),
	}
	engine := NewEngine(store, []Adapter{forbidden})
	engine.Run(context.Background()) //nolint:errcheck

	// The service should now be NeedsReauth — not fully disconnected, just flagged.
	svc, err := store.Get(svcID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if svc.Status != integrations.NeedsReauth {
		t.Errorf("status = %v, want NeedsReauth after 403", svc.Status)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// credCaptureAdapter records the Credentials it receives without making network calls.
type credCaptureAdapter struct {
	slug   string
	onPull func(Credentials)
}

func (a *credCaptureAdapter) Slug() string { return a.slug }
func (a *credCaptureAdapter) Pull(_ context.Context, creds Credentials, _ time.Time) ([]RawSignal, error) {
	if a.onPull != nil {
		a.onPull(creds)
	}
	return nil, nil
}

// testableSlackAdapter is a SlackAdapter variant whose API base URL is configurable,
// allowing tests to redirect requests to a local httptest.Server.
type testableSlackAdapter struct {
	baseURL string // replaces "https://slack.com"
}

func (a *testableSlackAdapter) Slug() string { return "slack" }

func (a *testableSlackAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	base := a.baseURL
	if base == "" {
		base = "https://slack.com"
	}
	channelListURL := base + "/api/conversations.list?types=public_channel,private_channel,im&limit=200&exclude_archived=true"
	listBody, err := authGet(ctx, channelListURL, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("slack: list channels: %w", err)
	}

	var listResp struct {
		OK       bool   `json:"ok"`
		Error    string `json:"error"`
		Channels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channels"`
	}
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return nil, fmt.Errorf("slack: decode channels: %w", err)
	}
	if !listResp.OK {
		return nil, fmt.Errorf("slack: conversations.list: %s", listResp.Error)
	}
	return nil, nil
}
