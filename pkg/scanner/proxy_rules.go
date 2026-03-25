package scanner

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ProxyRule is a gateway-level redaction/block rule for a vendor + data type.
type ProxyRule struct {
	ID        string    `json:"rule_id"`
	OrgID     int       `json:"org_id,omitempty"`
	VendorID  string    `json:"vendorId"`
	DataType  string    `json:"dataType"`
	Action    string    `json:"action"` // redact | block | mask
	CreatedAt time.Time `json:"createdAt"`
}

// HandleCreateProxyRule creates or replaces a proxy-level data-handling rule.
// POST /api/v1/scanner/proxy-rules
func HandleCreateProxyRule(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		VendorID string `json:"vendorId"`
		DataType string `json:"dataType"`
		Action   string `json:"action"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.VendorID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "vendorId is required"})
	}
	if req.DataType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "dataType is required"})
	}
	validActions := map[string]bool{"redact": true, "block": true, "mask": true}
	if !validActions[req.Action] {
		return c.Status(400).JSON(fiber.Map{"error": "action must be redact, block, or mask"})
	}

	rule, err := upsertProxyRule(orgID, req.VendorID, req.DataType, req.Action)
	if err != nil {
		log.Printf("scanner: HandleCreateProxyRule: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create proxy rule"})
	}
	return c.Status(201).JSON(rule)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func upsertProxyRule(orgID int, vendorID, dataType, action string) (*ProxyRule, error) {
	now := time.Now()
	id := newID()

	_, err := db.Exec(formatQuery(`
		INSERT INTO proxy_rules (id, org_id, vendor_id, data_type, action, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, vendor_id, data_type) DO UPDATE SET
			action     = EXCLUDED.action,
			created_at = EXCLUDED.created_at
	`), id, orgID, vendorID, dataType, action, now)
	if err != nil {
		return nil, err
	}

	var rule ProxyRule
	row := db.QueryRow(formatQuery(
		`SELECT id, org_id, vendor_id, data_type, action, created_at
		 FROM proxy_rules WHERE org_id = ? AND vendor_id = ? AND data_type = ?`,
	), orgID, vendorID, dataType)
	if err := row.Scan(&rule.ID, &rule.OrgID, &rule.VendorID, &rule.DataType, &rule.Action, &rule.CreatedAt); err != nil {
		return nil, err
	}
	return &rule, nil
}
