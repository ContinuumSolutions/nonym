package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// setupTOTPTestDB creates an in-memory SQLite DB with the full schema needed for TOTP tests.
func setupTOTPTestDB(t *testing.T) *sql.DB {
	t.Helper()
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	queries := []string{
		`CREATE TABLE organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			owner_id INTEGER,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO organizations (id, name, slug, is_active) VALUES (1, 'Test Org', 'test-org', 1)`,
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			role TEXT NOT NULL DEFAULT 'user',
			is_active BOOLEAN DEFAULT TRUE,
			email_verified BOOLEAN DEFAULT FALSE,
			last_login DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			totp_enabled BOOLEAN DEFAULT FALSE,
			totp_secret TEXT,
			totp_verified_at DATETIME
		)`,
		`CREATE TABLE totp_setup_sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			secret TEXT NOT NULL,
			attempt_count INTEGER DEFAULT 0,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE totp_backup_codes (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			code_hash TEXT NOT NULL,
			used_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE auth_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			user_id INTEGER,
			ip_address TEXT,
			user_agent TEXT,
			success BOOLEAN NOT NULL,
			error_reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := testDB.Exec(q); err != nil {
			t.Fatalf("setup query %q: %v", q, err)
		}
	}
	return testDB
}

// insertTestUser creates a user with a known password hash and returns the user.
func insertTestUser(t *testing.T, testDB *sql.DB, totpEnabled bool, totpSecret *string) *User {
	t.Helper()
	// password = "password123"
	hash, _ := hashPassword("password123")
	var secretVal interface{}
	if totpSecret != nil {
		secretVal = *totpSecret
	}
	res, err := testDB.Exec(
		`INSERT INTO users (organization_id, email, password_hash, first_name, last_name, role,
		 is_active, email_verified, totp_enabled, totp_secret)
		 VALUES (1, 'user@test.com', ?, 'Test', 'User', 'owner', 1, 1, ?, ?)`,
		hash, totpEnabled, secretVal,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, _ := res.LastInsertId()
	return &User{
		ID:             int(id),
		Email:          "user@test.com",
		PasswordHash:   hash,
		FirstName:      "Test",
		LastName:       "User",
		Role:           RoleOwner,
		OrganizationID: 1,
		IsActive:       true,
		TOTPEnabled:    totpEnabled,
		TOTPSecret:     totpSecret,
	}
}

func setupTOTPApp(testDB *sql.DB, user *User) *fiber.App {
	db = testDB
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	app := fiber.New()
	mockAuth := func(c *fiber.Ctx) error {
		c.Locals("user", user)
		return c.Next()
	}
	app.Get("/api/v1/auth/2fa/status", mockAuth, HandleTOTPStatus)
	app.Post("/api/v1/auth/2fa/setup/begin", mockAuth, HandleTOTPSetupBegin)
	app.Post("/api/v1/auth/2fa/setup/verify", mockAuth, HandleTOTPSetupVerify)
	app.Delete("/api/v1/auth/2fa", mockAuth, HandleTOTPDisable)
	app.Post("/api/v1/auth/2fa/backup-codes/regenerate", mockAuth, HandleTOTPRegenerateBackupCodes)
	app.Post("/api/v1/auth/2fa/challenge", HandleTOTPChallenge)
	return app
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ─── Status ───────────────────────────────────────────────────────────────────

func TestHandleTOTPStatus_Disabled(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("GET", "/api/v1/auth/2fa/status", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body TwoFAStatus
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Enabled {
		t.Error("expected enabled=false")
	}
}

// ─── Setup: begin ─────────────────────────────────────────────────────────────

func TestHandleTOTPSetupBegin_Success(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["session_id"] == "" || body["session_id"] == nil {
		t.Error("expected session_id")
	}
	if body["secret"] == nil {
		t.Error("expected secret")
	}
	uri, _ := body["otpauth_uri"].(string)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("unexpected otpauth_uri: %q", uri)
	}
}

func TestHandleTOTPSetupBegin_WrongPassword(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "wrongpassword"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandleTOTPSetupBegin_AlreadyEnabled(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	enc, _ := encryptTOTPSecret("TESTSECRET")
	user := insertTestUser(t, testDB, true, &enc)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 409 {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

// ─── Setup: verify ────────────────────────────────────────────────────────────

func TestHandleTOTPSetupVerify_Success(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	// Begin setup to get a real session
	beginReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "password123"}))
	beginReq.Header.Set("Content-Type", "application/json")
	beginResp, _ := app.Test(beginReq)
	var beginBody map[string]interface{}
	json.NewDecoder(beginResp.Body).Decode(&beginBody)

	sessionID := beginBody["session_id"].(string)
	secret := beginBody["secret"].(string)

	// Generate a valid TOTP code for the secret
	code, err := totpGenerateCodeAt(secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	verifyReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/verify",
		jsonBody(map[string]string{"session_id": sessionID, "totp_code": code}))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyResp, _ := app.Test(verifyReq)

	if verifyResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", verifyResp.StatusCode)
	}
	var verifyBody map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&verifyBody)
	codes, ok := verifyBody["backup_codes"].([]interface{})
	if !ok || len(codes) != 8 {
		t.Errorf("expected 8 backup codes, got %v", verifyBody["backup_codes"])
	}
}

func TestHandleTOTPSetupVerify_WrongCode(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	beginReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "password123"}))
	beginReq.Header.Set("Content-Type", "application/json")
	beginResp, _ := app.Test(beginReq)
	var beginBody map[string]interface{}
	json.NewDecoder(beginResp.Body).Decode(&beginBody)

	verifyReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/verify",
		jsonBody(map[string]string{"session_id": beginBody["session_id"].(string), "totp_code": "000000"}))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyResp, _ := app.Test(verifyReq)

	if verifyResp.StatusCode != 422 {
		t.Errorf("expected 422, got %d", verifyResp.StatusCode)
	}
}

func TestHandleTOTPSetupVerify_InvalidSession(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	verifyReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/verify",
		jsonBody(map[string]string{"session_id": "no-such-session", "totp_code": "123456"}))
	verifyReq.Header.Set("Content-Type", "application/json")
	verifyResp, _ := app.Test(verifyReq)

	if verifyResp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", verifyResp.StatusCode)
	}
}

// ─── Disable ──────────────────────────────────────────────────────────────────

func TestHandleTOTPDisable_Success(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	enc, _ := encryptTOTPSecret("TESTSECRET")
	user := insertTestUser(t, testDB, true, &enc)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/2fa",
		jsonBody(map[string]string{"password": "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 204 {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestHandleTOTPDisable_NotEnabled(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/2fa",
		jsonBody(map[string]string{"password": "password123"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleTOTPDisable_WrongPassword(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	enc, _ := encryptTOTPSecret("TESTSECRET")
	user := insertTestUser(t, testDB, true, &enc)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("DELETE", "/api/v1/auth/2fa",
		jsonBody(map[string]string{"password": "wrong"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// ─── MFA Challenge ────────────────────────────────────────────────────────────

func TestHandleTOTPChallenge_InvalidMFAToken(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/challenge",
		jsonBody(map[string]string{"mfa_token": "bad-token", "totp_code": "123456"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandleTOTPChallenge_WithBackupCode(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()

	// Set up a user with TOTP enabled
	secret, _ := generateTOTPSecret()
	enc, _ := encryptTOTPSecret(secret)
	user := insertTestUser(t, testDB, true, &enc)
	app := setupTOTPApp(testDB, user)

	// Store a known backup code
	codes := []string{"abcd-efgh"}
	storeBackupCodes(user.ID, codes)

	// Generate an MFA token
	mfaToken, _, _ := generateMFAToken(user.ID)

	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/challenge",
		jsonBody(map[string]string{"mfa_token": mfaToken, "totp_code": "abcd-efgh"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if body["token"] == nil {
		t.Error("expected JWT token in response")
	}
}

// ─── Rate limiting ────────────────────────────────────────────────────────────

func TestHandleTOTPSetupVerify_RateLimit(t *testing.T) {
	testDB := setupTOTPTestDB(t)
	defer testDB.Close()
	user := insertTestUser(t, testDB, false, nil)
	app := setupTOTPApp(testDB, user)

	// Begin
	beginReq := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/begin",
		jsonBody(map[string]string{"password": "password123"}))
	beginReq.Header.Set("Content-Type", "application/json")
	beginResp, _ := app.Test(beginReq)
	var beginBody map[string]interface{}
	json.NewDecoder(beginResp.Body).Decode(&beginBody)
	sessionID := beginBody["session_id"].(string)

	// Exhaust attempts
	for i := 0; i < maxSetupAttempts; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/verify",
			jsonBody(map[string]string{"session_id": sessionID, "totp_code": "000000"}))
		req.Header.Set("Content-Type", "application/json")
		app.Test(req)
	}

	// Next attempt should be rate-limited
	req := httptest.NewRequest("POST", "/api/v1/auth/2fa/setup/verify",
		jsonBody(map[string]string{"session_id": sessionID, "totp_code": "000000"}))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req)

	if resp.StatusCode != 429 {
		t.Errorf("expected 429 after exhausting attempts, got %d", resp.StatusCode)
	}
}
