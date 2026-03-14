package auth

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Organization represents an organization
type OrganizationData struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Industry    string                 `json:"industry"`
	Size        string                 `json:"size"`
	Country     string                 `json:"country"`
	Description string                 `json:"description"`
	Compliance  map[string]bool        `json:"compliance"`
	OwnerID     string                 `json:"owner_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// TeamMember represents a team member
type TeamMemberData struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	Status     string     `json:"status"`
	JoinedAt   time.Time  `json:"joined_at"`
	LastActive *time.Time `json:"last_active"`
}

// OrganizationUpdateRequest represents organization update request
type OrganizationUpdateRequestV1 struct {
	Name        string                 `json:"name"`
	Industry    string                 `json:"industry"`
	Size        string                 `json:"size"`
	Country     string                 `json:"country"`
	Description string                 `json:"description"`
	Compliance  map[string]bool        `json:"compliance"`
}

// TeamMemberInviteRequest represents team member invite request
type TeamMemberInviteRequestV1 struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Security settings structures
type TwoFactorUpdateRequestV1 struct {
	Enabled bool `json:"enabled"`
}

type SecuritySettingsRequestV1 struct {
	IPWhitelist      bool `json:"ipWhitelist"`
	RequireSignature bool `json:"requireSignature"`
	RateLimit        bool `json:"rateLimit"`
}

// UserSession represents a user session for display
type UserSessionData struct {
	ID         string    `json:"id"`
	Device     string    `json:"device"`
	Location   string    `json:"location"`
	Browser    string    `json:"browser"`
	LastActive time.Time `json:"last_active"`
	Current    bool      `json:"current"`
}

// HandleGetOrganizationV1 handles GET /api/v1/organization
func HandleGetOrganizationV1(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// For demo purposes, return a sample organization
	org := OrganizationData{
		ID:          "00000000-0000-0000-0000-000000000001",
		Name:        "Acme Corporation",
		Industry:    "technology",
		Size:        "51-200",
		Country:     "US",
		Description: "Leading technology company focused on AI and data privacy",
		Compliance: map[string]bool{
			"gdpr":  true,
			"hipaa": false,
			"ccpa":  true,
			"soc2":  true,
		},
		OwnerID:   user.ID,
		CreatedAt: time.Now().AddDate(0, -6, 0),
		UpdatedAt: time.Now(),
	}

	return c.JSON(fiber.Map{
		"organization": org,
	})
}

// HandleUpdateOrganizationV1 handles PUT /api/v1/organization
func HandleUpdateOrganizationV1(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req OrganizationUpdateRequestV1
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// In a real implementation, you'd update the database here
	// For now, just return success
	return c.JSON(fiber.Map{
		"message": "Organization updated successfully",
		"organization": OrganizationData{
			ID:          "00000000-0000-0000-0000-000000000001",
			Name:        req.Name,
			Industry:    req.Industry,
			Size:        req.Size,
			Country:     req.Country,
			Description: req.Description,
			Compliance:  req.Compliance,
			OwnerID:     user.ID,
			UpdatedAt:   time.Now(),
		},
	})
}

// HandleGetTeamMembersV1 handles GET /api/v1/team/members
func HandleGetTeamMembersV1(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Generate sample team members
	now := time.Now()
	lastHour := now.Add(-1 * time.Hour)
	yesterday := now.AddDate(0, 0, -1)
	members := []TeamMemberData{
		{
			ID:         "member_001",
			Name:       user.Name,
			Email:      user.Email,
			Role:       "owner",
			Status:     "active",
			JoinedAt:   now.AddDate(0, -3, 0),
			LastActive: &now,
		},
		{
			ID:         "member_002",
			Name:       "Jane Smith",
			Email:      "jane@acme.com",
			Role:       "admin",
			Status:     "active",
			JoinedAt:   now.AddDate(0, -1, 0),
			LastActive: &lastHour,
		},
		{
			ID:         "member_003",
			Name:       "Bob Johnson",
			Email:      "bob@acme.com",
			Role:       "member",
			Status:     "active",
			JoinedAt:   now.AddDate(0, 0, -7),
			LastActive: &yesterday,
		},
		{
			ID:         "member_004",
			Name:       "Alice Wilson",
			Email:      "alice@acme.com",
			Role:       "member",
			Status:     "pending",
			JoinedAt:   now.AddDate(0, 0, -2),
			LastActive: nil,
		},
	}

	return c.JSON(fiber.Map{
		"members": members,
	})
}

// HandleInviteTeamMemberV1 handles POST /api/v1/team/members
func HandleInviteTeamMemberV1(c *fiber.Ctx) error {
	_, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req TeamMemberInviteRequestV1
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Email == "" || req.Role == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Email and role are required",
		})
	}

	// Validate role
	validRoles := map[string]bool{
		"admin": true, "member": true, "viewer": true,
	}
	if !validRoles[req.Role] {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid role",
		})
	}

	// In a real implementation, you'd send an invitation email and store the invitation
	memberID := "member_" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// Extract name from email
	name := req.Email
	if atIndex := strings.Index(req.Email, "@"); atIndex > 0 {
		name = req.Email[:atIndex]
	}

	return c.Status(201).JSON(fiber.Map{
		"id":      memberID,
		"message": "Invitation sent successfully",
		"member": TeamMemberData{
			ID:       memberID,
			Name:     name,
			Email:    req.Email,
			Role:     req.Role,
			Status:   "pending",
			JoinedAt: time.Now(),
		},
	})
}

// HandleRemoveTeamMemberV1 handles DELETE /api/v1/team/members/:id
func HandleRemoveTeamMemberV1(c *fiber.Ctx) error {
	_, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	memberID := c.Params("id")
	if memberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Member ID is required",
		})
	}

	// Prevent removing yourself
	if memberID == "member_001" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot remove yourself from the organization",
		})
	}

	// In a real implementation, you'd remove the member from the organization
	return c.JSON(fiber.Map{
		"message": "Team member removed successfully",
	})
}

// HandleUpdateTwoFactorV1 handles PUT /api/v1/security/2fa
func HandleUpdateTwoFactorV1(c *fiber.Ctx) error {
	_, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req TwoFactorUpdateRequestV1
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// In a real implementation, you'd update the user's 2FA settings
	return c.JSON(fiber.Map{
		"message": "Two-factor authentication updated successfully",
		"enabled": req.Enabled,
	})
}

// HandleTerminateSessionV1 handles DELETE /api/v1/security/sessions/:id
func HandleTerminateSessionV1(c *fiber.Ctx) error {
	_, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	sessionID := c.Params("id")
	if sessionID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Session ID is required",
		})
	}

	// Prevent terminating current session
	if sessionID == "current_session" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot terminate current session",
		})
	}

	// In a real implementation, you'd terminate the specific session
	return c.JSON(fiber.Map{
		"message": "Session terminated successfully",
	})
}

// HandleUpdateSecuritySettingsV1 handles PUT /api/v1/security/settings
func HandleUpdateSecuritySettingsV1(c *fiber.Ctx) error {
	_, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	var req SecuritySettingsRequestV1
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// In a real implementation, you'd update the security settings in the database
	return c.JSON(fiber.Map{
		"message": "Security settings updated successfully",
		"settings": req,
	})
}
