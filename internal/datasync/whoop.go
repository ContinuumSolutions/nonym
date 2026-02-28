package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type WhoopAdapter struct{}

func (a *WhoopAdapter) Slug() string { return "whoop" }

func (a *WhoopAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	// WHOOP API v1 — recovery, sleep, and workout cycles.
	type whoopEndpoint struct {
		url      string
		category string
		title    string
	}

	startStr := since.UTC().Format(time.RFC3339)

	endpoints := []whoopEndpoint{
		{
			url:      fmt.Sprintf("https://api.prod.whoop.com/developer/v1/recovery/?start=%s&limit=25", startStr),
			category: "Health",
			title:    "Recovery",
		},
		{
			url:      fmt.Sprintf("https://api.prod.whoop.com/developer/v1/cycle/?start=%s&limit=25", startStr),
			category: "Health",
			title:    "Strain",
		},
		{
			url:      fmt.Sprintf("https://api.prod.whoop.com/developer/v1/activity/sleep/?start=%s&limit=25", startStr),
			category: "Health",
			title:    "Sleep",
		},
	}

	var signals []RawSignal

	for _, ep := range endpoints {
		body, err := authGet(ctx, ep.url, creds.OAuthAccessToken)
		if err != nil {
			continue
		}

		// All WHOOP list endpoints share the same pagination envelope.
		var resp struct {
			Records []json.RawMessage `json:"records"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		for _, raw := range resp.Records {
			// Parse the minimal fields common across recovery/cycle/sleep.
			var record struct {
				ID        int    `json:"id"`
				CreatedAt string `json:"created_at"`
				Score     struct {
					RecoveryScore float64 `json:"recovery_score"`
					SleepPerformance float64 `json:"sleep_performance_percentage"`
					Strain        float64 `json:"strain"`
				} `json:"score"`
			}
			if err := json.Unmarshal(raw, &record); err != nil {
				continue
			}

			occurred, _ := time.Parse(time.RFC3339, record.CreatedAt)
			if occurred.IsZero() {
				occurred = time.Now()
			}

			var scoreVal float64
			var scoreLabel string
			switch ep.title {
			case "Recovery":
				scoreVal = record.Score.RecoveryScore
				scoreLabel = fmt.Sprintf("%.0f%%", scoreVal)
			case "Sleep":
				scoreVal = record.Score.SleepPerformance
				scoreLabel = fmt.Sprintf("%.0f%%", scoreVal)
			case "Strain":
				scoreVal = record.Score.Strain
				scoreLabel = fmt.Sprintf("%.1f", scoreVal)
			}

			signals = append(signals, RawSignal{
				ServiceSlug: a.Slug(),
				Category:    ep.category,
				Title:       ep.title,
				Body:        fmt.Sprintf("%s: %s", ep.title, scoreLabel),
				Metadata: map[string]string{
					"record_id":   fmt.Sprintf("%d", record.ID),
					"score":       fmt.Sprintf("%.2f", scoreVal),
					"created_at":  record.CreatedAt,
				},
				OccurredAt: occurred,
			})
		}
	}

	return signals, nil
}
