package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LangChainAnalyzer provides enhanced data analysis using Python LangChain
type LangChainAnalyzer struct {
	scriptPath string
	dbPath     string
	enabled    bool
}

// LangChainResult represents the output from the Python LangChain script
type LangChainResult struct {
	QueryExplanation  string                 `json:"query_explanation"`
	GeneratedQueries  []string               `json:"generated_queries"`
	QueryResults      map[string]interface{} `json:"query_results"`
	Analysis          string                 `json:"analysis"`
	Timestamp         string                 `json:"timestamp"`
	Error             string                 `json:"error,omitempty"`
}

// NewLangChainAnalyzer creates a new LangChain-powered analyzer
func NewLangChainAnalyzer(dbPath string) *LangChainAnalyzer {
	// Find the script path relative to the project root
	wd, _ := os.Getwd()
	scriptPath := filepath.Join(wd, "scripts", "langchain_optimizer.py")

	// Check if script exists and Python is available
	enabled := checkLangChainAvailable(scriptPath)

	return &LangChainAnalyzer{
		scriptPath: scriptPath,
		dbPath:     dbPath,
		enabled:    enabled,
	}
}

// checkLangChainAvailable verifies LangChain dependencies are installed
func checkLangChainAvailable(scriptPath string) bool {
	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return false
	}

	// Check if Python and required packages are available
	cmd := exec.Command("python3", "-c", "import langchain, sqlite3; print('OK')")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// IsEnabled returns whether LangChain analysis is available
func (l *LangChainAnalyzer) IsEnabled() bool {
	return l.enabled
}

// AnalyzeWithLangChain performs enhanced analysis using LangChain
func (l *LangChainAnalyzer) AnalyzeWithLangChain(ctx context.Context, query, timeFrame string) (*LangChainResult, error) {
	if !l.enabled {
		return nil, fmt.Errorf("LangChain analyzer not available - ensure Python dependencies are installed")
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, "python3", l.scriptPath, query, timeFrame, l.dbPath)
	cmd.Dir = filepath.Dir(l.scriptPath)

	// Set environment variables for Ollama connection
	cmd.Env = os.Environ()
	if ollamaHost := os.Getenv("OLLAMA_HOST"); ollamaHost != "" {
		cmd.Env = append(cmd.Env, "OLLAMA_HOST="+ollamaHost)
	}

	// Execute and capture output
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("LangChain script failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute LangChain script: %w", err)
	}

	// Parse JSON response
	var result LangChainResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse LangChain output: %w", err)
	}

	// Check for errors in the result
	if result.Error != "" {
		return nil, fmt.Errorf("LangChain analysis error: %s", result.Error)
	}

	return &result, nil
}

// EnhancedDataAnalyzer combines the original analyzer with LangChain optimization
type EnhancedDataAnalyzer struct {
	*DataAnalyzer  // Embed original analyzer
	langchain      *LangChainAnalyzer
	useLangChain   bool
}

// NewEnhancedDataAnalyzer creates a hybrid analyzer with LangChain optimization
func NewEnhancedDataAnalyzer(client *Client, dbPath string) *EnhancedDataAnalyzer {
	originalAnalyzer := NewDataAnalyzer(client, nil) // We'll set db separately
	langchainAnalyzer := NewLangChainAnalyzer(dbPath)

	return &EnhancedDataAnalyzer{
		DataAnalyzer: originalAnalyzer,
		langchain:    langchainAnalyzer,
		useLangChain: langchainAnalyzer.IsEnabled(),
	}
}

// SetDatabase updates the database connection for the original analyzer
func (e *EnhancedDataAnalyzer) SetDatabase(db interface{}) {
	if sqlDB, ok := db.(*sql.DB); ok {
		e.DataAnalyzer.db = sqlDB
	}
}

// AnalyzeData performs optimized analysis, using LangChain when available
func (e *EnhancedDataAnalyzer) AnalyzeData(ctx context.Context, req AnalysisRequest) (string, error) {
	// Try LangChain first for complex queries
	if e.useLangChain && e.shouldUseLangChain(req.Query) {
		result, err := e.langchain.AnalyzeWithLangChain(ctx, req.Query, req.TimeFrame)
		if err == nil {
			return e.formatLangChainResponse(result), nil
		}

		// Log LangChain error but fallback to original analyzer
		fmt.Printf("LangChain analysis failed, falling back to basic analyzer: %v\n", err)
	}

	// Fallback to original analyzer
	return e.DataAnalyzer.AnalyzeData(ctx, req)
}

// shouldUseLangChain determines if a query would benefit from LangChain analysis
func (e *EnhancedDataAnalyzer) shouldUseLangChain(query string) bool {
	complexPatterns := []string{
		"correlations",
		"patterns",
		"trends",
		"relationships",
		"deep analysis",
		"comprehensive",
		"compare",
		"predict",
		"forecast",
	}

	queryLower := strings.ToLower(query)
	for _, pattern := range complexPatterns {
		if strings.Contains(queryLower, pattern) {
			return true
		}
	}

	return false
}

// formatLangChainResponse converts LangChain result to user-friendly format
func (e *EnhancedDataAnalyzer) formatLangChainResponse(result *LangChainResult) string {
	var response strings.Builder

	// Add timestamp and method indicator
	response.WriteString("🔍 **Enhanced Analysis with LangChain**\n\n")

	// Main analysis
	if result.Analysis != "" {
		response.WriteString(result.Analysis)
		response.WriteString("\n\n")
	}

	// Add query insights if available
	if result.QueryExplanation != "" {
		response.WriteString("## 📊 Analysis Method\n")
		response.WriteString(result.QueryExplanation)
		response.WriteString("\n\n")
	}

	// Add data sources info
	if len(result.QueryResults) > 0 {
		response.WriteString("## 📈 Data Sources\n")
		for queryName, queryData := range result.QueryResults {
			if data, ok := queryData.(map[string]interface{}); ok {
				if count, exists := data["count"]; exists {
					response.WriteString(fmt.Sprintf("- %s: %v records analyzed\n", queryName, count))
				}
			}
		}
		response.WriteString("\n")
	}

	response.WriteString("*Analysis powered by LangChain with optimized SQL generation*")

	return response.String()
}

// GetLangChainStatus returns the status of LangChain integration
func (e *EnhancedDataAnalyzer) GetLangChainStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":     e.useLangChain,
		"script_path": e.langchain.scriptPath,
	}

	if e.useLangChain {
		status["status"] = "Available"
	} else {
		status["status"] = "Unavailable - Install Python dependencies: pip install langchain"
	}

	return status
}
