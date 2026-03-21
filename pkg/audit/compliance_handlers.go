package audit

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/compliance"
)

// ─── Compliance Settings (Task 5) ────────────────────────────────────────────

// FineRate is a configurable per-event fine rate for a single framework.
type FineRate struct {
	Framework      string  `json:"framework"`
	PerEventAmount float64 `json:"per_event_amount"`
	Currency       string  `json:"currency"`
}

// ComplianceSettingsResponse is returned by GET /api/v1/settings/compliance.
type ComplianceSettingsResponse struct {
	FineRates  []FineRate `json:"fine_rates"`
	Disclaimer string     `json:"disclaimer"`
}

const complianceDisclaimer = "These estimates are for internal risk awareness only and do not constitute legal advice."

// orderedFineRateFrameworks lists the frameworks that carry statutory fines.
var orderedFineRateFrameworks = []string{"GDPR", "HIPAA", "PCI-DSS"}

// InitializeComplianceTables creates the compliance_fine_rates table and seeds
// default rates for every organization that does not already have rows.
func InitializeComplianceTables() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	createSQL := formatQuery(`CREATE TABLE IF NOT EXISTS compliance_fine_rates (
		id          TEXT PRIMARY KEY,
		organization_id INTEGER NOT NULL,
		framework   TEXT NOT NULL,
		per_event_amount DOUBLE PRECISION NOT NULL,
		currency    TEXT NOT NULL,
		updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_by  INTEGER,
		UNIQUE (organization_id, framework)
	)`)
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create compliance_fine_rates: %w", err)
	}

	// Seed defaults for all organizations that have no rates yet.
	seedSQL := formatQuery(`
		INSERT INTO compliance_fine_rates (id, organization_id, framework, per_event_amount, currency)
		SELECT ? || o.id || '_' || fw.framework, o.id, fw.framework, fw.amount, fw.currency
		FROM organizations o
		CROSS JOIN (
			VALUES ('GDPR', 20000.0, 'EUR'), ('HIPAA', 15000.0, 'USD'), ('PCI-DSS', 5000.0, 'USD')
		) AS fw(framework, amount, currency)
		WHERE NOT EXISTS (
			SELECT 1 FROM compliance_fine_rates r
			WHERE r.organization_id = o.id AND r.framework = fw.framework
		)`)
	db.Exec(seedSQL, "cfr_") // ignore error — table may not have organizations yet

	return nil
}

// SeedComplianceFineRates inserts default fine rates for a newly created org.
// It is safe to call if rows already exist (UNIQUE constraint prevents duplicates).
func SeedComplianceFineRates(orgID int) {
	if db == nil {
		return
	}
	for fw, def := range compliance.FineRateDefaults {
		id := fmt.Sprintf("cfr_%d_%s", orgID, fw)
		q := formatQuery(`INSERT INTO compliance_fine_rates (id, organization_id, framework, per_event_amount, currency)
			VALUES (?, ?, ?, ?, ?) ON CONFLICT (organization_id, framework) DO NOTHING`)
		db.Exec(q, id, orgID, fw, def.Amount, def.Currency)
	}
}

// HandleGetComplianceSettings handles GET /api/v1/settings/compliance
func HandleGetComplianceSettings(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	rates, err := getOrgFineRates(orgID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch compliance settings"})
	}

	return c.JSON(ComplianceSettingsResponse{
		FineRates:  rates,
		Disclaimer: complianceDisclaimer,
	})
}

// HandleUpdateComplianceSettings handles PUT /api/v1/settings/compliance
func HandleUpdateComplianceSettings(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok || orgID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	// Require admin role
	user, _ := c.Locals("user").(*userContext)
	if user != nil && user.Role != "admin" && user.Role != "owner" {
		return c.Status(403).JSON(fiber.Map{"error": "Admin role required"})
	}

	var req struct {
		FineRates []FineRate `json:"fine_rates"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	validFrameworks := map[string]bool{"GDPR": true, "HIPAA": true, "PCI-DSS": true}
	validCurrencies := map[string]bool{"EUR": true, "USD": true, "GBP": true, "CAD": true, "AUD": true}

	for _, r := range req.FineRates {
		if !validFrameworks[r.Framework] {
			return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("Unknown framework: %s", r.Framework)})
		}
		if r.PerEventAmount <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "per_event_amount must be > 0"})
		}
		if !validCurrencies[r.Currency] {
			return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("Unsupported currency: %s", r.Currency)})
		}
	}

	for _, r := range req.FineRates {
		id := fmt.Sprintf("cfr_%d_%s", orgID, r.Framework)
		q := formatQuery(`INSERT INTO compliance_fine_rates (id, organization_id, framework, per_event_amount, currency, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (organization_id, framework) DO UPDATE
			SET per_event_amount = EXCLUDED.per_event_amount,
			    currency = EXCLUDED.currency,
			    updated_at = EXCLUDED.updated_at`)
		if _, err := db.Exec(q, id, orgID, r.Framework, r.PerEventAmount, r.Currency, time.Now()); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update compliance settings"})
		}
	}

	rates, err := getOrgFineRates(orgID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch updated settings"})
	}

	return c.JSON(ComplianceSettingsResponse{
		FineRates:  rates,
		Disclaimer: complianceDisclaimer,
	})
}

// getOrgFineRates returns fine rates for an org, falling back to defaults when no rows exist.
func getOrgFineRates(orgID int) ([]FineRate, error) {
	if db == nil {
		return defaultFineRates(), nil
	}

	rows, err := db.Query(formatQuery(
		`SELECT framework, per_event_amount, currency FROM compliance_fine_rates WHERE organization_id = ? ORDER BY framework`),
		orgID)
	if err != nil {
		return defaultFineRates(), nil
	}
	defer rows.Close()

	rateMap := make(map[string]FineRate)
	for rows.Next() {
		var r FineRate
		if err := rows.Scan(&r.Framework, &r.PerEventAmount, &r.Currency); err == nil {
			rateMap[r.Framework] = r
		}
	}

	if len(rateMap) == 0 {
		return defaultFineRates(), nil
	}

	result := make([]FineRate, 0, len(orderedFineRateFrameworks))
	for _, fw := range orderedFineRateFrameworks {
		if r, ok := rateMap[fw]; ok {
			result = append(result, r)
		} else {
			// Fill missing framework with default
			if def, ok := compliance.FineRateDefaults[fw]; ok {
				result = append(result, FineRate{Framework: fw, PerEventAmount: def.Amount, Currency: def.Currency})
			}
		}
	}
	return result, nil
}

func defaultFineRates() []FineRate {
	rates := make([]FineRate, 0, len(orderedFineRateFrameworks))
	for _, fw := range orderedFineRateFrameworks {
		if def, ok := compliance.FineRateDefaults[fw]; ok {
			rates = append(rates, FineRate{Framework: fw, PerEventAmount: def.Amount, Currency: def.Currency})
		}
	}
	return rates
}

// ─── Compliance Summary Widget (Task 4) ──────────────────────────────────────

// ComplianceSummaryResponse is returned by the compliance-summary widget.
type ComplianceSummaryResponse struct {
	TimeRange                      string                `json:"time_range"`
	GeneratedAt                    time.Time             `json:"generated_at"`
	Frameworks                     []FrameworkSummary    `json:"frameworks"`
	TotalEstimatedExposureAvertedUSD float64             `json:"total_estimated_exposure_averted_usd"`
}

// FrameworkSummary holds per-framework compliance metrics.
type FrameworkSummary struct {
	Key              string              `json:"key"`
	Label            string              `json:"label"`
	EventsProtected  int64               `json:"events_protected"`
	Redactions       int64               `json:"redactions"`
	TopEntityTypes   []string            `json:"top_entity_types"`
	Citation         string              `json:"citation"`
	EstimatedExposure *ExposureEstimate  `json:"estimated_exposure_averted"`
}

// ExposureEstimate is the per-framework fine estimate.
type ExposureEstimate struct {
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	PerEventBasis float64 `json:"per_event_basis"`
	Disclaimer   string  `json:"disclaimer"`
}

// handleComplianceSummary is the widget handler for "compliance-summary".
func handleComplianceSummary(c *fiber.Ctx, orgID int) error {
	timeRange := c.Query("timeRange", "24h")
	interval, ok := map[string]string{
		"1h": "1 hour", "24h": "24 hours", "7d": "7 days", "30d": "30 days",
	}[timeRange]
	if !ok {
		interval = "24 hours"
		timeRange = "24h"
	}

	if db == nil {
		return c.JSON(buildEmptyComplianceSummary(timeRange))
	}

	since := time.Now().Add(-parseDuration(interval))

	// Fetch matching events with their metadata and framework tags.
	rows, err := db.Query(formatQuery(
		`SELECT compliance_frameworks, metadata
		 FROM events
		 WHERE organization_id = ? AND timestamp >= ?`),
		orgID, since)
	if err != nil {
		return c.JSON(buildEmptyComplianceSummary(timeRange))
	}
	defer rows.Close()

	type fwStats struct {
		events     int64
		redactions int64
		entities   map[string]int
	}
	stats := make(map[string]*fwStats)

	for rows.Next() {
		var frameworksJSON string
		var metadataJSON string

		if err := rows.Scan(&frameworksJSON, &metadataJSON); err != nil {
			continue
		}

		var frameworks []string
		json.Unmarshal([]byte(frameworksJSON), &frameworks)

		// Extract redaction_count from metadata
		var redactionCount int64
		var meta map[string]json.RawMessage
		if err := json.Unmarshal([]byte(metadataJSON), &meta); err == nil {
			if rcRaw, ok := meta["redaction_count"]; ok {
				var rc float64
				if json.Unmarshal(rcRaw, &rc) == nil {
					redactionCount = int64(rc)
				}
			}
		}

		// Extract entity types from metadata (meta already parsed above)
		var entityTypes []string
		if meta != nil {
			if rdRaw, ok := meta["redaction_details"]; ok {
				var rds []map[string]json.RawMessage
				if err := json.Unmarshal(rdRaw, &rds); err == nil {
					for _, rd := range rds {
						if etRaw, ok := rd["entity_type"]; ok {
							var et string
							json.Unmarshal(etRaw, &et)
							if et != "" {
								entityTypes = append(entityTypes, et)
							}
						}
					}
				}
			}
		}

		for _, fw := range frameworks {
			if stats[fw] == nil {
				stats[fw] = &fwStats{entities: make(map[string]int)}
			}
			stats[fw].events++
			stats[fw].redactions += redactionCount
			for _, et := range entityTypes {
				// Only count entity types relevant to this framework
				for _, fwForEt := range compliance.FrameworksForEntityType(et) {
					if fwForEt == fw {
						stats[fw].entities[et]++
					}
				}
			}
		}
	}

	// Load org fine rates for exposure estimates
	fineRates, _ := getOrgFineRates(orgID)
	rateMap := make(map[string]FineRate)
	for _, r := range fineRates {
		rateMap[r.Framework] = r
	}

	// Build response
	allFrameworks := []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"}
	summaries := make([]FrameworkSummary, 0, len(allFrameworks))
	var totalUSD float64

	for _, fw := range allFrameworks {
		s := stats[fw]
		var eventsProtected, redactions int64
		var topEntityTypes []string
		if s != nil {
			eventsProtected = s.events
			redactions = s.redactions
			topEntityTypes = topN(s.entities, 3)
		}
		if topEntityTypes == nil {
			topEntityTypes = []string{}
		}

		var exposure *ExposureEstimate
		if fw != "SOC2" {
			rate, hasRate := rateMap[fw]
			if !hasRate {
				if def, ok := compliance.FineRateDefaults[fw]; ok {
					rate = FineRate{Framework: fw, PerEventAmount: def.Amount, Currency: def.Currency}
				}
			}
			amount := rate.PerEventAmount * float64(eventsProtected)
			exposure = &ExposureEstimate{
				Amount:        amount,
				Currency:      rate.Currency,
				PerEventBasis: rate.PerEventAmount,
				Disclaimer:    "estimate",
			}
			// Convert to USD for total
			usd := amount
			if rate.Currency == "EUR" {
				usd = amount * compliance.EURtoUSD
			}
			totalUSD += usd
		}

		summaries = append(summaries, FrameworkSummary{
			Key:              fw,
			Label:            fw,
			EventsProtected:  eventsProtected,
			Redactions:       redactions,
			TopEntityTypes:   topEntityTypes,
			Citation:         compliance.Citations[fw],
			EstimatedExposure: exposure,
		})
	}

	return c.JSON(ComplianceSummaryResponse{
		TimeRange:                      timeRange,
		GeneratedAt:                    time.Now(),
		Frameworks:                     summaries,
		TotalEstimatedExposureAvertedUSD: totalUSD,
	})
}

func buildEmptyComplianceSummary(timeRange string) ComplianceSummaryResponse {
	summaries := make([]FrameworkSummary, 0, 4)
	for _, fw := range []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"} {
		var exposure *ExposureEstimate
		if fw != "SOC2" {
			def := compliance.FineRateDefaults[fw]
			exposure = &ExposureEstimate{Amount: 0, Currency: def.Currency, PerEventBasis: def.Amount, Disclaimer: "estimate"}
		}
		summaries = append(summaries, FrameworkSummary{
			Key: fw, Label: fw, TopEntityTypes: []string{},
			Citation: compliance.Citations[fw], EstimatedExposure: exposure,
		})
	}
	return ComplianceSummaryResponse{TimeRange: timeRange, GeneratedAt: time.Now(), Frameworks: summaries}
}

// parseDuration converts "1 hour", "7 days" etc. into a time.Duration.
func parseDuration(s string) time.Duration {
	switch s {
	case "1 hour":
		return time.Hour
	case "24 hours":
		return 24 * time.Hour
	case "7 days":
		return 7 * 24 * time.Hour
	case "30 days":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// topN returns the top-n keys from the counts map, sorted descending by count.
func topN(counts map[string]int, n int) []string {
	type kv struct {
		key   string
		count int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	// Simple selection sort for small maps
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].count > pairs[i].count {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	result := make([]string, 0, n)
	for i := 0; i < n && i < len(pairs); i++ {
		result = append(result, pairs[i].key)
	}
	return result
}

// userContext is a minimal type used to check the caller's role in compliance endpoints.
// The full auth.UserProfile is stored in c.Locals("user") but as an interface{}.
type userContext struct {
	Role string
}
