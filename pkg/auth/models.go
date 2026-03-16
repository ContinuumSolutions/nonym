package auth

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	Email          string     `json:"email" db:"email"`
	PasswordHash   string     `json:"-" db:"password_hash"`
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	Role           Role       `json:"role" db:"role"`
	OrganizationID uuid.UUID  `json:"organization_id" db:"organization_id"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	EmailVerified  bool       `json:"email_verified" db:"email_verified"`
	LastLogin      *time.Time `json:"last_login,omitempty" db:"last_login"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`

	// Relationship
	Organization *Organization `json:"organization,omitempty" db:"-"`
}

// Organization represents an organization in the system
type Organization struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description" db:"description"`
	OwnerID     uuid.UUID `json:"owner_id" db:"owner_id"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// UserSession represents an active user session
type UserSession struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	Token          string    `json:"-" db:"token"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
}

// Role represents user roles
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleViewer Role = "viewer"
)

// IsValid validates if the role is valid
func (r Role) IsValid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleUser, RoleViewer:
		return true
	default:
		return false
	}
}

// Request and Response types

// SignupRequest represents a signup request
type SignupRequest struct {
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	FirstName    string `json:"first_name" validate:"required"`
	LastName     string `json:"last_name" validate:"required"`
	Organization string `json:"organization,omitempty"`
	// For joining existing organization
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	InviteCode     *string    `json:"invite_code,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email          string     `json:"email" validate:"required,email"`
	Password       string     `json:"password" validate:"required"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token        string        `json:"token"`
	ExpiresAt    time.Time     `json:"expires_at"`
	User         *UserProfile  `json:"user"`
	Organization *Organization `json:"organization"`
}

// UserProfile represents a user profile (subset of User for responses)
type UserProfile struct {
	ID             uuid.UUID     `json:"id"`
	Email          string        `json:"email"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	FullName       string        `json:"full_name"`
	Role           Role          `json:"role"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	IsActive       bool          `json:"is_active"`
	EmailVerified  bool          `json:"email_verified"`
	CreatedAt      time.Time     `json:"created_at"`
	LastLogin      *time.Time    `json:"last_login,omitempty"`
	Organization   *Organization `json:"organization,omitempty"`
}

// FullName returns the user's full name
func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return u.Email
	}
	if u.LastName == "" {
		return u.FirstName
	}
	if u.FirstName == "" {
		return u.LastName
	}
	return u.FirstName + " " + u.LastName
}

// ToProfile converts a User to UserProfile
func (u *User) ToProfile() *UserProfile {
	return &UserProfile{
		ID:             u.ID,
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		FullName:       u.FullName(),
		Role:           u.Role,
		OrganizationID: u.OrganizationID,
		IsActive:       u.IsActive,
		EmailVerified:  u.EmailVerified,
		CreatedAt:      u.CreatedAt,
		LastLogin:      u.LastLogin,
		Organization:   u.Organization,
	}
}

// HasRole checks if user has the specified role
func (u *User) HasRole(role Role) bool {
	return u.Role == role
}

// IsAdmin checks if user is an admin or owner
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleOwner
}

// CanManageOrganization checks if user can manage organization settings
func (u *User) CanManageOrganization() bool {
	return u.Role == RoleOwner || u.Role == RoleAdmin
}