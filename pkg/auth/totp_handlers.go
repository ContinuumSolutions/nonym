package auth

import (
	"github.com/gofiber/fiber/v2"
)

// noStore sets Cache-Control: no-store on responses.
func noStore(c *fiber.Ctx) {
	c.Set("Cache-Control", "no-store")
}

// ─── Setup: begin ─────────────────────────────────────────────────────────────

// HandleTOTPSetupBegin handles POST /api/v1/auth/2fa/setup/begin
func HandleTOTPSetupBegin(c *fiber.Ctx) error {
	noStore(c)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Password is required"})
	}
	if !verifyPassword(user.PasswordHash, req.Password) {
		return c.Status(401).JSON(fiber.Map{"error": "Incorrect password"})
	}

	if user.TOTPEnabled {
		return c.Status(409).JSON(fiber.Map{"error": "Two-factor authentication is already enabled"})
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate TOTP secret"})
	}
	encryptedSecret, err := encryptTOTPSecret(secret)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to encrypt TOTP secret"})
	}

	sessionID, expiresAt, err := createSetupSession(user.ID, encryptedSecret)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create setup session"})
	}

	return c.JSON(fiber.Map{
		"session_id":  sessionID,
		"secret":      secret,
		"otpauth_uri": buildOTPAuthURI(secret, user.Email),
		"expires_at":  expiresAt,
	})
}

// ─── Setup: verify ────────────────────────────────────────────────────────────

// HandleTOTPSetupVerify handles POST /api/v1/auth/2fa/setup/verify
func HandleTOTPSetupVerify(c *fiber.Ctx) error {
	noStore(c)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		SessionID string `json:"session_id"`
		TOTPCode  string `json:"totp_code"`
	}
	if err := c.BodyParser(&req); err != nil || req.SessionID == "" || req.TOTPCode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "session_id and totp_code are required"})
	}

	session, err := getSetupSession(req.SessionID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid or expired session"})
	}
	if session.UserID != user.ID {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid or expired session"})
	}
	if session.AttemptCount >= maxSetupAttempts {
		return c.Status(429).JSON(fiber.Map{"error": "Too many attempts; start a new setup session"})
	}

	secret, err := decryptTOTPSecret(session.Secret)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to process session"})
	}

	if !verifyTOTPCode(secret, req.TOTPCode) {
		_ = incrementSetupAttempts(req.SessionID)
		logAuthEvent("totp_setup_verify_failed", user.ID, c.IP(), c.Get("User-Agent"), false, "wrong code")
		return c.Status(422).JSON(fiber.Map{"error": "Invalid or expired TOTP code"})
	}

	// Promote secret to users table
	if err := enableTOTP(user.ID, session.Secret); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to enable 2FA"})
	}
	_ = deleteSetupSession(user.ID)

	// Generate and store backup codes
	plainCodes, err := generateBackupCodes(backupCodeCount)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate backup codes"})
	}
	if err := storeBackupCodes(user.ID, plainCodes); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to store backup codes"})
	}

	logAuthEvent("totp_enabled", user.ID, c.IP(), c.Get("User-Agent"), true, "")
	return c.JSON(fiber.Map{"backup_codes": plainCodes})
}

// ─── Disable ──────────────────────────────────────────────────────────────────

// HandleTOTPDisable handles DELETE /api/v1/auth/2fa
func HandleTOTPDisable(c *fiber.Ctx) error {
	noStore(c)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Password is required"})
	}
	if !verifyPassword(user.PasswordHash, req.Password) {
		return c.Status(401).JSON(fiber.Map{"error": "Incorrect password"})
	}

	if !user.TOTPEnabled {
		return c.Status(404).JSON(fiber.Map{"error": "Two-factor authentication is not enabled"})
	}

	if err := disableTOTP(user.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to disable 2FA"})
	}
	_ = deleteBackupCodes(user.ID)
	_ = deleteSetupSession(user.ID)

	logAuthEvent("totp_disabled", user.ID, c.IP(), c.Get("User-Agent"), true, "")
	return c.SendStatus(204)
}

// ─── Status ───────────────────────────────────────────────────────────────────

// HandleTOTPStatus handles GET /api/v1/auth/2fa/status
func HandleTOTPStatus(c *fiber.Ctx) error {
	noStore(c)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	status, err := get2FAStatus(user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch 2FA status"})
	}
	return c.JSON(status)
}

// ─── Regenerate backup codes ──────────────────────────────────────────────────

// HandleTOTPRegenerateBackupCodes handles POST /api/v1/auth/2fa/backup-codes/regenerate
func HandleTOTPRegenerateBackupCodes(c *fiber.Ctx) error {
	noStore(c)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Password is required"})
	}
	if !verifyPassword(user.PasswordHash, req.Password) {
		return c.Status(401).JSON(fiber.Map{"error": "Incorrect password"})
	}
	if !user.TOTPEnabled {
		return c.Status(404).JSON(fiber.Map{"error": "Two-factor authentication is not enabled"})
	}

	plainCodes, err := generateBackupCodes(backupCodeCount)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate backup codes"})
	}
	if err := storeBackupCodes(user.ID, plainCodes); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to store backup codes"})
	}

	logAuthEvent("totp_backup_codes_regenerated", user.ID, c.IP(), c.Get("User-Agent"), true, "")
	return c.JSON(fiber.Map{"backup_codes": plainCodes})
}

// ─── MFA challenge (Phase 2 login step) ──────────────────────────────────────

// HandleTOTPChallenge handles POST /api/v1/auth/2fa/challenge
// No auth middleware needed — uses the short-lived mfa_token.
func HandleTOTPChallenge(c *fiber.Ctx) error {
	noStore(c)

	var req struct {
		MFAToken string `json:"mfa_token"`
		TOTPCode string `json:"totp_code"`
	}
	if err := c.BodyParser(&req); err != nil || req.MFAToken == "" || req.TOTPCode == "" {
		return c.Status(400).JSON(fiber.Map{"error": "mfa_token and totp_code are required"})
	}

	userID, err := validateMFAToken(req.MFAToken)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired MFA token"})
	}

	user, err := getUserByID(userID)
	if err != nil || !user.TOTPEnabled || user.TOTPSecret == nil {
		return c.Status(401).JSON(fiber.Map{"error": "2FA not configured for this account"})
	}

	// Check backup code format first (xxxx-xxxx)
	var valid bool
	if isBackupCodeFormat(req.TOTPCode) {
		valid = consumeBackupCode(user.ID, req.TOTPCode)
	} else {
		secret, decErr := decryptTOTPSecret(*user.TOTPSecret)
		if decErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
		}
		valid = verifyTOTPCode(secret, req.TOTPCode)
	}

	if !valid {
		logAuthEvent("totp_challenge_failed", user.ID, c.IP(), c.Get("User-Agent"), false, "wrong code")
		return c.Status(422).JSON(fiber.Map{"error": "Invalid TOTP code or backup code"})
	}

	// Issue full session JWT
	org, err := getOrganizationByID(user.OrganizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load organization"})
	}
	user.Organization = org

	token, expiresAt, err := generateJWTToken(user)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	logAuthEvent("totp_challenge_success", user.ID, c.IP(), c.Get("User-Agent"), true, "")
	return c.JSON(fiber.Map{
		"token":        token,
		"expires_at":   expiresAt,
		"user":         user.ToProfile(),
		"organization": org,
	})
}
