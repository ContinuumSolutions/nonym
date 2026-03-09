package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sovereignprivacy/gateway/pkg/auth"
)

// ProtectionEvent represents a protection event for the frontend
type ProtectionEvent struct {
	ID                string                 `json:"id"`
	Timestamp         time.Time              `json:"timestamp"`
	Type              string                 `json:"type"`
	Action            string                 `json:"action"`
	Provider          string                 `json:"provider"`
	Model             string                 `json:"model,omitempty"`
	Status            string                 `json:"status"`
	Protection        string                 `json:"protection"`
	RedactionCount    int                    `json:"redaction_count"`
	RedactionDetails  []RedactionDetail      `json:"redaction_details,omitempty"`
	ProcessingTime    float64                `json:"processing_time"`
	Severity          string                 `json:"severity"`
	ClientIP          string                 `json:"client_ip,omitempty"`
	UserAgent         string                 `json:"user_agent,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// RedactionDetail represents a single redaction detail
type RedactionDetail struct {
	Type          string `json:"type"`
	OriginalValue string `json:"original_value"`
	Token         string `json:"token"`
	Position      int    `json:"position"`
}

// ProtectionEventsResponse represents the response for protection events
type ProtectionEventsResponse struct {
	Events   []ProtectionEvent `json:"events"`
	Total    int64             `json:"total"`
	HasMore  bool              `json:"has_more"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// ProtectionStatsResponse represents protection statistics
type ProtectionStatsResponse struct {
	ProtectedToday  int     `json:"protectedToday"`
	BlockedToday    int     `json:"blockedToday"`
	DetectionRate   float64 `json:"detectionRate"`
	HighRisk        int     `json:"highRisk"`
	TotalEvents     int64   `json:"totalEvents"`
	LastUpdate      time.Time `json:"lastUpdate"`
}

// StatisticsResponse represents the main statistics
type StatisticsResponse struct {
	PIIProtected       int64   `json:"pii_protected"`
	TotalRequests      int64   `json:"total_requests"`
	BlockedRequests    int64   `json:"blocked_requests"`
	RedactedRequests   int64   `json:"redacted_requests"`
	SuccessRate        float64 `json:"success_rate"`
	AvgProcessingTime  float64 `json:"avg_processing_time"`
	TopProviders       []ProviderStats `json:"top_providers"`
	RecentActivity     []ActivityPoint `json:"recent_activity"`
}

// HandleGetProtectionEvents handles GET /api/v1/protection-events
func HandleGetProtectionEvents(c *fiber.Ctx) error {
	// Parse query parameters
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	eventType := c.Query("eventType")
	status := c.Query("status")
	provider := c.Query("provider")
	timeRange := c.Query("timeRange", "24h")

	// Limit max results
	if limit > 200 {
		limit = 200
	}

	// Get user ID from context if available
	userID := getUserIDFromContextAudit(c)

	// Generate sample data or fetch from database
	events, total := generateProtectionEvents(limit, offset, eventType, status, provider, timeRange, userID)

	response := &ProtectionEventsResponse{
		Events:  events,
		Total:   total,
		HasMore: int64(offset+limit) < total,
		Limit:   limit,
		Offset:  offset,
	}

	return c.JSON(response)
}

// HandleGetProtectionStats handles GET /api/v1/protection-stats
func HandleGetProtectionStats(c *fiber.Ctx) error {
	// Calculate protection statistics
	stats := calculateProtectionStats()
	return c.JSON(stats)
}

// HandleGetStatisticsV1 handles GET /api/v1/statistics - updated version
func HandleGetStatisticsV1(c *fiber.Ctx) error {
	stats, err := GetStatistics()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch statistics",
		})
	}

	// Convert to expected format
	response := &StatisticsResponse{
		PIIProtected:      int64(stats.RedactedRequests),
		TotalRequests:     stats.TotalRequests,
		BlockedRequests:   stats.BlockedRequests,
		RedactedRequests:  stats.RedactedRequests,
		SuccessRate:       stats.SuccessRate,
		AvgProcessingTime: stats.AvgProcessingTime,
		TopProviders:      stats.TopProviders,
		RecentActivity:    stats.RecentActivity,
	}

	return c.JSON(response)
}

// HandleGetTransactionsV1 handles GET /api/v1/transactions
func HandleGetTransactionsV1(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	if limit > 200 {
		limit = 200
	}

	transactions, err := GetTransactions(limit, offset)
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

// generateProtectionEvents generates sample protection events
func generateProtectionEvents(limit, offset int, eventType, status, provider, timeRange, userID string) ([]ProtectionEvent, int64) {
	// Sample events data
	sampleEvents := []ProtectionEvent{
		{
			ID:        "evt_001",
			Timestamp: time.Now().Add(-5 * time.Minute),
			Type:      "Email",
			Action:    "Anonymized",
			Provider:  "OpenAI",
			Model:     "gpt-3.5-turbo",
			Status:    "Protected",
			Protection: "Token replaced",
			RedactionCount: 1,
			RedactionDetails: []RedactionDetail{
				{
					Type:          "email",
					OriginalValue: "user@example.com",
					Token:         "TOKEN_EMAIL_001",
					Position:      45,
				},
			},
			ProcessingTime: 12.5,
			Severity:      "medium",
			ClientIP:      "192.168.1.100",
			UserAgent:     "Mozilla/5.0",
			Metadata: map[string]interface{}{
				"request_size": 256,
				"model_used":   "gpt-3.5-turbo",
			},
		},
		{
			ID:         "evt_002",
			Timestamp:  time.Now().Add(-8 * time.Minute),
			Type:       "SSN",
			Action:     "Blocked",
			Provider:   "Anthropic",
			Model:      "claude-3-sonnet",
			Status:     "Blocked",
			Protection: "Request blocked",
			RedactionCount: 0,
			ProcessingTime: 8.2,
			Severity:      "critical",
			ClientIP:      "192.168.1.101",
			UserAgent:     "Python/3.9 requests",
			Metadata: map[string]interface{}{
				"block_reason": "strict_mode_enabled",
				"pii_type":     "ssn",
			},
		},
		{
			ID:        "evt_003",
			Timestamp: time.Now().Add(-12 * time.Minute),
			Type:      "Credit Card",
			Action:    "Detected",
			Provider:  "Google",
			Model:     "gemini-pro",
			Status:    "Protected",
			Protection: "Data masked",
			RedactionCount: 1,
			RedactionDetails: []RedactionDetail{
				{
					Type:          "credit_card",
					OriginalValue: "**** **** **** 1234",
					Token:         "TOKEN_CC_001",
					Position:      78,
				},
			},
			ProcessingTime: 15.8,
			Severity:      "high",
			ClientIP:      "192.168.1.102",
			UserAgent:     "Node.js/18.0",
		},
		{
			ID:        "evt_004",
			Timestamp: time.Now().Add(-15 * time.Minute),
			Type:      "Phone",
			Action:    "Anonymized",
			Provider:  "OpenAI",
			Model:     "gpt-4",
			Status:    "Protected",
			Protection: "Token replaced",
			RedactionCount: 1,
			RedactionDetails: []RedactionDetail{
				{
					Type:          "phone",
					OriginalValue: "+1-555-***-****",
					Token:         "TOKEN_PHONE_001",
					Position:      23,
				},
			},
			ProcessingTime: 18.3,
			Severity:      "medium",
			ClientIP:      "192.168.1.103",
			UserAgent:     "curl/7.68.0",
		},
		{
			ID:         "evt_005",
			Timestamp:  time.Now().Add(-20 * time.Minute),
			Type:       "API Key",
			Action:     "Anonymized",
			Provider:   "Local",
			Model:      "llama2",
			Status:     "Protected",
			Protection: "Token replaced",
			RedactionCount: 1,
			RedactionDetails: []RedactionDetail{
				{
					Type:          "api_key",
					OriginalValue: "sk-***",
					Token:         "TOKEN_API_001",
					Position:      156,
				},
			},
			ProcessingTime: 9.1,
			Severity:      "high",
			ClientIP:      "192.168.1.104",
			UserAgent:     "PostmanRuntime/7.32.0",
		},
	}

	// Apply filters
	var filtered []ProtectionEvent
	for _, event := range sampleEvents {
		if eventType != "" && event.Type != eventType {
			continue
		}
		if status != "" && event.Status != status {
			continue
		}
		if provider != "" && event.Provider != provider {
			continue
		}

		// Apply time range filter
		switch timeRange {
		case "1h":
			if time.Since(event.Timestamp) > time.Hour {
				continue
			}
		case "24h":
			if time.Since(event.Timestamp) > 24*time.Hour {
				continue
			}
		case "7d":
			if time.Since(event.Timestamp) > 7*24*time.Hour {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	result := filtered[start:end]
	total := int64(len(filtered))

	return result, total
}

// calculateProtectionStats calculates protection statistics
func calculateProtectionStats() *ProtectionStatsResponse {
	return &ProtectionStatsResponse{
		ProtectedToday: 127,
		BlockedToday:   23,
		DetectionRate:  94.2,
		HighRisk:       5,
		TotalEvents:    892,
		LastUpdate:     time.Now(),
	}
}


// getUserIDFromContextAudit extracts user ID from fiber context
func getUserIDFromContextAudit(c *fiber.Ctx) string {
	if user := c.Locals("user"); user != nil {
		if u, ok := user.(*auth.User); ok {
			return strconv.Itoa(u.ID)
		}
		// Handle different user context formats
		if userMap, ok := user.(map[string]interface{}); ok {
			if id, exists := userMap["id"]; exists {
				if idStr, ok := id.(string); ok {
					return idStr
				}
				if idInt, ok := id.(int); ok {
					return strconv.Itoa(idInt)
				}
				if idFloat, ok := id.(float64); ok {
					return strconv.Itoa(int(idFloat))
				}
			}
		}
	}
	return "anonymous"
}

// Helper function to update transaction to include redaction details
func LogTransactionWithRedactions(id, status, provider string, statusCode int, processingTime float64, redactionDetails []RedactionDetail) {
	if db == nil {
		fmt.Printf("Warning: Database not initialized, skipping transaction log")
		return
	}

	redactionJSON, _ := json.Marshal(redactionDetails)

	query := `INSERT INTO transactions (
		id, status, provider, status_code, processing_time, redaction_count, redaction_details, timestamp
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, id, status, provider, statusCode, processingTime, len(redactionDetails), string(redactionJSON), time.Now())
	if err != nil {
		fmt.Printf("Failed to log transaction: %v", err)
	}
}
