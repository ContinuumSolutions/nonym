package scanner

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ShadowVendor is an unapproved vendor detected in proxy traffic.
type ShadowVendor struct {
	Host         string    `json:"host"`
	FirstSeen    time.Time `json:"firstSeen"`
	RequestCount int       `json:"requestCount"`
	PIIDetected  bool      `json:"piiDetected"`
}

// AllowlistEntry records an approve/block decision for a shadow vendor host.
type AllowlistEntry struct {
	ID        string    `json:"id"`
	OrgID     int       `json:"org_id,omitempty"`
	Host      string    `json:"host"`
	Action    string    `json:"action"` // approve | block
	CreatedAt time.Time `json:"createdAt"`
}

// HandleGetDetectedVendors returns shadow SaaS vendors detected in proxy traffic
// that are not in the org's approved vendor connections.
// GET /api/v1/scanner/detected-vendors
func HandleGetDetectedVendors(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	vendors, err := queryDetectedVendors(orgID)
	if err != nil {
		log.Printf("scanner: HandleGetDetectedVendors: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch detected vendors"})
	}
	return c.JSON(vendors)
}

// HandleUpsertVendorAllowlist approves or blocks a shadow vendor host.
// POST /api/v1/scanner/vendor-allowlist
func HandleUpsertVendorAllowlist(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Host   string `json:"host"`
		Action string `json:"action"` // approve | block
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Host == "" {
		return c.Status(400).JSON(fiber.Map{"error": "host is required"})
	}
	if req.Action != "approve" && req.Action != "block" {
		return c.Status(400).JSON(fiber.Map{"error": "action must be approve or block"})
	}

	entry, err := upsertAllowlistEntry(orgID, req.Host, req.Action)
	if err != nil {
		log.Printf("scanner: HandleUpsertVendorAllowlist: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update allowlist"})
	}
	return c.Status(201).JSON(entry)
}

// ── DB queries ────────────────────────────────────────────────────────────────

func queryDetectedVendors(orgID int) ([]ShadowVendor, error) {
	// Build set of approved hosts: connected vendor_connections + allowlisted hosts.
	approvedHosts, err := buildApprovedHostSet(orgID)
	if err != nil {
		return []ShadowVendor{}, nil
	}

	// Query distinct vendor_name values from the last 90 days.
	q := fmt.Sprintf(`
		SELECT
			vendor_name                                     AS host,
			MIN(created_at)                                 AS first_seen,
			COUNT(*)                                        AS request_count,
			MAX(CASE WHEN redaction_count > 0 THEN 1 ELSE 0 END) AS pii_detected
		FROM transactions
		WHERE organization_id = ?
		  AND vendor_name IS NOT NULL
		  AND vendor_name != ''
		  AND created_at >= %s
		GROUP BY vendor_name
		ORDER BY request_count DESC
	`, sinceExpr(90))

	rows, err := db.Query(formatQuery(q), orgID)
	if err != nil {
		// Transactions table may not exist in test environments.
		log.Printf("scanner: queryDetectedVendors: %v", err)
		return []ShadowVendor{}, nil
	}
	defer rows.Close()

	var out []ShadowVendor
	for rows.Next() {
		var sv ShadowVendor
		var piiInt int
		if err := rows.Scan(&sv.Host, &sv.FirstSeen, &sv.RequestCount, &piiInt); err != nil {
			continue
		}
		sv.PIIDetected = piiInt > 0

		// Exclude approved hosts.
		if approvedHosts[sv.Host] {
			continue
		}
		out = append(out, sv)
	}
	if out == nil {
		out = []ShadowVendor{}
	}
	return out, nil
}

// buildApprovedHostSet returns a set of host identifiers that should be excluded
// from shadow-vendor results: connected vendor_connections (by vendor slug) and
// explicitly approved allowlist entries.
func buildApprovedHostSet(orgID int) (map[string]bool, error) {
	approved := map[string]bool{}

	// Connected vendor connections.
	conns, err := listVendorConnections(orgID, "connected")
	if err == nil {
		for _, vc := range conns {
			approved[vc.Vendor] = true
		}
	}

	// Explicitly approved entries (ignore blocked ones — those stay in the list).
	rows, err := db.Query(formatQuery(
		`SELECT host FROM shadow_vendor_allowlist WHERE org_id = ? AND action = 'approve'`,
	), orgID)
	if err != nil {
		return approved, nil
	}
	defer rows.Close()
	for rows.Next() {
		var host string
		if rows.Scan(&host) == nil {
			approved[host] = true
		}
	}
	return approved, nil
}

func upsertAllowlistEntry(orgID int, host, action string) (*AllowlistEntry, error) {
	now := time.Now()
	id := newID()

	_, err := db.Exec(formatQuery(`
		INSERT INTO shadow_vendor_allowlist (id, org_id, host, action, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(org_id, host) DO UPDATE SET action = EXCLUDED.action
	`), id, orgID, host, action, now)
	if err != nil {
		return nil, err
	}

	var entry AllowlistEntry
	row := db.QueryRow(formatQuery(
		`SELECT id, org_id, host, action, created_at FROM shadow_vendor_allowlist WHERE org_id = ? AND host = ?`,
	), orgID, host)
	if err := row.Scan(&entry.ID, &entry.OrgID, &entry.Host, &entry.Action, &entry.CreatedAt); err != nil {
		return nil, err
	}
	return &entry, nil
}
