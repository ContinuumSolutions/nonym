package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sovereignprivacy/gateway/pkg/auth/models"
)

// AuthRepository defines the data layer interface for authentication operations
type AuthRepository interface {
	// User operations
	CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, updates *models.UpdateUserRequest) (*models.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error)
	SetUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	SetEmailVerified(ctx context.Context, userID uuid.UUID, verified bool) error

	// Organization operations
	CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error)
	GetOrganizationByID(ctx context.Context, id uuid.UUID) (*models.Organization, error)
	GetOrganizationBySlug(ctx context.Context, slug string) (*models.Organization, error)
	UpdateOrganization(ctx context.Context, id uuid.UUID, updates *models.UpdateOrganizationRequest) (*models.Organization, error)
	DeleteOrganization(ctx context.Context, id uuid.UUID) error
	ListOrganizations(ctx context.Context, limit, offset int) ([]*models.Organization, int, error)
	GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.User, int, error)
	GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error)

	// Session operations
	CreateSession(ctx context.Context, session *UserSession) error
	GetSession(ctx context.Context, sessionID uuid.UUID) (*UserSession, error)
	DeleteSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredSessions(ctx context.Context) (int, error)
	UpdateSessionActivity(ctx context.Context, sessionID uuid.UUID) error

	// Token operations
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	DeleteUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
	DeleteExpiredRefreshTokens(ctx context.Context) (int, error)

	// Audit operations
	LogAuthEvent(ctx context.Context, event *AuthEvent) error
	GetAuthEvents(ctx context.Context, filter *AuthEventFilter) ([]*AuthEvent, int, error)

	// Password reset operations
	CreatePasswordReset(ctx context.Context, reset *PasswordReset) error
	GetPasswordReset(ctx context.Context, token string) (*PasswordReset, error)
	DeletePasswordReset(ctx context.Context, token string) error
	DeleteExpiredPasswordResets(ctx context.Context) (int, error)

	// Transaction support
	WithTx(ctx context.Context, fn func(AuthRepository) error) error

	// Health and migration support
	Ping(ctx context.Context) error
	Close() error
}

// UserSession represents an active user session
type UserSession struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	SessionToken   string    `json:"session_token" db:"session_token"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at" db:"last_accessed_at"`
	IsActive       bool      `json:"is_active" db:"is_active"`
}

// RefreshToken represents a refresh token for JWT renewal
type RefreshToken struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	TokenHash      string    `json:"token_hash" db:"token_hash"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at" db:"last_used_at"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	IsRevoked      bool      `json:"is_revoked" db:"is_revoked"`
}

// AuthEvent represents an authentication event for audit logging
type AuthEvent struct {
	ID             uuid.UUID `json:"id" db:"id"`
	Type           string    `json:"type" db:"type"`
	UserID         *uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id" db:"organization_id"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	Success        bool      `json:"success" db:"success"`
	ErrorReason    string    `json:"error_reason" db:"error_reason"`
	Metadata       string    `json:"metadata" db:"metadata"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// AuthEventFilter for querying auth events
type AuthEventFilter struct {
	UserID         *uuid.UUID
	OrganizationID *uuid.UUID
	Type           string
	Success        *bool
	IPAddress      string
	StartDate      *time.Time
	EndDate        *time.Time
	Limit          int
	Offset         int
}

// PasswordReset represents a password reset request
type PasswordReset struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Token     string    `json:"token" db:"token"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UsedAt    *time.Time `json:"used_at" db:"used_at"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
}

// Auth event types
const (
	AuthEventLogin              = "login"
	AuthEventLoginFailed        = "login_failed"
	AuthEventLogout             = "logout"
	AuthEventRegister           = "register"
	AuthEventPasswordChange     = "password_change"
	AuthEventPasswordReset      = "password_reset"
	AuthEventTokenRefresh       = "token_refresh"
	AuthEventTokenRevoke        = "token_revoke"
	AuthEventAccountLocked      = "account_locked"
	AuthEventAccountUnlocked    = "account_unlocked"
	AuthEventRoleChanged        = "role_changed"
	AuthEventEmailChanged       = "email_changed"
	AuthEventEmailVerified      = "email_verified"
	AuthEventSuspiciousActivity = "suspicious_activity"
)