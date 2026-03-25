package audit

import (
	"fmt"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Dashboard Widget Configuration Types
type WidgetConfig struct {
	ID                     string          `json:"id"`
	Type                   string          `json:"type"`
	Title                  string          `json:"title"`
	Subtitle               *string         `json:"subtitle"`
	Endpoint               string          `json:"endpoint"`
	RefreshIntervalSeconds int             `json:"refresh_interval_seconds"`
	Grid                   GridPlacement   `json:"grid"`
	Display                DisplayOptions  `json:"display"`
}

type GridPlacement struct {
	Row     int `json:"row"`
	ColSpan int `json:"col_span"`
	Order   int `json:"order"`
}

type DisplayOptions struct {
	ColorScheme       string   `json:"color_scheme"`
	Icon              *string  `json:"icon"`
	ValueFormat       string   `json:"value_format"`
	ThresholdWarning  *float64 `json:"threshold_warning"`
	ThresholdCritical *float64 `json:"threshold_critical"`
}

type DashboardLayoutResponse struct {
	Widgets []WidgetConfig `json:"widgets"`
}

// Widget Data Response Types
type StatCardData struct {
	Value       float64 `json:"value"`
	Unit        *string `json:"unit"`
	Delta       float64 `json:"delta"`
	DeltaPeriod string  `json:"delta_period"`
}

type SuccessRateData struct {
	SuccessRate      float64 `json:"success_rate"`
	RedactedRequests int64   `json:"redacted_requests"`
	TotalRequests    int64   `json:"total_requests"`
}

type ProtectionSummaryData struct {
	Rows []ProtectionRow `json:"rows"`
}

type ProtectionRow struct {
	Key   string      `json:"key"`
	Label string      `json:"label"`
	Value interface{} `json:"value"`
	Color string      `json:"color"`
}

type GatewayStatusData struct {
	Status      string             `json:"status"`
	Uptime      int64              `json:"uptime"`
	Components  []ComponentStatus  `json:"components"`
	MemoryUsage MemoryUsage        `json:"memory_usage"`
}

type ComponentStatus struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Provider bool   `json:"provider"`
}

type MemoryUsage struct {
	Heap  int64 `json:"heap"`
	Stack int64 `json:"stack"`
	Total int64 `json:"total"`
}

type TopProvidersData struct {
	Providers []ProviderData `json:"providers"`
}

type ProviderData struct {
	Provider string  `json:"provider"`
	Label    string  `json:"label"`
	Requests int64   `json:"requests"`
	Percent  float64 `json:"percent"`
	Color    string  `json:"color"`
}

type ActivityChartData struct {
	PeriodLabel string         `json:"period_label"`
	Points      []ActivityData `json:"points"`
}

type ActivityData struct {
	Timestamp string `json:"timestamp"`
	Count     int64  `json:"count"`
}

type LiveStatsData struct {
	Rows []LiveStatRow `json:"rows"`
}

type LiveStatRow struct {
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Value     float64    `json:"value"`
	Format    string     `json:"format"`
	ColorRule *ColorRule `json:"color_rule,omitempty"`
}

type ColorRule struct {
	ThresholdWarning  float64 `json:"threshold_warning"`
	ThresholdCritical float64 `json:"threshold_critical"`
	Direction         string  `json:"direction"`
}

// Global variables to track application state
var (
	startTime         = time.Now()
	lastMinuteCounter int64
	lastUpdateTime    = time.Now()
	currentRPS        float64
)

// HandleGetDashboardLayout returns the widget configuration
func HandleGetDashboardLayout(c *fiber.Ctx) error {
	widgets := []WidgetConfig{
		{
			ID:                     "stat-total-requests",
			Type:                   "stat_card",
			Title:                  "Total Requests",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/stat-total-requests",
			RefreshIntervalSeconds: 30,
			Grid:                   GridPlacement{Row: 1, ColSpan: 3, Order: 0},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "stat-pii-protected",
			Type:                   "stat_card",
			Title:                  "PII Protected",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/stat-pii-protected",
			RefreshIntervalSeconds: 30,
			Grid:                   GridPlacement{Row: 1, ColSpan: 3, Order: 1},
			Display:                DisplayOptions{ColorScheme: "green", ValueFormat: "number"},
		},
		{
			ID:                     "stat-blocked-requests",
			Type:                   "stat_card",
			Title:                  "Blocked Requests",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/stat-blocked-requests",
			RefreshIntervalSeconds: 30,
			Grid:                   GridPlacement{Row: 1, ColSpan: 3, Order: 2},
			Display:                DisplayOptions{ColorScheme: "red", ValueFormat: "number"},
		},
		{
			ID:                     "stat-avg-latency",
			Type:                   "stat_card",
			Title:                  "Avg Latency",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/stat-avg-latency",
			RefreshIntervalSeconds: 30,
			Grid:                   GridPlacement{Row: 1, ColSpan: 3, Order: 3},
			Display:                DisplayOptions{ColorScheme: "purple", ValueFormat: "duration_ms"},
		},
		{
			ID:                     "success-rate",
			Type:                   "success_rate",
			Title:                  "Success Rate",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/success-rate",
			RefreshIntervalSeconds: 60,
			Grid:                   GridPlacement{Row: 2, ColSpan: 4, Order: 0},
			Display:                DisplayOptions{ColorScheme: "green", ValueFormat: "percent"},
		},
		{
			ID:                     "protection-summary",
			Type:                   "protection_summary",
			Title:                  "Today's Protection",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/protection-summary",
			RefreshIntervalSeconds: 60,
			Grid:                   GridPlacement{Row: 2, ColSpan: 4, Order: 1},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "gateway-status",
			Type:                   "gateway_status",
			Title:                  "Gateway Status",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/gateway-status",
			RefreshIntervalSeconds: 15,
			Grid:                   GridPlacement{Row: 2, ColSpan: 4, Order: 2},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "top-providers",
			Type:                   "top_providers",
			Title:                  "Top Providers",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/top-providers",
			RefreshIntervalSeconds: 60,
			Grid:                   GridPlacement{Row: 3, ColSpan: 4, Order: 0},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "recent-activity",
			Type:                   "activity_chart",
			Title:                  "Recent Activity",
			Subtitle:               stringPtr("Last 24 hours"),
			Endpoint:               "/api/v1/dashboard/widgets/recent-activity",
			RefreshIntervalSeconds: 300,
			Grid:                   GridPlacement{Row: 3, ColSpan: 4, Order: 1},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "live-stats",
			Type:                   "live_stats",
			Title:                  "Live Stats",
			Subtitle:               nil,
			Endpoint:               "/api/v1/dashboard/widgets/live-stats",
			RefreshIntervalSeconds: 5,
			Grid:                   GridPlacement{Row: 3, ColSpan: 4, Order: 2},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
		{
			ID:                     "compliance-summary",
			Type:                   "compliance_summary",
			Title:                  "Compliance Shield",
			Subtitle:               stringPtr("Regulatory framework coverage"),
			Endpoint:               "/api/v1/dashboard/widgets/compliance-summary",
			RefreshIntervalSeconds: 300,
			Grid:                   GridPlacement{Row: 4, ColSpan: 12, Order: 0},
			Display:                DisplayOptions{ColorScheme: "blue", ValueFormat: "number"},
		},
	}

	return c.JSON(DashboardLayoutResponse{Widgets: widgets})
}

// HandleGetWidgetData returns data for a specific widget
func HandleGetWidgetData(c *fiber.Ctx) error {
	widgetID := c.Params("widget_id")

	// Extract organization ID from context (set by middleware)
	orgID, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Organization context required",
		})
	}

	switch widgetID {
	case "stat-total-requests":
		return handleStatTotalRequests(c, orgID)
	case "stat-pii-protected":
		return handleStatPIIProtected(c, orgID)
	case "stat-blocked-requests":
		return handleStatBlockedRequests(c, orgID)
	case "stat-avg-latency":
		return handleStatAvgLatency(c, orgID)
	case "success-rate":
		return handleSuccessRate(c, orgID)
	case "protection-summary":
		return handleProtectionSummary(c, orgID)
	case "gateway-status":
		return handleGatewayStatus(c, orgID)
	case "top-providers":
		return handleTopProviders(c, orgID)
	case "recent-activity":
		return handleRecentActivity(c, orgID)
	case "live-stats":
		return handleLiveStats(c, orgID)
	case "compliance-summary":
		return handleComplianceSummary(c, orgID)
	default:
		return c.Status(404).JSON(fiber.Map{
			"error":     "Widget not found",
			"widget_id": widgetID,
		})
	}
}

// Widget handler implementations
func handleStatTotalRequests(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(StatCardData{Value: 0, Delta: 0, DeltaPeriod: "vs last 24h"})
	}

	var current, previous int64

	// Current period (last 24h)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND organization_id = ?"), orgID).Scan(&current)

	// Previous period (24-48h ago)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW() - INTERVAL '24 hours' AND organization_id = ?"), orgID).Scan(&previous)

	delta := float64(current - previous)
	return c.JSON(StatCardData{
		Value:       float64(current),
		Delta:       delta,
		DeltaPeriod: "vs last 24h",
	})
}

func handleStatPIIProtected(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(StatCardData{Value: 0, Delta: 0, DeltaPeriod: "vs last 24h"})
	}

	var current, previous int64

	// Current period - requests with redactions
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND redaction_count > 0 AND organization_id = ?"), orgID).Scan(&current)

	// Previous period
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW() - INTERVAL '24 hours' AND redaction_count > 0 AND organization_id = ?"), orgID).Scan(&previous)

	delta := float64(current - previous)
	return c.JSON(StatCardData{
		Value:       float64(current),
		Delta:       delta,
		DeltaPeriod: "vs last 24h",
	})
}

func handleStatBlockedRequests(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(StatCardData{Value: 0, Delta: 0, DeltaPeriod: "vs last 24h"})
	}

	var current, previous int64

	// Current period - blocked requests
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND status = 'blocked' AND organization_id = ?"), orgID).Scan(&current)

	// Previous period
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW() - INTERVAL '24 hours' AND status = 'blocked' AND organization_id = ?"), orgID).Scan(&previous)

	delta := float64(current - previous)
	return c.JSON(StatCardData{
		Value:       float64(current),
		Delta:       delta,
		DeltaPeriod: "vs last 24h",
	})
}

func handleStatAvgLatency(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(StatCardData{Value: 0, Unit: stringPtr("ms"), Delta: 0, DeltaPeriod: "vs last 24h"})
	}

	var current, previous float64

	// Current period average latency
	db.QueryRow(formatQuery("SELECT AVG(processing_time_ms) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND processing_time_ms > 0 AND organization_id = ?"), orgID).Scan(&current)

	// Previous period average latency
	db.QueryRow(formatQuery("SELECT AVG(processing_time_ms) FROM transactions WHERE created_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW() - INTERVAL '24 hours' AND processing_time_ms > 0 AND organization_id = ?"), orgID).Scan(&previous)

	delta := current - previous
	return c.JSON(StatCardData{
		Value:       current,
		Unit:        stringPtr("ms"),
		Delta:       delta,
		DeltaPeriod: "vs last 24h",
	})
}

func handleSuccessRate(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(SuccessRateData{SuccessRate: 0, RedactedRequests: 0, TotalRequests: 0})
	}

	var total, successful, redacted int64

	// Get counts for last 24h
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND organization_id = ?"), orgID).Scan(&total)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND status = 'success' AND organization_id = ?"), orgID).Scan(&successful)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND redaction_count > 0 AND organization_id = ?"), orgID).Scan(&redacted)

	var successRate float64
	if total > 0 {
		successRate = float64(successful) / float64(total) * 100
	}

	return c.JSON(SuccessRateData{
		SuccessRate:      successRate,
		RedactedRequests: redacted,
		TotalRequests:    total,
	})
}

func handleProtectionSummary(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(ProtectionSummaryData{Rows: []ProtectionRow{}})
	}

	var protected, blocked, total int64

	// Get today's stats
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE DATE(created_at) = CURRENT_DATE AND redaction_count > 0 AND organization_id = ?"), orgID).Scan(&protected)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE DATE(created_at) = CURRENT_DATE AND status = 'blocked' AND organization_id = ?"), orgID).Scan(&blocked)
	db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE DATE(created_at) = CURRENT_DATE AND organization_id = ?"), orgID).Scan(&total)

	var detectionRate float64
	if total > 0 {
		detectionRate = float64(protected+blocked) / float64(total) * 100
	}

	rows := []ProtectionRow{
		{Key: "protected", Label: "Protected", Value: protected, Color: "green"},
		{Key: "blocked", Label: "Blocked", Value: blocked, Color: "red"},
		{Key: "detection_rate", Label: "Detection Rate", Value: formatPercent(detectionRate), Color: "accent"},
	}

	return c.JSON(ProtectionSummaryData{Rows: rows})
}

func handleGatewayStatus(c *fiber.Ctx, orgID int) error {
	uptime := int64(time.Since(startTime).Seconds())

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Mock component statuses (in real implementation, these would be actual health checks)
	components := []ComponentStatus{
		{Key: "database", Name: "Database", Status: "healthy", Provider: false},
		{Key: "ner_engine", Name: "NER Engine", Status: "healthy", Provider: false},
		{Key: "router", Name: "Router", Status: "healthy", Provider: false},
		{Key: "audit", Name: "Audit System", Status: "healthy", Provider: false},
		{Key: "openai", Name: "OpenAI", Status: "healthy", Provider: true},
		{Key: "anthropic", Name: "Anthropic", Status: "healthy", Provider: true},
		{Key: "google", Name: "Google", Status: "healthy", Provider: true},
		{Key: "local", Name: "Local LLM", Status: "offline", Provider: true},
	}

	return c.JSON(GatewayStatusData{
		Status:     "healthy",
		Uptime:     uptime,
		Components: components,
		MemoryUsage: MemoryUsage{
			Heap:  int64(m.HeapInuse / 1024 / 1024),     // MB
			Stack: int64(m.StackInuse / 1024 / 1024),    // MB
			Total: int64(m.Sys / 1024 / 1024),           // MB
		},
	})
}

func handleTopProviders(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(TopProvidersData{Providers: []ProviderData{}})
	}

	rows, err := db.Query(formatQuery(`
		SELECT provider, COUNT(*) as requests,
		       COUNT(*) * 100.0 / (SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '24 hours' AND organization_id = ?) as percent
		FROM transactions
		WHERE created_at >= NOW() - INTERVAL '24 hours' AND organization_id = ? AND provider != ''
		GROUP BY provider
		ORDER BY requests DESC
		LIMIT 10`), orgID, orgID)

	if err != nil {
		return c.JSON(TopProvidersData{Providers: []ProviderData{}})
	}
	defer rows.Close()

	providers := []ProviderData{}
	providerColors := map[string]string{
		"openai":    "#10A37F",
		"anthropic": "#C7A97B",
		"google":    "#4285F4",
		"local":     "#BF5AF2",
	}

	for rows.Next() {
		var p ProviderData
		rows.Scan(&p.Provider, &p.Requests, &p.Percent)
		p.Label = capitalizeProvider(p.Provider)
		if color, ok := providerColors[p.Provider]; ok {
			p.Color = color
		} else {
			p.Color = "#6B7280"
		}
		providers = append(providers, p)
	}

	return c.JSON(TopProvidersData{Providers: providers})
}

func handleRecentActivity(c *fiber.Ctx, orgID int) error {
	if db == nil {
		return c.JSON(ActivityChartData{PeriodLabel: "Last 24 hours", Points: []ActivityData{}})
	}

	rows, err := db.Query(formatQuery(`
		SELECT DATE_TRUNC('hour', created_at) as hour, COUNT(*) as count
		FROM transactions
		WHERE created_at >= NOW() - INTERVAL '24 hours' AND organization_id = ?
		GROUP BY hour
		ORDER BY hour ASC`), orgID)

	if err != nil {
		return c.JSON(ActivityChartData{PeriodLabel: "Last 24 hours", Points: []ActivityData{}})
	}
	defer rows.Close()

	points := []ActivityData{}
	for rows.Next() {
		var hourStr string
		var count int64
		rows.Scan(&hourStr, &count)

		// Convert to ISO 8601 timestamp
		t, _ := time.Parse("2006-01-02 15:04:05", hourStr)
		points = append(points, ActivityData{
			Timestamp: t.Format(time.RFC3339),
			Count:     count,
		})
	}

	return c.JSON(ActivityChartData{
		PeriodLabel: "Last 24 hours",
		Points:      points,
	})
}

func handleLiveStats(c *fiber.Ctx, orgID int) error {
	// Update request rate calculation
	updateRequestRate(orgID)

	var errorRate float64
	if db != nil {
		var errors, total int64
		db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '5 minutes' AND status != 'success' AND organization_id = ?"), orgID).Scan(&errors)
		db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '5 minutes' AND organization_id = ?"), orgID).Scan(&total)
		if total > 0 {
			errorRate = float64(errors) / float64(total)
		}
	}

	uptime := time.Since(startTime).Seconds()
	cacheHitRate := 0.85 // Mock value - in real implementation would get from cache system

	rows := []LiveStatRow{
		{Key: "requests_per_second", Label: "Req / sec", Value: currentRPS, Format: "rate_per_sec"},
		{Key: "active_connections", Label: "Active Connections", Value: 12, Format: "number"}, // Mock value
		{Key: "cache_hit_rate", Label: "Cache Hit Rate", Value: cacheHitRate, Format: "percent"},
		{
			Key:    "error_rate",
			Label:  "Error Rate",
			Value:  errorRate,
			Format: "percent",
			ColorRule: &ColorRule{
				ThresholdWarning:  0.03,
				ThresholdCritical: 0.05,
				Direction:         "higher_is_worse",
			},
		},
		{Key: "uptime", Label: "Uptime", Value: uptime, Format: "duration_human"},
	}

	return c.JSON(LiveStatsData{Rows: rows})
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func capitalizeProvider(provider string) string {
	switch provider {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google"
	case "local":
		return "Local LLM"
	default:
		return provider
	}
}

func updateRequestRate(orgID int) {
	now := time.Now()
	if now.Sub(lastUpdateTime) >= time.Minute {
		if db != nil {
			var count int64
			db.QueryRow(formatQuery("SELECT COUNT(*) FROM transactions WHERE created_at >= NOW() - INTERVAL '1 minute' AND organization_id = ?"), orgID).Scan(&count)
			currentRPS = float64(count) / 60.0
		}
		lastUpdateTime = now
	}
}

