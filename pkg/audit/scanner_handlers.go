package audit

import (
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ContinuumSolutions/nonym/pkg/router"
)

// ScanFinding represents a detected vendor SDK configuration in the user's
// environment that is not yet protected by Nonym.
type ScanFinding struct {
	VendorID    string `json:"vendor_id"`
	VendorName  string `json:"vendor_name"`
	Indicator   string `json:"indicator"`   // what was detected (env var name, etc.)
	Value       string `json:"value"`       // partially redacted detected value
	Protected   bool   `json:"protected"`   // true if Nonym is already intercepting
	Risk        string `json:"risk"`        // "high", "medium", "low"
	Description string `json:"description"`
}

// ScanResult is the response for the vendor scanner endpoint.
type ScanResult struct {
	ScannedAt    time.Time     `json:"scanned_at"`
	TotalVendors int           `json:"total_vendors"`
	Findings     []ScanFinding `json:"findings"`
	Protected    int           `json:"protected"`
	Unprotected  int           `json:"unprotected"`
	Instructions string        `json:"instructions"`
}

// sentry DSN pattern: https://<key>@<host>/<project>
var sentryDSNPattern = regexp.MustCompile(`https?://[a-f0-9]+@[^/]+/\d+`)

// HandleScanVendors scans the provided configuration payload for known vendor
// SDK fingerprints and reports whether each vendor is routed through Nonym.
// POST /api/v1/scanner
//
// Request body (JSON):
//
//	{
//	  "env":    { "SENTRY_DSN": "https://...", "DD_API_KEY": "..." },
//	  "config": { "sentryDsn": "https://...", "posthogKey": "..." }
//	}
//
// All keys/values are inspected client-side; nothing is persisted.
func HandleScanVendors(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}

	var payload struct {
		Env    map[string]string `json:"env"`
		Config map[string]string `json:"config"`
	}
	_ = c.BodyParser(&payload)

	// Merge all submitted key/value pairs for scanning.
	combined := make(map[string]string)
	for k, v := range payload.Env {
		combined[k] = v
	}
	for k, v := range payload.Config {
		combined[k] = v
	}

	findings := scanForVendors(combined)
	protected := 0
	for _, f := range findings {
		if f.Protected {
			protected++
		}
	}

	result := ScanResult{
		ScannedAt:    time.Now(),
		TotalVendors: len(router.VendorCatalog),
		Findings:     findings,
		Protected:    protected,
		Unprotected:  len(findings) - protected,
		Instructions: "For each unprotected vendor, visit POST /api/v1/vendors/setup to configure Nonym protection.",
	}
	return c.JSON(result)
}

// HandleScanSentry is the free-tier Sentry-specific scanner.
// GET /api/v1/scanner/sentry
//
// Query params: dsn=<sentry-dsn>  (optional; used to check if already proxied)
func HandleScanSentry(c *fiber.Ctx) error {
	dsn := c.Query("dsn")

	var finding *ScanFinding
	if dsn != "" {
		if sentryDSNPattern.MatchString(dsn) {
			protected := strings.Contains(dsn, "nonym") || c.Query("proxied") == "true"
			finding = &ScanFinding{
				VendorID:   "sentry",
				VendorName: "Sentry",
				Indicator:  "SENTRY_DSN",
				Value:      maskDSN(dsn),
				Protected:  protected,
				Risk:       "high",
				Description: "Sentry captures exception messages, stack traces, and request/response data. " +
					"Without Nonym, PII such as names, emails, and API keys is sent to sentry.io.",
			}
		}
	}

	profile := router.GetVendorProfile("sentry")
	resp := fiber.Map{
		"vendor":  profile,
		"finding": finding,
		"setup_guide": fiber.Map{
			"proxy_method": fiber.Map{
				"description": "Replace your Sentry DSN host with your Nonym gateway endpoint.",
				"steps": []string{
					"1. Note your current SENTRY_DSN value.",
					"2. Replace the ingest host with your Nonym gateway: https://<key>@<nonym-host>/vendor-proxy/sentry/<project>",
					"3. Nonym will redact PII from error payloads and forward clean events to sentry.io.",
				},
			},
			"sdk_method": fiber.Map{
				"description": "Use @nonym/sentry as a drop-in replacement for @sentry/node or @sentry/browser.",
				"steps": []string{
					"1. npm install @nonym/sentry",
					"2. Replace `import * as Sentry from '@sentry/node'` with `import * as Sentry from '@nonym/sentry'`",
					"3. Set NONYM_API_KEY environment variable.",
					"4. Initialize as normal — Nonym intercepts beforeSend automatically.",
				},
			},
		},
	}
	return c.JSON(resp)
}

// scanForVendors inspects a flat map of environment/config keys and values,
// returning a finding for each vendor whose fingerprint is detected.
func scanForVendors(kv map[string]string) []ScanFinding {
	var findings []ScanFinding

	// Per-vendor fingerprint rules: env var prefixes and config key patterns.
	fingerprints := []struct {
		vendorID string
		keys     []string
		risk     string
	}{
		{"sentry", []string{"SENTRY_", "sentryDsn", "sentryKey", "sentryDSN"}, "high"},
		{"datadog", []string{"DD_", "DATADOG_", "datadogApiKey", "ddApiKey"}, "high"},
		{"posthog", []string{"POSTHOG_", "posthogKey", "posthogApiKey", "NEXT_PUBLIC_POSTHOG"}, "medium"},
		{"mixpanel", []string{"MIXPANEL_", "mixpanelToken"}, "medium"},
		{"newrelic", []string{"NEW_RELIC_", "NEWRELIC_", "newRelicKey"}, "high"},
	}

	for _, fp := range fingerprints {
		for key, val := range kv {
			if matchesFingerprint(key, fp.keys) {
				profile := router.GetVendorProfile(fp.vendorID)
				name := fp.vendorID
				if profile != nil {
					name = profile.Name
				}
				protected := strings.Contains(strings.ToLower(val), "nonym")
				findings = append(findings, ScanFinding{
					VendorID:    fp.vendorID,
					VendorName:  name,
					Indicator:   key,
					Value:       maskValue(val),
					Protected:   protected,
					Risk:        fp.risk,
					Description: buildFindingDescription(fp.vendorID, name, protected),
				})
				break // one finding per vendor
			}
		}
	}
	return findings
}

func matchesFingerprint(key string, patterns []string) bool {
	upper := strings.ToUpper(key)
	for _, p := range patterns {
		if strings.HasPrefix(upper, strings.ToUpper(p)) || strings.EqualFold(key, p) {
			return true
		}
	}
	return false
}

func maskValue(v string) string {
	if len(v) <= 8 {
		return strings.Repeat("*", len(v))
	}
	return v[:4] + strings.Repeat("*", len(v)-8) + v[len(v)-4:]
}

func maskDSN(dsn string) string {
	// Keep the host visible, redact the key portion.
	if idx := strings.Index(dsn, "@"); idx > 8 {
		return dsn[:8] + strings.Repeat("*", idx-8) + dsn[idx:]
	}
	return maskValue(dsn)
}

func buildFindingDescription(vendorID, vendorName string, protected bool) string {
	if protected {
		return vendorName + " is detected and appears to be routed through Nonym."
	}
	switch vendorID {
	case "sentry":
		return "Sentry is configured but not protected. Exception data including emails, IPs, and request bodies is being sent to sentry.io without PII redaction."
	case "datadog":
		return "Datadog is configured but not protected. APM traces and log data containing PII is being forwarded to datadoghq.com."
	case "posthog":
		return "PostHog is configured but not protected. User analytics events and person properties containing emails and IPs are being sent to posthog.com."
	default:
		return vendorName + " is configured but not protected by Nonym."
	}
}
