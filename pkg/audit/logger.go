package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sovereignprivacy/gateway/pkg/ner"
	_ "modernc.org/sqlite"
)

// Transaction represents a logged transaction
type Transaction struct {
	ID               string                `json:"id" db:"id"`
	Timestamp        time.Time             `json:"timestamp" db:"timestamp"`
	Status           string                `json:"status" db:"status"`
	Provider         string                `json:"provider" db:"provider"`
	StatusCode       int                   `json:"status_code" db:"status_code"`
	ProcessingTime   float64               `json:"processing_time" db:"processing_time"`
	RedactionCount   int                   `json:"redaction_count" db:"redaction_count"`
	RedactionDetails []ner.RedactionDetail `json:"redaction_details"`
	ClientIP         string                `json:"client_ip" db:"client_ip"`
	UserAgent        string                `json:"user_agent" db:"user_agent"`
	ErrorMessage     string                `json:"error_message,omitempty" db:"error_message"`
	OrganizationID   int                   `json:"organization_id" db:"organization_id"`
	UserID           int                   `json:"user_id,omitempty" db:"user_id"`
}

// Statistics represents aggregated statistics
type Statistics struct {
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	BlockedRequests    int64   `json:"blocked_requests"`
	RedactedRequests   int64   `json:"redacted_requests"`
	SuccessRate        float64 `json:"success_rate"`
	AvgProcessingTime  float64 `json:"avg_processing_time"`
	TopProviders       []ProviderStats `json:"top_providers"`
	RecentActivity     []ActivityPoint `json:"recent_activity"`
}

type ProviderStats struct {
	Provider string `json:"provider"`
	Requests int64  `json:"requests"`
	Percent  float64 `json:"percent"`
}

type ActivityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}

// Settings represents gateway configuration
type Settings struct {
	StrictMode       bool     `json:"strict_mode"`
	AllowedProviders []string `json:"allowed_providers"`
	BlockedEntities  []string `json:"blocked_entities"`
	RetentionDays    int      `json:"retention_days"`
	LogLevel         string   `json:"log_level"`
}

var (
	db       *sql.DB
	settings *Settings
)

// Initialize sets up the audit database and default settings
func Initialize(databasePath string) error {
	var err error

	// Open SQLite database
	db, err = sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Load settings
	settings = &Settings{
		StrictMode:       false,
		AllowedProviders: []string{"openai", "anthropic", "google", "local"},
		BlockedEntities:  []string{},
		RetentionDays:    30,
		LogLevel:         "info",
	}

	log.Println("Audit system initialized successfully")
	return nil
}

func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL,
			provider TEXT NOT NULL,
			status_code INTEGER DEFAULT 0,
			processing_time REAL DEFAULT 0,
			redaction_count INTEGER DEFAULT 0,
			redaction_details TEXT DEFAULT '[]',
			client_ip TEXT DEFAULT '',
			user_agent TEXT DEFAULT '',
			error_message TEXT DEFAULT '',
			organization_id INTEGER NOT NULL,
			user_id INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_provider ON transactions(provider)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_organization ON transactions(organization_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_user ON transactions(user_id)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}

// LogTransaction records a transaction in the audit log with organization context
func LogTransaction(id, status, provider string, statusCode int, redactionDetails []ner.RedactionDetail, organizationID, userID int) {
	if db == nil {
		log.Printf("Warning: Database not initialized, skipping transaction log")
		return
	}

	redactionJSON, _ := json.Marshal(redactionDetails)

	query := `INSERT INTO transactions (
		id, status, provider, status_code, redaction_count, redaction_details, organization_id, user_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, id, status, provider, statusCode, len(redactionDetails), string(redactionJSON), organizationID, userID)
	if err != nil {
		log.Printf("Failed to log transaction: %v", err)
	}

	// Notify about significant transactions
	broadcastTransaction(&Transaction{
		ID:               id,
		Timestamp:        time.Now(),
		Status:           status,
		Provider:         provider,
		StatusCode:       statusCode,
		RedactionCount:   len(redactionDetails),
		RedactionDetails: redactionDetails,
		OrganizationID:   organizationID,
		UserID:           userID,
	})
}

// GetTransactions retrieves transactions with pagination, scoped to organization
func GetTransactions(limit, offset int, organizationID int) ([]Transaction, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `SELECT id, timestamp, status, provider, status_code, processing_time,
			  redaction_count, redaction_details, client_ip, user_agent, error_message, organization_id, user_id
			  FROM transactions WHERE organization_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`

	rows, err := db.Query(query, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var redactionDetailsJSON string

		err := rows.Scan(&t.ID, &t.Timestamp, &t.Status, &t.Provider, &t.StatusCode,
			&t.ProcessingTime, &t.RedactionCount, &redactionDetailsJSON,
			&t.ClientIP, &t.UserAgent, &t.ErrorMessage, &t.OrganizationID, &t.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Parse redaction details
		json.Unmarshal([]byte(redactionDetailsJSON), &t.RedactionDetails)
		transactions = append(transactions, t)
	}

	return transactions, nil
}

// GetStatistics calculates and returns system statistics for an organization
func GetStatistics(organizationID int) (*Statistics, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := &Statistics{}

	// Total requests
	err := db.QueryRow("SELECT COUNT(*) FROM transactions WHERE organization_id = ?", organizationID).Scan(&stats.TotalRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to get total requests: %w", err)
	}

	// Successful requests
	err = db.QueryRow("SELECT COUNT(*) FROM transactions WHERE status = 'success' AND organization_id = ?", organizationID).Scan(&stats.SuccessfulRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to get successful requests: %w", err)
	}

	// Blocked requests
	err = db.QueryRow("SELECT COUNT(*) FROM transactions WHERE status = 'blocked' AND organization_id = ?", organizationID).Scan(&stats.BlockedRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked requests: %w", err)
	}

	// Redacted requests
	err = db.QueryRow("SELECT COUNT(*) FROM transactions WHERE redaction_count > 0 AND organization_id = ?", organizationID).Scan(&stats.RedactedRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to get redacted requests: %w", err)
	}

	// Calculate success rate
	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests) * 100
	}

	// Average processing time
	err = db.QueryRow("SELECT AVG(processing_time) FROM transactions WHERE processing_time > 0 AND organization_id = ?", organizationID).Scan(&stats.AvgProcessingTime)
	if err != nil {
		stats.AvgProcessingTime = 0
	}

	// Top providers
	providerRows, err := db.Query(`SELECT provider, COUNT(*) as requests
									FROM transactions WHERE organization_id = ?
									GROUP BY provider
									ORDER BY requests DESC LIMIT 5`, organizationID)
	if err == nil {
		defer providerRows.Close()
		for providerRows.Next() {
			var ps ProviderStats
			providerRows.Scan(&ps.Provider, &ps.Requests)
			if stats.TotalRequests > 0 {
				ps.Percent = float64(ps.Requests) / float64(stats.TotalRequests) * 100
			}
			stats.TopProviders = append(stats.TopProviders, ps)
		}
	}

	// Recent activity (last 24 hours, hourly buckets)
	activityRows, err := db.Query(`SELECT
		datetime(timestamp, 'start of hour') as hour,
		COUNT(*) as count
		FROM transactions
		WHERE timestamp >= datetime('now', '-24 hours') AND organization_id = ?
		GROUP BY hour
		ORDER BY hour DESC`, organizationID)
	if err == nil {
		defer activityRows.Close()
		for activityRows.Next() {
			var ap ActivityPoint
			var hourStr string
			activityRows.Scan(&hourStr, &ap.Count)
			ap.Timestamp, _ = time.Parse("2006-01-02 15:04:05", hourStr)
			stats.RecentActivity = append(stats.RecentActivity, ap)
		}
	}

	return stats, nil
}

// HTTP Handlers

// HandleGetTransactions handles GET /api/transactions with organization context
func HandleGetTransactions(c *fiber.Ctx) error {
	// Extract organization ID from context (set by middleware)
	organizationID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	if limit > 200 {
		limit = 200
	}

	transactions, err := GetTransactions(limit, offset, organizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch transactions",
		})
	}

	return c.JSON(fiber.Map{
		"transactions": transactions,
		"limit":        limit,
		"offset":       offset,
	})
}

// HandleGetStatistics handles GET /api/statistics with organization context
func HandleGetStatistics(c *fiber.Ctx) error {
	// Extract organization ID from context (set by middleware)
	organizationID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	stats, err := GetStatistics(organizationID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch statistics",
		})
	}

	return c.JSON(stats)
}

// HandleGetSettings handles GET /api/settings
func HandleGetSettings(c *fiber.Ctx) error {
	return c.JSON(settings)
}

// HandleUpdateSettings handles PUT /api/settings
func HandleUpdateSettings(c *fiber.Ctx) error {
	var newSettings Settings
	if err := c.BodyParser(&newSettings); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid settings format",
		})
	}

	// Update global settings
	settings = &newSettings

	// Persist to database (optional)
	settingsJSON, _ := json.Marshal(settings)
	db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES ('main', ?)", string(settingsJSON))

	return c.JSON(fiber.Map{
		"message": "Settings updated successfully",
		"settings": settings,
	})
}

// Real-time updates handling
var (
	activeConnections = 0
)

func HandleWebSocket(c *fiber.Ctx) error {
	// Note: This is a simplified WebSocket handler
	// In production, you'd properly integrate with Fiber's WebSocket support
	return c.JSON(fiber.Map{
		"message": "WebSocket endpoint available",
		"note":    "Use dedicated WebSocket client library",
	})
}

func broadcastTransaction(transaction *Transaction) {
	// In a real implementation, this would broadcast to connected WebSocket clients
	// For now, we'll just log significant transactions
	if transaction.Status == "blocked" || transaction.RedactionCount > 0 {
		log.Printf("Significant transaction: %s - %s (redactions: %d)",
			transaction.ID, transaction.Status, transaction.RedactionCount)
	}
}
// GetDatabase returns the database connection for sharing with other packages
func GetDatabase() *sql.DB {
	return db
}
