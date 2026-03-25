package scanner

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
)

// AiTrafficEntry summarises proxy traffic and PII exposure for one AI vendor.
type AiTrafficEntry struct {
	VendorID         string `json:"vendorId"`
	VendorName       string `json:"vendorName"`
	PromptCount      int    `json:"promptCount"`
	PIIDetectedCount int    `json:"piiDetectedCount"`
	PIIRedactedCount int    `json:"piiRedactedCount"`
	// baaStatus: "signed" | "missing" | "not_applicable"
	// "not_applicable" for non-HIPAA vendors; "signed"/"missing" driven by DPA registry.
	BAAStatus string `json:"baaStatus"`
	Period    string `json:"period"`
}

// HandleGetAITraffic returns aggregated AI proxy traffic per vendor for the period.
// GET /api/v1/scanner/ai-traffic?period=30d
func HandleGetAITraffic(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	period := c.Query("period", "30d")
	days, err := parsePeriodDays(period)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "period must be 7d, 30d, or 90d"})
	}

	entries, err := queryAITraffic(orgID, days, period)
	if err != nil {
		log.Printf("scanner: HandleGetAITraffic: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch AI traffic"})
	}
	return c.JSON(entries)
}

// ── DB query ──────────────────────────────────────────────────────────────────

func queryAITraffic(orgID, days int, period string) ([]AiTrafficEntry, error) {
	// AI provider names → display names (matches values stored in transactions.provider).
	providerNames := map[string]string{
		"openai":    "OpenAI",
		"anthropic": "Anthropic",
		"google":    "Google AI",
		"mistral":   "Mistral",
		"cohere":    "Cohere",
		"local":     "Local / Self-hosted",
	}

	q := fmt.Sprintf(`
		SELECT
			provider,
			COUNT(*)                                                      AS prompt_count,
			COUNT(CASE WHEN redaction_count > 0 THEN 1 END)              AS pii_detected_count,
			COALESCE(SUM(redaction_count), 0)                            AS pii_redacted_count
		FROM transactions
		WHERE organization_id = ?
		  AND provider IS NOT NULL
		  AND provider != ''
		  AND created_at >= %s
		GROUP BY provider
		ORDER BY prompt_count DESC
	`, sinceExpr(days))

	rows, err := db.Query(formatQuery(q), orgID)
	if err != nil {
		// Transactions table may not exist in test environments.
		log.Printf("scanner: queryAITraffic: %v", err)
		return []AiTrafficEntry{}, nil
	}
	defer rows.Close()

	dpaMap := getDPAStatusMap(orgID)

	// HIPAA-relevant AI vendors (receiving PHI) require a BAA.
	hipaaVendors := map[string]bool{"openai": true, "anthropic": true, "google": true}

	var out []AiTrafficEntry
	for rows.Next() {
		var e AiTrafficEntry
		if err := rows.Scan(&e.VendorID, &e.PromptCount, &e.PIIDetectedCount, &e.PIIRedactedCount); err != nil {
			continue
		}
		if name, ok := providerNames[e.VendorID]; ok {
			e.VendorName = name
		} else {
			e.VendorName = e.VendorID
		}
		e.Period = period

		// BAA status: only applicable for HIPAA-relevant AI vendors.
		if hipaaVendors[e.VendorID] {
			if dpaStatus, ok := dpaMap[e.VendorID]; ok && dpaStatus == "signed" {
				e.BAAStatus = "signed"
			} else {
				e.BAAStatus = "missing"
			}
		} else {
			e.BAAStatus = "not_applicable"
		}

		out = append(out, e)
	}
	if out == nil {
		out = []AiTrafficEntry{}
	}
	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parsePeriodDays(period string) (int, error) {
	switch period {
	case "7d":
		return 7, nil
	case "30d":
		return 30, nil
	case "90d":
		return 90, nil
	default:
		return 0, fmt.Errorf("invalid period %q", period)
	}
}
