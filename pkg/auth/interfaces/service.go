package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/ContinuumSolutions/nonym/pkg/auth/models"
)

// AuthService defines the business logic interface for authentication operations
type AuthService interface {
	// Authentication operations
	Register(ctx context.Context, req *models.RegisterRequest) (*models.RegisterResponse, error)
	Login(ctx context.Context, req *models.LoginRequest) (*models.LoginResponse, error)
	Logout(ctx context.Context, userID uuid.UUID, sessionToken string) error
	RefreshToken(ctx context.Context, req *models.RefreshTokenRequest) (*models.TokenPair, error)
	ValidateToken(ctx context.Context, token string) (*models.User, error)

	// User management
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error)
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, req *models.UpdateUserRequest) (*models.UserProfile, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req *models.ChangePasswordRequest) error
	DeleteUser(ctx context.Context, userID uuid.UUID) error

	// Password reset
	RequestPasswordReset(ctx context.Context, req *models.ResetPasswordRequest) error
	ConfirmPasswordReset(ctx context.Context, req *models.ConfirmResetPasswordRequest) error

	// Organization management
	CreateOrganization(ctx context.Context, req *models.CreateOrganizationRequest) (*models.Organization, error)
	GetOrganization(ctx context.Context, orgID uuid.UUID) (*models.Organization, error)
	UpdateOrganization(ctx context.Context, orgID uuid.UUID, req *models.UpdateOrganizationRequest) (*models.Organization, error)
	DeleteOrganization(ctx context.Context, orgID uuid.UUID) error
	GetOrganizationMembers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.OrganizationMember, int, error)
	GetOrganizationStats(ctx context.Context, orgID uuid.UUID) (*models.OrganizationStats, error)

	// Admin operations
	ListUsers(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*models.UserProfile, int, error)
	CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.UserProfile, error)
	UpdateUserRole(ctx context.Context, userID uuid.UUID, role models.Role) error
	SetUserActive(ctx context.Context, userID uuid.UUID, active bool) error

	// Audit and security
	GetAuthEvents(ctx context.Context, filter *AuthEventFilter) ([]*models.AuthEventResponse, int, error)
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*UserSession, error)

	// Health check
	HealthCheck(ctx context.Context) (*models.HealthResponse, error)
}

// PasswordHasher defines password hashing operations
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) error
	NeedsRehash(hash string) bool
}

// TokenGenerator defines JWT token operations
type TokenGenerator interface {
	GenerateAccessToken(user *models.User) (string, error)
	GenerateRefreshToken(user *models.User) (string, error)
	ValidateAccessToken(token string) (*TokenClaims, error)
	ValidateRefreshToken(token string) (*TokenClaims, error)
	RevokeToken(token string) error
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID         uuid.UUID   `json:"user_id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	Email          string      `json:"email"`
	Role           models.Role `json:"role"`
	SessionID      uuid.UUID   `json:"session_id"`
	IssuedAt       int64       `json:"iat"`
	ExpiresAt      int64       `json:"exp"`
	TokenType      string      `json:"token_type"` // "access" or "refresh"
}

// Validator defines input validation operations
type Validator interface {
	ValidateRegisterRequest(req *models.RegisterRequest) error
	ValidateLoginRequest(req *models.LoginRequest) error
	ValidateUpdateUserRequest(req *models.UpdateUserRequest) error
	ValidateCreateOrganizationRequest(req *models.CreateOrganizationRequest) error
	ValidateUpdateOrganizationRequest(req *models.UpdateOrganizationRequest) error
	ValidatePasswordStrength(password string, policy *models.PasswordPolicy) error
	ValidateEmail(email string) error
	ValidateRole(role models.Role) error
}

// AuditLogger defines audit logging operations
type AuditLogger interface {
	LogAuthEvent(ctx context.Context, event *AuthEvent) error
	LogSecurityEvent(ctx context.Context, userID *uuid.UUID, orgID *uuid.UUID, eventType string, details map[string]interface{}) error
}

// RateLimiter defines rate limiting operations
type RateLimiter interface {
	AllowLogin(ctx context.Context, identifier string) (bool, error)
	AllowRegistration(ctx context.Context, identifier string) (bool, error)
	AllowPasswordReset(ctx context.Context, identifier string) (bool, error)
	RecordFailedLogin(ctx context.Context, identifier string) error
	ClearFailedLogins(ctx context.Context, identifier string) error
}

// EmailService defines email operations for notifications
type EmailService interface {
	SendWelcomeEmail(ctx context.Context, user *models.User, org *models.Organization) error
	SendPasswordResetEmail(ctx context.Context, user *models.User, resetToken string) error
	SendEmailVerificationEmail(ctx context.Context, user *models.User, verificationToken string) error
	SendOrganizationInviteEmail(ctx context.Context, invite *InviteRequest) error
}

// InviteRequest represents organization invitation
type InviteRequest struct {
	OrganizationID uuid.UUID   `json:"organization_id"`
	Email          string      `json:"email"`
	Role           models.Role `json:"role"`
	InviterName    string      `json:"inviter_name"`
	InviteCode     string      `json:"invite_code"`
	ExpiresAt      int64       `json:"expires_at"`
	Message        string      `json:"message,omitempty"`
}