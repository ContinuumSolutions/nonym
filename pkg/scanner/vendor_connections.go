package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// HandleTestCredentials tests credentials inline without requiring an existing connection.
// POST /api/v1/vendor-connections/test
func HandleTestCredentials(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
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

	vc := &VendorConnection{
		Vendor:      req.Vendor,
		AuthType:    req.AuthType,
		Credentials: req.Credentials,
	}
	result := testConnection(vc)
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

	// Update status based on test result.
	newStatus := "disconnected"
	errMsg := ""
	if result.Success {
		newStatus = "connected"
		now := time.Now()
		updateVendorConnectionStatus(id, newStatus, "", &now, vc.LastScanAt)
	} else {
		errMsg = result.Message
		updateVendorConnectionStatus(id, "error", errMsg, vc.ConnectedAt, vc.LastScanAt)
	}
	_ = newStatus

	return c.JSON(result)
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
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	EventsAccessible *int   `json:"events_accessible,omitempty"`
}

// testConnection validates credentials against the vendor.
func testConnection(vc *VendorConnection) ConnectionResult {
	switch vc.Vendor {
	case "sentry":
		token, _ := vc.Credentials["token"].(string)
		if token == "" {
			token, _ = vc.Credentials["api_key"].(string)
		}
		if token == "" {
			token, _ = vc.Credentials["auth_token"].(string)
		}
		if len(token) < 8 {
			return ConnectionResult{Success: false, Message: "Sentry auth token is missing or too short"}
		}
		return testSentryCredentials(token)

	case "datadog":
		apiKey, _ := vc.Credentials["api_key"].(string)
		appKey, _ := vc.Credentials["app_key"].(string)
		if apiKey == "" || appKey == "" {
			return ConnectionResult{Success: false, Message: "Datadog requires both api_key and app_key"}
		}
		return ConnectionResult{Success: true, Message: "Datadog credentials validated (format check passed)"}

	case "mixpanel":
		serviceAccount, _ := vc.Credentials["service_account"].(string)
		secret, _ := vc.Credentials["secret"].(string)
		if serviceAccount == "" || secret == "" {
			return ConnectionResult{Success: false, Message: "Mixpanel requires service_account and secret"}
		}
		return ConnectionResult{Success: true, Message: "Mixpanel credentials validated (format check passed)"}

	case "stripe":
		key, _ := vc.Credentials["api_key"].(string)
		if !strings.HasPrefix(key, "sk_") {
			return ConnectionResult{Success: false, Message: "Stripe API key must start with sk_"}
		}
		return ConnectionResult{Success: true, Message: "Stripe credentials validated (format check passed)"}

	default:
		if len(vc.Credentials) == 0 {
			return ConnectionResult{Success: false, Message: "No credentials provided"}
		}
		return ConnectionResult{Success: true, Message: "Credentials format accepted"}
	}
}

// testSentryCredentials verifies a Sentry auth token by calling the real API.
func testSentryCredentials(token string) ConnectionResult {
	req, err := http.NewRequest("GET", "https://sentry.io/api/0/organizations/?member=1", nil)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Failed to build request: %v", err)}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Could not reach Sentry API: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return ConnectionResult{Success: false, Message: "Invalid or expired token — check that it has org:read, project:read, and event:read scopes"}
	}
	if resp.StatusCode >= 400 {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Sentry API error (HTTP %d): %s", resp.StatusCode, truncateStr(string(body), 200))}
	}

	var orgs []struct {
		Slug string `json:"slug"`
	}
	if err := json.Unmarshal(body, &orgs); err != nil {
		return ConnectionResult{Success: false, Message: "Unexpected response from Sentry API"}
	}

	n := len(orgs)
	return ConnectionResult{
		Success:          true,
		Message:          fmt.Sprintf("Connected — %d organization(s) accessible", n),
		EventsAccessible: &n,
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
