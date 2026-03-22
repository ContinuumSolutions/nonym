package scanner

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Detection is a single PII/sensitive-data match found in a text.
type Detection struct {
	DataType   string
	Value      string  // raw matched value (not persisted; masked before storage)
	Masked     string  // masked representation, e.g. "j***@example.com"
	Confidence float64 // 0.0–1.0
	RuleID     string
	RiskLevel  string
}

// rule is an internal detection rule.
type rule struct {
	id         string
	pattern    *regexp.Regexp
	dataType   string
	riskLevel  string
	confidence float64
}

var detectionRules []*rule

func init() {
	detectionRules = []*rule{
		{
			id:         "email",
			pattern:    regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`),
			dataType:   "email",
			riskLevel:  "high",
			confidence: 0.95,
		},
		{
			id:         "phone_e164",
			pattern:    regexp.MustCompile(`\+\d{10,15}\b`),
			dataType:   "phone",
			riskLevel:  "high",
			confidence: 0.80,
		},
		{
			id:         "phone_us",
			pattern:    regexp.MustCompile(`\(?\d{3}\)?[\-.\s]\d{3}[\-.\s]\d{4}\b`),
			dataType:   "phone",
			riskLevel:  "high",
			confidence: 0.75,
		},
		{
			id:         "api_key_openai",
			pattern:    regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
			dataType:   "api_key",
			riskLevel:  "high",
			confidence: 0.98,
		},
		{
			id:         "api_key_anthropic",
			pattern:    regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-]{20,}`),
			dataType:   "api_key",
			riskLevel:  "high",
			confidence: 0.98,
		},
		{
			id:         "jwt",
			pattern:    regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+\.[a-zA-Z0-9_\-]+`),
			dataType:   "token",
			riskLevel:  "high",
			confidence: 0.92,
		},
		{
			id:         "ssn",
			pattern:    regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			dataType:   "financial",
			riskLevel:  "high",
			confidence: 0.85,
		},
		{
			// Finds candidate digit sequences for credit card (Luhn validated below).
			id:         "credit_card",
			pattern:    regexp.MustCompile(`\b(?:\d[ \-]?){13,19}\b`),
			dataType:   "financial",
			riskLevel:  "high",
			confidence: 0.90,
		},
		{
			id:         "ip_address",
			pattern:    regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
			dataType:   "ip_address",
			riskLevel:  "medium",
			confidence: 0.70,
		},
		{
			id:         "keyword_password",
			pattern:    regexp.MustCompile(`(?i)password\s*[=:]\s*\S+`),
			dataType:   "token",
			riskLevel:  "high",
			confidence: 0.88,
		},
		{
			id:         "keyword_secret",
			pattern:    regexp.MustCompile(`(?i)secret\s*[=:]\s*\S+`),
			dataType:   "token",
			riskLevel:  "high",
			confidence: 0.85,
		},
		{
			id:         "keyword_health",
			pattern:    regexp.MustCompile(`(?i)\b(diagnosis|prescription|patient|medical record|PHI)\b`),
			dataType:   "health",
			riskLevel:  "high",
			confidence: 0.80,
		},
	}
}

// sensitiveFieldNames are field names whose mere presence flags a finding.
var sensitiveFieldNames = map[string]bool{
	"password": true, "passwd": true, "secret": true, "token": true,
	"auth": true, "authorization": true, "ssn": true, "dob": true,
	"birthday": true,
}

// Detect scans text and returns all detections, deduplicating by value.
func Detect(text string) []Detection {
	seen := map[string]bool{}
	var results []Detection

	for _, r := range detectionRules {
		matches := r.pattern.FindAllString(text, -1)
		for _, m := range matches {
			if r.id == "credit_card" {
				digits := stripNonDigits(m)
				if !luhnCheck(digits) {
					continue
				}
			}
			if r.id == "ip_address" && !validIP(m) {
				continue
			}
			key := r.id + ":" + m
			if seen[key] {
				continue
			}
			seen[key] = true
			results = append(results, Detection{
				DataType:   r.dataType,
				Value:      m,
				Masked:     maskValue(r.dataType, m),
				Confidence: r.confidence,
				RuleID:     r.id,
				RiskLevel:  r.riskLevel,
			})
		}
	}
	return results
}

// DetectInField runs field-name heuristics in addition to value scanning.
func DetectInField(fieldName, value string) []Detection {
	results := Detect(value)
	lower := strings.ToLower(fieldName)
	if sensitiveFieldNames[lower] && len(value) > 0 && len(results) == 0 {
		// Field name is sensitive but regex didn't catch it — flag as token/medium.
		results = append(results, Detection{
			DataType:   "token",
			Value:      value,
			Masked:     maskValue("token", value),
			Confidence: 0.60,
			RuleID:     "keyword_field_" + lower,
			RiskLevel:  "medium",
		})
	}
	return results
}

// ── Masking ───────────────────────────────────────────────────────────────────

func maskValue(dataType, value string) string {
	switch dataType {
	case "email":
		return maskEmail(value)
	case "phone":
		return maskPhone(value)
	case "api_key", "token":
		return maskAPIKey(value)
	case "financial":
		return maskFinancial(value)
	case "ip_address":
		return maskIP(value)
	default:
		return maskGeneric(value)
	}
}

func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 1 {
		return "***@" + email[at+1:]
	}
	return string(email[0]) + strings.Repeat("*", at-1) + email[at:]
}

func maskPhone(phone string) string {
	digits := stripNonDigits(phone)
	if len(digits) < 4 {
		return "****"
	}
	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:6] + strings.Repeat("*", len(key)-10) + key[len(key)-4:]
}

func maskFinancial(v string) string {
	digits := stripNonDigits(v)
	if len(digits) < 4 {
		return strings.Repeat("*", len(digits))
	}
	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

func maskIP(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return "***.***.***.**"
	}
	return fmt.Sprintf("***.***.***.%s", parts[3])
}

func maskGeneric(v string) string {
	if len(v) <= 4 {
		return strings.Repeat("*", len(v))
	}
	return v[:2] + strings.Repeat("*", len(v)-4) + v[len(v)-2:]
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func stripNonDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// luhnCheck validates a digit string using the Luhn algorithm.
func luhnCheck(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	nDigits := len(digits)
	parity := nDigits % 2
	for i := 0; i < nDigits; i++ {
		d := int(digits[i] - '0')
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// validIP checks that each octet is 0–255.
func validIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 3 {
			return false
		}
		v := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
			v = v*10 + int(c-'0')
		}
		if v > 255 {
			return false
		}
	}
	return true
}

// ── Compliance Mapping ────────────────────────────────────────────────────────

// ComplianceFor returns the compliance impacts for a given data type.
func ComplianceFor(dataType string) []ComplianceImpact {
	m := map[string][]ComplianceImpact{
		"email": {
			{Framework: "GDPR", Article: "Art. 4(1) — Personal Data Definition", RiskLevel: "high"},
			{Framework: "CCPA", Article: "§1798.140(o) — Personal Information", RiskLevel: "high"},
			{Framework: "SOC2", Article: "CC6 — Logical Access Controls", RiskLevel: "medium"},
		},
		"phone": {
			{Framework: "GDPR", Article: "Art. 4(1) — Personal Data Definition", RiskLevel: "high"},
			{Framework: "CCPA", Article: "§1798.140(o) — Personal Information", RiskLevel: "high"},
		},
		"ip_address": {
			{Framework: "GDPR", Article: "Recital 30 — Online Identifiers", RiskLevel: "medium"},
		},
		"health": {
			{Framework: "HIPAA", Article: "45 CFR §164.514 — PHI", RiskLevel: "high"},
			{Framework: "GDPR", Article: "Art. 9 — Special Category Data", RiskLevel: "high"},
		},
		"financial": {
			{Framework: "PCI-DSS", Article: "Req. 3.3 — Cardholder Data Protection", RiskLevel: "high"},
			{Framework: "GDPR", Article: "Art. 4(1) — Personal Data Definition", RiskLevel: "high"},
		},
		"api_key": {
			{Framework: "SOC2", Article: "CC6 — Logical Access Controls", RiskLevel: "high"},
		},
		"token": {
			{Framework: "SOC2", Article: "CC6 — Logical Access Controls", RiskLevel: "high"},
		},
		"name": {
			{Framework: "GDPR", Article: "Art. 4(1) — Personal Data Definition", RiskLevel: "high"},
			{Framework: "CCPA", Article: "§1798.140(o) — Personal Information", RiskLevel: "high"},
		},
	}
	if impacts, ok := m[dataType]; ok {
		return impacts
	}
	return []ComplianceImpact{}
}

// ── Fix Templates ─────────────────────────────────────────────────────────────

// FixesFor returns pre-authored remediation snippets for a vendor + rule combination.
func FixesFor(vendor, ruleID string) []Fix {
	type key struct{ vendor, ruleID string }
	templates := map[key][]Fix{
		{"sentry", "email"}: {
			{
				Language:    "javascript",
				Description: "Strip email and IP from Sentry user context in beforeSend",
				Code: `Sentry.init({
  beforeSend(event) {
    if (event.user) {
      delete event.user.email;
      delete event.user.ip_address;
    }
    return event;
  }
});`,
			},
		},
		{"datadog", "email"}: {
			{
				Language:    "bash",
				Description: "Add a Datadog log redaction rule to scrub emails",
				Code: `DD_LOGS_CONFIG_REDACTION_RULES='[
  {
    "pattern": "[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,}",
    "replace_placeholder": "[REDACTED_EMAIL]"
  }
]'`,
			},
		},
		{"sentry", "api_key_openai"}: {
			{
				Language:    "javascript",
				Description: "Scrub API keys from Sentry breadcrumbs and extra context",
				Code: `Sentry.init({
  beforeSend(event) {
    if (event.extra) {
      Object.keys(event.extra).forEach(k => {
        if (k.toLowerCase().includes('key') || k.toLowerCase().includes('token')) {
          delete event.extra[k];
        }
      });
    }
    return event;
  }
});`,
			},
		},
	}

	if fixes, ok := templates[key{vendor, ruleID}]; ok {
		return fixes
	}
	return []Fix{}
}

// ── Risk Scoring ──────────────────────────────────────────────────────────────

// OrgRiskScore computes the org-level risk score from open finding counts.
// score = 100 - (high*15 + medium*5 + low*1), clamped to [0,100].
func OrgRiskScore(fc FindingCounts) int {
	score := 100 - (fc.High*15 + fc.Medium*5 + fc.Low*1)
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// DetectionRiskLevel maps detection confidence + data type to a risk level.
func DetectionRiskLevel(d Detection, externalVendor bool) string {
	if externalVendor && d.Confidence >= 0.85 {
		return "high"
	}
	if externalVendor && d.Confidence >= 0.60 {
		return "medium"
	}
	return d.RiskLevel
}
