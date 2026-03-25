package scanner

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleListVendorConnections returns all vendor connections for the org.
// GET /api/v1/vendor-connections
func HandleListVendorConnections(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	statusFilter := c.Query("status")
	connections, err := listVendorConnections(orgID, statusFilter)
	if err != nil {
		log.Printf("scanner: HandleListVendorConnections: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch vendor connections"})
	}
	for i := range connections {
		maskCredentials(&connections[i])
	}
	return c.JSON(fiber.Map{"connections": connections, "total": len(connections)})
}

// HandleCreateVendorConnection creates a new vendor connection.
// POST /api/v1/vendor-connections
func HandleCreateVendorConnection(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Vendor      string                 `json:"vendor"`
		AuthType    string                 `json:"auth_type"`
		Credentials map[string]interface{} `json:"credentials"`
		Settings    map[string]interface{} `json:"settings"`
		DisplayName string                 `json:"display_name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Vendor == "" {
		return c.Status(400).JSON(fiber.Map{"error": "vendor is required"})
	}
	if IsProxyVendor(req.Vendor) {
		return c.Status(400).JSON(fiber.Map{"error": req.Vendor + " is managed via the AI proxy and cannot be added as a scanner vendor connection"})
	}
	if req.AuthType == "" {
		req.AuthType = "api_key"
	}
	validAuthTypes := map[string]bool{"api_key": true, "oauth": true}
	if !validAuthTypes[req.AuthType] {
		return c.Status(400).JSON(fiber.Map{"error": "auth_type must be api_key or oauth"})
	}
	if len(req.Credentials) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "credentials are required"})
	}

	displayName := req.DisplayName
	if displayName == "" {
		// Capitalise first letter without using deprecated strings.Title.
		r := req.Vendor
		if len(r) > 0 {
			displayName = strings.ToUpper(r[:1]) + r[1:]
		} else {
			displayName = r
		}
	}
	if req.Settings == nil {
		req.Settings = map[string]interface{}{}
	}

	now := time.Now()
	vc := &VendorConnection{
		ID:          newID(),
		OrgID:       orgID,
		Vendor:      req.Vendor,
		DisplayName: displayName,
		Status:      "disconnected",
		AuthType:    req.AuthType,
		Credentials: req.Credentials,
		Settings:    req.Settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := insertVendorConnection(vc); err != nil {
		log.Printf("scanner: HandleCreateVendorConnection: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create vendor connection"})
	}

	// ON CONFLICT DO UPDATE preserves the original row's ID, so re-fetch the
	// canonical record to ensure the response ID matches what is in the DB.
	if canonical, err := getVendorConnectionByVendor(orgID, req.Vendor); err == nil {
		vc = canonical
	}
	maskCredentials(vc)
	return c.Status(201).JSON(vc)
}

// HandleDeleteVendorConnection removes a vendor connection.
// DELETE /api/v1/vendor-connections/:id
func HandleDeleteVendorConnection(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	id := c.Params("id")
	if err := deleteVendorConnection(orgID, id); err != nil {
		log.Printf("scanner: HandleDeleteVendorConnection: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete vendor connection"})
	}
	return c.SendStatus(204)
}

// HandleTestCredentials tests credentials inline. If a connection for this
// org+vendor already exists its status is updated; if none exists and the test
// succeeds a new connection is created with status "connected" so the vendor
// is immediately available for scanning without a separate create call.
// POST /api/v1/vendor-connections/test
func HandleTestCredentials(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Vendor      string                 `json:"vendor"`
		AuthType    string                 `json:"auth_type"`
		Credentials map[string]interface{} `json:"credentials"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Vendor == "" {
		return c.Status(400).JSON(fiber.Map{"error": "vendor is required"})
	}
	if len(req.Credentials) == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "credentials are required"})
	}
	if req.AuthType == "" {
		req.AuthType = "api_key"
	}

	vc := &VendorConnection{
		Vendor:      req.Vendor,
		AuthType:    req.AuthType,
		Credentials: req.Credentials,
	}
	result := testConnection(vc)

	now := time.Now()

	// Auto-detect region from the submitted credentials (uses the temp vc which
	// has the real, unmasked credentials the user just provided).
	detectedRegion := detectRegion(vc)

	if existing, err := getVendorConnectionByVendor(orgID, req.Vendor); err == nil {
		// Connection already exists — update its status.
		if result.Success {
			updateVendorConnectionStatus(existing.ID, "connected", "", &now, existing.LastScanAt)
			existing.Status = "connected"
			existing.ConnectedAt = &now
			existing.ErrorMessage = ""
			// Update region if we detected one and the user hasn't manually overridden it.
			if existing.HostingRegion == "" && detectedRegion != "" {
				updateVendorHostingRegion(existing.ID, detectedRegion)
				existing.HostingRegion = detectedRegion
			}
		} else {
			updateVendorConnectionStatus(existing.ID, "error", result.Message, existing.ConnectedAt, existing.LastScanAt)
			existing.Status = "error"
			existing.ErrorMessage = result.Message
		}
		existing.UpdatedAt = now
		maskCredentials(existing)
		result.Connection = existing
	} else if result.Success {
		// No connection exists yet — create one marked as connected so the
		// vendor is available for scanning immediately.
		displayName := req.Vendor
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}
		newVC := &VendorConnection{
			ID:            newID(),
			OrgID:         orgID,
			Vendor:        req.Vendor,
			DisplayName:   displayName,
			Status:        "connected",
			AuthType:      req.AuthType,
			Credentials:   req.Credentials,
			Settings:      map[string]interface{}{},
			HostingRegion: detectedRegion,
			ConnectedAt:   &now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := insertVendorConnection(newVC); err != nil {
			log.Printf("scanner: HandleTestCredentials: create connection: %v", err)
		} else {
			// Re-fetch the canonical record (ON CONFLICT may have preserved an older row).
			if canonical, err := getVendorConnectionByVendor(orgID, req.Vendor); err == nil {
				maskCredentials(canonical)
				result.Connection = canonical
			}
		}
	}

	return c.JSON(result)
}

// HandleTestVendorConnection tests the credentials for a vendor connection.
// POST /api/v1/vendor-connections/:id/test
func HandleTestVendorConnection(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	id := c.Params("id")
	vc, err := getVendorConnection(orgID, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Vendor connection not found"})
	}

	result := testConnection(vc)

	now := time.Now()
	if result.Success {
		updateVendorConnectionStatus(id, "connected", "", &now, vc.LastScanAt)
		vc.Status = "connected"
		vc.ConnectedAt = &now
		vc.ErrorMessage = ""
		// Auto-detect and persist hosting region if not already set by the user.
		if vc.HostingRegion == "" {
			if region := detectRegion(vc); region != "" {
				updateVendorHostingRegion(id, region)
				vc.HostingRegion = region
			}
		}
	} else {
		updateVendorConnectionStatus(id, "error", result.Message, vc.ConnectedAt, vc.LastScanAt)
		vc.Status = "error"
		vc.ErrorMessage = result.Message
	}
	vc.UpdatedAt = now
	maskCredentials(vc)
	result.Connection = vc

	return c.JSON(result)
}

// HandleUpdateVendorConnection updates mutable fields on a vendor connection.
// PATCH /api/v1/vendor-connections/:id
func HandleUpdateVendorConnection(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	id := c.Params("id")
	vc, err := getVendorConnection(orgID, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Vendor connection not found"})
	}

	var req struct {
		DisplayName   *string `json:"display_name"`
		HostingRegion *string `json:"hosting_region"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.DisplayName != nil {
		if err := updateVendorDisplayName(id, *req.DisplayName); err != nil {
			log.Printf("scanner: HandleUpdateVendorConnection display_name: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update display name"})
		}
		vc.DisplayName = *req.DisplayName
	}
	if req.HostingRegion != nil {
		if err := updateVendorHostingRegion(id, *req.HostingRegion); err != nil {
			log.Printf("scanner: HandleUpdateVendorConnection hosting_region: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update hosting region"})
		}
		vc.HostingRegion = *req.HostingRegion
	}

	vc.UpdatedAt = time.Now()
	maskCredentials(vc)
	return c.JSON(vc)
}

// HandleTriggerVendorScan triggers a scan for a single vendor connection.
// POST /api/v1/vendor-connections/:id/scan
func HandleTriggerVendorScan(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	id := c.Params("id")
	vc, err := getVendorConnection(orgID, id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Vendor connection not found"})
	}

	scan, err := startScan(orgID, []string{vc.Vendor}, []VendorConnection{*vc}, "manual")
	if err != nil {
		log.Printf("scanner: HandleTriggerVendorScan: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start scan"})
	}
	return c.Status(202).JSON(fiber.Map{"scan_id": scan.ID})
}

// ── Connection test helper ────────────────────────────────────────────────────

// ConnectionResult is the response for vendor connection tests.
type ConnectionResult struct {
	Success          bool              `json:"success"`
	Message          string            `json:"message"`
	EventsAccessible *int              `json:"events_accessible,omitempty"`
	Connection       *VendorConnection `json:"connection,omitempty"`
}

// testConnection delegates credential testing to the registered connector.
// All validation logic (format checks, real API calls) lives in each
// connector's TestConnection method — this function is a thin dispatcher only.
func testConnection(vc *VendorConnection) ConnectionResult {
	if c := connectorFor(vc.Vendor); c != nil {
		return c.TestConnection(vc)
	}
	// Fallback for vendors without a registered connector.
	if len(vc.Credentials) == 0 {
		return ConnectionResult{Success: false, Message: "No credentials provided"}
	}
	return ConnectionResult{Success: true, Message: "Credentials format accepted"}
}

// maskCredentials replaces credential values with masked representations.
func maskCredentials(vc *VendorConnection) {
	masked := map[string]interface{}{}
	for k, v := range vc.Credentials {
		if s, ok := v.(string); ok && len(s) > 0 {
			masked[k] = maskAPIKey(s)
		} else {
			masked[k] = "****"
		}
	}
	vc.Credentials = masked
}
