package scanner

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleListFindings returns findings with optional filters.
// GET /api/v1/findings
func HandleListFindings(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 200 {
		limit = 200
	}

	filter := FindingFilter{
		OrgID:     orgID,
		Vendor:    c.Query("vendor"),
		RiskLevel: c.Query("risk_level"),
		DataType:  c.Query("data_type"),
		Status:    c.Query("status"),
		Limit:     limit,
		Offset:    offset,
	}

	findings, err := listFindings(filter)
	if err != nil {
		log.Printf("scanner: HandleListFindings: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch findings"})
	}
	return c.JSON(fiber.Map{"findings": findings, "total": len(findings)})
}

// HandleGetFinding returns a single finding by ID.
// GET /api/v1/findings/:id
func HandleGetFinding(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	finding, err := getFinding(orgID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Finding not found"})
	}
	return c.JSON(finding)
}

// HandlePatchFinding updates a finding's status.
// PATCH /api/v1/findings/:id
func HandlePatchFinding(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validStatuses := map[string]bool{"resolved": true, "suppressed": true, "open": true}
	if !validStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{"error": "status must be open, resolved, or suppressed"})
	}

	if err := patchFinding(orgID, c.Params("id"), req.Status, nil); err != nil {
		log.Printf("scanner: HandlePatchFinding patch: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update finding"})
	}

	finding, err := getFinding(orgID, c.Params("id"))
	if err != nil {
		log.Printf("scanner: HandlePatchFinding get: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve updated finding"})
	}
	return c.JSON(finding)
}
