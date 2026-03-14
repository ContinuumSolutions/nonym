package models

import (
	"time"

	"github.com/google/uuid"
)

// RegisterResponse represents successful registration response
type RegisterResponse struct {
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
	Token        *TokenPair    `json:"tokens"`
	Message      string        `json:"message"`
}

// LoginResponse represents successful login response
type LoginResponse struct {
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
	Token        *TokenPair    `json:"tokens"`
	ExpiresAt    time.Time     `json:"expires_at"`
	Message      string        `json:"message"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	TokenType             string    `json:"token_type"` // "Bearer"
}

// UserProfile represents user profile response (safe for external consumption)
type UserProfile struct {
	ID             uuid.UUID     `json:"id"`
	Email          string        `json:"email"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	FullName       string        `json:"full_name"`
	Role           Role          `json:"role"`
	IsActive       bool          `json:"is_active"`
	EmailVerified  bool          `json:"email_verified"`
	LastLogin      *time.Time    `json:"last_login,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	Organization   *Organization `json:"organization,omitempty"`
}

// OrganizationMember represents a member in organization context
type OrganizationMember struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	FullName      string     `json:"full_name"`
	Role          Role       `json:"role"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	JoinedAt      time.Time  `json:"joined_at"`
}

// AuthEventResponse represents authentication event for audit logs
type AuthEventResponse struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	UserID      uuid.UUID `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	OrgID       uuid.UUID `json:"org_id"`
	OrgName     string    `json:"org_name"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Success     bool      `json:"success"`
	ErrorReason string    `json:"error_reason,omitempty"`
	Metadata    string    `json:"metadata,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// OrganizationStats represents organization statistics
type OrganizationStats struct {
	TotalUsers    int `json:"total_users"`
	ActiveUsers   int `json:"active_users"`
	AdminUsers    int `json:"admin_users"`
	PendingUsers  int `json:"pending_users"`
	LoginEvents   int `json:"recent_login_events"`
	FailedLogins  int `json:"recent_failed_logins"`
}

// ValidationErrorResponse represents validation error details
type ValidationErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Value   any    `json:"value,omitempty"`
}

// ErrorResponse represents standard error response
type ErrorResponse struct {
	Error      string                      `json:"error"`
	Code       string                      `json:"code"`
	Message    string                      `json:"message"`
	Details    string                      `json:"details,omitempty"`
	Validation []*ValidationErrorResponse  `json:"validation,omitempty"`
	Timestamp  time.Time                   `json:"timestamp"`
	TraceID    string                      `json:"trace_id,omitempty"`
}

// HealthResponse represents system health check
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
	Uptime    string            `json:"uptime"`
}

// NewUserProfile creates a UserProfile from User model
func NewUserProfile(user *User) *UserProfile {
	profile := &UserProfile{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		FullName:      user.FullName(),
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		LastLogin:     user.LastLogin,
		CreatedAt:     user.CreatedAt,
	}

	if user.Organization != nil {
		profile.Organization = user.Organization
	}

	return profile
}

// NewOrganizationMember creates OrganizationMember from User model
func NewOrganizationMember(user *User) *OrganizationMember {
	return &OrganizationMember{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		FullName:      user.FullName(),
		Role:          user.Role,
		IsActive:      user.IsActive,
		EmailVerified: user.EmailVerified,
		LastLogin:     user.LastLogin,
		JoinedAt:      user.CreatedAt,
	}
}