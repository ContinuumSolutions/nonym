package audit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sovereignprivacy/gateway/pkg/ner"
)

// Event represents a protection event
type Event struct {
	ID          string                `json:"id" db:"id"`
	Timestamp   time.Time             `json:"timestamp" db:"timestamp"`
	Type        string                `json:"type" db:"type"` // pii_detected, request_blocked, provider_error, rate_limit_exceeded
	PIIType     string                `json:"pii_type,omitempty" db:"pii_type"`
	Action      string                `json:"action" db:"action"` // anonymized, blocked, detected
	RequestID   string                `json:"request_id" db:"request_id"`
	UserID      string                `json:"user_id,omitempty" db:"user_id"`
	Provider    string                `json:"provider,omitempty" db:"provider"`
	Model       string                `json:"model,omitempty" db:"model"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Severity    string                `json:"severity" db:"severity"` // low, medium, high, critical
	Status      string                `json:"status" db:"status"` // open, resolved, ignored
	Description string                `json:"description,omitempty" db:"description"`
}

// EventFilter represents filtering options for events
type EventFilter struct {
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
	Type        string    `json:"type,omitempty"`
	PIIType     string    `json:"pii_type,omitempty"`
	Provider    string    `json:"provider,omitempty"`
	Severity    string    `json:"severity,omitempty"`
	Status      string    `json:"status,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	UserID      string    `json:"user_id,omitempty"`
}

// EventsResponse represents the response for events API
type EventsResponse struct {
	Events   []Event `json:"events"`
	Total    int64   `json:"total"`
	HasMore  bool    `json:"has_more"`
	Limit    int     `json:"limit"`
	Offset   int     `json:"offset"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID          string    `json:"id" db:"id"`
	URL         string    `json:"url" db:"url"`
	Events      []string  `json:"events" db:"events"`
	Secret      string    `json:"secret,omitempty" db:"secret"`
	Status      string    `json:"status" db:"status"` // active, disabled, failed
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	LastTrigger time.Time `json:"last_trigger,omitempty" db:"last_trigger"`
	UserID      string    `json:"user_id" db:"user_id"`
}

// WebhookRequest represents a webhook creation request
type WebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret,omitempty"`
}

// LogEvent creates and stores a new protection event
func LogEvent(eventType, piiType, action, requestID, provider, model, userID string, redactionDetails []ner.RedactionDetail) *Event {
	event := &Event{
		ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Type:      eventType,
		PIIType:   piiType,
		Action:    action,
		RequestID: requestID,
		UserID:    userID,
		Provider:  provider,
		Model:     model,
		Metadata: map[string]interface{}{
			"redaction_count":   len(redactionDetails),
			"redaction_details": redactionDetails,
		},
		Severity:    getSeverityForPIIType(piiType),
		Status:      "open",
		Description: generateEventDescription(action, piiType),
	}

	// Store in database
	if err := storeEvent(event); err != nil {
		fmt.Printf("Failed to store event: %v\n", err)
	}

	// Trigger webhooks asynchronously
	go triggerWebhooks(event)

	return event
}

// getSeverityForPIIType determines the severity level for a PII type
func getSeverityForPIIType(piiType string) string {
	switch piiType {
	case "ssn", "credit_card", "api_key":
		return "critical"
	case "email", "phone":
		return "high"
	case "address", "ip_address":
		return "medium"
	default:
		return "low"
	}
}

// generateEventDescription creates a human-readable description
func generateEventDescription(action, piiType string) string {
	switch action {
	case "anonymized":
		return fmt.Sprintf("PII type '%s' was detected and anonymized with secure token", piiType)
	case "blocked":
		return fmt.Sprintf("Request blocked due to detected '%s' in strict mode", piiType)
	case "detected":
		return fmt.Sprintf("PII type '%s' was detected and logged", piiType)
	default:
		return fmt.Sprintf("Protection event for '%s'", piiType)
	}
}

// storeEvent saves an event to the database
func storeEvent(event *Event) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	metadataJSON, _ := json.Marshal(event.Metadata)

	query := `INSERT INTO events (
		id, timestamp, type, pii_type, action, request_id, user_id,
		provider, model, metadata, severity, status, description
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query,
		event.ID, event.Timestamp, event.Type, event.PIIType, event.Action,
		event.RequestID, event.UserID, event.Provider, event.Model,
		string(metadataJSON), event.Severity, event.Status, event.Description)

	return err
}

// GetEvents retrieves events with filtering
func GetEvents(filter EventFilter) (*EventsResponse, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Build query with filters
	query := `SELECT id, timestamp, type, pii_type, action, request_id, user_id,
			  provider, model, metadata, severity, status, description
			  FROM events WHERE 1=1`
	args := []interface{}{}

	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, filter.Type)
	}
	if filter.PIIType != "" {
		query += " AND pii_type = ?"
		args = append(args, filter.PIIType)
	}
	if filter.Provider != "" {
		query += " AND provider = ?"
		args = append(args, filter.Provider)
	}
	if filter.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filter.Severity)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, filter.UserID)
	}
	if !filter.StartTime.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	// Count total for pagination
	countQuery := formatQuery("SELECT COUNT(*) FROM (" + query + ")")
	var total int64
	db.QueryRow(countQuery, args...).Scan(&total)

	// Add ordering and pagination
	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, filter.Limit, filter.Offset)

	// Format query for PostgreSQL parameter binding
	query = formatQuery(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	rowCount := 0
	for rows.Next() {
		rowCount++
		var event Event
		var metadataJSON string
		// Handle nullable fields
		var piiType *string
		var requestID *string
		var userID *string
		var provider *string
		var model *string
		var description *string

		err := rows.Scan(&event.ID, &event.Timestamp, &event.Type, &piiType,
			&event.Action, &requestID, &userID, &provider,
			&model, &metadataJSON, &event.Severity, &event.Status, &description)
		if err != nil {
			continue
		}

		// Handle nullable fields
		if piiType != nil {
			event.PIIType = *piiType
		}
		if requestID != nil {
			event.RequestID = *requestID
		}
		if userID != nil {
			event.UserID = *userID
		}
		if provider != nil {
			event.Provider = *provider
		}
		if model != nil {
			event.Model = *model
		}
		if description != nil {
			event.Description = *description
		}

		// Parse metadata
		json.Unmarshal([]byte(metadataJSON), &event.Metadata)
		events = append(events, event)
	}

	return &EventsResponse{
		Events:  events,
		Total:   total,
		HasMore: int64(filter.Offset+filter.Limit) < total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

// HTTP Handlers

// HandleGetEvents handles GET /api/v1/events
func HandleGetEvents(c *fiber.Ctx) error {
	// Parse query parameters
	filter := EventFilter{
		Limit:    c.QueryInt("limit", 50),
		Offset:   c.QueryInt("offset", 0),
		Type:     c.Query("type"),
		PIIType:  c.Query("pii_type"),
		Provider: c.Query("provider"),
		Severity: c.Query("severity"),
		Status:   c.Query("status"),
	}

	// Limit max results
	if filter.Limit > 200 {
		filter.Limit = 200
	}

	// Parse time filters
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = t
		}
	}

	// Get user ID from context if available
	if user := c.Locals("user"); user != nil {
		if u, ok := user.(map[string]interface{}); ok {
			if userID, ok := u["id"].(string); ok {
				filter.UserID = userID
			}
		}
	}

	events, err := GetEvents(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch events",
		})
	}

	return c.JSON(events)
}

// HandleGetEvent handles GET /api/v1/events/:id
func HandleGetEvent(c *fiber.Ctx) error {
	eventID := c.Params("id")
	if eventID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Event ID is required",
		})
	}

	// Get single event
	filter := EventFilter{Limit: 1, Offset: 0}
	events, err := GetEvents(filter)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch event",
		})
	}

	if len(events.Events) == 0 {
		return c.Status(404).JSON(fiber.Map{
			"error": "Event not found",
		})
	}

	return c.JSON(events.Events[0])
}

// HandleUpdateEventStatus handles PATCH /api/v1/events/:id/status
func HandleUpdateEventStatus(c *fiber.Ctx) error {
	eventID := c.Params("id")
	if eventID == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Event ID is required",
		})
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	validStatuses := map[string]bool{
		"open": true, "resolved": true, "ignored": true,
	}
	if !validStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid status",
		})
	}

	// Update event status
	query := `UPDATE events SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, req.Status, eventID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to update event status",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Event status updated successfully",
	})
}

// HandleCreateWebhook handles POST /api/v1/events/webhook
func HandleCreateWebhook(c *fiber.Ctx) error {
	var req WebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.URL == "" || len(req.Events) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "URL and events are required",
		})
	}

	// Create webhook
	webhook := &Webhook{
		ID:        fmt.Sprintf("wh_%d", time.Now().UnixNano()),
		URL:       req.URL,
		Events:    req.Events,
		Secret:    req.Secret,
		Status:    "active",
		CreatedAt: time.Now(),
		UserID:    getUserIDFromContext(c),
	}

	// Store webhook
	eventsJSON, _ := json.Marshal(webhook.Events)
	query := `INSERT INTO webhooks (id, url, events, secret, status, created_at, user_id)
			  VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, webhook.ID, webhook.URL, string(eventsJSON),
		webhook.Secret, webhook.Status, webhook.CreatedAt, webhook.UserID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create webhook",
		})
	}

	return c.Status(201).JSON(webhook)
}

// HandleGetWebhooks handles GET /api/v1/events/webhooks
func HandleGetWebhooks(c *fiber.Ctx) error {
	userID := getUserIDFromContext(c)

	query := `SELECT id, url, events, status, created_at, last_trigger
			  FROM webhooks WHERE user_id = ? ORDER BY created_at DESC`
	rows, err := db.Query(query, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch webhooks",
		})
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var webhook Webhook
		var eventsJSON string

		err := rows.Scan(&webhook.ID, &webhook.URL, &eventsJSON,
			&webhook.Status, &webhook.CreatedAt, &webhook.LastTrigger)
		if err != nil {
			continue
		}

		json.Unmarshal([]byte(eventsJSON), &webhook.Events)
		webhook.UserID = userID
		webhooks = append(webhooks, webhook)
	}

	return c.JSON(fiber.Map{
		"webhooks": webhooks,
	})
}

// Helper functions

func getUserIDFromContext(c *fiber.Ctx) string {
	if user := c.Locals("user"); user != nil {
		if u, ok := user.(map[string]interface{}); ok {
			if userID, ok := u["id"].(string); ok {
				return userID
			}
			if userID, ok := u["id"].(float64); ok {
				return strconv.Itoa(int(userID))
			}
		}
	}
	return "unknown"
}

func triggerWebhooks(event *Event) {
	// Implementation for triggering webhooks would go here
	// This would make HTTP POST requests to registered webhook URLs
	// For now, we'll just log that webhooks would be triggered
	fmt.Printf("Triggering webhooks for event: %s (type: %s)\n", event.ID, event.Type)
}

// Initialize events tables
func InitializeEventsTables() error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS events (
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
		`CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_events_type ON events(type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity)`,
		`CREATE TABLE IF NOT EXISTS webhooks (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			events TEXT NOT NULL,
			secret TEXT,
			status TEXT DEFAULT 'active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_trigger DATETIME,
			user_id TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_user_id ON webhooks(user_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query %s: %w", query, err)
		}
	}

	return nil
}
