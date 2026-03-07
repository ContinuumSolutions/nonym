package chat

import (
	"database/sql"
	"strings"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/gofiber/fiber/v2"
)

// AnalysisHandler extends SimpleHandler with data analysis capabilities
type AnalysisHandler struct {
	*SimpleHandler
	analyzer *ai.DataAnalyzer
}

// NewAnalysisHandler creates an enhanced chat handler with data analysis
func NewAnalysisHandler(
	base *SimpleHandler,
	db *sql.DB,
) *AnalysisHandler {
	analyzer := ai.NewDataAnalyzer(base.ai, db)
	return &AnalysisHandler{
		SimpleHandler: base,
		analyzer:      analyzer,
	}
}

// RegisterRoutes mounts both regular chat and analysis endpoints
func (h *AnalysisHandler) RegisterRoutes(r fiber.Router) {
	// Standard chat routes
	h.SimpleHandler.RegisterRoutes(r)

	// Enhanced analysis route
	r.Post("/chat/analyze", h.analyzeData)
}

// analyzeData handles POST /chat/analyze for data-driven responses
func (h *AnalysisHandler) analyzeData(c *fiber.Ctx) error {
	var req struct {
		Message   string   `json:"message"`
		TimeFrame string   `json:"time_frame"` // "week", "month", "quarter"
		Focus     []string `json:"focus"`      // ["signals", "biometrics", "notifications"]
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message is required"})
	}

	// Set defaults
	if req.TimeFrame == "" {
		req.TimeFrame = "week"
	}

	// Perform data analysis
	analysisReq := ai.AnalysisRequest{
		Query:     req.Message,
		TimeFrame: req.TimeFrame,
		Focus:     req.Focus,
	}

	reply, err := h.analyzer.AnalyzeData(c.Context(), analysisReq)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Save to chat history
	_ = h.history.Append("user", req.Message)
	_ = h.history.Append("kernel", reply)

	return c.JSON(Response{
		Reply:     reply,
		Timestamp: c.Context().Time(),
	})
}

// Enhanced chat method that can detect analysis requests in regular chat
func (h *AnalysisHandler) smartChat(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Detect if this is an analysis request
	if h.isAnalysisRequest(req.Message) {
		// Redirect to analysis
		analysisReq := ai.AnalysisRequest{
			Query:     req.Message,
			TimeFrame: h.inferTimeFrame(req.Message),
			Focus:     h.inferFocus(req.Message),
		}

		reply, err := h.analyzer.AnalyzeData(c.Context(), analysisReq)
		if err == nil {
			// Save to history and return
			_ = h.history.Append("user", req.Message)
			_ = h.history.Append("kernel", reply)

			return c.JSON(Response{
				Reply:     reply,
				Timestamp: c.Context().Time(),
			})
		}
		// If analysis fails, fall back to regular chat
	}

	// Use regular chat handling
	return h.SimpleHandler.chat(c)
}

// isAnalysisRequest detects if user wants data analysis
func (h *AnalysisHandler) isAnalysisRequest(message string) bool {
	lower := strings.ToLower(message)

	analysisKeywords := []string{
		"what should i prioritize",
		"what needs attention",
		"analyze my",
		"review my",
		"what have i missed",
		"what's urgent",
		"show me patterns",
		"trending",
		"summary",
		"overview",
		"analyze",
		"prioritize",
		"focus on",
	}

	for _, keyword := range analysisKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return false
}

// inferTimeFrame extracts time frame from user message
func (h *AnalysisHandler) inferTimeFrame(message string) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "week") || strings.Contains(lower, "7 day") {
		return "week"
	}
	if strings.Contains(lower, "month") || strings.Contains(lower, "30 day") {
		return "month"
	}
	if strings.Contains(lower, "quarter") || strings.Contains(lower, "3 month") {
		return "quarter"
	}

	return "week" // Default
}

// inferFocus determines what data to focus on from user message
func (h *AnalysisHandler) inferFocus(message string) []string {
	lower := strings.ToLower(message)
	var focus []string

	if strings.Contains(lower, "email") || strings.Contains(lower, "message") || strings.Contains(lower, "signal") {
		focus = append(focus, "signals")
	}
	if strings.Contains(lower, "health") || strings.Contains(lower, "mood") || strings.Contains(lower, "stress") || strings.Contains(lower, "sleep") {
		focus = append(focus, "biometrics")
	}
	if strings.Contains(lower, "notification") || strings.Contains(lower, "alert") {
		focus = append(focus, "notifications")
	}

	// If no specific focus, analyze everything
	return focus
}
