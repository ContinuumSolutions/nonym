package ner

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// EntityType represents different types of detected entities
type EntityType string

const (
	EntityEmail       EntityType = "EMAIL"
	EntityPhone       EntityType = "PHONE"
	EntitySSN         EntityType = "SSN"
	EntityCreditCard  EntityType = "CREDIT_CARD"
	EntityIPAddress   EntityType = "IP_ADDRESS"
	EntityPerson      EntityType = "PERSON"
	EntityLocation    EntityType = "LOCATION"
	EntityOrganization EntityType = "ORGANIZATION"
	EntityAPIKey      EntityType = "API_KEY"
	EntityPassword    EntityType = "PASSWORD"
)

// RedactionDetail contains information about what was redacted
type RedactionDetail struct {
	EntityType    EntityType `json:"entity_type"`
	OriginalText  string     `json:"original_text"`
	RedactedText  string     `json:"redacted_text"`
	StartIndex    int        `json:"start_index"`
	EndIndex      int        `json:"end_index"`
	Confidence    float64    `json:"confidence"`
	Timestamp     time.Time  `json:"timestamp"`
}

// NEREngine handles Named Entity Recognition and anonymization
type NEREngine struct {
	patterns       map[EntityType]*regexp.Regexp
	tokenMap       map[string]string
	reverseTokenMap map[string]string
	mutex          sync.RWMutex
	strictMode     bool
}

var (
	globalEngine *NEREngine
	initOnce     sync.Once
)

// Initialize sets up the NER engine with predefined patterns
func Initialize() error {
	var err error
	initOnce.Do(func() {
		globalEngine = &NEREngine{
			patterns:       make(map[EntityType]*regexp.Regexp),
			tokenMap:       make(map[string]string),
			reverseTokenMap: make(map[string]string),
			strictMode:     false, // Default to anonymize mode
		}
		err = globalEngine.loadPatterns()
	})
	return err
}

// loadPatterns initializes regex patterns for different entity types
func (ne *NEREngine) loadPatterns() error {
	patterns := map[EntityType]string{
		// Email addresses
		EntityEmail: `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,

		// Phone numbers (US format)
		EntityPhone: `\b(?:\+?1[-.\s]?)?(?:\([0-9]{3}\)|[0-9]{3})[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b`,

		// Social Security Numbers
		EntitySSN: `\b\d{3}-?\d{2}-?\d{4}\b`,

		// Credit Card Numbers (basic pattern)
		EntityCreditCard: `\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|6(?:011|5[0-9][0-9])[0-9]{12}|3[47][0-9]{13}|3[0-9]{13}|2131|1800|35\d{3})\d*\b`,

		// IP Addresses
		EntityIPAddress: `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,

		// API Keys (common formats)
		EntityAPIKey: `\b(?:sk|pk|api)[-_]?(?:test_|live_)?[a-zA-Z0-9]{16,64}\b`,

		// Simple password detection (followed by common indicators)
		EntityPassword: `(?i)(?:password|passwd|pwd|pass)[:=\s]+[^\s\n\r]{6,}`,
	}

	for entityType, pattern := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile pattern for %s: %w", entityType, err)
		}
		ne.patterns[entityType] = compiled
	}

	return nil
}

// ProcessContent analyzes content and applies anonymization
func ProcessContent(content string) (string, []RedactionDetail, error) {
	if globalEngine == nil {
		return content, nil, fmt.Errorf("NER engine not initialized")
	}

	return globalEngine.processContent(content)
}

func (ne *NEREngine) processContent(content string) (string, []RedactionDetail, error) {
	var redactionDetails []RedactionDetail
	processedContent := content

	ne.mutex.Lock()
	defer ne.mutex.Unlock()

	// Process each entity type
	for entityType, pattern := range ne.patterns {
		matches := pattern.FindAllStringSubmatchIndex(processedContent, -1)

		// Process matches in reverse order to maintain indices
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			if len(match) < 2 {
				continue
			}

			start, end := match[0], match[1]
			originalText := processedContent[start:end]

			// Generate anonymized token
			token := ne.generateToken(entityType)

			// Store mapping for later restoration
			ne.tokenMap[token] = originalText
			ne.reverseTokenMap[originalText] = token

			// Create redaction detail
			detail := RedactionDetail{
				EntityType:   entityType,
				OriginalText: originalText,
				RedactedText: token,
				StartIndex:   start,
				EndIndex:     end,
				Confidence:   ne.calculateConfidence(entityType, originalText),
				Timestamp:    time.Now(),
			}
			redactionDetails = append(redactionDetails, detail)

			// Replace in content
			processedContent = processedContent[:start] + token + processedContent[end:]
		}
	}

	return processedContent, redactionDetails, nil
}

// DeAnonymizeContent restores original content from anonymized tokens
func DeAnonymizeContent(content string, redactionDetails []RedactionDetail) (string, error) {
	if globalEngine == nil {
		return content, fmt.Errorf("NER engine not initialized")
	}

	return globalEngine.deAnonymizeContent(content, redactionDetails)
}

func (ne *NEREngine) deAnonymizeContent(content string, redactionDetails []RedactionDetail) (string, error) {
	ne.mutex.RLock()
	defer ne.mutex.RUnlock()

	restoredContent := content

	// Replace tokens with original text
	for _, detail := range redactionDetails {
		if originalText, exists := ne.tokenMap[detail.RedactedText]; exists {
			restoredContent = strings.ReplaceAll(restoredContent, detail.RedactedText, originalText)
		}
	}

	return restoredContent, nil
}

// ShouldBlock determines if content should be blocked based on sensitivity
func ShouldBlock(redactionDetails []RedactionDetail) bool {
	if globalEngine == nil || !globalEngine.strictMode {
		return false
	}

	// Block if any high-sensitivity entities are detected
	for _, detail := range redactionDetails {
		if isHighSensitivity(detail.EntityType) && detail.Confidence > 0.8 {
			return true
		}
	}

	return false
}

// GetStatus returns the current status of the NER engine
func GetStatus() map[string]interface{} {
	if globalEngine == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	globalEngine.mutex.RLock()
	defer globalEngine.mutex.RUnlock()

	return map[string]interface{}{
		"status":         "operational",
		"patterns_loaded": len(globalEngine.patterns),
		"tokens_cached":  len(globalEngine.tokenMap),
		"strict_mode":    globalEngine.strictMode,
	}
}

// ExtractTokenMap returns the token mapping for debugging
func ExtractTokenMap(redactionDetails []RedactionDetail) map[string]string {
	tokenMap := make(map[string]string)
	for _, detail := range redactionDetails {
		tokenMap[detail.RedactedText] = detail.OriginalText
	}
	return tokenMap
}

// Helper functions

func (ne *NEREngine) generateToken(entityType EntityType) string {
	// Generate secure random token
	bytes := make([]byte, 8)
	rand.Read(bytes)
	randomHex := hex.EncodeToString(bytes)

	return fmt.Sprintf("{{%s_%s}}", entityType, strings.ToUpper(randomHex[:8]))
}

func (ne *NEREngine) calculateConfidence(entityType EntityType, text string) float64 {
	// Simple confidence calculation based on entity type and text characteristics
	switch entityType {
	case EntityEmail:
		if strings.Contains(text, "@") && strings.Contains(text, ".") {
			return 0.95
		}
	case EntitySSN:
		if len(strings.ReplaceAll(text, "-", "")) == 9 {
			return 0.9
		}
	case EntityCreditCard:
		// Luhn algorithm could be applied here for higher confidence
		return 0.85
	case EntityIPAddress:
		parts := strings.Split(text, ".")
		if len(parts) == 4 {
			return 0.9
		}
	case EntityAPIKey:
		if len(text) >= 16 {
			return 0.8
		}
	}

	return 0.7 // Default confidence
}

func isHighSensitivity(entityType EntityType) bool {
	highSensitivityTypes := map[EntityType]bool{
		EntitySSN:        true,
		EntityCreditCard: true,
		EntityAPIKey:     true,
		EntityPassword:   true,
	}

	return highSensitivityTypes[entityType]
}

// SetStrictMode enables or disables strict blocking mode
func SetStrictMode(enabled bool) {
	if globalEngine != nil {
		globalEngine.mutex.Lock()
		globalEngine.strictMode = enabled
		globalEngine.mutex.Unlock()
	}
}