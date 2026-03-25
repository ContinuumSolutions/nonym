package scanner

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v2"
)

// reportsDir returns the directory used to persist generated PDF files.
// Override with the REPORTS_DIR environment variable.
func reportsDir() string {
	if d := os.Getenv("REPORTS_DIR"); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "nonym-reports")
}

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

	// Generate the share token upfront so we can return the download URL immediately.
	shareToken := newShareToken()
	expires := time.Now().Add(30 * 24 * time.Hour)

	report := &Report{
		ID:        newID(),
		OrgID:     orgID,
		Framework: req.Framework,
		TimeRange: req.TimeRange,
		Options:   req.Options,
		Status:    "pending",
		ShareToken: shareToken,
		ExpiresAt: &expires,
		CreatedAt: time.Now(),
	}
	if err := insertReport(report); err != nil {
		log.Printf("scanner: HandleGenerateReport: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create report"})
	}

	baseURL := c.BaseURL()
	go generateReport(report, baseURL)

	downloadURL := fmt.Sprintf("%s/api/v1/reports/share/%s/download", baseURL, shareToken)
	return c.Status(202).JSON(fiber.Map{
		"report_id":    report.ID,
		"share_token":  shareToken,
		"download_url": downloadURL,
		"status":       "pending",
		"message":      "Report generation started. Use the download_url once status is 'done'.",
	})
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

// HandleDownloadReport serves the PDF for authenticated users.
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
		return c.Status(422).JSON(fiber.Map{"error": "Report is not ready for download", "status": report.Status})
	}
	return servePDF(c, report)
}

// HandleGetSharedReport returns report metadata by share token (public, no auth).
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

// HandleDownloadSharedReport serves the PDF via share token (public, no auth).
// GET /api/v1/reports/share/:token/download
func HandleDownloadSharedReport(c *fiber.Ctx) error {
	token := c.Params("token")
	report, err := getReportByShareToken(token)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Report not found or share link has expired"})
	}
	if report.ExpiresAt != nil && report.ExpiresAt.Before(time.Now()) {
		return c.Status(410).JSON(fiber.Map{"error": "Share link has expired"})
	}
	if report.Status != "done" || report.FileURL == "" {
		return c.Status(422).JSON(fiber.Map{"error": "Report is not ready for download", "status": report.Status})
	}
	return servePDF(c, report)
}

// servePDF sends the PDF file from disk as an inline attachment.
func servePDF(c *fiber.Ctx, report *Report) error {
	data, err := os.ReadFile(report.FileURL)
	if err != nil {
		log.Printf("scanner: servePDF: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Report file unavailable"})
	}
	filename := fmt.Sprintf("%s_report_%s.pdf", strings.ToLower(report.Framework), report.ID[:8])
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))
	c.Set("Cache-Control", "no-store")
	return c.Send(data)
}

// ── Report generation ─────────────────────────────────────────────────────────

// parseTimeRange converts a human-readable time-range string (e.g. "Last 30 days",
// "Last 6 months") into a cutoff time.  Returns nil when the string is not
// recognised — callers treat nil as "no time filter".
func parseTimeRange(s string) *time.Time {
	s = strings.TrimSpace(s)
	re := regexp.MustCompile(`(?i)^last\s+(\d+)\s+(day|days|month|months|year|years)$`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return nil
	}
	unit := strings.ToLower(m[2])
	var cutoff time.Time
	switch {
	case strings.HasPrefix(unit, "day"):
		cutoff = time.Now().AddDate(0, 0, -n)
	case strings.HasPrefix(unit, "month"):
		cutoff = time.Now().AddDate(0, -n, 0)
	case strings.HasPrefix(unit, "year"):
		cutoff = time.Now().AddDate(-n, 0, 0)
	default:
		return nil
	}
	return &cutoff
}

// generateReport renders a PDF, persists it to disk, and marks the record done.
func generateReport(report *Report, baseURL string) {
	updateReport(report.ID, "generating", "", report.ShareToken, report.ExpiresAt, report.ExpiresAt)

	dir := reportsDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("scanner: generateReport: mkdir %s: %v", dir, err)
		updateReport(report.ID, "failed", "", report.ShareToken, nil, report.ExpiresAt)
		return
	}

	orgName := getOrgName(report.OrgID)

	since := parseTimeRange(report.TimeRange)
	findings, err := listFindings(FindingFilter{OrgID: report.OrgID, Since: since, Limit: 500})
	if err != nil {
		log.Printf("scanner: generateReport: listFindings: %v", err)
		findings = []Finding{}
	}

	// Filter findings relevant to this framework.
	findings = filterByFramework(findings, report.Framework)

	pdfBytes, err := renderPDF(report, orgName, findings)
	if err != nil {
		log.Printf("scanner: generateReport: renderPDF: %v", err)
		updateReport(report.ID, "failed", "", report.ShareToken, nil, report.ExpiresAt)
		return
	}

	filePath := filepath.Join(dir, report.ID+".pdf")
	if err := os.WriteFile(filePath, pdfBytes, 0640); err != nil {
		log.Printf("scanner: generateReport: write %s: %v", filePath, err)
		updateReport(report.ID, "failed", "", report.ShareToken, nil, report.ExpiresAt)
		return
	}

	now := time.Now()
	updateReport(report.ID, "done", filePath, report.ShareToken, &now, report.ExpiresAt)
	log.Printf("scanner: report %s generated → %s", report.ID, filePath)
}

// filterByFramework keeps findings that have a compliance impact for the given framework.
// For "Custom" we return all findings.
func filterByFramework(findings []Finding, framework string) []Finding {
	if framework == "Custom" {
		return findings
	}
	var out []Finding
	for _, f := range findings {
		for _, ci := range f.ComplianceImpact {
			if strings.EqualFold(ci.Framework, framework) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// renderPDF builds the compliance PDF and returns it as bytes.
// orgName is the human-readable name of the organisation that owns the report.
func renderPDF(report *Report, orgName string, findings []Finding) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.SetAutoPageBreak(true, 20)

	// tr converts UTF-8 text to the cp1252 encoding fpdf uses internally.
	// Without this, multi-byte characters (em-dash, section sign, etc.) appear
	// as mojibake (e.g. "â€"" instead of "-").
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// Footer must be registered before any AddPage call so it renders on all pages.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(156, 163, 175)
		pdf.CellFormat(0, 10,
			fmt.Sprintf("Nonym Privacy Gateway - %s Compliance Report - Page %d",
				report.Framework, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	// ── Cover page ──────────────────────────────────────────────────────────
	pdf.AddPage()

	// Header bar
	pdf.SetFillColor(30, 64, 175) // indigo-800
	pdf.Rect(0, 0, 210, 50, "F")

	pdf.SetFont("Helvetica", "B", 26)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(20, 12)
	pdf.Cell(170, 12, "Nonym Privacy Gateway")

	pdf.SetFont("Helvetica", "", 14)
	pdf.SetXY(20, 28)
	pdf.Cell(170, 10, fmt.Sprintf("%s Compliance Report", report.Framework))

	// Report details box — 5 rows x 10 mm spacing + padding = 70 mm tall
	pdf.SetFillColor(243, 244, 246) // gray-100
	pdf.SetTextColor(31, 41, 55)    // gray-800
	pdf.RoundedRect(20, 60, 170, 70, 3, "1234", "F")

	detailRow := func(y float64, label, value string) {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetXY(30, y)
		pdf.Cell(50, 8, label)
		pdf.SetFont("Helvetica", "", 11)
		pdf.Cell(100, 8, tr(value))
	}
	detailRow(68, "Organisation:", orgName)
	detailRow(79, "Framework:", report.Framework)
	detailRow(90, "Time Range:", report.TimeRange)
	detailRow(101, "Generated:", time.Now().Format("02 Jan 2006, 15:04 UTC"))
	detailRow(112, "Report ID:", report.ID)

	// Confidentiality notice
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(107, 114, 128) // gray-500
	pdf.SetXY(20, 142)
	pdf.MultiCell(170, 5,
		"CONFIDENTIAL - This document is generated by Nonym and intended solely for the "+
			"named organisation. Do not distribute without authorisation.",
		"", "C", false)

	// ── Executive Summary ───────────────────────────────────────────────────
	pdf.AddPage()
	sectionHeader(pdf, "Executive Summary")

	high, medium, low := countByRisk(findings)
	total := len(findings)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetX(20)
	summary := fmt.Sprintf(
		"This report covers findings relevant to %s for the period '%s'. "+
			"A total of %d finding(s) were identified: %d high-risk, %d medium-risk, and %d low-risk.",
		report.Framework, report.TimeRange, total, high, medium, low,
	)
	pdf.MultiCell(170, 6, tr(summary), "", "L", false)
	pdf.Ln(6)

	// Risk summary table
	riskTableHeader(pdf)
	riskTableRow(pdf, "High", high, true)
	riskTableRow(pdf, "Medium", medium, false)
	riskTableRow(pdf, "Low", low, true)
	riskTableRow(pdf, "Total", total, false)

	// ── Framework context ───────────────────────────────────────────────────
	pdf.Ln(10)
	sectionHeader(pdf, fmt.Sprintf("%s Overview", report.Framework))
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetX(20)
	pdf.MultiCell(170, 6, tr(frameworkSummary(report.Framework)), "", "L", false)

	// ── Findings detail ─────────────────────────────────────────────────────
	if len(findings) > 0 {
		pdf.AddPage()
		sectionHeader(pdf, "Detailed Findings")
		for i, f := range findings {
			renderFinding(pdf, tr, i+1, f)
		}
	} else {
		pdf.Ln(8)
		pdf.SetFont("Helvetica", "I", 11)
		pdf.SetTextColor(107, 114, 128)
		pdf.SetX(20)
		pdf.Cell(170, 8, "No findings were identified for this framework and time range.")
	}

	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("pdf build error: %w", err)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// ── PDF helper functions ──────────────────────────────────────────────────────

func sectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetX(20)
	pdf.Cell(170, 8, title)
	// Ln(10) = 8mm cell height + 2mm gap so the rule sits below the text,
	// not through the middle of it (the previous Ln(3) caused that bug).
	pdf.Ln(10)
	pdf.SetDrawColor(30, 64, 175)
	pdf.Line(20, pdf.GetY(), 190, pdf.GetY())
	pdf.Ln(6)
}

func riskTableHeader(pdf *fpdf.Fpdf) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(30, 64, 175)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetX(20)
	pdf.CellFormat(85, 7, "Risk Level", "1", 0, "C", true, 0, "")
	pdf.CellFormat(85, 7, "Count", "1", 1, "C", true, 0, "")
}

func riskTableRow(pdf *fpdf.Fpdf, label string, count int, shaded bool) {
	pdf.SetFont("Helvetica", "", 10)
	if shaded {
		pdf.SetFillColor(243, 244, 246)
	} else {
		pdf.SetFillColor(255, 255, 255)
	}
	pdf.SetTextColor(31, 41, 55)
	pdf.SetX(20)
	pdf.CellFormat(85, 7, label, "1", 0, "L", true, 0, "")
	pdf.CellFormat(85, 7, fmt.Sprintf("%d", count), "1", 1, "C", true, 0, "")
}

func renderFinding(pdf *fpdf.Fpdf, tr func(string) string, idx int, f Finding) {
	// Check remaining space; add page if needed.
	if pdf.GetY() > 240 {
		pdf.AddPage()
	}

	riskColor := map[string][3]int{
		"high":   {220, 38, 38},
		"medium": {217, 119, 6},
		"low":    {22, 163, 74},
	}
	col := riskColor[f.RiskLevel]
	if col == [3]int{} {
		col = [3]int{107, 114, 128}
	}

	// Finding header row
	pdf.SetFillColor(249, 250, 251)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetX(20)
	pdf.CellFormat(130, 7, tr(fmt.Sprintf("%d. %s", idx, f.Title)), "LTB", 0, "L", true, 0, "")
	pdf.SetFillColor(col[0], col[1], col[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(40, 7, strings.ToUpper(f.RiskLevel), "RTB", 1, "C", true, 0, "")

	// Details
	pdf.SetFillColor(255, 255, 255)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(55, 65, 81)
	pdf.SetX(20)
	pdf.CellFormat(170, 6, tr(fmt.Sprintf("Vendor: %s  |  Data Type: %s  |  Status: %s  |  Occurrences: %d",
		f.Vendor, f.DataType, f.Status, f.Occurrences)), "LRB", 1, "L", true, 0, "")

	if f.Description != "" {
		pdf.SetX(20)
		pdf.MultiCell(170, 5, tr(f.Description), "LRB", "L", false)
	}

	// Compliance articles
	if len(f.ComplianceImpact) > 0 {
		articles := make([]string, 0, len(f.ComplianceImpact))
		for _, ci := range f.ComplianceImpact {
			articles = append(articles, fmt.Sprintf("%s %s", ci.Framework, ci.Article))
		}
		pdf.SetX(20)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(107, 114, 128)
		pdf.MultiCell(170, 5, tr("Compliance: "+strings.Join(articles, ", ")), "LRB", "L", false)
	}

	pdf.Ln(4)
}

func countByRisk(findings []Finding) (high, medium, low int) {
	for _, f := range findings {
		switch f.RiskLevel {
		case "high":
			high++
		case "medium":
			medium++
		default:
			low++
		}
	}
	return
}

func frameworkSummary(framework string) string {
	switch framework {
	case "GDPR":
		return "The General Data Protection Regulation (GDPR) establishes requirements for the lawful " +
			"processing of personal data of EU residents. Key obligations include purpose limitation, " +
			"data minimisation, and the right to erasure. Findings below highlight data types and " +
			"transmission patterns that may constitute a breach of Articles 5, 25, or 32."
	case "HIPAA":
		return "The Health Insurance Portability and Accountability Act (HIPAA) requires covered entities " +
			"and business associates to protect the confidentiality, integrity, and availability of " +
			"Protected Health Information (PHI). Findings may relate to the Security Rule (45 CFR §164.312) " +
			"or the Privacy Rule (45 CFR §164.502)."
	case "SOC2":
		return "SOC 2 Trust Service Criteria require organisations to demonstrate controls over security, " +
			"availability, processing integrity, confidentiality, and privacy. Findings below relate to " +
			"CC6 (Logical and Physical Access), CC7 (System Operations), and P-series privacy criteria."
	default:
		return "This custom report aggregates all findings detected within the specified time range. " +
			"Review each finding and apply the appropriate remediation steps."
	}
}

// newShareToken generates a cryptographically random 32-byte hex token.
func newShareToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
