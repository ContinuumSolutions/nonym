package scanner

import (
	"database/sql"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// ── Model ─────────────────────────────────────────────────────────────────────

// DpaRecord holds an org's DPA status for a single vendor.
// status: "signed" | "missing" | "expired" | "review_needed"
type DpaRecord struct {
	ID           string     `json:"id"`
	OrgID        int        `json:"org_id,omitempty"`
	VendorID     string     `json:"vendorId"`
	Status       string     `json:"status"`
	Region       string     `json:"region"`
	LastReviewed *time.Time `json:"lastReviewed"`
	ExpiresAt    *time.Time `json:"expiresAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// HandleGetDPARegistry returns all DPA records for the org.
// GET /api/v1/scanner/dpa-registry
func HandleGetDPARegistry(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	records, err := listDPARecords(orgID)
	if err != nil {
		log.Printf("scanner: HandleGetDPARegistry: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch DPA registry"})
	}
	return c.JSON(records)
}

// HandleUpsertDPARecord creates or updates a DPA record for a specific vendor.
// PUT /api/v1/scanner/dpa-registry/:vendor_id
func HandleUpsertDPARecord(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	vendorID := c.Params("vendor_id")
	if vendorID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "vendor_id is required"})
	}

	var req struct {
		Status       string     `json:"status"`
		Region       string     `json:"region"`
		LastReviewed *time.Time `json:"lastReviewed"`
		ExpiresAt    *time.Time `json:"expiresAt"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validStatuses := map[string]bool{
		"signed": true, "missing": true, "expired": true, "review_needed": true,
	}
	if req.Status != "" && !validStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{"error": "status must be signed, missing, expired, or review_needed"})
	}
	if req.Status == "" {
		req.Status = "missing"
	}

	record, err := upsertDPARecord(orgID, vendorID, req.Status, req.Region, req.LastReviewed, req.ExpiresAt)
	if err != nil {
		log.Printf("scanner: HandleUpsertDPARecord: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update DPA record"})
	}
	return c.JSON(record)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func listDPARecords(orgID int) ([]DpaRecord, error) {
	rows, err := db.Query(formatQuery(
		`SELECT id, org_id, vendor_id, status, region, last_reviewed, expires_at, created_at, updated_at
		 FROM dpa_records WHERE org_id = ? ORDER BY vendor_id`,
	), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DpaRecord
	for rows.Next() {
		var r DpaRecord
		var lastReviewed, expiresAt sql.NullTime
		if err := rows.Scan(
			&r.ID, &r.OrgID, &r.VendorID, &r.Status, &r.Region,
			&lastReviewed, &expiresAt, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			log.Printf("scanner: listDPARecords scan: %v", err)
			continue
		}
		if lastReviewed.Valid {
			r.LastReviewed = &lastReviewed.Time
		}
		if expiresAt.Valid {
			r.ExpiresAt = &expiresAt.Time
		}
		out = append(out, r)
	}
	if out == nil {
		out = []DpaRecord{}
	}
	return out, nil
}

func upsertDPARecord(orgID int, vendorID, status, region string, lastReviewed, expiresAt *time.Time) (*DpaRecord, error) {
	now := time.Now()
	id := newID()

	_, err := db.Exec(formatQuery(`
		INSERT INTO dpa_records (id, org_id, vendor_id, status, region, last_reviewed, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org_id, vendor_id) DO UPDATE SET
			status        = EXCLUDED.status,
			region        = EXCLUDED.region,
			last_reviewed = EXCLUDED.last_reviewed,
			expires_at    = EXCLUDED.expires_at,
			updated_at    = EXCLUDED.updated_at
	`), id, orgID, vendorID, status, region, lastReviewed, expiresAt, now, now)
	if err != nil {
		return nil, err
	}

	row := db.QueryRow(formatQuery(
		`SELECT id, org_id, vendor_id, status, region, last_reviewed, expires_at, created_at, updated_at
		 FROM dpa_records WHERE org_id = ? AND vendor_id = ?`,
	), orgID, vendorID)

	var r DpaRecord
	var lr, ea sql.NullTime
	if err := row.Scan(
		&r.ID, &r.OrgID, &r.VendorID, &r.Status, &r.Region,
		&lr, &ea, &r.CreatedAt, &r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if lr.Valid {
		r.LastReviewed = &lr.Time
	}
	if ea.Valid {
		r.ExpiresAt = &ea.Time
	}
	return &r, nil
}

// GetDPAStatusByVendor returns a map of vendorID → status for the given org.
// Used by other handlers (e.g. ai-traffic) to enrich responses.
func getDPAStatusMap(orgID int) map[string]string {
	rows, err := db.Query(formatQuery(
		`SELECT vendor_id, status FROM dpa_records WHERE org_id = ?`,
	), orgID)
	if err != nil {
		return map[string]string{}
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var vid, status string
		if rows.Scan(&vid, &status) == nil {
			m[vid] = status
		}
	}
	return m
}
