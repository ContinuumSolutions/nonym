package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type OuraAdapter struct{}

func (a *OuraAdapter) Slug() string { return "oura" }

func (a *OuraAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	startDate := since.Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")

	// Oura v2 API — daily sleep, readiness, and activity summaries.
	type ouraEndpoint struct {
		path     string
		category string
		title    string
	}

	endpoints := []ouraEndpoint{
		{
			path:     fmt.Sprintf("https://api.ouraring.com/v2/usercollection/daily_sleep?start_date=%s&end_date=%s", startDate, endDate),
			category: "Health",
			title:    "Sleep Score",
		},
		{
			path:     fmt.Sprintf("https://api.ouraring.com/v2/usercollection/daily_readiness?start_date=%s&end_date=%s", startDate, endDate),
			category: "Health",
			title:    "Readiness Score",
		},
		{
			path:     fmt.Sprintf("https://api.ouraring.com/v2/usercollection/daily_activity?start_date=%s&end_date=%s", startDate, endDate),
			category: "Health",
			title:    "Activity Score",
		},
	}

	var signals []RawSignal

	for _, ep := range endpoints {
		body, err := authGet(ctx, ep.path, creds.OAuthAccessToken)
		if err != nil {
			continue
		}

		var resp struct {
			Data []struct {
				Day   string `json:"day"`
				Score int    `json:"score"`
				// Common contributor fields vary per endpoint; capture as raw JSON.
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		for _, d := range resp.Data {
			occurred, _ := time.Parse("2006-01-02", d.Day)

			signals = append(signals, RawSignal{
				ServiceSlug: a.Slug(),
				Category:    ep.category,
				Title:       ep.title,
				Body:        fmt.Sprintf("Score: %d", d.Score),
				Metadata: map[string]string{
					"date":  d.Day,
					"score": fmt.Sprintf("%d", d.Score),
				},
				OccurredAt: occurred,
			})
		}
	}

	return signals, nil
}
