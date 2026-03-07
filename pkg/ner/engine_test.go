package ner

import (
	"strings"
	"testing"
)

func TestNEREngineInitialization(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	status := GetStatus()
	if status["status"] != "operational" {
		t.Errorf("Expected status to be operational, got %v", status["status"])
	}

	patternsLoaded, ok := status["patterns_loaded"].(int)
	if !ok || patternsLoaded == 0 {
		t.Errorf("Expected patterns to be loaded, got %v", patternsLoaded)
	}
}

func TestEmailDetection(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	testCases := []struct {
		input         string
		shouldDetect  bool
		description   string
	}{
		{
			input:        "Contact me at john.doe@example.com for details",
			shouldDetect: true,
			description:  "Standard email format",
		},
		{
			input:        "My email is admin+test@company.org",
			shouldDetect: true,
			description:  "Email with plus sign",
		},
		{
			input:        "Send it to user123@test-domain.co.uk",
			shouldDetect: true,
			description:  "Email with numbers and hyphens",
		},
		{
			input:        "This is not an email: john.doe.example.com",
			shouldDetect: false,
			description:  "Missing @ symbol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			processed, details, err := ProcessContent(tc.input)
			if err != nil {
				t.Fatalf("ProcessContent failed: %v", err)
			}

			hasEmailDetection := false
			for _, detail := range details {
				if detail.EntityType == EntityEmail {
					hasEmailDetection = true
					break
				}
			}

			if tc.shouldDetect && !hasEmailDetection {
				t.Errorf("Expected to detect email in: %s", tc.input)
			}

			if !tc.shouldDetect && hasEmailDetection {
				t.Errorf("Should not detect email in: %s", tc.input)
			}

			if tc.shouldDetect {
				// Verify content was actually modified
				if processed == tc.input {
					t.Errorf("Content should have been modified but wasn't")
				}
				// Verify no original email remains in processed text
				if strings.Contains(processed, "@") && !strings.Contains(processed, "{{EMAIL_") {
					t.Errorf("Original email still present in processed content: %s", processed)
				}
			}
		})
	}
}

func TestSSNDetection(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	testCases := []struct {
		input         string
		shouldDetect  bool
		description   string
	}{
		{
			input:        "My SSN is 123-45-6789",
			shouldDetect: true,
			description:  "Standard SSN with dashes",
		},
		{
			input:        "SSN: 123456789",
			shouldDetect: true,
			description:  "SSN without dashes",
		},
		{
			input:        "Not an SSN: 12-345-6789",
			shouldDetect: false,
			description:  "Wrong format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			_, details, err := ProcessContent(tc.input)
			if err != nil {
				t.Fatalf("ProcessContent failed: %v", err)
			}

			hasSSNDetection := false
			for _, detail := range details {
				if detail.EntityType == EntitySSN {
					hasSSNDetection = true
					break
				}
			}

			if tc.shouldDetect != hasSSNDetection {
				t.Errorf("SSN detection mismatch for: %s. Expected: %v, Got: %v",
					tc.input, tc.shouldDetect, hasSSNDetection)
			}

			if tc.shouldDetect && len(details) == 0 {
				t.Errorf("Content should have been processed but no details found: %s", tc.input)
			}
		})
	}
}

func TestCreditCardDetection(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	testCases := []struct {
		input         string
		shouldDetect  bool
		description   string
	}{
		{
			input:        "Card number: 4111111111111111",
			shouldDetect: true,
			description:  "Valid Visa card number",
		},
		{
			input:        "Use card 5555555555554444 for payment",
			shouldDetect: true,
			description:  "Valid MasterCard number",
		},
		{
			input:        "Not a card: 1234567890123456",
			shouldDetect: false,
			description:  "Invalid card pattern",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			_, details, err := ProcessContent(tc.input)
			if err != nil {
				t.Fatalf("ProcessContent failed: %v", err)
			}

			hasCreditCardDetection := false
			for _, detail := range details {
				if detail.EntityType == EntityCreditCard {
					hasCreditCardDetection = true
					break
				}
			}

			if tc.shouldDetect != hasCreditCardDetection {
				t.Errorf("Credit card detection mismatch for: %s. Expected: %v, Got: %v",
					tc.input, tc.shouldDetect, hasCreditCardDetection)
			}
		})
	}
}

func TestAPIKeyDetection(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	testCases := []struct {
		input         string
		shouldDetect  bool
		description   string
	}{
		{
			input:        "API key: sk-1234567890abcdef1234567890abcdef",
			shouldDetect: true,
			description:  "OpenAI-style API key",
		},
		{
			input:        "Use pk_test_1234567890abcdef for testing",
			shouldDetect: true,
			description:  "Stripe-style public key",
		},
		{
			input:        "The api_secret is api-xyz789012345678901234567890",
			shouldDetect: true,
			description:  "Generic API key format",
		},
		{
			input:        "Not an API key: some-short-text",
			shouldDetect: false,
			description:  "Too short to be API key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			_, details, err := ProcessContent(tc.input)
			if err != nil {
				t.Fatalf("ProcessContent failed: %v", err)
			}

			hasAPIKeyDetection := false
			for _, detail := range details {
				if detail.EntityType == EntityAPIKey {
					hasAPIKeyDetection = true
					break
				}
			}

			if tc.shouldDetect != hasAPIKeyDetection {
				t.Errorf("API key detection mismatch for: %s. Expected: %v, Got: %v",
					tc.input, tc.shouldDetect, hasAPIKeyDetection)
			}
		})
	}
}

func TestDeAnonymization(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	originalContent := "Contact John at john@example.com or call 555-123-4567"

	// First anonymize
	anonymized, details, err := ProcessContent(originalContent)
	if err != nil {
		t.Fatalf("ProcessContent failed: %v", err)
	}

	if len(details) == 0 {
		t.Fatalf("Expected some entities to be detected")
	}

	// Verify anonymization worked
	if anonymized == originalContent {
		t.Fatalf("Content should have been anonymized")
	}

	// Mock AI response that includes the tokens
	aiResponse := "You can reach out via " + details[0].RedactedText
	if len(details) > 1 {
		aiResponse += " or " + details[1].RedactedText
	}

	// De-anonymize the response
	restored, err := DeAnonymizeContent(aiResponse, details)
	if err != nil {
		t.Fatalf("DeAnonymizeContent failed: %v", err)
	}

	// Verify original content was restored in the response
	for _, detail := range details {
		if !strings.Contains(restored, detail.OriginalText) {
			t.Errorf("Original text '%s' not found in restored content: %s",
				detail.OriginalText, restored)
		}
		if strings.Contains(restored, detail.RedactedText) {
			t.Errorf("Redacted token '%s' still present in restored content: %s",
				detail.RedactedText, restored)
		}
	}
}

func TestStrictMode(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	// Test with strict mode disabled (default)
	SetStrictMode(false)

	content := "My SSN is 123-45-6789 and my card is 4111111111111111"
	_, details, err := ProcessContent(content)
	if err != nil {
		t.Fatalf("ProcessContent failed: %v", err)
	}

	// Should not block in anonymize mode
	if ShouldBlock(details) {
		t.Errorf("Should not block in anonymize mode")
	}

	// Enable strict mode
	SetStrictMode(true)

	// Should block high-sensitivity content
	if !ShouldBlock(details) {
		t.Errorf("Should block in strict mode with high-sensitivity data")
	}

	// Test with low-sensitivity content
	emailContent := "Email me at test@example.com"
	_, emailDetails, err := ProcessContent(emailContent)
	if err != nil {
		t.Fatalf("ProcessContent failed: %v", err)
	}

	// Should not block email in strict mode (lower sensitivity)
	if ShouldBlock(emailDetails) {
		t.Errorf("Should not block email in strict mode")
	}
}

func TestMultipleEntityTypes(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	content := `Hi, I'm John Doe. My email is john@example.com,
		SSN: 123-45-6789, and card number 4111111111111111.
		Call me at 555-123-4567 or use API key sk-1234567890abcdef.`

	processed, details, err := ProcessContent(content)
	if err != nil {
		t.Fatalf("ProcessContent failed: %v", err)
	}

	expectedTypes := map[EntityType]bool{
		EntityEmail:      true,
		EntitySSN:        true,
		EntityCreditCard: true,
		EntityPhone:      true,
		EntityAPIKey:     true,
	}

	detectedTypes := make(map[EntityType]bool)
	for _, detail := range details {
		detectedTypes[detail.EntityType] = true
	}

	for expectedType := range expectedTypes {
		if !detectedTypes[expectedType] {
			t.Errorf("Expected to detect %s but didn't", expectedType)
		}
	}

	// Verify all sensitive data was removed from processed content
	sensitivePatterns := []string{
		"john@example.com",
		"123-45-6789",
		"4111111111111111",
		"555-123-4567",
		"sk-1234567890abcdef",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(processed, pattern) {
			t.Errorf("Sensitive data '%s' still present in processed content", pattern)
		}
	}

	// Verify tokens were generated
	tokenCount := 0
	for _, detail := range details {
		if strings.Contains(processed, detail.RedactedText) {
			tokenCount++
		}
	}

	if tokenCount != len(details) {
		t.Errorf("Not all tokens found in processed content. Expected: %d, Found: %d",
			len(details), tokenCount)
	}
}

func TestConfidenceScoring(t *testing.T) {
	err := Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize NER engine: %v", err)
	}

	testCases := []struct {
		input              string
		entityType         EntityType
		expectedMinConfidence float64
		description        string
	}{
		{
			input:              "test@example.com",
			entityType:         EntityEmail,
			expectedMinConfidence: 0.9,
			description:        "Clear email should have high confidence",
		},
		{
			input:              "123-45-6789",
			entityType:         EntitySSN,
			expectedMinConfidence: 0.85,
			description:        "Proper SSN format should have high confidence",
		},
		{
			input:              "192.168.1.1",
			entityType:         EntityIPAddress,
			expectedMinConfidence: 0.85,
			description:        "Valid IP should have high confidence",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			_, details, err := ProcessContent(tc.input)
			if err != nil {
				t.Fatalf("ProcessContent failed: %v", err)
			}

			found := false
			for _, detail := range details {
				if detail.EntityType == tc.entityType {
					found = true
					if detail.Confidence < tc.expectedMinConfidence {
						t.Errorf("Confidence too low for %s. Expected >= %.2f, got %.2f",
							tc.entityType, tc.expectedMinConfidence, detail.Confidence)
					}
					break
				}
			}

			if !found {
				t.Errorf("Entity type %s not detected in: %s", tc.entityType, tc.input)
			}
		})
	}
}