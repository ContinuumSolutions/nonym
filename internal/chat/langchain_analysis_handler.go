package chat

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/gofiber/fiber/v2"
)

// LangChainAnalysisHandler extends SimpleHandler with LangChain-powered data analysis
type LangChainAnalysisHandler struct {
	*SimpleHandler
	analyzer *ai.EnhancedDataAnalyzer
}

// NewLangChainAnalysisHandler creates a handler with LangChain optimization
func NewLangChainAnalysisHandler(
	base *SimpleHandler,
	db *sql.DB,
	dbPath string,
) *LangChainAnalysisHandler {
	analyzer := ai.NewEnhancedDataAnalyzer(base.ai, dbPath)
	analyzer.SetDatabase(db)

	return &LangChainAnalysisHandler{
		SimpleHandler: base,
		analyzer:      analyzer,
	}
}

// RegisterRoutes mounts both regular chat and enhanced analysis endpoints
func (h *LangChainAnalysisHandler) RegisterRoutes(r fiber.Router) {
	// Standard chat routes
	h.SimpleHandler.RegisterRoutes(r)

	// Enhanced analysis routes
	r.Post("/chat/analyze", h.analyzeData)
	r.Post("/chat/langchain", h.langChainAnalysis)
	r.Get("/chat/langchain/status", h.getLangChainStatus)
}

// analyzeData handles POST /chat/analyze with enhanced LangChain capabilities
func (h *LangChainAnalysisHandler) analyzeData(c *fiber.Ctx) error {
	var req struct {
		Message   string   `json:"message"`
		TimeFrame string   `json:"time_frame"` // "week", "month", "quarter"
		Focus     []string `json:"focus"`      // ["signals", "biometrics", "notifications"]
		UseLangChain bool  `json:"use_langchain,omitempty"` // Force LangChain usage
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

	// Perform enhanced data analysis
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

// langChainAnalysis handles POST /chat/langchain for explicit LangChain usage
func (h *LangChainAnalysisHandler) langChainAnalysis(c *fiber.Ctx) error {
	var req struct {
		Message   string `json:"message"`
		TimeFrame string `json:"time_frame"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if strings.TrimSpace(req.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "message is required"})
	}

	if req.TimeFrame == "" {
		req.TimeFrame = "week"
	}

	// Force LangChain analysis
	if !h.analyzer.IsLangChainEnabled() {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "LangChain analysis not available. Install Python dependencies: pip install langchain langchain-community",
		})
	}

	result, err := h.analyzer.AnalyzeWithLangChain(c.Context(), req.Message, req.TimeFrame)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Format the response
	reply := h.formatLangChainResult(result)

	// Save to chat history
	_ = h.history.Append("user", req.Message)
	_ = h.history.Append("kernel", reply)

	return c.JSON(struct {
		Reply         string      `json:"reply"`
		LangChainData interface{} `json:"langchain_data"`
		Timestamp     interface{} `json:"timestamp"`
	}{
		Reply:         reply,
		LangChainData: result,
		Timestamp:     c.Context().Time(),
	})
}

// getLangChainStatus handles GET /chat/langchain/status
func (h *LangChainAnalysisHandler) getLangChainStatus(c *fiber.Ctx) error {
	status := h.analyzer.GetLangChainStatus()
	return c.JSON(status)
}

// formatLangChainResult converts LangChain output to user-friendly format
func (h *LangChainAnalysisHandler) formatLangChainResult(result *ai.LangChainResult) string {
	var response strings.Builder

	response.WriteString("🚀 **LangChain Enhanced Analysis**\n\n")

	if result.Analysis != "" {
		response.WriteString(result.Analysis)
		response.WriteString("\n\n")
	}

	if result.QueryExplanation != "" {
		response.WriteString("## 🔍 Query Strategy\n")
		response.WriteString(result.QueryExplanation)
		response.WriteString("\n\n")
	}

	if len(result.GeneratedQueries) > 0 {
		response.WriteString("## 📊 SQL Queries Generated\n")
		for i, query := range result.GeneratedQueries {
			response.WriteString(fmt.Sprintf("**Query %d:**\n", i+1))
			response.WriteString("```sql\n")
			response.WriteString(query)
			response.WriteString("\n```\n\n")
		}
	}

	response.WriteString("*Powered by LangChain with optimized multi-step reasoning*")
	return response.String()
}

// Enhanced chat method that intelligently chooses between basic and LangChain analysis
func (h *LangChainAnalysisHandler) smartChat(c *fiber.Ctx) error {
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Detect if this is a complex analysis request
	if h.isComplexAnalysisRequest(req.Message) && h.analyzer.IsLangChainEnabled() {
		// Use LangChain for complex analysis
		result, err := h.analyzer.AnalyzeWithLangChain(c.Context(), req.Message, "week")
		if err == nil {
			reply := h.formatLangChainResult(result)

			// Save to history and return
			_ = h.history.Append("user", req.Message)
			_ = h.history.Append("kernel", reply)

			return c.JSON(Response{
				Reply:     reply,
				Timestamp: c.Context().Time(),
			})
		}
		// If LangChain fails, fall through to regular chat
	}

	// Use regular chat handling
	return h.SimpleHandler.chat(c)
}

// isComplexAnalysisRequest detects queries that would benefit from LangChain
func (h *LangChainAnalysisHandler) isComplexAnalysisRequest(message string) bool {
	lower := strings.ToLower(message)

	complexPatterns := []string{
		"correlations between",
		"relationship between",
		"patterns in",
		"trends over",
		"compare my",
		"deep analysis",
		"comprehensive",
		"predict",
		"forecast",
		"multi-step",
		"complex",
	}

	for _, pattern := range complexPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}
