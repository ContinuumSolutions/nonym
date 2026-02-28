package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type FitbitAdapter struct{}

func (a *FitbitAdapter) Slug() string { return "fitbit" }

func (a *FitbitAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	today := time.Now().Format("2006-01-02")
	sinceDate := since.Format("2006-01-02")

	var signals []RawSignal

	// Sleep logs for the date range.
	sleepURL := fmt.Sprintf(
		"https://api.fitbit.com/1.2/user/-/sleep/date/%s/%s.json",
		sinceDate, today,
	)
	if sleepBody, err := authGet(ctx, sleepURL, creds.OAuthAccessToken); err == nil {
		var sleepResp struct {
			Sleep []struct {
				DateOfSleep   string `json:"dateOfSleep"`
				Efficiency    int    `json:"efficiency"`
				MinutesAsleep int    `json:"minutesAsleep"`
			} `json:"sleep"`
		}
		if json.Unmarshal(sleepBody, &sleepResp) == nil {
			for _, s := range sleepResp.Sleep {
				occurred, _ := time.Parse("2006-01-02", s.DateOfSleep)
				signals = append(signals, RawSignal{
					ServiceSlug: a.Slug(),
					Category:    "Health",
					Title:       "Sleep",
					Body:        fmt.Sprintf("%d min asleep, efficiency %d%%", s.MinutesAsleep, s.Efficiency),
					Metadata: map[string]string{
						"date":           s.DateOfSleep,
						"minutes_asleep": fmt.Sprintf("%d", s.MinutesAsleep),
						"efficiency":     fmt.Sprintf("%d", s.Efficiency),
					},
					OccurredAt: occurred,
				})
			}
		}
	}

	// Activity summary for today.
	actURL := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/date/%s.json", today)
	if actBody, err := authGet(ctx, actURL, creds.OAuthAccessToken); err == nil {
		var actResp struct {
			Summary struct {
				Steps             int `json:"steps"`
				CaloriesOut       int `json:"caloriesOut"`
				FairlyActiveMin   int `json:"fairlyActiveMinutes"`
				VeryActiveMin     int `json:"veryActiveMinutes"`
			} `json:"summary"`
		}
		if json.Unmarshal(actBody, &actResp) == nil {
			signals = append(signals, RawSignal{
				ServiceSlug: a.Slug(),
				Category:    "Health",
				Title:       "Activity",
				Body:        fmt.Sprintf("%d steps, %d calories", actResp.Summary.Steps, actResp.Summary.CaloriesOut),
				Metadata: map[string]string{
					"steps":          fmt.Sprintf("%d", actResp.Summary.Steps),
					"calories":       fmt.Sprintf("%d", actResp.Summary.CaloriesOut),
					"active_minutes": fmt.Sprintf("%d", actResp.Summary.FairlyActiveMin+actResp.Summary.VeryActiveMin),
				},
				OccurredAt: time.Now().Truncate(24 * time.Hour),
			})
		}
	}

	return signals, nil
}
