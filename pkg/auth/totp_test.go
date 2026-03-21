package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTOTPSecret(t *testing.T) {
	// Need jwtSecret for encryption fallback
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")

	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	if len(secret) < 16 {
		t.Errorf("secret too short: %q", secret)
	}
	// Base32: only uppercase letters and 2-7
	for _, ch := range secret {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= '2' && ch <= '7') || ch == '=') {
			t.Errorf("invalid base32 char %q in secret %q", ch, secret)
		}
	}
}

func TestVerifyTOTPCode_InvalidCode(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}
	if verifyTOTPCode(secret, "000000") {
		// Very unlikely to be valid
		t.Log("000000 was valid (extremely unlikely but not a bug)")
	}
	if verifyTOTPCode(secret, "abc") {
		t.Error("malformed code should not validate")
	}
}

func TestVerifyTOTPCode_ValidCode(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	// Use pquerna/otp to generate a valid code and immediately verify it
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("generateTOTPSecret: %v", err)
	}

	// Import the totp package inline for test
	code, err := totpGenerateCode(secret)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !verifyTOTPCode(secret, code) {
		t.Errorf("valid code %q should pass verification", code)
	}
}

func TestEncryptDecryptTOTPSecret(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	plain := "JBSWY3DPEHPK3PXP"
	enc, err := encryptTOTPSecret(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if enc == plain {
		t.Error("ciphertext should differ from plaintext")
	}
	dec, err := decryptTOTPSecret(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != plain {
		t.Errorf("roundtrip: got %q, want %q", dec, plain)
	}
}

func TestEncryptDecryptDifferentEachTime(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	plain := "TESTSECRET"
	enc1, _ := encryptTOTPSecret(plain)
	enc2, _ := encryptTOTPSecret(plain)
	if enc1 == enc2 {
		t.Error("each encryption should produce a different ciphertext (random nonce)")
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := generateBackupCodes(8)
	if err != nil {
		t.Fatalf("generateBackupCodes: %v", err)
	}
	if len(codes) != 8 {
		t.Errorf("expected 8 codes, got %d", len(codes))
	}
	seen := map[string]bool{}
	for _, code := range codes {
		parts := strings.Split(code, "-")
		if len(parts) != 2 || len(parts[0]) != 4 || len(parts[1]) != 4 {
			t.Errorf("invalid backup code format: %q", code)
		}
		if seen[code] {
			t.Errorf("duplicate backup code: %q", code)
		}
		seen[code] = true
	}
}

func TestIsBackupCodeFormat(t *testing.T) {
	cases := []struct {
		code string
		ok   bool
	}{
		{"abcd-efgh", true},
		{"1234-5678", true},
		{"ab-cdef", false},
		{"abcdefgh", false},
		{"abc-defg", false},
		{"abcd-ef", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isBackupCodeFormat(tc.code)
		if got != tc.ok {
			t.Errorf("isBackupCodeFormat(%q) = %v, want %v", tc.code, got, tc.ok)
		}
	}
}

func TestBuildOTPAuthURI(t *testing.T) {
	uri := buildOTPAuthURI("JBSWY3DPEHPK3PXP", "user@example.com")
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("unexpected URI: %q", uri)
	}
	if !strings.Contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Errorf("missing secret in URI: %q", uri)
	}
	if !strings.Contains(uri, "issuer=SovereignPrivacyGateway") {
		t.Errorf("missing issuer in URI: %q", uri)
	}
}

func TestMFAToken_RoundTrip(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	token, expiresAt, err := generateMFAToken(42)
	if err != nil {
		t.Fatalf("generateMFAToken: %v", err)
	}
	if expiresAt.Before(time.Now()) {
		t.Error("expiresAt should be in the future")
	}
	userID, err := validateMFAToken(token)
	if err != nil {
		t.Fatalf("validateMFAToken: %v", err)
	}
	if userID != 42 {
		t.Errorf("userID: got %d, want 42", userID)
	}
}

func TestMFAToken_Invalid(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	_, err := validateMFAToken("not-a-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestMFAToken_WrongPurpose(t *testing.T) {
	jwtSecret = []byte("test-secret-32-bytes-padded-here!")
	// A normal JWT (not mfa_challenge purpose) should be rejected
	user := &User{ID: 1, Email: "a@b.com", Role: RoleUser, OrganizationID: 1}
	token, _, _ := generateJWTToken(user)
	_, err := validateMFAToken(token)
	if err == nil {
		t.Error("normal JWT should be rejected as MFA token")
	}
}

// totpGenerateCode is a test helper to produce a live TOTP code.
func totpGenerateCode(secret string) (string, error) {
	// Use the pquerna/otp library directly
	return totpGenerateCodeAt(secret, time.Now())
}
