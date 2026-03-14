package models

import (
	"time"

	"github.com/google/uuid"
)

// RegisterRequest represents a user registration request with validation
type RegisterRequest struct {
	Email        string `json:"email" validate:"required,email,max=255"`
	Password     string `json:"password" validate:"required,min=12,max=128"`
	FirstName    string `json:"first_name" validate:"required,min=1,max=100"`
	LastName     string `json:"last_name" validate:"required,min=1,max=100"`
	Organization string `json:"organization" validate:"required,min=2,max=255"`

	// Optional fields for joining existing organization
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	InviteCode     *string    `json:"invite_code,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email          string     `json:"email" validate:"required,email"`
	Password       string     `json:"password" validate:"required"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	RememberMe     bool       `json:"remember_me"`
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// CreateUserRequest represents internal user creation (admin action)
type CreateUserRequest struct {
	ID             uuid.UUID `json:"id,omitempty"`
	OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
	Email          string    `json:"email" validate:"required,email,max=255"`
	PasswordHash   string    `json:"password_hash" validate:"required"`
	FirstName      string    `json:"first_name" validate:"required,min=1,max=100"`
	LastName       string    `json:"last_name" validate:"required,min=1,max=100"`
	Role           Role      `json:"role" validate:"required"`
	IsActive       bool      `json:"is_active"`
	EmailVerified  bool      `json:"email_verified"`
}

// UpdateUserRequest represents user update request
type UpdateUserRequest struct {
	FirstName *string `json:"first_name,omitempty" validate:"omitempty,min=1,max=100"`
	LastName  *string `json:"last_name,omitempty" validate:"omitempty,min=1,max=100"`
	Role      *Role   `json:"role,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
}

// CreateOrganizationRequest represents organization creation request
type CreateOrganizationRequest struct {
	Name        string   `json:"name" validate:"required,min=2,max=255"`
	Description string   `json:"description" validate:"max=1000"`
	Settings    Settings `json:"settings"`
}

// UpdateOrganizationRequest represents organization update request
type UpdateOrganizationRequest struct {
	Name        *string   `json:"name,omitempty" validate:"omitempty,min=2,max=255"`
	Description *string   `json:"description,omitempty" validate:"omitempty,max=1000"`
	Settings    *Settings `json:"settings,omitempty"`
	IsActive    *bool     `json:"is_active,omitempty"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=12,max=128"`
}

// ResetPasswordRequest represents password reset request
type ResetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ConfirmResetPasswordRequest represents password reset confirmation
type ConfirmResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=12,max=128"`
}

// ValidateInviteRequest represents invite validation
type ValidateInviteRequest struct {
	InviteCode     string    `json:"invite_code" validate:"required"`
	OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
}

// CreateInviteRequest represents invite creation
type CreateInviteRequest struct {
	OrganizationID uuid.UUID `json:"organization_id" validate:"required"`
	Email          string    `json:"email" validate:"required,email"`
	Role           Role      `json:"role" validate:"required"`
	ExpiresAt      time.Time `json:"expires_at" validate:"required"`
	Message        string    `json:"message,omitempty" validate:"max=500"`
}