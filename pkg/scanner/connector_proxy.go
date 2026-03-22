package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// proxyVendors is the set of vendor slugs that are served via Nonym's own proxy.
// Their scan data comes from the transactions table, not from an external API call.
// These vendors are handled on the AI proxy side and are not available as scanner
// vendor connections.
var proxyVendors = map[string]bool{
	"openai":    true,
	"anthropic": true,
}

// IsProxyVendor reports whether a vendor slug is an AI proxy vendor.
// Proxy vendors cannot be added as scanner vendor connections.
func IsProxyVendor(vendor string) bool {
	return proxyVendors[vendor]
}

// ── Connector ─────────────────────────────────────────────────────────────────

type proxyConnector struct {
	vendor string
}

func (p *proxyConnector) Vendor() string { return p.vendor }

// FetchEvents queries the transactions table for recent proxy traffic for this
// vendor and converts each NER redaction detail into a NormalizedEvent whose
// PreDetected field carries the already-identified detection so that Detect()
// is skipped — avoiding double-counting and catching entity types (PERSON,
// LOCATION, …) that the scanner's regex engine does not handle.
func (p *proxyConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialised")
	}

	const lookbackDays = 30
	const maxRows = 500

	since := time.Now().AddDate(0, 0, -lookbackDays)

	rows, err := db.Query(formatQuery(`
		SELECT COALESCE(request_id, CAST(id AS TEXT)),
		       COALESCE(path, '/v1/chat/completions'),
		       entities_detected
		FROM   transactions
		WHERE  organization_id = ?
		  AND  provider        = ?
		  AND  redaction_count > 0
		  AND  created_at     >= ?
		ORDER  BY created_at DESC
		LIMIT  ?
	`), vc.OrgID, p.vendor, since, maxRows)
	if err != nil {
		return nil, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	var events []NormalizedEvent
	for rows.Next() {
		var txID, path, entitiesJSON string
		if err := rows.Scan(&txID, &path, &entitiesJSON); err != nil {
			log.Printf("proxy connector (%s): scan row: %v", p.vendor, err)
			continue
		}

		var rawEntities []proxyEntity
		if err := json.Unmarshal([]byte(entitiesJSON), &rawEntities); err != nil {
			log.Printf("proxy connector (%s): unmarshal entities tx=%s: %v", p.vendor, txID, err)
			continue
		}

		for i, e := range rawEntities {
			dt := nerEntityToDataType(e.EntityType)
			if dt == "" {
				continue // entity type we don't track
			}
			det := Detection{
				DataType:   dt,
				Value:      e.OriginalText,
				Masked:     maskValue(dt, e.OriginalText),
				Confidence: e.Confidence,
				RuleID:     "ner_" + e.EntityType,
				RiskLevel:  dataTypeRiskLevel(dt),
			}
			events = append(events, NormalizedEvent{
				VendorID: p.vendor,
				EventID:  fmt.Sprintf("%s_entity_%d", txID, i),
				Source:   fmt.Sprintf("proxy.request.%s", e.EntityType),
				Text:     e.OriginalText,
				Metadata: map[string]string{
					"endpoint":    path,
					"tx_id":       txID,
					"entity_type": e.EntityType,
				},
				PreDetected: []Detection{det},
			})
		}
	}

	return events, rows.Err()
}

// ── Minimal struct for parsing entities_detected JSON ─────────────────────────

// proxyEntity mirrors the fields of ner.RedactionDetail that we care about,
// without importing pkg/ner (which would create a dependency cycle risk).
type proxyEntity struct {
	EntityType   string  `json:"entity_type"`
	OriginalText string  `json:"original_text"`
	Confidence   float64 `json:"confidence"`
}

// ── Mappings ──────────────────────────────────────────────────────────────────

// nerEntityToDataType maps NER engine entity type strings to scanner data types.
// Returns "" for entity types we do not create findings for.
func nerEntityToDataType(entityType string) string {
	switch entityType {
	case "EMAIL":
		return "email"
	case "PHONE":
		return "phone"
	case "SSN", "NIN", "CREDIT_CARD", "CARD_CVV", "IBAN":
		return "financial"
	case "IP_ADDRESS":
		return "ip_address"
	case "API_KEY", "PASSWORD":
		return "api_key"
	case "JWT", "BEARER_TOKEN":
		return "token"
	case "PERSON", "LOCATION", "ADDRESS", "ORGANIZATION":
		return "name"
	case "DATE":
		return "" // dates alone are not PII findings
	default:
		return ""
	}
}

// dataTypeRiskLevel returns the default risk level for a scanner data type.
func dataTypeRiskLevel(dataType string) string {
	switch dataType {
	case "financial", "api_key", "token":
		return "high"
	case "email", "phone", "name":
		return "high"
	case "ip_address":
		return "medium"
	default:
		return "low"
	}
}
