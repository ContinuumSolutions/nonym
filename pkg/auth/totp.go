package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

const (
	totpIssuer       = "SovereignPrivacyGateway"
	totpPeriod       = 30
	totpDigits       = otp.DigitsSix
	totpAlgorithm    = otp.AlgorithmSHA1
	backupCodeCount  = 8
	mfaTokenExpiry   = 15 * time.Minute
	setupSessionTTL  = 10 * time.Minute
	maxSetupAttempts = 5
)

// backupCodeCharset excludes ambiguous characters (0, O, I, l, 1)
const backupCodeCharset = "abcdefghjkmnpqrstuvwxyz23456789"

// TOTPSetupSession holds a pending (unverified) TOTP setup state.
type TOTPSetupSession struct {
	ID           string
	UserID       int
	Secret       string // encrypted
	AttemptCount int
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// TwoFAStatus is the response for GET /api/v1/auth/2fa/status.
type TwoFAStatus struct {
	Enabled              bool       `json:"enabled"`
	VerifiedAt           *time.Time `json:"verified_at,omitempty"`
	BackupCodesRemaining int        `json:"backup_codes_remaining"`
}

// ─── Crypto helpers ───────────────────────────────────────────────────────────

// generateTOTPSecret generates a 20-byte, base32-encoded TOTP secret.
func generateTOTPSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: "user",
		Period:      totpPeriod,
		Digits:      totpDigits,
		Algorithm:   totpAlgorithm,
		SecretSize:  20,
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}
	return key.Secret(), nil
}

// buildOTPAuthURI builds the otpauth URI for QR code generation.
func buildOTPAuthURI(secret, email string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		totpIssuer, email, secret, totpIssuer,
	)
}

// verifyTOTPCode validates a 6-digit TOTP code with ±1 step (30 s) drift.
func verifyTOTPCode(secret, code string) bool {
	valid, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    totpPeriod,
		Skew:      1,
		Digits:    totpDigits,
		Algorithm: totpAlgorithm,
	})
	return err == nil && valid
}

// totpEncryptionKey returns the 32-byte AES key for TOTP secret encryption.
// Reads TOTP_ENCRYPTION_KEY (64-char hex) from env; falls back to SHA-256(jwtSecret).
func totpEncryptionKey() ([]byte, error) {
	if envKey := os.Getenv("TOTP_ENCRYPTION_KEY"); envKey != "" {
		key, err := hex.DecodeString(envKey)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("TOTP_ENCRYPTION_KEY must be a 64-char hex string (32 bytes)")
		}
		return key, nil
	}
	h := sha256.Sum256(jwtSecret)
	return h[:], nil
}

// encryptTOTPSecret encrypts a plaintext secret with AES-256-GCM.
// Output: base64(nonce || ciphertext).
func encryptTOTPSecret(plaintext string) (string, error) {
	key, err := totpEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptTOTPSecret decrypts a base64-encoded AES-256-GCM ciphertext.
func decryptTOTPSecret(encoded string) (string, error) {
	key, err := totpEncryptionKey()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// generateBackupCodes generates n backup codes in xxxx-xxxx format.
func generateBackupCodes(n int) ([]string, error) {
	charset := []byte(backupCodeCharset)
	codes := make([]string, n)
	for i := range codes {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return nil, err
		}
		chars := make([]byte, 8)
		for j, b := range buf {
			chars[j] = charset[int(b)%len(charset)]
		}
		codes[i] = string(chars[:4]) + "-" + string(chars[4:])
	}
	return codes, nil
}

// hashBackupCode bcrypt-hashes a backup code (cost 10).
func hashBackupCode(code string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(code), 10)
	return string(h), err
}

// isBackupCodeFormat returns true if the string matches xxxx-xxxx.
func isBackupCodeFormat(code string) bool {
	parts := strings.Split(code, "-")
	return len(parts) == 2 && len(parts[0]) == 4 && len(parts[1]) == 4
}

// generateMFAToken creates a short-lived JWT for the MFA challenge step.
func generateMFAToken(userID int) (string, time.Time, error) {
	expiresAt := time.Now().Add(mfaTokenExpiry)
	claims := jwt.MapClaims{
		"user_id": strconv.Itoa(userID),
		"purpose": "mfa_challenge",
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenString, expiresAt, nil
}

// validateMFAToken validates an MFA token and returns the user ID.
func validateMFAToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid or expired MFA token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid claims")
	}
	if purpose, _ := claims["purpose"].(string); purpose != "mfa_challenge" {
		return 0, fmt.Errorf("invalid token purpose")
	}
	userIDStr, _ := claims["user_id"].(string)
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID in token")
	}
	return userID, nil
}

// ─── Database helpers ─────────────────────────────────────────────────────────

// createSetupSession upserts a TOTP setup session for the user.
func createSetupSession(userID int, encryptedSecret string) (string, time.Time, error) {
	// Delete any existing session for this user first
	_ = deleteSetupSession(userID)

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(setupSessionTTL)

	query := formatQuery(`INSERT INTO totp_setup_sessions (id, user_id, secret, expires_at, created_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	_, err := db.Exec(query, sessionID, userID, encryptedSecret, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create setup session: %w", err)
	}
	return sessionID, expiresAt, nil
}

// getSetupSession retrieves a setup session by ID.
func getSetupSession(sessionID string) (*TOTPSetupSession, error) {
	s := &TOTPSetupSession{}
	query := formatQuery(`SELECT id, user_id, secret, attempt_count, expires_at, created_at
		FROM totp_setup_sessions WHERE id = ?`)
	err := db.QueryRow(query, sessionID).Scan(
		&s.ID, &s.UserID, &s.Secret, &s.AttemptCount, &s.ExpiresAt, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found or expired")
	}
	if err != nil {
		return nil, err
	}
	if time.Now().After(s.ExpiresAt) {
		return nil, fmt.Errorf("session not found or expired")
	}
	return s, nil
}

// incrementSetupAttempts increments the attempt counter for a setup session.
func incrementSetupAttempts(sessionID string) error {
	query := formatQuery(`UPDATE totp_setup_sessions SET attempt_count = attempt_count + 1 WHERE id = ?`)
	_, err := db.Exec(query, sessionID)
	return err
}

// deleteSetupSession deletes all setup sessions for a user.
func deleteSetupSession(userID int) error {
	query := formatQuery(`DELETE FROM totp_setup_sessions WHERE user_id = ?`)
	_, err := db.Exec(query, userID)
	return err
}

// enableTOTP activates 2FA for a user and stores the encrypted secret.
func enableTOTP(userID int, encryptedSecret string) error {
	query := formatQuery(`UPDATE users SET totp_enabled = true, totp_secret = ?, totp_verified_at = CURRENT_TIMESTAMP
		WHERE id = ?`)
	_, err := db.Exec(query, encryptedSecret, userID)
	return err
}

// disableTOTP clears all 2FA data for a user.
func disableTOTP(userID int) error {
	query := formatQuery(`UPDATE users SET totp_enabled = false, totp_secret = NULL, totp_verified_at = NULL
		WHERE id = ?`)
	_, err := db.Exec(query, userID)
	return err
}

// storeBackupCodes deletes old codes and inserts fresh hashed ones.
func storeBackupCodes(userID int, codes []string) error {
	// Delete old codes
	if err := deleteBackupCodes(userID); err != nil {
		return err
	}
	for _, code := range codes {
		h, err := hashBackupCode(code)
		if err != nil {
			return fmt.Errorf("hash backup code: %w", err)
		}
		id := uuid.New().String()
		query := formatQuery(`INSERT INTO totp_backup_codes (id, user_id, code_hash, created_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)`)
		if _, err := db.Exec(query, id, userID, h); err != nil {
			return fmt.Errorf("insert backup code: %w", err)
		}
	}
	return nil
}

// deleteBackupCodes removes all backup codes for a user.
func deleteBackupCodes(userID int) error {
	query := formatQuery(`DELETE FROM totp_backup_codes WHERE user_id = ?`)
	_, err := db.Exec(query, userID)
	return err
}

// countRemainingBackupCodes returns the number of unused backup codes.
func countRemainingBackupCodes(userID int) int {
	var count int
	query := formatQuery(`SELECT COUNT(*) FROM totp_backup_codes WHERE user_id = ? AND used_at IS NULL`)
	db.QueryRow(query, userID).Scan(&count)
	return count
}

// consumeBackupCode checks plaintext code against stored hashes; marks as used on match.
func consumeBackupCode(userID int, code string) bool {
	rows, err := db.Query(
		formatQuery(`SELECT id, code_hash FROM totp_backup_codes WHERE user_id = ? AND used_at IS NULL`),
		userID,
	)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			// Mark as used
			db.Exec(formatQuery(`UPDATE totp_backup_codes SET used_at = CURRENT_TIMESTAMP WHERE id = ?`), id)
			return true
		}
	}
	return false
}

// get2FAStatus returns the 2FA status for a user.
func get2FAStatus(userID int) (*TwoFAStatus, error) {
	var enabled bool
	var verifiedAt sql.NullTime
	query := formatQuery(`SELECT COALESCE(totp_enabled, false), totp_verified_at FROM users WHERE id = ?`)
	if err := db.QueryRow(query, userID).Scan(&enabled, &verifiedAt); err != nil {
		return nil, err
	}
	status := &TwoFAStatus{
		Enabled:              enabled,
		BackupCodesRemaining: 0,
	}
	if verifiedAt.Valid {
		t := verifiedAt.Time
		status.VerifiedAt = &t
	}
	if enabled {
		status.BackupCodesRemaining = countRemainingBackupCodes(userID)
	}
	return status, nil
}

// logAuthEvent records a 2FA audit event.
func logAuthEvent(eventType string, userID int, ip, userAgent string, success bool, reason string) {
	query := formatQuery(`INSERT INTO auth_events (type, user_id, ip_address, user_agent, success, error_reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	if _, err := db.Exec(query, eventType, userID, ip, userAgent, success, reason); err != nil {
		log.Printf("Warning: failed to log auth event %s: %v", eventType, err)
	}
}
