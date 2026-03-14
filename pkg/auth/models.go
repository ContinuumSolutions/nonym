package auth

import (
	"time"
)

// Organization represents an organization in the system
type Organization struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Slug        string    `json:"slug" db:"slug"`
	Industry    string    `json:"industry" db:"industry"`
	Size        string    `json:"size" db:"size"`
	Country     string    `json:"country" db:"country"`
	Description string    `json:"description" db:"description"`
	OwnerID     string    `json:"owner_id" db:"owner_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// User represents a user in the system
type User struct {
	ID             string     `json:"id" db:"id"`
	Email          string     `json:"email" db:"email"`
	Password       string     `json:"-" db:"password"` // Never include in JSON responses
	Name           string     `json:"name" db:"name"`
	Role           string     `json:"role" db:"role"`
	OrganizationID string     `json:"organization_id" db:"organization_id"`
	Active         bool       `json:"active" db:"active"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin      *time.Time `json:"last_login,omitempty" db:"last_login"`
	// Organization relationship (populated when needed)
	Organization *Organization `json:"organization,omitempty" db:"-"`
}

// UserSession represents an active user session
type UserSession struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	OrganizationID string    `json:"organization_id" db:"organization_id"`
	Token          string    `json:"-" db:"token"` // JWT token hash
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email        string `json:"email" validate:"required,email"`
	Password     string `json:"password" validate:"required,min=8"`
	Name         string `json:"name,omitempty"`
	FirstName    string `json:"firstName,omitempty"`
	LastName     string `json:"lastName,omitempty"`
	Organization string `json:"organization,omitempty"`
	// For existing organization registration
	OrganizationID *string `json:"organization_id,omitempty"`
	InviteCode     *string `json:"invite_code,omitempty"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email          string `json:"email" validate:"required,email"`
	Password       string `json:"password" validate:"required"`
	OrganizationID *string `json:"organization_id,omitempty"` // Optional organization selection
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token        string        `json:"token"`
	ExpiresAt    time.Time     `json:"expires_at"`
	User         *User         `json:"user"`
	Organization *Organization `json:"organization"`
}

// UserProfile represents a user profile (subset of User for responses)
type UserProfile struct {
	ID             string        `json:"id"`
	Email          string        `json:"email"`
	Name           string        `json:"name"`
	Role           string        `json:"role"`
	OrganizationID string        `json:"organization_id"`
	Active         bool          `json:"active"`
	CreatedAt      time.Time     `json:"created_at"`
	LastLogin      *time.Time    `json:"last_login,omitempty"`
	Organization   *Organization `json:"organization,omitempty"`
}

// OrganizationCreateRequest represents an organization creation request
type OrganizationCreateRequest struct {
	Name        string `json:"name" validate:"required"`
	Industry    string `json:"industry,omitempty"`
	Size        string `json:"size,omitempty"`
	Country     string `json:"country,omitempty"`
	Description string `json:"description,omitempty"`
}

// OrganizationUpdateRequest represents an organization update request
type OrganizationUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Size        string `json:"size,omitempty"`
	Country     string `json:"country,omitempty"`
	Description string `json:"description,omitempty"`
}

// TeamMember represents a team member with organization context
type TeamMember struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	JoinedAt   time.Time  `json:"joined_at"`
	LastActive *time.Time `json:"last_active"`
}

// TeamInviteRequest represents a team invitation request
type TeamInviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required"`
}
