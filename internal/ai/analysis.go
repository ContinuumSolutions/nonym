package ai

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// AnalysisRequest represents a user request for data analysis
type AnalysisRequest struct {
	Query     string    `json:"query"`
	TimeFrame string    `json:"time_frame"` // "week", "month", "quarter"
	Focus     []string  `json:"focus"`      // ["signals", "biometrics", "notifications"]
}

// DataAnalyzer provides AI-powered analysis of user's historical data
type DataAnalyzer struct {
	client *Client
	db     *sql.DB
}

// NewDataAnalyzer creates a new analyzer instance
func NewDataAnalyzer(client *Client, db *sql.DB) *DataAnalyzer {
	return &DataAnalyzer{
		client: client,
		db:     db,
	}
}

// AnalyzeData performs grounded analysis based on actual user data
func (a *DataAnalyzer) AnalyzeData(ctx context.Context, req AnalysisRequest) (string, error) {
	// 1. Query actual data from database
	dataContext, err := a.buildDataContext(req.TimeFrame, req.Focus)
	if err != nil {
		return "", fmt.Errorf("failed to query data: %w", err)
	}

	// 2. Build analysis prompt with actual data
	systemPrompt := a.buildAnalysisPrompt(req.Query, dataContext)

	// 3. Get AI analysis with strict grounding
	turns := []ChatTurn{
		{Role: "user", Content: req.Query},
	}

	response, err := a.client.Chat(ctx, systemPrompt, turns)
	if err != nil {
		return "", fmt.Errorf("AI analysis failed: %w", err)
	}

	return response, nil
}

// buildDataContext queries the database for relevant historical data
func (a *DataAnalyzer) buildDataContext(timeFrame string, focus []string) (string, error) {
	var sb strings.Builder
	now := time.Now().UTC()

	// Determine time window
	var since time.Time
	switch timeFrame {
	case "week":
		since = now.AddDate(0, 0, -7)
	case "month":
		since = now.AddDate(0, -1, 0)
	case "quarter":
		since = now.AddDate(0, -3, 0)
	default:
		since = now.AddDate(0, 0, -7) // Default to week
	}

	sinceUnix := since.Unix()

	// Query signals data
	if contains(focus, "signals") || len(focus) == 0 {
		if err := a.addSignalsData(&sb, sinceUnix); err != nil {
			return "", err
		}
	}

	// Query biometrics data
	if contains(focus, "biometrics") || len(focus) == 0 {
		if err := a.addBiometricsData(&sb, sinceUnix); err != nil {
			return "", err
		}
	}

	// Query notifications data
	if contains(focus, "notifications") || len(focus) == 0 {
		if err := a.addNotificationsData(&sb, sinceUnix); err != nil {
			return "", err
		}
	}

	return sb.String(), nil
}

func (a *DataAnalyzer) addSignalsData(sb *strings.Builder, sinceUnix int64) error {
	// Query signals from the last period
	query := `
		SELECT
			service_slug,
			COUNT(*) as total_signals,
			SUM(CASE WHEN json_extract(analysis, '$.priority') = 'high' THEN 1 ELSE 0 END) as high_priority,
			SUM(CASE WHEN json_extract(analysis, '$.is_relevant') = 'true' THEN 1 ELSE 0 END) as relevant,
			SUM(CASE WHEN json_extract(analysis, '$.needs_reply') = 'true' THEN 1 ELSE 0 END) as need_reply,
			SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 2 THEN 1 ELSE 0 END) as ignored
		FROM signals
		WHERE processed_at >= ?
		GROUP BY service_slug
		ORDER BY total_signals DESC
	`

	rows, err := a.db.Query(query, sinceUnix)
	if err != nil {
		return err
	}
	defer rows.Close()

	sb.WriteString("## SIGNALS ANALYSIS\n")
	sb.WriteString("Service breakdown for the period:\n")

	totalSignals := 0
	totalHighPriority := 0
	totalPending := 0

	for rows.Next() {
		var service string
		var total, highPri, relevant, needReply, pending, completed, ignored int

		err := rows.Scan(&service, &total, &highPri, &relevant, &needReply, &pending, &completed, &ignored)
		if err != nil {
			continue
		}

		totalSignals += total
		totalHighPriority += highPri
		totalPending += pending

		fmt.Fprintf(sb, "- %s: %d total (%d high priority, %d relevant, %d need replies, %d pending)\n",
			service, total, highPri, relevant, needReply, pending)
	}

	fmt.Fprintf(sb, "\nOVERALL: %d total signals, %d high priority, %d still pending\n\n",
		totalSignals, totalHighPriority, totalPending)

	return nil
}

func (a *DataAnalyzer) addBiometricsData(sb *strings.Builder, sinceUnix int64) error {
	// Query biometrics history
	query := `
		SELECT
			recorded_at,
			mood,
			stress_level,
			sleep,
			energy
		FROM check_in_history
		WHERE recorded_at >= ?
		ORDER BY recorded_at DESC
		LIMIT 30
	`

	rows, err := a.db.Query(query, sinceUnix)
	if err != nil {
		return err
	}
	defer rows.Close()

	sb.WriteString("## BIOMETRICS TRENDS\n")

	var checkins []map[string]interface{}
	var avgMood, avgStress, avgSleep, avgEnergy float64
	count := 0

	for rows.Next() {
		var recordedAt int64
		var mood, stress, energy int
		var sleep float64

		if err := rows.Scan(&recordedAt, &mood, &stress, &sleep, &energy); err != nil {
			continue
		}

		checkins = append(checkins, map[string]interface{}{
			"date":    time.Unix(recordedAt, 0).Format("Jan 2"),
			"mood":    mood,
			"stress":  stress,
			"sleep":   sleep,
			"energy":  energy,
		})

		avgMood += float64(mood)
		avgStress += float64(stress)
		avgSleep += sleep
		avgEnergy += float64(energy)
		count++
	}

	if count > 0 {
		avgMood /= float64(count)
		avgStress /= float64(count)
		avgSleep /= float64(count)
		avgEnergy /= float64(count)

		fmt.Fprintf(sb, "Average scores: Mood %.1f/10, Stress %.1f/10, Sleep %.1fh, Energy %.1f/10\n",
			avgMood, avgStress, avgSleep, avgEnergy)

		// Identify concerning patterns
		if avgStress > 6 {
			sb.WriteString("⚠️  HIGH STRESS PATTERN detected\n")
		}
		if avgSleep < 6 {
			sb.WriteString("⚠️  LOW SLEEP PATTERN detected\n")
		}
		if avgMood < 5 {
			sb.WriteString("⚠️  LOW MOOD PATTERN detected\n")
		}

		// Show recent entries
		sb.WriteString("Recent check-ins:\n")
		for i, entry := range checkins {
			if i >= 7 { break } // Show last 7 days
			fmt.Fprintf(sb, "- %s: mood %d, stress %d, sleep %.1fh, energy %d\n",
				entry["date"], entry["mood"], entry["stress"], entry["sleep"], entry["energy"])
		}
	} else {
		sb.WriteString("No biometric data available for this period.\n")
	}
	sb.WriteString("\n")

	return nil
}

func (a *DataAnalyzer) addNotificationsData(sb *strings.Builder, sinceUnix int64) error {
	// Query recent notifications
	query := `
		SELECT type, title, body, read, created_at
		FROM notifications
		WHERE created_at >= ?
		ORDER BY created_at DESC
		LIMIT 20
	`

	rows, err := a.db.Query(query, sinceUnix)
	if err != nil {
		return err
	}
	defer rows.Close()

	sb.WriteString("## NOTIFICATIONS\n")

	unreadCount := 0
	totalCount := 0

	for rows.Next() {
		var notifType, title, body string
		var read bool
		var createdAt int64

		if err := rows.Scan(&notifType, &title, &body, &read, &createdAt); err != nil {
			continue
		}

		totalCount++
		if !read {
			unreadCount++
		}

		status := "✓"
		if !read {
			status = "•"
		}

		date := time.Unix(createdAt, 0).Format("Jan 2")
		fmt.Fprintf(sb, "%s [%s] %s: %s\n", status, date, title, body)
	}

	fmt.Fprintf(sb, "\nTotal: %d notifications (%d unread)\n\n", totalCount, unreadCount)
	return nil
}

func (a *DataAnalyzer) buildAnalysisPrompt(userQuery string, dataContext string) string {
	return fmt.Sprintf(`You are a data analyst for a personal AI agent. Your job is to analyze the user's actual data and provide insights.

CRITICAL RULES:
1. Base your analysis ONLY on the data provided below
2. If the data is insufficient, explicitly state what's missing
3. Never invent or assume information not present in the data
4. Focus on actionable insights and patterns
5. Highlight urgent issues that need attention

USER QUERY: %s

ACTUAL USER DATA:
%s

Instructions:
- Analyze the data above to answer the user's question
- Identify the top 3 areas that need attention
- Suggest specific, actionable steps
- If you see concerning patterns (high stress, many pending signals, etc.), prioritize those
- Keep recommendations practical and time-bound

Respond in a clear, direct manner. Start with a brief summary, then provide detailed analysis.`, userQuery, dataContext)
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
