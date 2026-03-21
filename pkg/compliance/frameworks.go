// Package compliance provides authoritative mapping from detected entity types
// to regulatory compliance frameworks, and utilities for computing per-event
// framework attribution.
package compliance

import (
	"strings"

	"github.com/ContinuumSolutions/nonym/pkg/ner"
)

// Citations maps each framework to its primary regulatory citation.
var Citations = map[string]string{
	"GDPR":    "Art. 5(1)(c) · Data Minimisation",
	"HIPAA":   "45 CFR § 164 · PHI Safeguards",
	"PCI-DSS": "Req. 3.3 · Cardholder Data Protection",
	"SOC2":    "CC6 · Logical Access Controls",
}

// FineRateDefault holds the default per-event fine estimate for a framework.
type FineRateDefault struct {
	Amount   float64
	Currency string
}

// FineRateDefaults are the system-wide defaults used when an organization has
// not configured custom rates. SOC2 has no statutory fine so it is omitted.
var FineRateDefaults = map[string]FineRateDefault{
	"GDPR":    {20000, "EUR"},
	"HIPAA":   {15000, "USD"},
	"PCI-DSS": {5000, "USD"},
}

// EURtoUSD is the fixed conversion rate used for total-exposure rollup.
const EURtoUSD = 1.08

// orderedFrameworks defines the canonical display order.
var orderedFrameworks = []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"}

// entityFrameworkMap is the authoritative mapping from canonical entity type
// (UPPER_SNAKE_CASE) to the set of compliance frameworks that govern it.
//
// Sources:
//   - GDPR: Article 5(1)(c) Data Minimisation; Article 4(1) Personal Data
//   - HIPAA: 45 CFR § 164.514(b)(2) 18 PHI identifiers; § 164.312 Technical Safeguards
//   - PCI-DSS v4.0: Requirement 3.3 Sensitive Auth Data; Req 3.4 PAN Protection
//   - SOC2: CC6 Logical and Physical Access; CC9 Risk Mitigation
var entityFrameworkMap = map[string][]string{
	"PERSON":       {"GDPR", "HIPAA", "SOC2"},
	"EMAIL":        {"GDPR", "HIPAA", "SOC2"},
	"PHONE":        {"GDPR", "HIPAA"},
	"SSN":          {"GDPR", "HIPAA", "SOC2"},
	"NIN":          {"GDPR", "SOC2"},
	"ADDRESS":      {"GDPR", "HIPAA"},
	"LOCATION":     {"GDPR", "HIPAA"}, // legacy alias for ADDRESS
	"ORGANIZATION": {"GDPR", "SOC2"},
	"DATE":         {"GDPR", "HIPAA"},
	"CREDIT_CARD":  {"GDPR", "PCI-DSS", "SOC2"},
	"CARD_CVV":     {"GDPR", "PCI-DSS", "SOC2"},
	"IBAN":         {"GDPR", "PCI-DSS", "SOC2"},
	"IP_ADDRESS":   {"GDPR", "SOC2"},
}

// FrameworksForEntityType returns the compliance frameworks that apply to the
// given entity type. The entityType is normalized to UPPER_SNAKE_CASE before
// lookup. Returns nil when the type is not in the mapping.
func FrameworksForEntityType(entityType string) []string {
	return entityFrameworkMap[strings.ToUpper(entityType)]
}

// FrameworksForEvent returns a deduplicated, deterministically ordered slice of
// framework names that apply to any of the detected entity types in the event.
func FrameworksForEvent(redactionDetails []ner.RedactionDetail) []string {
	seen := make(map[string]bool)
	for _, d := range redactionDetails {
		for _, fw := range FrameworksForEntityType(string(d.EntityType)) {
			seen[fw] = true
		}
	}
	var result []string
	for _, fw := range orderedFrameworks {
		if seen[fw] {
			result = append(result, fw)
		}
	}
	return result
}
