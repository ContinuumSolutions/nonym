package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system with complete audit trail
type User struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	OrganizationID uuid.UUID  `json:"organization_id" db:"organization_id"`
	Email          string     `json:"email" db:"email"`
	PasswordHash   string     `json:"-" db:"password_hash"` // Never expose in JSON
	FirstName      string     `json:"first_name" db:"first_name"`
	LastName       string     `json:"last_name" db:"last_name"`
	Role           Role       `json:"role" db:"role"`
	IsActive       bool       `json:"is_active" db:"is_active"`
	EmailVerified  bool       `json:"email_verified" db:"email_verified"`
	LastLogin      *time.Time `json:"last_login,omitempty" db:"last_login"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`

	// Relationships (populated when needed)
	Organization *Organization `json:"organization,omitempty" db:"-"`
}

// Organization represents an organization with complete multi-tenant isolation
type Organization struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Description string    `json:"description" db:"description"`
	Settings    Settings  `json:"settings" db:"settings"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Role represents user roles with proper RBAC
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleUser     Role = "user"
	RoleViewer   Role = "viewer"
	RoleOwner    Role = "owner"
)

// IsValid validates if the role is a valid enum value
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser, RoleViewer, RoleOwner:
		return true
	default:
		return false
	}
}

// Settings represents organization-specific settings
type Settings struct {
	PasswordPolicy PasswordPolicy `json:"password_policy"`
	SessionTimeout int           `json:"session_timeout_minutes"`
	MFARequired    bool          `json:"mfa_required"`
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	MaxLength        int  `json:"max_length"`
	RequireUppercase bool `json:"require_uppercase"`
	RequireLowercase bool `json:"require_lowercase"`
	RequireNumbers   bool `json:"require_numbers"`
	RequireSymbols   bool `json:"require_symbols"`
	DisallowCommon   bool `json:"disallow_common"`
}

// DefaultPasswordPolicy returns secure default password policy
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:        12,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumbers:   true,
		RequireSymbols:   true,
		DisallowCommon:   true,
	}
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