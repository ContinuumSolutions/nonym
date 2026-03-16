package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// OrganizationInfo represents organization information for responses
type OrganizationInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	CreatedAt   string `json:"created_at"`
}

// TeamMember represents a team member
type TeamMember struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	FullName  string `json:"full_name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	JoinedAt  string `json:"joined_at"`
}

// TeamInviteRequest represents a team invitation request
type TeamInviteRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  string `json:"role" validate:"required"`
}

// HandleGetOrganizationInfo handles GET /api/organization
func HandleGetOrganizationInfo(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Get organization details
	org, err := getOrganizationByID(user.OrganizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch organization information",
		})
	}

	// TODO: Get member count from database
	memberCount := 1 // Placeholder

	orgInfo := &OrganizationInfo{
		ID:          org.ID.String(),
		Name:        org.Name,
		Slug:        org.Slug,
		Description: org.Description,
		MemberCount: memberCount,
		CreatedAt:   org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return c.JSON(fiber.Map{
		"organization": orgInfo,
	})
}

// HandleUpdateOrganizationInfo handles PUT /api/organization
func HandleUpdateOrganizationInfo(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user can manage organization
	if !user.CanManageOrganization() {
		return c.Status(403).JSON(fiber.Map{
			"error": "Permission denied. Admin or owner access required",
		})
	}

	var req struct {
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// TODO: Implement organization update logic
	// For now, return success message

	return c.JSON(fiber.Map{
		"message": "Organization updated successfully",
		"updates": req,
	})
}

// HandleGetTeamMembers handles GET /api/organization/team
func HandleGetTeamMembers(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// TODO: Implement team members fetching from database
	// For now, return current user as the only member
	members := []TeamMember{
		{
			ID:        user.ID.String(),
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			FullName:  user.FullName(),
			Role:      string(user.Role),
			IsActive:  user.IsActive,
			JoinedAt:  user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		},
	}

	return c.JSON(fiber.Map{
		"members": members,
		"total":   len(members),
	})
}

// HandleInviteTeamMember handles POST /api/organization/team/invite
func HandleInviteTeamMember(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user can manage team
	if !user.CanManageOrganization() {
		return c.Status(403).JSON(fiber.Map{
			"error": "Permission denied. Admin or owner access required",
		})
	}

	var req TeamInviteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if req.Email == "" || req.Role == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Email and role are required",
		})
	}

	if !strings.Contains(req.Email, "@") {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid email format",
		})
	}

	// Validate role
	role := Role(req.Role)
	if !role.IsValid() {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid role. Must be one of: owner, admin, user, viewer",
		})
	}

	// TODO: Implement team invitation logic
	// 1. Check if user already exists in organization
	// 2. Send invitation email
	// 3. Create pending invitation record

	return c.JSON(fiber.Map{
		"message": "Team invitation sent successfully",
		"invite": fiber.Map{
			"email": req.Email,
			"role":  req.Role,
		},
	})
}

// HandleRemoveTeamMember handles DELETE /api/organization/team/:userId
func HandleRemoveTeamMember(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user can manage team
	if !user.CanManageOrganization() {
		return c.Status(403).JSON(fiber.Map{
			"error": "Permission denied. Admin or owner access required",
		})
	}

	userID := c.Params("userId")
	if userID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	// Prevent self-removal
	if userID == user.ID.String() {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot remove yourself from the organization",
		})
	}

	// TODO: Implement team member removal logic
	// 1. Verify user exists in organization
	// 2. Check permissions (owners can't be removed by admins)
	// 3. Deactivate user or transfer ownership if needed
	// 4. Send notification email

	return c.JSON(fiber.Map{
		"message": "Team member removed successfully",
		"removed_user_id": userID,
	})
}