package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleListReports returns all reports for the org.
// GET /api/v1/reports
func HandleListReports(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	reports, err := listReports(orgID)
	if err != nil {
		log.Printf("scanner: HandleListReports: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch reports"})
	}
	return c.JSON(fiber.Map{"reports": reports, "total": len(reports)})
}

// HandleGenerateReport creates a report record and queues async generation.
// POST /api/v1/reports/generate
func HandleGenerateReport(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		Framework string                 `json:"framework"`
		TimeRange string                 `json:"time_range"`
		Options   map[string]interface{} `json:"options"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validFrameworks := map[string]bool{"GDPR": true, "SOC2": true, "HIPAA": true, "Custom": true}
	if !validFrameworks[req.Framework] {
		return c.Status(400).JSON(fiber.Map{"error": "framework must be GDPR, SOC2, HIPAA, or Custom"})
	}
	if req.TimeRange == "" {
		return c.Status(400).JSON(fiber.Map{"error": "time_range is required"})
	}
	if req.Options == nil {
		req.Options = map[string]interface{}{}
	}

	report := &Report{
		ID:        newID(),
		OrgID:     orgID,
		Framework: req.Framework,
		TimeRange: req.TimeRange,
		Options:   req.Options,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := insertReport(report); err != nil {
		log.Printf("scanner: HandleGenerateReport: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create report"})
	}

	// Generate asynchronously.
	go generateReport(report)

	return c.Status(202).JSON(fiber.Map{"report_id": report.ID})
}

// HandleGetReport returns a single report by ID.
// GET /api/v1/reports/:id
func HandleGetReport(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	report, err := getReport(orgID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Report not found"})
	}
	return c.JSON(report)
}

// HandleDownloadReport redirects to the signed file URL.
// GET /api/v1/reports/:id/download
func HandleDownloadReport(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	report, err := getReport(orgID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Report not found"})
	}
	if report.Status != "done" || report.FileURL == "" {
		return c.Status(422).JSON(fiber.Map{"error": "Report is not ready for download"})
	}
	return c.Redirect(report.FileURL, 302)
}

// HandleGetSharedReport returns a report by share token (public, no auth).
// GET /api/v1/reports/share/:token
func HandleGetSharedReport(c *fiber.Ctx) error {
	token := c.Params("token")
	report, err := getReportByShareToken(token)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Report not found or share link has expired"})
	}
	if report.ExpiresAt != nil && report.ExpiresAt.Before(time.Now()) {
		return c.Status(410).JSON(fiber.Map{"error": "Share link has expired"})
	}
	return c.JSON(report)
}

// ── Report generation ─────────────────────────────────────────────────────────

// generateReport simulates async report generation.
// In production this would render a PDF and upload to object storage.
func generateReport(report *Report) {
	// Mark as generating.
	updateReport(report.ID, "generating", "", "", nil, nil)

	// Simulate work.
	time.Sleep(2 * time.Second)

	now := time.Now()
	shareToken := newShareToken()
	expires := now.Add(30 * 24 * time.Hour) // 30 days

	// In production: render PDF → upload → set file_url.
	// For now we mark done with a placeholder URL.
	fileURL := ""

	updateReport(report.ID, "done", fileURL, shareToken, &now, &expires)
}

// newShareToken generates a cryptographically random 32-byte hex token.
func newShareToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
