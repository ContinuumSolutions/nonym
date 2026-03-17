package audit

import (
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create tables using the same schema as production
	queries := []string{
		`CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL,
			provider TEXT NOT NULL,
			status_code INTEGER DEFAULT 0,
			processing_time_ms REAL DEFAULT 0,
			redaction_count INTEGER DEFAULT 0,
			entities_detected TEXT DEFAULT '[]',
			ip_address TEXT,
			user_agent TEXT,
			organization_id INTEGER NOT NULL,
			user_id INTEGER
		)`,
		`CREATE TABLE events (
			id TEXT PRIMARY KEY,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			type TEXT NOT NULL,
			pii_type TEXT,
			action TEXT NOT NULL,
			request_id TEXT,
			user_id TEXT,
			provider TEXT,
			model TEXT,
			metadata TEXT DEFAULT '{}',
			severity TEXT DEFAULT 'low',
			status TEXT DEFAULT 'open',
			description TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}
	}

	return db
}

func TestGetTransactions(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Backup original db and replace with test db
	originalDB := db
	db = testDB
	defer func() { db = originalDB }()

	// Insert test data
	testData := []struct {
		status     string
		provider   string
		statusCode int
		orgID      int
		userID     int
	}{
		{"success", "openai", 200, 1, 1},
		{"success", "anthropic", 200, 1, 1},
		{"blocked", "google", 400, 1, 1},
		{"success", "openai", 200, 2, 2}, // Different organization
	}

	for _, data := range testData {
		_, err := testDB.Exec(`
			INSERT INTO transactions (status, provider, status_code, organization_id, user_id, entities_detected)
			VALUES (?, ?, ?, ?, ?, ?)
		`, data.status, data.provider, data.statusCode, data.orgID, data.userID, "[]")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Test cases
	tests := []struct {
		name           string
		limit          int
		offset         int
		organizationID string
		expectedCount  int
		expectError    bool
	}{
		{
			name:           "Get transactions for org 1",
			limit:          10,
			offset:         0,
			organizationID: "1",
			expectedCount:  3,
			expectError:    false,
		},
		{
			name:           "Get transactions for org 2",
			limit:          10,
			offset:         0,
			organizationID: "2",
			expectedCount:  1,
			expectError:    false,
		},
		{
			name:           "Get transactions with limit",
			limit:          2,
			offset:         0,
			organizationID: "1",
			expectedCount:  2,
			expectError:    false,
		},
		{
			name:           "Get transactions with offset",
			limit:          10,
			offset:         1,
			organizationID: "1",
			expectedCount:  2,
			expectError:    false,
		},
		{
			name:           "Invalid organization ID",
			limit:          10,
			offset:         0,
			organizationID: "invalid",
			expectedCount:  0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transactions, err := GetTransactions(tt.limit, tt.offset, tt.organizationID)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(transactions) != tt.expectedCount {
				t.Errorf("Expected %d transactions, got %d", tt.expectedCount, len(transactions))
			}

			// Verify transaction properties
			for _, tx := range transactions {
				if tx.ID == "" {
					t.Error("Transaction ID should not be empty")
				}
				if tx.OrganizationID == 0 {
					t.Error("Organization ID should not be zero")
				}
			}
		})
	}
}

func TestGetEvents(t *testing.T) {
	// Setup test database
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Backup original db and replace with test db
	originalDB := db
	db = testDB
	defer func() { db = originalDB }()

	// Insert test data
	testData := []struct {
		id          string
		eventType   string
		piiType     string
		action      string
		provider    string
		severity    string
		status      string
		description string
	}{
		{"evt_1", "pii_detected", "email", "anonymized", "openai", "high", "open", "Email detected"},
		{"evt_2", "pii_detected", "phone", "anonymized", "anthropic", "medium", "open", "Phone detected"},
		{"evt_3", "request_blocked", "ssn", "blocked", "google", "critical", "open", "SSN blocked"},
		{"evt_4", "pii_detected", "email", "anonymized", "openai", "low", "resolved", "Email resolved"},
	}

	for _, data := range testData {
		_, err := testDB.Exec(`
			INSERT INTO events (id, type, pii_type, action, provider, severity, status, description)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, data.id, data.eventType, data.piiType, data.action, data.provider, data.severity, data.status, data.description)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Test cases
	tests := []struct {
		name          string
		filter        EventFilter
		expectedCount int
		expectError   bool
	}{
		{
			name: "Get all events",
			filter: EventFilter{
				Limit:  10,
				Offset: 0,
			},
			expectedCount: 4,
			expectError:   false,
		},
		{
			name: "Filter by type",
			filter: EventFilter{
				Limit:  10,
				Offset: 0,
				Type:   "pii_detected",
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "Filter by provider",
			filter: EventFilter{
				Limit:    10,
				Offset:   0,
				Provider: "openai",
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "Filter by severity",
			filter: EventFilter{
				Limit:    10,
				Offset:   0,
				Severity: "critical",
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "Filter by status",
			filter: EventFilter{
				Limit:  10,
				Offset: 0,
				Status: "resolved",
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "Limit and offset",
			filter: EventFilter{
				Limit:  2,
				Offset: 1,
			},
			expectedCount: 2,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := GetEvents(tt.filter)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(response.Events) != tt.expectedCount {
				// Debug: Check what was actually returned
				t.Logf("Filter: %+v", tt.filter)
				t.Logf("Response: Total=%d, Events=%d", response.Total, len(response.Events))

				// Check if any events exist in database at all
				var count int
				testDB.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
				t.Logf("Total events in database: %d", count)

				t.Errorf("Expected %d events, got %d", tt.expectedCount, len(response.Events))
			}

			// Verify event properties
			for _, event := range response.Events {
				if event.ID == "" {
					t.Error("Event ID should not be empty")
				}
				if event.Type == "" {
					t.Error("Event type should not be empty")
				}
				if event.Action == "" {
					t.Error("Event action should not be empty")
				}
			}

			// Verify pagination info
			if response.Limit != tt.filter.Limit {
				t.Errorf("Expected limit %d, got %d", tt.filter.Limit, response.Limit)
			}
			if response.Offset != tt.filter.Offset {
				t.Errorf("Expected offset %d, got %d", tt.filter.Offset, response.Offset)
			}
		})
	}
}

func TestTransactionScanningWithNullableFields(t *testing.T) {
	// Test that nullable fields (ip_address, user_agent) are handled correctly
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Backup original db and replace with test db
	originalDB := db
	db = testDB
	defer func() { db = originalDB }()

	// Insert transaction with NULL ip_address and user_agent
	_, err := testDB.Exec(`
		INSERT INTO transactions (status, provider, status_code, organization_id, user_id, entities_detected, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "success", "openai", 200, 1, 1, "[]", nil, nil)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Insert transaction with actual ip_address and user_agent
	_, err = testDB.Exec(`
		INSERT INTO transactions (status, provider, status_code, organization_id, user_id, entities_detected, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "success", "anthropic", 200, 1, 1, "[]", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	transactions, err := GetTransactions(10, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(transactions))
	}

	// Check that nullable fields are handled correctly
	foundNullFields := false
	foundNonNullFields := false

	for _, tx := range transactions {
		if tx.ClientIP == "" && tx.UserAgent == "" {
			foundNullFields = true
		}
		if tx.ClientIP != "" && tx.UserAgent != "" {
			foundNonNullFields = true
		}
	}

	if !foundNullFields {
		t.Error("Should have found transaction with null IP and user agent")
	}
	if !foundNonNullFields {
		t.Error("Should have found transaction with non-null IP and user agent")
	}
}

func TestTransactionNullProcessingTime(t *testing.T) {
	// Test that nullable processing_time_ms field is handled correctly
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Backup original db and replace with test db
	originalDB := db
	db = testDB
	defer func() { db = originalDB }()

	// Insert transaction with NULL processing_time_ms
	_, err := testDB.Exec(`
		INSERT INTO transactions (status, provider, status_code, organization_id, user_id, entities_detected, processing_time_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "success", "openai", 200, 1, 1, "[]", nil)
	if err != nil {
		t.Fatalf("Failed to insert test data with NULL processing_time_ms: %v", err)
	}

	// Insert transaction with actual processing_time_ms
	_, err = testDB.Exec(`
		INSERT INTO transactions (status, provider, status_code, organization_id, user_id, entities_detected, processing_time_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "success", "anthropic", 200, 1, 1, "[]", 125.5)
	if err != nil {
		t.Fatalf("Failed to insert test data with processing_time_ms: %v", err)
	}

	transactions, err := GetTransactions(10, 0, "1")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(transactions) != 2 {
		t.Fatalf("Expected 2 transactions, got %d", len(transactions))
	}

	// Check that NULL and non-NULL processing times are handled correctly
	foundNullProcessingTime := false
	foundNonNullProcessingTime := false

	for _, tx := range transactions {
		if tx.ProcessingTime == 0 && tx.Provider == "openai" {
			foundNullProcessingTime = true
		}
		if tx.ProcessingTime == 125.5 && tx.Provider == "anthropic" {
			foundNonNullProcessingTime = true
		}
	}

	if !foundNullProcessingTime {
		t.Error("Should have found transaction with null processing time (set to 0)")
	}
	if !foundNonNullProcessingTime {
		t.Error("Should have found transaction with non-null processing time (125.5)")
	}
}

func TestEventJSONParsing(t *testing.T) {
	// Test that event metadata JSON is parsed correctly
	testDB := setupTestDB(t)
	defer testDB.Close()

	// Backup original db and replace with test db
	originalDB := db
	db = testDB
	defer func() { db = originalDB }()

	// Insert event with simple JSON metadata
	metadata := map[string]interface{}{
		"redaction_count": 2,
		"request_size":    1024,
		"response_time":   150.5,
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err := testDB.Exec(`
		INSERT INTO events (id, type, action, metadata)
		VALUES (?, ?, ?, ?)
	`, "evt_json_test", "pii_detected", "anonymized", string(metadataJSON))
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	response, err := GetEvents(EventFilter{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(response.Events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(response.Events))
	}

	event := response.Events[0]
	if event.Metadata == nil {
		t.Error("Event metadata should not be nil")
	}

	// Check that metadata was parsed correctly
	if redactionCount, ok := event.Metadata["redaction_count"]; !ok {
		t.Error("Metadata should contain redaction_count")
	} else if count, ok := redactionCount.(float64); !ok || count != 2 {
		t.Errorf("Expected redaction_count to be 2, got %v", redactionCount)
	}

	if requestSize, ok := event.Metadata["request_size"]; !ok {
		t.Error("Metadata should contain request_size")
	} else if size, ok := requestSize.(float64); !ok || size != 1024 {
		t.Errorf("Expected request_size to be 1024, got %v", requestSize)
	}
}