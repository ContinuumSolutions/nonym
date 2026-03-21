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
	EntityEmail        EntityType = "EMAIL"
	EntityPhone        EntityType = "PHONE"
	EntitySSN          EntityType = "SSN"
	EntityCreditCard   EntityType = "CREDIT_CARD"
	EntityCardCVV      EntityType = "CARD_CVV"
	EntityIBAN         EntityType = "IBAN"
	EntityIPAddress    EntityType = "IP_ADDRESS"
	EntityPerson       EntityType = "PERSON"
	EntityLocation     EntityType = "LOCATION" // legacy; prefer EntityAddress
	EntityAddress      EntityType = "ADDRESS"
	EntityOrganization EntityType = "ORGANIZATION"
	EntityDate         EntityType = "DATE"
	EntityNIN          EntityType = "NIN"
	EntityAPIKey       EntityType = "API_KEY"
	EntityPassword     EntityType = "PASSWORD"
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
	patterns        map[EntityType]*regexp.Regexp
	tokenMap        map[string]string
	reverseTokenMap map[string]string
	mutex           sync.RWMutex
	strictMode      bool
	useML           bool // when true, ML gRPC service is used for PERSON/LOCATION/ORG
}

var (
	globalEngine *NEREngine
	initOnce     sync.Once
)

// Initialize sets up the NER engine with predefined patterns.
// It also attempts to connect to the ML gRPC server if NER_GRPC_HOST is set.
func Initialize() error {
	var err error
	initOnce.Do(func() {
		globalEngine = &NEREngine{
			patterns:        make(map[EntityType]*regexp.Regexp),
			tokenMap:        make(map[string]string),
			reverseTokenMap: make(map[string]string),
			strictMode:      false,
		}
		err = globalEngine.loadPatterns()
		if err != nil {
			return
		}
		// Attempt ML gRPC init; failure is non-fatal — regex engine still works.
		if grpcErr := InitGRPCClient(); grpcErr == nil {
			globalEngine.useML = true
		}
	})
	return err
}

// EnableML explicitly enables or disables the ML gRPC backend.
func EnableML(enabled bool) {
	if globalEngine != nil {
		globalEngine.mutex.Lock()
		globalEngine.useML = enabled
		globalEngine.mutex.Unlock()
	}
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

	// --- ML-based NER (PERSON / LOCATION / ORGANIZATION) ---
	if ne.useML && IsGRPCAvailable() {
		mlDetails, mlContent, err := ne.processMLEntities(processedContent)
		if err == nil {
			redactionDetails = append(redactionDetails, mlDetails...)
			processedContent = mlContent
		}
		// on error, fall through to regex-only for all entity types
	}

	// --- Regex-based NER ---
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

			token := ne.generateToken(entityType)
			ne.tokenMap[token] = originalText
			ne.reverseTokenMap[originalText] = token

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
			processedContent = processedContent[:start] + token + processedContent[end:]
		}
	}

	// Post-processing: reclassify phone detections that are actually card PANs or CVVs.
	redactionDetails = reclassifyCardPANs(processedContent, redactionDetails)

	return processedContent, redactionDetails, nil
}

// processMLEntities calls the GLiNER gRPC service and redacts entities in-place.
// Spans are applied in reverse order so that earlier offsets remain valid.
func (ne *NEREngine) processMLEntities(content string) ([]RedactionDetail, string, error) {
	spans, err := AnnotateML(content, DefaultMLLabels, 0.5)
	if err != nil {
		return nil, content, err
	}

	// Sort spans in descending order of start position
	for i := 0; i < len(spans); i++ {
		for j := i + 1; j < len(spans); j++ {
			if spans[j].Start > spans[i].Start {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}

	var details []RedactionDetail
	for _, span := range spans {
		if span.Start < 0 || span.End > len(content) || span.Start >= span.End {
			continue
		}
		originalText := content[span.Start:span.End]
		entityType := mlLabelToEntityType(span.Label)
		token := ne.generateToken(entityType)
		ne.tokenMap[token] = originalText
		ne.reverseTokenMap[originalText] = token

		details = append(details, RedactionDetail{
			EntityType:   entityType,
			OriginalText: originalText,
			RedactedText: token,
			StartIndex:   span.Start,
			EndIndex:     span.End,
			Confidence:   span.Score,
			Timestamp:    time.Now(),
		})
		content = content[:span.Start] + token + content[span.End:]
	}
	return details, content, nil
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

// luhnCheck returns true when the digit string passes the Luhn algorithm.
// The input should contain only ASCII digits.
func luhnCheck(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	for i, ch := range digits {
		n := int(ch - '0')
		// Double every second digit from the right (even position from right = odd index from right).
		if (len(digits)-i)%2 == 0 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return sum%10 == 0
}

// reclassifyCardPANs performs a secondary pass over PHONE detections and
// reclassifies them as CREDIT_CARD (via Luhn) or CARD_CVV (via context).
// content is the original text used for CVV context lookup.
func reclassifyCardPANs(content string, details []RedactionDetail) []RedactionDetail {
	for i, d := range details {
		if d.EntityType != EntityPhone {
			continue
		}

		// Strip non-digit characters for numeric analysis.
		stripped := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, d.OriginalText)

		if luhnCheck(stripped) {
			details[i].EntityType = EntityCreditCard
			details[i].Confidence = 0.95
			continue
		}

		// CVV heuristic: 3–4 digits with "cvv"/"cvc"/"security code" nearby.
		if len(stripped) >= 3 && len(stripped) <= 4 {
			lo := d.StartIndex - 40
			if lo < 0 {
				lo = 0
			}
			hi := d.EndIndex + 40
			if hi > len(content) {
				hi = len(content)
			}
			ctx := strings.ToLower(content[lo:hi])
			if strings.Contains(ctx, "cvv") || strings.Contains(ctx, "cvc") || strings.Contains(ctx, "security code") {
				details[i].EntityType = EntityCardCVV
				details[i].Confidence = 0.85
			}
		}
	}
	return details
}

// SetStrictMode enables or disables strict blocking mode
func SetStrictMode(enabled bool) {
	if globalEngine != nil {
		globalEngine.mutex.Lock()
		globalEngine.strictMode = enabled
		globalEngine.mutex.Unlock()
	}
}