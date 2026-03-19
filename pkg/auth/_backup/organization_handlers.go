package auth

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleGetOrganizationInfo handles GET /api/v1/organization
func HandleGetOrganizationInfo(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Get fresh organization data
	organization, err := GetOrganization(user.OrganizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch organization",
		})
	}

	return c.JSON(fiber.Map{
		"organization": organization,
	})
}

// HandleUpdateOrganizationInfo handles PUT /api/v1/organization
func HandleUpdateOrganizationInfo(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user has permission to update organization (admin or owner)
	if user.Role != "admin" && user.Role != "owner" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Insufficient permissions to update organization",
		})
	}

	var req OrganizationUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	organization, err := UpdateOrganization(user.OrganizationID, &req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":      "Organization updated successfully",
		"organization": organization,
	})
}

// HandleGetTeamMembers handles GET /api/v1/team/members
func HandleGetTeamMembers(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	members, err := GetOrganizationMembers(strconv.Itoa(user.OrganizationID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch team members",
		})
	}

	return c.JSON(fiber.Map{
		"members": members,
		"total":   len(members),
	})
}

// HandleInviteTeamMember handles POST /api/v1/team/members
func HandleInviteTeamMember(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user has permission to invite (admin or owner)
	if user.Role != "admin" && user.Role != "owner" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Insufficient permissions to invite team members",
		})
	}

	var req TeamInviteRequest
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
		"admin": true, "user": true, "viewer": true,
	}
	if !validRoles[req.Role] {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid role. Valid roles are: admin, user, viewer",
		})
	}

	// In a real implementation, you'd send an invitation email
	// For now, we'll create a placeholder response
	memberID := "invite_" + strconv.FormatInt(time.Now().UnixNano(), 10)

	return c.Status(201).JSON(fiber.Map{
		"message": "Team member invitation sent successfully",
		"invite": fiber.Map{
			"id":     memberID,
			"email":  req.Email,
			"role":   req.Role,
			"status": "pending",
		},
	})
}

// HandleRemoveTeamMember handles DELETE /api/v1/team/members/:id
func HandleRemoveTeamMember(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Check if user has permission to remove members (admin or owner)
	if user.Role != "admin" && user.Role != "owner" {
		return c.Status(403).JSON(fiber.Map{
			"error": "Insufficient permissions to remove team members",
		})
	}

	memberID := c.Params("id")
	if memberID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Member ID is required",
		})
	}

	// Prevent removing yourself
	if memberID == strconv.Itoa(user.ID) {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot remove yourself from the organization",
		})
	}

	err := RemoveOrganizationMember(memberID, strconv.Itoa(user.OrganizationID))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Team member removed successfully",
	})
}

// Organization member management functions

// GetOrganizationMembers retrieves all members of an organization
func GetOrganizationMembers(organizationID string) ([]TeamMember, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, email, name, role, active, created_at, last_login
			  FROM users WHERE organization_id = ? AND active = true
			  ORDER BY created_at ASC`

	rows, err := db.Query(query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query organization members: %w", err)
	}
	defer rows.Close()

	var members []TeamMember
	for rows.Next() {
		var member TeamMember
		var lastLoginNull sql.NullTime

		err := rows.Scan(&member.ID, &member.Email, &member.Name, &member.Role,
			&member.Status, &member.JoinedAt, &lastLoginNull)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}

		// Convert status from boolean to string
		if member.Status == "1" || member.Status == "true" {
			member.Status = "active"
		} else {
			member.Status = "inactive"
		}

		if lastLoginNull.Valid {
			member.LastActive = &lastLoginNull.Time
		}

		members = append(members, member)
	}

	return members, nil
}

// RemoveOrganizationMember removes a member from an organization
func RemoveOrganizationMember(memberID string, organizationID string) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Convert memberID to int
	id, err := strconv.Atoi(memberID)
	if err != nil {
		return fmt.Errorf("invalid member ID")
	}

	query := `UPDATE users SET active = false WHERE id = ? AND organization_id = ?`
	result, err := db.Exec(query, id, organizationID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("member not found or access denied")
	}

	return nil
}