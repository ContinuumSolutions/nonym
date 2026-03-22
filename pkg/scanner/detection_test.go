package scanner

import (
	"strings"
	"testing"
)

// ── Detect ────────────────────────────────────────────────────────────────────

func TestDetect_Email(t *testing.T) {
	detections := Detect("Contact us at support@example.com for help.")
	found := findByDataType(detections, "email")
	if found == nil {
		t.Fatal("expected email detection, got none")
	}
	if found.RuleID != "email" {
		t.Errorf("expected ruleID 'email', got %q", found.RuleID)
	}
	if found.Masked == "" || !strings.Contains(found.Masked, "@") {
		t.Errorf("masked email should retain '@', got %q", found.Masked)
	}
	if found.Confidence < 0.90 {
		t.Errorf("expected high confidence for email, got %f", found.Confidence)
	}
}

func TestDetect_PhoneE164(t *testing.T) {
	detections := Detect("Call us at +12025550142 for support.")
	found := findByDataType(detections, "phone")
	if found == nil {
		t.Fatal("expected phone detection, got none")
	}
	if found.RuleID != "phone_e164" {
		t.Errorf("expected ruleID 'phone_e164', got %q", found.RuleID)
	}
}

func TestDetect_PhoneUS(t *testing.T) {
	detections := Detect("Call us at (202) 555-0142 for help.")
	found := findByDataType(detections, "phone")
	if found == nil {
		t.Fatal("expected phone detection, got none")
	}
}

func TestDetect_OpenAIKey(t *testing.T) {
	detections := Detect("The key is sk-abcdefghijklmnopqrstuvwxyz01 in config.")
	found := findByDataType(detections, "api_key")
	if found == nil {
		t.Fatal("expected api_key detection, got none")
	}
	if found.RuleID != "api_key_openai" {
		t.Errorf("expected ruleID 'api_key_openai', got %q", found.RuleID)
	}
	if found.RiskLevel != "high" {
		t.Errorf("expected high risk, got %q", found.RiskLevel)
	}
}

func TestDetect_AnthropicKey(t *testing.T) {
	detections := Detect("key=sk-ant-abcdefghijklmnopqrstuvwxyz0123")
	found := findByDataType(detections, "api_key")
	if found == nil {
		t.Fatal("expected api_key detection, got none")
	}
	if found.RuleID != "api_key_anthropic" {
		t.Errorf("expected ruleID 'api_key_anthropic', got %q", found.RuleID)
	}
}

func TestDetect_JWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	detections := Detect("Authorization: " + jwt)
	found := findByDataType(detections, "token")
	if found == nil {
		t.Fatal("expected token detection for JWT, got none")
	}
	if found.RuleID != "jwt" {
		t.Errorf("expected ruleID 'jwt', got %q", found.RuleID)
	}
}

func TestDetect_SSN(t *testing.T) {
	detections := Detect("SSN: 123-45-6789")
	found := findByDataType(detections, "financial")
	if found == nil {
		t.Fatal("expected financial detection for SSN, got none")
	}
	if found.RuleID != "ssn" {
		t.Errorf("expected ruleID 'ssn', got %q", found.RuleID)
	}
}

func TestDetect_IPAddress(t *testing.T) {
	detections := Detect("Client IP: 192.168.1.100")
	found := findByDataType(detections, "ip_address")
	if found == nil {
		t.Fatal("expected ip_address detection, got none")
	}
	if found.RiskLevel != "medium" {
		t.Errorf("expected medium risk for IP, got %q", found.RiskLevel)
	}
}

func TestDetect_InvalidIP_NotDetected(t *testing.T) {
	// 999.999.999.999 is not a valid IP.
	detections := Detect("address: 999.999.999.999")
	found := findByDataType(detections, "ip_address")
	if found != nil {
		t.Errorf("invalid IP should not be detected, got %+v", found)
	}
}

func TestDetect_KeywordPassword(t *testing.T) {
	detections := Detect("password = super_secret_123!")
	found := findByDataType(detections, "token")
	if found == nil {
		t.Fatal("expected token detection for password keyword, got none")
	}
}

func TestDetect_KeywordSecret(t *testing.T) {
	detections := Detect("secret: abc123")
	found := findByDataType(detections, "token")
	if found == nil {
		t.Fatal("expected token detection for secret keyword, got none")
	}
}

func TestDetect_HealthKeyword(t *testing.T) {
	detections := Detect("Patient diagnosis: type 2 diabetes")
	found := findByDataType(detections, "health")
	if found == nil {
		t.Fatal("expected health detection, got none")
	}
}

func TestDetect_NoFalsePositiveOnEmptyText(t *testing.T) {
	detections := Detect("")
	if len(detections) != 0 {
		t.Errorf("expected no detections on empty string, got %d", len(detections))
	}
}

func TestDetect_Deduplication(t *testing.T) {
	text := "Email: user@example.com, again: user@example.com"
	detections := Detect(text)
	emailCount := 0
	for _, d := range detections {
		if d.DataType == "email" {
			emailCount++
		}
	}
	if emailCount != 1 {
		t.Errorf("expected 1 deduplicated email detection, got %d", emailCount)
	}
}

// ── Luhn ─────────────────────────────────────────────────────────────────────

func TestLuhn_ValidVisa(t *testing.T) {
	// Valid Visa test number.
	if !luhnCheck("4111111111111111") {
		t.Error("4111111111111111 should pass Luhn check")
	}
}

func TestLuhn_ValidMastercard(t *testing.T) {
	if !luhnCheck("5500005555555559") {
		t.Error("5500005555555559 should pass Luhn check")
	}
}

func TestLuhn_InvalidNumber(t *testing.T) {
	if luhnCheck("1234567890123456") {
		t.Error("1234567890123456 should fail Luhn check")
	}
}

func TestLuhn_TooShort(t *testing.T) {
	if luhnCheck("4111111") {
		t.Error("7-digit number should fail (too short)")
	}
}

// ── Masking ───────────────────────────────────────────────────────────────────

func TestMaskEmail(t *testing.T) {
	masked := maskEmail("john@example.com")
	if !strings.Contains(masked, "@example.com") {
		t.Errorf("masked email should retain domain, got %q", masked)
	}
	if masked == "john@example.com" {
		t.Error("masked email should differ from original")
	}
}

func TestMaskEmail_Short(t *testing.T) {
	masked := maskEmail("a@b.com")
	if masked == "" {
		t.Error("masked email should not be empty")
	}
}

func TestMaskAPIKey(t *testing.T) {
	masked := maskAPIKey("sk-abcdefghijklmnopqrstuvwxyz")
	if masked == "sk-abcdefghijklmnopqrstuvwxyz" {
		t.Error("API key should be masked")
	}
	if len(masked) == 0 {
		t.Error("masked API key should not be empty")
	}
}

func TestMaskIP(t *testing.T) {
	masked := maskIP("192.168.1.100")
	if !strings.HasSuffix(masked, ".100") {
		t.Errorf("masked IP should retain last octet, got %q", masked)
	}
}

func TestMaskFinancial(t *testing.T) {
	masked := maskFinancial("4111111111111111")
	if !strings.HasSuffix(masked, "1111") {
		t.Errorf("masked financial should retain last 4 digits, got %q", masked)
	}
}

// ── ComplianceFor ─────────────────────────────────────────────────────────────

func TestComplianceFor_Email(t *testing.T) {
	impacts := ComplianceFor("email")
	if len(impacts) == 0 {
		t.Fatal("expected compliance impacts for email")
	}
	hasGDPR := false
	for _, ci := range impacts {
		if ci.Framework == "GDPR" {
			hasGDPR = true
		}
	}
	if !hasGDPR {
		t.Error("expected GDPR in email compliance impacts")
	}
}

func TestComplianceFor_Health(t *testing.T) {
	impacts := ComplianceFor("health")
	hasHIPAA := false
	for _, ci := range impacts {
		if ci.Framework == "HIPAA" {
			hasHIPAA = true
		}
	}
	if !hasHIPAA {
		t.Error("expected HIPAA in health compliance impacts")
	}
}

func TestComplianceFor_Unknown(t *testing.T) {
	impacts := ComplianceFor("nonexistent_type")
	if len(impacts) != 0 {
		t.Errorf("expected no impacts for unknown type, got %d", len(impacts))
	}
}

// ── OrgRiskScore ──────────────────────────────────────────────────────────────

func TestOrgRiskScore_NoFindings(t *testing.T) {
	score := OrgRiskScore(FindingCounts{})
	if score != 100 {
		t.Errorf("expected score 100 with no findings, got %d", score)
	}
}

func TestOrgRiskScore_Clamped(t *testing.T) {
	// Many high findings should clamp to 0.
	score := OrgRiskScore(FindingCounts{High: 100})
	if score != 0 {
		t.Errorf("expected score 0 for many high findings, got %d", score)
	}
}

func TestOrgRiskScore_Calculation(t *testing.T) {
	// 2 high (30), 1 medium (5), 1 low (1) → 100-36 = 64
	score := OrgRiskScore(FindingCounts{High: 2, Medium: 1, Low: 1, Total: 4})
	if score != 64 {
		t.Errorf("expected score 64, got %d", score)
	}
}

// ── FixesFor ──────────────────────────────────────────────────────────────────

func TestFixesFor_SentryEmail(t *testing.T) {
	fixes := FixesFor("sentry", "email")
	if len(fixes) == 0 {
		t.Fatal("expected fixes for sentry+email")
	}
	if fixes[0].Language != "javascript" {
		t.Errorf("expected javascript fix, got %q", fixes[0].Language)
	}
	if fixes[0].Code == "" {
		t.Error("fix code should not be empty")
	}
}

func TestFixesFor_Unknown(t *testing.T) {
	fixes := FixesFor("unknown_vendor", "unknown_rule")
	if len(fixes) != 0 {
		t.Errorf("expected no fixes for unknown vendor+rule, got %d", len(fixes))
	}
}

// ── DetectInField ─────────────────────────────────────────────────────────────

func TestDetectInField_SensitiveFieldName(t *testing.T) {
	// "password" field with a value that doesn't match regex patterns.
	detections := DetectInField("password", "mysecretpass")
	if len(detections) == 0 {
		t.Fatal("expected detection on sensitive field name 'password'")
	}
	if detections[0].DataType != "token" {
		t.Errorf("expected token data type, got %q", detections[0].DataType)
	}
}

func TestDetectInField_NormalFieldName(t *testing.T) {
	// Non-sensitive field name should not add extra detection.
	detections := DetectInField("username", "john_doe")
	if len(detections) != 0 {
		t.Errorf("expected no detections for benign field, got %d", len(detections))
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func findByDataType(detections []Detection, dataType string) *Detection {
	for i := range detections {
		if detections[i].DataType == dataType {
			return &detections[i]
		}
	}
	return nil
}

// ── ValidIP ───────────────────────────────────────────────────────────────────

func TestValidIP(t *testing.T) {
	cases := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"999.1.1.1", false},
		{"1.2.3", false},
		{"1.2.3.4.5", false},
		{"abc.def.ghi.jkl", false},
	}
	for _, tc := range cases {
		got := validIP(tc.ip)
		if got != tc.valid {
			t.Errorf("validIP(%q) = %v, want %v", tc.ip, got, tc.valid)
		}
	}
}
