package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
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
	organizationID, ok := c.Locals("organization_id").(int)
	if !ok || organizationID == 0 {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 200 {
		limit = 200
	}

	filter := EventFilter{
		Limit:          limit,
		Offset:         offset,
		OrganizationID: organizationID,
		Type:           c.Query("eventType"), // Map eventType to Type for consistency with frontend
		PIIType:        c.Query("pii_type"),
		Provider:       c.Query("provider"),
		Severity:       c.Query("severity"),
		Status:         c.Query("status"),
		UserID:         c.Query("user_id"),
	}

	// Call the actual database query function
	eventsResponse, err := GetEvents(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch protection events: %v", err),
		})
	}

	// Convert Event to ProtectionEvent format
	protectionEvents := make([]ProtectionEvent, len(eventsResponse.Events))
	for i, event := range eventsResponse.Events {
		protectionEvents[i] = ProtectionEvent{
			ID:             event.ID,
			Timestamp:      event.Timestamp,
			Type:           event.Type,
			Action:         event.Action,
			Provider:       event.Provider,
			Model:          event.Model,
			Status:         event.Status,
			Protection:     event.PIIType,
			RedactionCount: getInt(event.Metadata, "redaction_count"),
			ProcessingTime: 0, // Not tracked in current event model
			Severity:       event.Severity,
			Metadata:       event.Metadata,
		}

		// Extract redaction details from metadata if available
		if redactionDetails, ok := event.Metadata["redaction_details"]; ok {
			if details, ok := redactionDetails.([]interface{}); ok {
				protectionEvents[i].RedactionDetails = make([]RedactionDetail, len(details))
				for j, detail := range details {
					if detailMap, ok := detail.(map[string]interface{}); ok {
						protectionEvents[i].RedactionDetails[j] = RedactionDetail{
							Type:          getString(detailMap, "type"),
							OriginalValue: getString(detailMap, "original_value"),
							Token:         getString(detailMap, "token"),
							Position:      getInt(detailMap, "position"),
						}
					}
				}
			}
		}
	}

	response := &ProtectionEventsResponse{
		Events:  protectionEvents,
		Total:   eventsResponse.Total,
		HasMore: eventsResponse.HasMore,
		Limit:   eventsResponse.Limit,
		Offset:  eventsResponse.Offset,
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
	// Extract organization ID from context (set by middleware)
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	organizationID := strconv.Itoa(orgID)
	stats, err := GetStatistics(organizationID)
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
	// Extract organization ID from context (set by middleware)
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	if limit > 200 {
		limit = 200
	}

	// Call the actual database query function
	organizationIDStr := strconv.Itoa(orgID)
	transactions, err := GetTransactions(limit, offset, organizationIDStr)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch transactions: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"transactions": transactions,
		"limit":        limit,
		"offset":       offset,
		"total":        len(transactions),
	})
}


// calculateProtectionStats calculates protection statistics
func calculateProtectionStats() *ProtectionStatsResponse {
	// TODO: Implement actual statistics calculation from database
	// For now, return zero values until real event logging is implemented
	return &ProtectionStatsResponse{
		ProtectedToday: 0,
		BlockedToday:   0,
		DetectionRate:  0.0,
		HighRisk:       0,
		TotalEvents:    0,
		LastUpdate:     time.Now(),
	}
}



// Helper function to update transaction to include redaction details
func LogTransactionWithRedactions(id, status, provider string, statusCode int, processingTime float64, redactionDetails []RedactionDetail) {
	if db == nil {
		fmt.Printf("Warning: Database not initialized, skipping transaction log")
		return
	}

	redactionJSON, _ := json.Marshal(redactionDetails)

	query := formatQuery(`INSERT INTO transactions (
		request_id, method, path, provider, status, status_code, processing_time_ms, redaction_count, entities_detected
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)

	_, err := db.Exec(query, id, "POST", "/v1/chat/completions", provider, status, statusCode, processingTime, len(redactionDetails), string(redactionJSON))
	if err != nil {
		fmt.Printf("Failed to log transaction: %v", err)
	}
}

// Helper functions for metadata conversion
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if intVal, err := strconv.Atoi(v); err == nil {
				return intVal
			}
		}
	}
	return 0
}
