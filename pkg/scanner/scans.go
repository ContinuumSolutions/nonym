package scanner

import (
	"bufio"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HandleListScans returns scans for the org.
// GET /api/v1/scans
func HandleListScans(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}

	scans, err := listScans(orgID, limit, offset)
	if err != nil {
		log.Printf("scanner: HandleListScans: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch scans"})
	}
	return c.JSON(fiber.Map{"scans": scans, "total": len(scans)})
}

// HandleCreateScan starts a new scan across all connected vendors (or a subset).
// POST /api/v1/scans
func HandleCreateScan(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var req struct {
		VendorIDs []string `json:"vendor_ids"` // empty = all connected vendors
	}
	_ = c.BodyParser(&req)

	// Fetch connected vendor connections.
	connections, err := listVendorConnections(orgID, "connected")
	if err != nil {
		log.Printf("scanner: HandleCreateScan: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch vendor connections"})
	}

	// Filter to requested vendor IDs if provided.
	if len(req.VendorIDs) > 0 {
		wanted := map[string]bool{}
		for _, id := range req.VendorIDs {
			wanted[id] = true
		}
		filtered := []VendorConnection{}
		for _, vc := range connections {
			if wanted[vc.ID] || wanted[vc.Vendor] {
				filtered = append(filtered, vc)
			}
		}
		connections = filtered
	}

	if len(connections) == 0 {
		return c.Status(422).JSON(fiber.Map{"error": "No connected vendors available for scanning"})
	}

	vendorNames := make([]string, len(connections))
	for i, vc := range connections {
		vendorNames[i] = vc.Vendor
	}

	scan, err := startScan(orgID, vendorNames, connections, "manual")
	if err != nil {
		log.Printf("scanner: HandleCreateScan startScan: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to start scan"})
	}
	return c.Status(202).JSON(fiber.Map{"scan_id": scan.ID})
}

// HandleGetScan returns a single scan.
// GET /api/v1/scans/:id
func HandleGetScan(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	scan, err := getScan(orgID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Scan not found"})
	}
	return c.JSON(scan)
}

// HandleScanStatus streams SSE progress events for a scan.
// GET /api/v1/scans/:id/status
func HandleScanStatus(c *fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	scan, err := getScan(orgID, c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Scan not found"})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send current status immediately.
		switch scan.Status {
		case "done":
			fmt.Fprintf(w, "data: {\"event\":\"done\",\"findings_count\":%d,\"scan_id\":\"%s\"}\n\n",
				scan.FindingsCount, scan.ID)
		case "failed":
			fmt.Fprintf(w, "data: {\"event\":\"error\",\"message\":\"%s\",\"scan_id\":\"%s\"}\n\n",
				strings.ReplaceAll(scan.ErrorMessage, `"`, `\"`), scan.ID)
		case "running":
			fmt.Fprintf(w, "data: {\"event\":\"progress\",\"phase\":\"running\",\"percent\":50,\"scan_id\":\"%s\"}\n\n",
				scan.ID)
		default:
			fmt.Fprintf(w, "data: {\"event\":\"progress\",\"phase\":\"pending\",\"percent\":0,\"scan_id\":\"%s\"}\n\n",
				scan.ID)
		}
		w.Flush()
	})
	return nil
}

// ── Scan orchestration ────────────────────────────────────────────────────────

// sseEvent is sent during an async scan.
type sseEvent struct {
	Type    string
	Payload string
}

// activeScanChannels allows the SSE endpoint to stream events from running scans.
var (
	activeScanMu       sync.RWMutex
	activeScanChannels = map[string]chan sseEvent{}
)

// startScan creates a scan record and runs the scan pipeline in the background.
func startScan(orgID int, vendorNames []string, connections []VendorConnection, triggeredBy string) (*Scan, error) {
	now := time.Now()
	scan := &Scan{
		ID:          newID(),
		OrgID:       orgID,
		VendorIDs:   vendorNames,
		Status:      "pending",
		TriggeredBy: triggeredBy,
		CreatedAt:   now,
	}
	if err := insertScan(scan); err != nil {
		return nil, err
	}

	// Register SSE channel before goroutine starts.
	ch := make(chan sseEvent, 50)
	activeScanMu.Lock()
	activeScanChannels[scan.ID] = ch
	activeScanMu.Unlock()

	go runScan(scan, connections, ch)
	return scan, nil
}

// runScan executes the scan pipeline for all provided vendor connections.
func runScan(scan *Scan, connections []VendorConnection, events chan<- sseEvent) {
	defer func() {
		activeScanMu.Lock()
		delete(activeScanChannels, scan.ID)
		activeScanMu.Unlock()
		close(events)
	}()

	startedAt := time.Now()
	updateScanStatus(scan.ID, "running", 0, &startedAt, nil, "")
	events <- sseEvent{Type: "progress", Payload: `{"event":"progress","phase":"started","percent":0}`}

	totalFindings := 0
	var scanErr string

	for i, vc := range connections {
		percent := (i * 80) / len(connections)
		events <- sseEvent{Type: "progress", Payload: fmt.Sprintf(
			`{"event":"progress","vendor":%q,"phase":"fetching","percent":%d}`, vc.Vendor, percent)}

		// Mark vendor as scanning.
		updateVendorConnectionStatus(vc.ID, "scanning", "", vc.ConnectedAt, vc.LastScanAt)

		findings, err := scanVendor(scan, &vc)
		if err != nil {
			scanErr = err.Error()
			updateVendorConnectionStatus(vc.ID, "error", scanErr, vc.ConnectedAt, vc.LastScanAt)
			continue
		}

		now := time.Now()
		updateVendorConnectionStatus(vc.ID, "connected", "", vc.ConnectedAt, &now)

		for _, f := range findings {
			totalFindings++
			events <- sseEvent{Type: "finding", Payload: fmt.Sprintf(
				`{"event":"finding","finding_id":%q,"vendor":%q,"risk_level":%q}`,
				f.ID, f.Vendor, f.RiskLevel)}
		}
	}

	completedAt := time.Now()
	finalStatus := "done"
	if scanErr != "" && totalFindings == 0 {
		finalStatus = "failed"
	}
	updateScanStatus(scan.ID, finalStatus, totalFindings, &startedAt, &completedAt, scanErr)

	events <- sseEvent{Type: "done", Payload: fmt.Sprintf(
		`{"event":"done","findings_count":%d,"scan_id":%q}`, totalFindings, scan.ID)}
}

// scanVendor fetches events from a vendor, detects PII, and persists findings.
// In Phase 1, it operates on sample/test data from vendor credentials.
// Replace with real HTTP client calls per connector in Phase 2.
func scanVendor(scan *Scan, vc *VendorConnection) ([]Finding, error) {
	// Build normalised events from stored credentials/settings context.
	// This is a Phase-1 placeholder: in production, each vendor connector
	// calls the vendor API and returns real events.
	sampleTexts := extractSampleTexts(vc)

	var findings []Finding
	for _, event := range sampleTexts {
		detections := Detect(event.Text)
		for _, d := range detections {
			// Deduplication: check for existing open finding with same signature.
			existingID, err := deduplicateFinding(scan.OrgID, vc.Vendor, d.DataType, event.Source, event.Metadata["endpoint"])
			if err != nil || existingID != "" {
				continue // already counted
			}

			f := buildFinding(scan, vc, event, d)
			if err := insertFinding(&f); err == nil {
				findings = append(findings, f)
			}
		}
	}
	return findings, nil
}

// extractSampleTexts builds NormalizedEvents from the vendor connection's stored context.
// This stub is replaced by real connector calls in Phase 2.
func extractSampleTexts(vc *VendorConnection) []NormalizedEvent {
	var events []NormalizedEvent
	for k, v := range vc.Credentials {
		if s, ok := v.(string); ok {
			events = append(events, NormalizedEvent{
				VendorID: vc.Vendor,
				EventID:  "cred:" + k,
				Source:   "credentials." + k,
				Text:     s,
				Metadata: map[string]string{"endpoint": ""},
			})
		}
	}
	for k, v := range vc.Settings {
		if s, ok := v.(string); ok {
			events = append(events, NormalizedEvent{
				VendorID: vc.Vendor,
				EventID:  "settings:" + k,
				Source:   "settings." + k,
				Text:     s,
				Metadata: map[string]string{"endpoint": ""},
			})
		}
	}
	return events
}

// buildFinding constructs a Finding from a Detection.
func buildFinding(scan *Scan, vc *VendorConnection, event NormalizedEvent, d Detection) Finding {
	now := time.Now()
	title := fmt.Sprintf("%s detected in %s via %s",
		strings.ToUpper(d.DataType), event.Source, vc.Vendor)
	description := fmt.Sprintf(
		"Rule %q matched at %s. Sample (masked): %s",
		d.RuleID, event.Source, d.Masked)

	return Finding{
		ID:                 newID(),
		OrgID:              scan.OrgID,
		ScanID:             scan.ID,
		VendorConnectionID: vc.ID,
		Vendor:             vc.Vendor,
		DataType:           d.DataType,
		RiskLevel:          DetectionRiskLevel(d, true),
		Title:              title,
		Description:        description,
		Location:           event.Source,
		Endpoint:           event.Metadata["endpoint"],
		Occurrences:        1,
		SampleMasked:       d.Masked,
		Status:             "open",
		ComplianceImpact:   ComplianceFor(d.DataType),
		Fixes:              FixesFor(vc.Vendor, d.RuleID),
		FirstSeenAt:        now,
		LastSeenAt:         now,
		CreatedAt:          now,
	}
}
