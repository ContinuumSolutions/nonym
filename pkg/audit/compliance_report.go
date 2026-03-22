package audit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/compliance"
)

// ComplianceReport is a full downloadable compliance report for one framework.
type ComplianceReport struct {
	Framework       string              `json:"framework"`
	Citation        string              `json:"citation"`
	GeneratedAt     time.Time           `json:"generated_at"`
	OrganizationID  int                 `json:"organization_id"`
	TimeRange       string              `json:"time_range"`
	PeriodStart     time.Time           `json:"period_start"`
	PeriodEnd       time.Time           `json:"period_end"`
	Summary         ReportSummary       `json:"summary"`
	EntityBreakdown []EntityCount       `json:"entity_breakdown"`
	DailyActivity   []DailyCount        `json:"daily_activity"`
	Exposure        *ExposureEstimate   `json:"estimated_exposure_averted,omitempty"`
	Recommendations []string            `json:"recommendations"`
}

// ReportSummary contains high-level metrics for the report period.
type ReportSummary struct {
	TotalRequests        int64   `json:"total_requests"`
	ProtectedRequests    int64   `json:"protected_requests"`
	BlockedRequests      int64   `json:"blocked_requests"`
	TotalRedactions      int64   `json:"total_redactions"`
	UniqueEntityTypes    int     `json:"unique_entity_types"`
	ComplianceScore      float64 `json:"compliance_score"` // 0-100
}

// EntityCount counts occurrences of one entity type in the report period.
type EntityCount struct {
	EntityType string `json:"entity_type"`
	Count      int64  `json:"count"`
}

// DailyCount is one day's redaction count for the activity chart.
type DailyCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int64  `json:"count"`
}

// HandleGetComplianceReport handles GET /api/v1/compliance/reports/:framework
//
// Path param: framework — one of "gdpr", "hipaa", "pci" (case-insensitive)
// Query param: timeRange — "7d" | "30d" | "90d" (default "30d")
func HandleGetComplianceReport(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	frameworkParam := c.Params("framework")
	framework := normalizeFramework(frameworkParam)
	if framework == "" {
		return c.Status(400).JSON(fiber.Map{
			"error":     "Unknown framework",
			"supported": []string{"gdpr", "hipaa", "pci"},
		})
	}

	timeRange := c.Query("timeRange", "30d")
	since, periodLabel := parseReportTimeRange(timeRange)

	report, err := buildComplianceReport(orgID, framework, periodLabel, since)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("Failed to build report: %v", err)})
	}
	return c.JSON(report)
}

// HandleListComplianceReports returns metadata for all available framework reports.
// GET /api/v1/compliance/reports
func HandleListComplianceReports(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	type reportMeta struct {
		Framework   string `json:"framework"`
		Citation    string `json:"citation"`
		Description string `json:"description"`
		Endpoint    string `json:"endpoint"`
	}

	reports := []reportMeta{
		{
			Framework:   "GDPR",
			Citation:    compliance.Citations["GDPR"],
			Description: "General Data Protection Regulation — covers personal data processing for EU residents including names, emails, IPs, and location data.",
			Endpoint:    "/api/v1/compliance/reports/gdpr",
		},
		{
			Framework:   "HIPAA",
			Citation:    compliance.Citations["HIPAA"],
			Description: "Health Insurance Portability and Accountability Act — covers Protected Health Information (PHI) including names, SSNs, dates, and medical record numbers.",
			Endpoint:    "/api/v1/compliance/reports/hipaa",
		},
		{
			Framework:   "PCI-DSS",
			Citation:    compliance.Citations["PCI-DSS"],
			Description: "Payment Card Industry Data Security Standard — covers cardholder data including PANs, CVVs, and IBANs.",
			Endpoint:    "/api/v1/compliance/reports/pci",
		},
	}

	return c.JSON(fiber.Map{
		"reports": reports,
		"note":    "Each report is generated on demand for the requested time range.",
	})
}

func normalizeFramework(s string) string {
	switch s {
	case "gdpr", "GDPR":
		return "GDPR"
	case "hipaa", "HIPAA":
		return "HIPAA"
	case "pci", "pci-dss", "PCI", "PCI-DSS":
		return "PCI-DSS"
	default:
		return ""
	}
}

func parseReportTimeRange(tr string) (time.Time, string) {
	now := time.Now()
	switch tr {
	case "7d":
		return now.AddDate(0, 0, -7), "7d"
	case "90d":
		return now.AddDate(0, 0, -90), "90d"
	default:
		return now.AddDate(0, 0, -30), "30d"
	}
}

func buildComplianceReport(orgID int, framework, timeRange string, since time.Time) (*ComplianceReport, error) {
	report := &ComplianceReport{
		Framework:      framework,
		Citation:       compliance.Citations[framework],
		GeneratedAt:    time.Now(),
		OrganizationID: orgID,
		TimeRange:      timeRange,
		PeriodStart:    since,
		PeriodEnd:      time.Now(),
		Recommendations: recommendationsFor(framework),
	}

	if db == nil {
		report.Summary = ReportSummary{ComplianceScore: 100}
		report.EntityBreakdown = []EntityCount{}
		report.DailyActivity = []DailyCount{}
		return report, nil
	}

	// --- Summary counts ---
	db.QueryRow(formatQuery(`SELECT COUNT(*) FROM transactions WHERE organization_id = ? AND created_at >= ?`),
		orgID, since).Scan(&report.Summary.TotalRequests)
	db.QueryRow(formatQuery(`SELECT COUNT(*) FROM transactions WHERE organization_id = ? AND created_at >= ? AND redaction_count > 0`),
		orgID, since).Scan(&report.Summary.ProtectedRequests)
	db.QueryRow(formatQuery(`SELECT COUNT(*) FROM transactions WHERE organization_id = ? AND created_at >= ? AND status = 'blocked'`),
		orgID, since).Scan(&report.Summary.BlockedRequests)
	db.QueryRow(formatQuery(`SELECT COALESCE(SUM(redaction_count),0) FROM transactions WHERE organization_id = ? AND created_at >= ?`),
		orgID, since).Scan(&report.Summary.TotalRedactions)

	if report.Summary.TotalRequests > 0 {
		protected := float64(report.Summary.ProtectedRequests + report.Summary.BlockedRequests)
		report.Summary.ComplianceScore = (protected / float64(report.Summary.TotalRequests)) * 100
	} else {
		report.Summary.ComplianceScore = 100
	}

	// --- Entity breakdown from events ---
	rows, err := db.Query(formatQuery(
		`SELECT compliance_frameworks, metadata FROM events WHERE organization_id = ? AND timestamp >= ?`),
		orgID, since)
	if err == nil {
		defer rows.Close()
		entityCounts := make(map[string]int64)
		for rows.Next() {
			var fwJSON, metaJSON string
			if rows.Scan(&fwJSON, &metaJSON) != nil {
				continue
			}
			var frameworks []string
			json.Unmarshal([]byte(fwJSON), &frameworks)
			if !containsStr(frameworks, framework) {
				continue
			}
			var meta map[string]json.RawMessage
			if json.Unmarshal([]byte(metaJSON), &meta) != nil {
				continue
			}
			if rdRaw, ok := meta["redaction_details"]; ok {
				var rds []map[string]json.RawMessage
				if json.Unmarshal(rdRaw, &rds) == nil {
					for _, rd := range rds {
						if etRaw, ok2 := rd["entity_type"]; ok2 {
							var et string
							json.Unmarshal(etRaw, &et)
							if et != "" && containsStr(compliance.FrameworksForEntityType(et), framework) {
								entityCounts[et]++
							}
						}
					}
				}
			}
		}
		for et, count := range entityCounts {
			report.EntityBreakdown = append(report.EntityBreakdown, EntityCount{EntityType: et, Count: count})
		}
		report.Summary.UniqueEntityTypes = len(entityCounts)
	}
	if report.EntityBreakdown == nil {
		report.EntityBreakdown = []EntityCount{}
	}

	// --- Daily activity (last N days, redaction counts) ---
	dailyRows, err := db.Query(formatQuery(
		`SELECT DATE(created_at) as day, COALESCE(SUM(redaction_count),0)
		 FROM transactions WHERE organization_id = ? AND created_at >= ?
		 GROUP BY day ORDER BY day ASC`), orgID, since)
	if err == nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var dc DailyCount
			dailyRows.Scan(&dc.Date, &dc.Count)
			report.DailyActivity = append(report.DailyActivity, dc)
		}
	}
	if report.DailyActivity == nil {
		report.DailyActivity = []DailyCount{}
	}

	// --- Exposure estimate ---
	if framework != "SOC2" {
		fineRates, _ := getOrgFineRates(orgID)
		for _, r := range fineRates {
			if r.Framework == framework {
				amount := r.PerEventAmount * float64(report.Summary.ProtectedRequests)
				report.Exposure = &ExposureEstimate{
					Amount:        amount,
					Currency:      r.Currency,
					PerEventBasis: r.PerEventAmount,
					Disclaimer:    "Estimate only — not legal advice.",
				}
				break
			}
		}
	}

	return report, nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func recommendationsFor(framework string) []string {
	base := []string{
		"Enable strict mode to block requests containing critical PII instead of anonymising.",
		"Review the entity breakdown above and configure per-entity allow-lists for legitimate use cases.",
		"Set up webhook notifications for high-severity protection events.",
	}
	switch framework {
	case "GDPR":
		return append([]string{
			"Ensure you have a legal basis (Art. 6) for each category of personal data flowing through your AI stack.",
			"Document Nonym as a technical measure under GDPR Art. 25 (Data Protection by Design).",
		}, base...)
	case "HIPAA":
		return append([]string{
			"Verify Nonym is included in your BAA (Business Associate Agreement) with any covered entity.",
			"Enable audit-log retention of at least 6 years to satisfy HIPAA record-keeping requirements.",
		}, base...)
	case "PCI-DSS":
		return append([]string{
			"Confirm that raw PANs (Primary Account Numbers) never appear in logs — enable strict blocking for CREDIT_CARD and CARD_CVV.",
			"Include Nonym in your annual PCI-DSS assessment scope documentation.",
		}, base...)
	}
	return base
}
