package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type GoogleCalendarAdapter struct{}

func (a *GoogleCalendarAdapter) Slug() string { return "google-calendar" }

func (a *GoogleCalendarAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	url := fmt.Sprintf(
		"https://www.googleapis.com/calendar/v3/calendars/primary/events?timeMin=%s&timeMax=%s&singleEvents=true&orderBy=startTime&maxResults=50",
		since.UTC().Format(time.RFC3339),
		since.Add(72*time.Hour).UTC().Format(time.RFC3339),
	)

	body, err := authGet(ctx, url, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("google-calendar: list events: %w", err)
	}

	var resp struct {
		Items []struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Location    string `json:"location"`
			Start       struct {
				DateTime string `json:"dateTime"`
				Date     string `json:"date"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
			} `json:"end"`
			Attendees []struct {
				Email       string `json:"email"`
				DisplayName string `json:"displayName"`
			} `json:"attendees"`
			Organizer struct {
				Email string `json:"email"`
			} `json:"organizer"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("google-calendar: decode: %w", err)
	}

	var signals []RawSignal
	for _, item := range resp.Items {
		start, _ := time.Parse(time.RFC3339, item.Start.DateTime)
		if start.IsZero() {
			start, _ = time.Parse("2006-01-02", item.Start.Date)
		}
		attendees := make([]string, 0, len(item.Attendees))
		for _, att := range item.Attendees {
			name := att.DisplayName
			if name == "" {
				name = att.Email
			}
			attendees = append(attendees, name)
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Calendar",
			Title:       item.Summary,
			Body:        item.Description,
			Metadata: map[string]string{
				"start":     item.Start.DateTime,
				"end":       item.End.DateTime,
				"attendees": strings.Join(attendees, ", "),
				"organizer": item.Organizer.Email,
				"location":  item.Location,
			},
			OccurredAt: start,
		})
	}
	return signals, nil
}
