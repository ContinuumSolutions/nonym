# Two-Factor Authentication — Backend Specification

## Overview

TOTP-based 2FA for individual user accounts. Uses RFC 6238 (time-based one-time passwords), compatible with Google Authenticator, Authy, 1Password, etc.

**Login is not gated by 2FA yet.** The frontend mockup is in Settings → Security. Once the backend is ready, login should be updated to challenge users who have 2FA enabled.

---

## Database Changes

### `users` table — add columns

| Column | Type | Default | Notes |
|---|---|---|---|
| `totp_enabled` | `boolean` | `false` | Whether 2FA is active for this user |
| `totp_secret` | `text` (encrypted) | `null` | Base32-encoded TOTP secret. Encrypt at rest. |
| `totp_verified_at` | `timestamptz` | `null` | When the user first verified their TOTP setup |

### New table: `totp_backup_codes`

```sql
CREATE TABLE totp_backup_codes (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash   text NOT NULL,      -- bcrypt hash of the code
  used_at     timestamptz,        -- null = still valid
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON totp_backup_codes (user_id);
```

Each user gets 8 backup codes. Codes are stored hashed (bcrypt, cost 10). A code is invalidated after first use.

### New table: `totp_setup_sessions`

Temporary table to hold unverified setup state (cleaned up after 10 minutes or on completion).

```sql
CREATE TABLE totp_setup_sessions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  secret      text NOT NULL,   -- encrypted Base32 secret, not yet committed
  expires_at  timestamptz NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

---

## API Endpoints

All endpoints require `Authorization: Bearer <jwt>`.

---

### POST `/api/v1/auth/2fa/setup/begin`

Start the 2FA setup flow. Verifies the user's password, generates a TOTP secret, saves it to `totp_setup_sessions`, and returns the secret + QR code URI.

**Request:**
```json
{ "password": "current-password" }
```

**Response `200`:**
```json
{
  "session_id": "uuid",
  "secret": "JBSWY3DPEHPK3PXP",
  "otpauth_uri": "otpauth://totp/SPG:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=SovereignPrivacyGateway&algorithm=SHA1&digits=6&period=30",
  "expires_at": "2026-03-16T12:10:00Z"
}
```

The frontend uses `otpauth_uri` to generate the QR code (via a library like `qrcode`). The `secret` is shown as the manual entry key.

**Errors:**
- `400` — missing password
- `401` — wrong password
- `409` — 2FA is already enabled for this user

**Notes:**
- If a setup session for this user already exists, delete and replace it.
- Secret must be generated with a cryptographically secure random source (e.g. `crypto/rand`), encoded as Base32.
- Recommended: 20-byte (160-bit) secret.

---

### POST `/api/v1/auth/2fa/setup/verify`

Confirm the user has successfully configured their authenticator by submitting a valid TOTP code. On success: promotes the pending secret to the `users` table, generates 8 backup codes, deletes the setup session.

**Request:**
```json
{
  "session_id": "uuid",
  "totp_code": "123456"
}
```

**Response `200`:**
```json
{
  "backup_codes": [
    "a3bk-p7mw",
    "r9cx-t4nh",
    "q6jv-s2fy",
    "w8nm-h5j3",
    "k4ts-b7rp",
    "m2wq-f8cx",
    "n7hv-k3sq",
    "j5pb-r6tn"
  ]
}
```

Backup codes are returned **once, in plaintext**, immediately after setup. They are never returned again. The frontend must prompt the user to save them.

**Errors:**
- `400` — invalid or expired `session_id`
- `422` — `totp_code` wrong or expired (the 30-second window)
- `429` — rate limited (max 5 attempts per session)

**Notes:**
- Accept ±1 step (30 s) clock drift tolerance.
- After success, store `totp_secret` (encrypted) in `users.totp_secret`, set `totp_enabled = true`, `totp_verified_at = now()`.
- Generate backup codes: 8 random alphanumeric strings, format `xxxx-xxxx` (lowercase, ambiguous chars removed), bcrypt-hash and store in `totp_backup_codes`.
- Return the plaintext codes in this response only.

---

### DELETE `/api/v1/auth/2fa`

Disable 2FA for the current user. Requires password confirmation.

**Request:**
```json
{ "password": "current-password" }
```

**Response `204`** (no body)

**Errors:**
- `401` — wrong password
- `404` — 2FA is not enabled for this user

**Notes:**
- Clear `users.totp_secret`, `users.totp_enabled`, `users.totp_verified_at`.
- Delete all rows in `totp_backup_codes` for this user.
- Delete any pending `totp_setup_sessions` for this user.

---

### GET `/api/v1/auth/2fa/status`

Returns the current 2FA state for the authenticated user. Used by the Settings page on load to show the correct initial toggle state.

**Response `200`:**
```json
{
  "enabled": true,
  "verified_at": "2026-03-10T09:14:22Z",
  "backup_codes_remaining": 6
}
```

---

### POST `/api/v1/auth/2fa/backup-codes/regenerate`

Regenerates backup codes (e.g. after the user has used most of them). Requires password confirmation.

**Request:**
```json
{ "password": "current-password" }
```

**Response `200`:**
```json
{
  "backup_codes": [ "...", "..." ]
}
```

---

## Login Flow Changes (phase 2)

> **Not implemented in the frontend yet.** Document here so the backend can be built ahead of the UI update.

### Step 1 — Credentials

`POST /api/v1/auth/login` behaviour changes when a user has `totp_enabled = true`:

Instead of returning a full JWT, return a short-lived **MFA challenge token**:

```json
{
  "mfa_required": true,
  "mfa_token": "<signed-jwt-with-15min-expiry>",
  "mfa_token_expires_at": "2026-03-16T12:15:00Z"
}
```

The `mfa_token` encodes `{ user_id, purpose: "mfa_challenge" }` and is only valid for step 2.

### Step 2 — TOTP challenge

**POST `/api/v1/auth/2fa/challenge`**

```json
{
  "mfa_token": "<from step 1>",
  "totp_code": "123456"       // TOTP code OR a backup code
}
```

**Response `200`** (same as current login response):
```json
{
  "token": "<full session jwt>",
  "expires_at": "...",
  "user": { ... },
  "organization": { ... }
}
```

**Errors:**
- `401` — invalid/expired `mfa_token`
- `422` — wrong TOTP code (increment attempt counter)
- `429` — too many attempts (5 max); account temporarily locked

**Backup code logic:** If `totp_code` is 9 characters and matches the `xxxx-xxxx` format, attempt to validate against `totp_backup_codes` (bcrypt comparison). On match, mark that code row with `used_at = now()`.

---

## Security Requirements

- TOTP secret must be **encrypted at rest** (AES-256-GCM or equivalent) using a server-side key from env/secrets manager. Never store or log plaintext secrets.
- Backup code hashes use **bcrypt** (cost ≥ 10). Never store or return plaintext codes after the initial setup response.
- Rate limiting on all 2FA endpoints: 5 attempts per IP per 15 minutes; additionally 5 attempts per user per setup session.
- All 2FA API responses must include `Cache-Control: no-store`.
- `totp_setup_sessions` rows must be cleaned up by a background job or TTL index (expire after 10 minutes).
- Audit log every 2FA enable, disable, successful challenge, and failed challenge with `user_id`, `ip`, `user_agent`, `timestamp`.

---

## TOTP Library Recommendations

| Language | Library |
|---|---|
| Go | `github.com/pquerna/otp` |
| Node/TS | `otpauth` or `speakeasy` |
| Python | `pyotp` |

Use **SHA-1** (RFC 6238 default), 6 digits, 30-second period — this matches all major authenticator apps.

---

## Frontend Integration Checklist (when backend is ready)

- [ ] `GET /api/v1/auth/2fa/status` on Settings mount → set `twoFactorEnabled` initial state
- [ ] `POST /api/v1/auth/2fa/setup/begin` in `confirmTfaPassword()` → use real `otpauth_uri` to render QR via `qrcode` library
- [ ] `POST /api/v1/auth/2fa/setup/verify` in `confirmTfaCode()` → receive real backup codes
- [ ] `DELETE /api/v1/auth/2fa` in `disableTfa()`
- [ ] Update login view: detect `mfa_required: true` in login response → show TOTP input step
- [ ] `POST /api/v1/auth/2fa/challenge` from login TOTP step
