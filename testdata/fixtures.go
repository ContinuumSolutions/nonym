package testdata

import (
	"database/sql"
	"time"

	"github.com/sovereignprivacy/gateway/pkg/auth"
	"github.com/sovereignprivacy/gateway/pkg/ner"
)

// TestFixtures provides common test data and utilities
type TestFixtures struct {
	Users    []auth.User
	APIKeys  []auth.APIKey
	RedactionDetails []ner.RedactionDetail
}

// NewTestFixtures creates a new set of test fixtures
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{
		Users:    createTestUsers(),
		APIKeys:  createTestAPIKeys(),
		RedactionDetails: createTestRedactionDetails(),
	}
}

func createTestUsers() []auth.User {
	now := time.Now()
	return []auth.User{
		{
			ID:        1,
			Email:     "admin@test.com",
			Password:  "$2a$10$test.hash.here",  // bcrypt hash of "admin123"
			Name:      "Test Admin",
			Role:      "admin",
			Active:    true,
			CreatedAt: now.AddDate(0, -1, 0),
			UpdatedAt: now,
			LastLogin: &now,
		},
		{
			ID:        2,
			Email:     "user@test.com",
			Password:  "$2a$10$test.hash.here2", // bcrypt hash of "user123"
			Name:      "Test User",
			Role:      "user",
			Active:    true,
			CreatedAt: now.AddDate(0, 0, -7),
			UpdatedAt: now,
			LastLogin: nil,
		},
		{
			ID:        3,
			Email:     "inactive@test.com",
			Password:  "$2a$10$test.hash.here3",
			Name:      "Inactive User",
			Role:      "user",
			Active:    false,
			CreatedAt: now.AddDate(0, 0, -30),
			UpdatedAt: now.AddDate(0, 0, -30),
			LastLogin: nil,
		},
	}
}

func createTestAPIKeys() []auth.APIKey {
	now := time.Now()
	expiry := now.AddDate(0, 6, 0) // 6 months from now

	return []auth.APIKey{
		{
			ID:          "key_001",
			Name:        "Test Read Key",
			KeyHash:     "$2a$10$test.api.key.hash.1",
			MaskedKey:   "spg_123••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••4567",
			Permissions: "read",
			UserID:      "1",
			CreatedAt:   now.AddDate(0, -1, 0),
			ExpiresAt:   &expiry,
			Status:      "active",
			LastUsed:    &now,
		},
		{
			ID:          "key_002",
			Name:        "Test Write Key",
			KeyHash:     "$2a$10$test.api.key.hash.2",
			MaskedKey:   "spg_abc••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••def",
			Permissions: "write",
			UserID:      "2",
			CreatedAt:   now.AddDate(0, 0, -7),
			ExpiresAt:   nil,
			Status:      "active",
			LastUsed:    nil,
		},
		{
			ID:          "key_003",
			Name:        "Revoked Key",
			KeyHash:     "$2a$10$test.api.key.hash.3",
			MaskedKey:   "spg_xyz••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••••890",
			Permissions: "admin",
			UserID:      "1",
			CreatedAt:   now.AddDate(0, 0, -14),
			ExpiresAt:   nil,
			Status:      "revoked",
			LastUsed:    &now,
		},
	}
}

func createTestRedactionDetails() []ner.RedactionDetail {
	return []ner.RedactionDetail{
		{
			EntityType:   ner.EntityEmail,
			OriginalText: "john.doe@example.com",
			RedactedText: "{{EMAIL_ABC123}}",
			Confidence:   0.95,
			StartIndex:   10,
			EndIndex:     30,
		},
		{
			EntityType:   ner.EntityPhone,
			OriginalText: "555-123-4567",
			RedactedText: "{{PHONE_XYZ789}}",
			Confidence:   0.92,
			StartIndex:   45,
			EndIndex:     57,
		},
		{
			EntityType:   ner.EntitySSN,
			OriginalText: "123-45-6789",
			RedactedText: "{{SSN_DEF456}}",
			Confidence:   0.98,
			StartIndex:   70,
			EndIndex:     81,
		},
		{
			EntityType:   ner.EntityCreditCard,
			OriginalText: "4111111111111111",
			RedactedText: "{{CARD_GHI789}}",
			Confidence:   0.97,
			StartIndex:   100,
			EndIndex:     116,
		},
		{
			EntityType:   ner.EntityAPIKey,
			OriginalText: "sk-1234567890abcdef1234567890abcdef",
			RedactedText: "{{API_KEY_JKL012}}",
			Confidence:   0.99,
			StartIndex:   130,
			EndIndex:     163,
		},
		{
			EntityType:   ner.EntityIPAddress,
			OriginalText: "192.168.1.100",
			RedactedText: "{{IP_MNO345}}",
			Confidence:   0.88,
			StartIndex:   180,
			EndIndex:     193,
		},
	}
}

// Database setup helpers
func SetupTestDatabase(db *sql.DB) error {
	// Create tables if they don't exist
	tables := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT DEFAULT 'user',
			active BOOLEAN DEFAULT true,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			key_hash TEXT NOT NULL,
			masked_key TEXT NOT NULL,
			permissions TEXT NOT NULL,
			user_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			status TEXT DEFAULT 'active',
			last_used DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS organizations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			industry TEXT,
			size TEXT,
			country TEXT,
			description TEXT,
			owner_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return err
		}
	}

	return nil
}

func PopulateTestData(db *sql.DB, fixtures *TestFixtures) error {
	// Insert test users
	userStmt := `INSERT OR REPLACE INTO users (id, email, password, name, role, active, created_at, updated_at, last_login)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, user := range fixtures.Users {
		_, err := db.Exec(userStmt, user.ID, user.Email, user.Password, user.Name,
			user.Role, user.Active, user.CreatedAt, user.UpdatedAt, user.LastLogin)
		if err != nil {
			return err
		}
	}

	// Insert test API keys
	keyStmt := `INSERT OR REPLACE INTO api_keys (id, name, key_hash, masked_key, permissions, user_id, created_at, expires_at, status, last_used)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for _, key := range fixtures.APIKeys {
		_, err := db.Exec(keyStmt, key.ID, key.Name, key.KeyHash, key.MaskedKey,
			key.Permissions, key.UserID, key.CreatedAt, key.ExpiresAt, key.Status, key.LastUsed)
		if err != nil {
			return err
		}
	}

	return nil
}

// Test data generators
func GenerateTestContent() map[string]string {
	return map[string]string{
		"clean": "This is a clean message with no sensitive information.",
		"email": "Please contact me at john.doe@example.com for more information.",
		"phone": "You can reach me at 555-123-4567 during business hours.",
		"ssn": "My SSN is 123-45-6789 for verification purposes.",
		"credit_card": "Use card number 4111111111111111 for payment.",
		"api_key": "The API key is sk-1234567890abcdef1234567890abcdef.",
		"ip_address": "The server is located at 192.168.1.100.",
		"mixed": "Contact John at john.doe@example.com, phone 555-123-4567, SSN 123-45-6789.",
		"high_sensitivity": "SSN: 123-45-6789, Card: 4111111111111111, API: sk-test123",
	}
}

func GenerateTestRequests() map[string]interface{} {
	return map[string]interface{}{
		"openai_chat": map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "Hello, how are you?",
				},
			},
			"temperature": 0.7,
			"max_tokens":  150,
		},
		"openai_completion": map[string]interface{}{
			"model":      "text-davinci-003",
			"prompt":     "Complete this sentence: The weather today is",
			"max_tokens": 50,
		},
		"anthropic_message": map[string]interface{}{
			"model": "claude-3-haiku",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "What is the capital of France?",
				},
			},
			"max_tokens": 100,
		},
		"with_pii": map[string]interface{}{
			"model": "gpt-3.5-turbo",
			"messages": []map[string]interface{}{
				{
					"role":    "user",
					"content": "My email is test@example.com and my SSN is 123-45-6789",
				},
			},
		},
	}
}

func GenerateTestResponses() map[string]interface{} {
	return map[string]interface{}{
		"openai_chat_success": map[string]interface{}{
			"id":      "chatcmpl-test123",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "gpt-3.5-turbo",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello! I'm doing well, thank you for asking.",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     12,
				"completion_tokens": 15,
				"total_tokens":      27,
			},
		},
		"anthropic_success": map[string]interface{}{
			"id":      "msg-test123",
			"type":    "message",
			"role":    "assistant",
			"model":   "claude-3-haiku",
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": "The capital of France is Paris.",
				},
			},
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 8,
			},
		},
		"error_response": map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid request format",
				"type":    "invalid_request_error",
				"code":    "bad_request",
			},
		},
	}
}

// Helper functions for tests
func GetTestJWT() string {
	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJyb2xlIjoidXNlciIsImV4cCI6OTk5OTk5OTk5OSwiaWF0IjoxNjAwMDAwMDAwfQ.test-signature"
}

func GetTestAPIKey() string {
	return "spg_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
}

// Mock data for different scenarios
type TestScenario struct {
	Name        string
	Description string
	Input       interface{}
	Expected    interface{}
	ShouldError bool
	ErrorType   string
}

func GetTestScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "clean_content",
			Description: "Content with no PII should pass through unchanged",
			Input:       "This is a clean message.",
			Expected:    "This is a clean message.",
			ShouldError: false,
		},
		{
			Name:        "email_detection",
			Description: "Email addresses should be detected and anonymized",
			Input:       "Contact me at john@example.com",
			Expected:    map[string]interface{}{
				"detected_entities": 1,
				"entity_types":      []string{"email"},
			},
			ShouldError: false,
		},
		{
			Name:        "multiple_pii_types",
			Description: "Multiple PII types should be detected",
			Input:       "Email: test@example.com, Phone: 555-1234, SSN: 123-45-6789",
			Expected: map[string]interface{}{
				"detected_entities": 3,
				"entity_types":      []string{"email", "phone", "ssn"},
			},
			ShouldError: false,
		},
		{
			Name:        "high_sensitivity_blocking",
			Description: "High sensitivity content should be blocked in strict mode",
			Input:       "SSN: 123-45-6789, Credit Card: 4111111111111111",
			Expected: map[string]interface{}{
				"blocked":      true,
				"sensitivity":  "high",
				"block_reason": "high_sensitivity_content",
			},
			ShouldError: false,
		},
	}
}

// Environment setup helpers
func SetupTestEnvironment() map[string]string {
	return map[string]string{
		"JWT_SECRET":           "test-jwt-secret-key-for-testing",
		"DB_PATH":              ":memory:",
		"LOG_LEVEL":            "error",
		"OPENAI_API_KEY":       "test-openai-key",
		"ANTHROPIC_API_KEY":    "test-anthropic-key",
		"OPENAI_BASE_URL":      "http://localhost:8001",
		"ANTHROPIC_BASE_URL":   "http://localhost:8002",
		"LOCAL_LLM_URL":        "http://localhost:11434",
		"REDIS_URL":            "redis://localhost:6379",
		"ENABLE_PII_DETECTION": "true",
		"STRICT_MODE":          "false",
		"AUDIT_ENABLED":        "true",
		"MAX_REQUEST_SIZE":     "10485760", // 10MB
		"REQUEST_TIMEOUT":      "30s",
		"RATE_LIMIT":           "100",
	}
}