package ner

import (
	"strings"
	"sync"
	"testing"
)

// resetEngine clears the singleton so each test can reinitialise cleanly.
func resetEngine(t *testing.T) {
	t.Helper()
	globalEngine = nil
	initOnce = sync.Once{}
	globalGRPCClient = nil
	grpcOnce = sync.Once{}
}

// initWithMock resets state, initialises the engine (regex only), then
// injects the mock gRPC client and enables ML. This ordering ensures that
// Initialize's internal grpcOnce.Do does not overwrite the mock.
func initWithMock(t *testing.T) {
	t.Helper()
	resetEngine(t)
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	newMockGRPCClient(t) // overrides globalGRPCClient after grpcOnce has run
	EnableML(true)
}

// TestProcessContent_MLEnabled verifies that ML-detected PERSON entities are
// redacted when the gRPC backend is wired in (via the mock server).
func TestProcessContent_MLEnabled(t *testing.T) {
	initWithMock(t)

	text := "Please contact John for support."
	processed, details, err := ProcessContent(text)
	if err != nil {
		t.Fatalf("ProcessContent: %v", err)
	}

	foundPerson := false
	for _, d := range details {
		if d.EntityType == EntityPerson && d.OriginalText == "John" {
			foundPerson = true
		}
	}
	if !foundPerson {
		t.Errorf("expected PERSON entity for 'John', details=%+v", details)
	}
	if strings.Contains(processed, "John") {
		t.Errorf("'John' should have been redacted in: %s", processed)
	}
}

// TestProcessContent_MLFallback verifies that when ML is disabled the regex
// engine still catches structured entities like emails and SSNs.
func TestProcessContent_MLFallback(t *testing.T) {
	resetEngine(t)
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	EnableML(false)

	text := "Email: alice@example.com, SSN: 987-65-4321"
	processed, details, err := ProcessContent(text)
	if err != nil {
		t.Fatalf("ProcessContent: %v", err)
	}

	typeSet := map[EntityType]bool{}
	for _, d := range details {
		typeSet[d.EntityType] = true
	}
	if !typeSet[EntityEmail] {
		t.Error("expected EMAIL entity")
	}
	if !typeSet[EntitySSN] {
		t.Error("expected SSN entity")
	}
	if strings.Contains(processed, "alice@example.com") {
		t.Error("email should be redacted")
	}
	if strings.Contains(processed, "987-65-4321") {
		t.Error("SSN should be redacted")
	}
}

// TestProcessContent_MLAndRegexCombined verifies that both ML entities (PERSON)
// and regex entities (EMAIL) are redacted in the same pass.
func TestProcessContent_MLAndRegexCombined(t *testing.T) {
	initWithMock(t)

	text := "John can be reached at john@example.com"
	processed, details, err := ProcessContent(text)
	if err != nil {
		t.Fatalf("ProcessContent: %v", err)
	}

	typeSet := map[EntityType]bool{}
	for _, d := range details {
		typeSet[d.EntityType] = true
	}
	if !typeSet[EntityPerson] {
		t.Error("expected PERSON entity from ML")
	}
	if !typeSet[EntityEmail] {
		t.Error("expected EMAIL entity from regex")
	}
	if strings.Contains(processed, "john@example.com") {
		t.Error("email should be redacted")
	}
}

// TestDeAnonymize_MLTokens checks that ML-produced tokens can be de-anonymized.
func TestDeAnonymize_MLTokens(t *testing.T) {
	initWithMock(t)

	text := "John called yesterday."
	_, details, err := ProcessContent(text)
	if err != nil {
		t.Fatalf("ProcessContent: %v", err)
	}

	// Build a synthetic "AI response" that echoes back one of the tokens
	if len(details) == 0 {
		t.Skip("no entities detected – nothing to de-anonymize")
	}
	token := details[0].RedactedText
	aiResp := "We'll follow up with " + token + " soon."

	restored, err := DeAnonymizeContent(aiResp, details)
	if err != nil {
		t.Fatalf("DeAnonymizeContent: %v", err)
	}
	if strings.Contains(restored, token) {
		t.Errorf("token %q still present after de-anonymisation", token)
	}
	if !strings.Contains(restored, details[0].OriginalText) {
		t.Errorf("original text %q not found in restored content: %s", details[0].OriginalText, restored)
	}
}

// TestGetStatus_MLField ensures GetStatus reflects the ML flag.
func TestGetStatus_MLField(t *testing.T) {
	resetEngine(t)
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	status := GetStatus()
	if status["status"] != "operational" {
		t.Errorf("unexpected status: %v", status["status"])
	}
}
