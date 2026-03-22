package audit

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/router"
)

// VendorIntegration represents an org's configured vendor integration.
type VendorIntegration struct {
	ID             string    `json:"id"`
	OrganizationID int       `json:"organization_id"`
	VendorID       string    `json:"vendor_id"`
	VendorName     string    `json:"vendor_name"`
	Method         string    `json:"method"`  // "proxy" or "sdk_wrapper"
	Status         string    `json:"status"`  // "active", "paused"
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// InitializeVendorTables creates the vendor_integrations table.
func InitializeVendorTables() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := db.Exec(formatQuery(`CREATE TABLE IF NOT EXISTS vendor_integrations (
		id              TEXT PRIMARY KEY,
		organization_id INTEGER NOT NULL,
		vendor_id       TEXT NOT NULL,
		vendor_name     TEXT NOT NULL,
		method          TEXT NOT NULL DEFAULT 'proxy',
		status          TEXT NOT NULL DEFAULT 'active',
		created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (organization_id, vendor_id)
	)`))
	return err
}

// HandleListVendors returns the full vendor catalog.
// GET /api/v1/vendors
func HandleListVendors(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	return c.JSON(fiber.Map{
		"vendors": router.VendorCatalog,
		"total":   len(router.VendorCatalog),
	})
}

// HandleGetVendor returns a single vendor profile by ID.
// GET /api/v1/vendors/:id
func HandleGetVendor(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	profile := router.GetVendorProfile(c.Params("id"))
	if profile == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Vendor not found"})
	}
	return c.JSON(profile)
}

// HandleGetConfiguredVendors returns the vendors the org has set up.
// GET /api/v1/vendors/configured
func HandleGetConfiguredVendors(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	integrations, err := getVendorIntegrations(orgID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch configured vendors"})
	}
	return c.JSON(fiber.Map{
		"integrations": integrations,
		"total":        len(integrations),
	})
}

// HandleSetupVendor creates or updates a vendor integration for an org.
// POST /api/v1/vendors/setup
func HandleSetupVendor(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		VendorID string `json:"vendor_id"`
		Method   string `json:"method"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.VendorID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "vendor_id is required"})
	}

	profile := router.GetVendorProfile(req.VendorID)
	if profile == nil {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("Unknown vendor: %s", req.VendorID)})
	}

	method := req.Method
	if method == "" {
		method = "proxy"
	}
	validMethods := map[string]bool{"proxy": true, "sdk_wrapper": true, "agent_config": true}
	if !validMethods[method] {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("Invalid method: %s", method)})
	}

	id := fmt.Sprintf("vi_%d_%s", orgID, req.VendorID)
	now := time.Now()

	if db != nil {
		q := formatQuery(`INSERT INTO vendor_integrations (id, organization_id, vendor_id, vendor_name, method, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'active', ?, ?)
			ON CONFLICT (organization_id, vendor_id) DO UPDATE
			SET method = EXCLUDED.method, status = 'active', updated_at = EXCLUDED.updated_at`)
		if _, err := db.Exec(q, id, orgID, req.VendorID, profile.Name, method, now, now); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to save vendor integration"})
		}
	}

	integration := VendorIntegration{
		ID:             id,
		OrganizationID: orgID,
		VendorID:       req.VendorID,
		VendorName:     profile.Name,
		Method:         method,
		Status:         "active",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	return c.Status(201).JSON(fiber.Map{
		"integration": integration,
		"setup_instructions": buildSetupInstructions(profile, method),
	})
}

// HandleRemoveVendor deletes a vendor integration.
// DELETE /api/v1/vendors/configured/:vendor_id
func HandleRemoveVendor(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	vendorID := c.Params("vendor_id")
	if db != nil {
		db.Exec(formatQuery(`DELETE FROM vendor_integrations WHERE organization_id = ? AND vendor_id = ?`), orgID, vendorID)
	}
	return c.JSON(fiber.Map{"message": "Vendor integration removed"})
}

func getVendorIntegrations(orgID int) ([]VendorIntegration, error) {
	if db == nil {
		return []VendorIntegration{}, nil
	}
	rows, err := db.Query(formatQuery(
		`SELECT id, organization_id, vendor_id, vendor_name, method, status, created_at, updated_at
		 FROM vendor_integrations WHERE organization_id = ? ORDER BY created_at DESC`), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var integrations []VendorIntegration
	for rows.Next() {
		var v VendorIntegration
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.VendorID, &v.VendorName, &v.Method, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			continue
		}
		integrations = append(integrations, v)
	}
	if integrations == nil {
		integrations = []VendorIntegration{}
	}
	return integrations, nil
}

func buildSetupInstructions(profile *router.VendorProfile, method string) map[string]interface{} {
	instructions := map[string]interface{}{
		"vendor":  profile.Name,
		"method":  method,
		"summary": fmt.Sprintf("Route all %s traffic through Nonym to redact PII before it leaves your infrastructure.", profile.Name),
	}

	switch method {
	case "sdk_wrapper":
		if profile.NonymSDK != "" {
			instructions["steps"] = []string{
				fmt.Sprintf("1. Install %s: `npm install %s`", profile.NonymSDK, profile.NonymSDK),
				fmt.Sprintf("2. Replace `import * as %s from '%s'` with `import * as %s from '%s'`", profile.ID, profile.SDKPackages[0], profile.ID, profile.NonymSDK),
				"3. Set NONYM_API_KEY environment variable to your Nonym API key.",
				fmt.Sprintf("4. All %s data will now be scrubbed of PII before transmission.", profile.Name),
			}
		} else {
			instructions["steps"] = []string{
				"1. SDK wrapper not yet available for this vendor.",
				"2. Use the proxy method instead.",
			}
		}
	case "proxy":
		instructions["steps"] = []string{
			"1. Point the vendor SDK's ingest URL to your Nonym gateway.",
			fmt.Sprintf("2. Set the DSN/endpoint to https://<your-nonym-host>/vendor-proxy/%s", profile.ID),
			"3. Nonym will redact PII and forward clean data to the vendor.",
			fmt.Sprintf("4. Set X-Nonym-Vendor: %s header if configuring manually.", profile.ID),
		}
		instructions["known_hosts"] = profile.KnownHosts
	}

	return instructions
}

