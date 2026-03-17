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

	// Fetch events from database (or return empty if not implemented)
	events, total := fetchProtectionEvents(limit, offset, eventType, status, provider, timeRange, userID)

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

	// Debug: Return organization info to verify context extraction
	return c.JSON(fiber.Map{
		"debug":        "Organization context working!",
		"org_id":       orgID,
		"limit":        limit,
		"offset":       offset,
		"transactions": []interface{}{},
	})
}

// fetchProtectionEvents fetches protection events from database
func fetchProtectionEvents(limit, offset int, eventType, status, provider, timeRange, userID string) ([]ProtectionEvent, int64) {
	// TODO: Implement database query to fetch actual protection events
	// For now, return empty data until real event logging is implemented

	var events []ProtectionEvent
	var total int64 = 0

	// Apply filters would go here when implemented
	_ = eventType
	_ = status
	_ = provider
	_ = timeRange
	_ = userID

	return events, total
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
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := db.Exec(query, id, status, provider, statusCode, processingTime, len(redactionDetails), string(redactionJSON), time.Now())
	if err != nil {
		fmt.Printf("Failed to log transaction: %v", err)
	}
}
