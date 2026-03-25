package scanner

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

// HandleScannerOverview returns a high-level summary of the org's scanner state.
// GET /api/v1/scanner/overview
func HandleScannerOverview(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	// Count connected vendors.
	connections, err := listVendorConnections(orgID, "connected")
	if err != nil {
		log.Printf("scanner: HandleScannerOverview listVendorConnections: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch vendor connections"})
	}

	// Finding counts.
	fc, err := findingCounts(orgID)
	if err != nil {
		log.Printf("scanner: HandleScannerOverview findingCounts: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch finding counts"})
	}

	// Last scan timestamp.
	scans, err := listScans(orgID, 1, 0)
	var lastScanAt *string
	if err == nil && len(scans) > 0 {
		if scans[0].CompletedAt != nil {
			s := scans[0].CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			lastScanAt = &s
		}
	}

	// Compliance snapshot.
	compliance := buildComplianceSnapshot(orgID)

	overview := fiber.Map{
		"vendors_connected": len(connections),
		"findings":          fc,
		"risk_score":        OrgRiskScore(fc),
		"compliance":        compliance,
		"last_scan_at":      lastScanAt,
	}
	return c.JSON(overview)
}

// HandleScannerFlows returns data-flow nodes and edges.
// GET /api/v1/scanner/flows
func HandleScannerFlows(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	connections, err := listVendorConnections(orgID, "")
	if err != nil {
		log.Printf("scanner: HandleScannerFlows: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch vendor connections"})
	}

	// App node (the user's application).
	nodes := []FlowNode{
		{ID: "app", Label: "Your Application", Type: "app"},
	}
	var edges []FlowEdge

	for _, vc := range connections {
		nodes = append(nodes, FlowNode{
			ID:    vc.ID,
			Label: vc.DisplayName,
			Type:  "vendor",
		})

		// Aggregate findings for this vendor connection.
		filter := FindingFilter{
			OrgID:  orgID,
			Vendor: vc.Vendor,
			Status: "open",
			Limit:  1000,
			Offset: 0,
		}
		findings, _ := listFindings(filter)
		count := len(findings)
		riskLevel := "low"
		for _, f := range findings {
			if f.RiskLevel == "high" {
				riskLevel = "high"
				break
			}
			if f.RiskLevel == "medium" {
				riskLevel = "medium"
			}
		}

		edges = append(edges, FlowEdge{
			From:          "app",
			To:            vc.ID,
			FindingsCount: count,
			RiskLevel:     riskLevel,
		})
	}

	if edges == nil {
		edges = []FlowEdge{}
	}

	return c.JSON(fiber.Map{
		"nodes": nodes,
		"edges": edges,
	})
}

// buildComplianceSnapshot counts open findings per framework.
func buildComplianceSnapshot(orgID int) ComplianceSnapshot {
	frameworks := map[string]*int{
		"GDPR":  new(int),
		"SOC2":  new(int),
		"HIPAA": new(int),
	}

	filter := FindingFilter{OrgID: orgID, Status: "open", Limit: 1000, Offset: 0}
	findings, _ := listFindings(filter)

	for _, f := range findings {
		for _, ci := range f.ComplianceImpact {
			if ptr, ok := frameworks[ci.Framework]; ok {
				*ptr++
			}
		}
	}

	statusFor := func(count int) string {
		if count == 0 {
			return "ok"
		}
		if count <= 5 {
			return "warning"
		}
		return "critical"
	}

	return ComplianceSnapshot{
		GDPR:  FrameworkStatus{Violations: *frameworks["GDPR"], Status: statusFor(*frameworks["GDPR"])},
		SOC2:  FrameworkStatus{Violations: *frameworks["SOC2"], Status: statusFor(*frameworks["SOC2"])},
		HIPAA: FrameworkStatus{Violations: *frameworks["HIPAA"], Status: statusFor(*frameworks["HIPAA"])},
	}
}
