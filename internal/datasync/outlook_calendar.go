package datasync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type OutlookCalendarAdapter struct{}

func (a *OutlookCalendarAdapter) Slug() string { return "outlook-calendar" }

func (a *OutlookCalendarAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	end := since.Add(72 * time.Hour)
	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/calendarView?startDateTime=%s&endDateTime=%s&$select=subject,bodyPreview,start,end,attendees,organizer,location&$top=50&$orderby=start/dateTime",
		since.UTC().Format(time.RFC3339),
		end.UTC().Format(time.RFC3339),
	)

	body, err := authGet(ctx, url, creds.OAuthAccessToken)
	if err != nil {
		return nil, fmt.Errorf("outlook-calendar: list events: %w", err)
	}

	var resp struct {
		Value []struct {
			Subject     string `json:"subject"`
			BodyPreview string `json:"bodyPreview"`
			Start       struct {
				DateTime string `json:"dateTime"`
			} `json:"start"`
			End struct {
				DateTime string `json:"dateTime"`
			} `json:"end"`
			Location struct {
				DisplayName string `json:"displayName"`
			} `json:"location"`
			Organizer struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"organizer"`
			Attendees []struct {
				EmailAddress struct {
					Name    string `json:"name"`
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"attendees"`
		} `json:"value"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("outlook-calendar: decode: %w", err)
	}

	var signals []RawSignal
	for _, item := range resp.Value {
		// Graph returns dateTime without Z suffix; treat as UTC
		startStr := item.Start.DateTime
		if !strings.HasSuffix(startStr, "Z") {
			startStr += "Z"
		}
		start, _ := time.Parse(time.RFC3339, startStr)

		attendees := make([]string, 0, len(item.Attendees))
		for _, att := range item.Attendees {
			name := att.EmailAddress.Name
			if name == "" {
				name = att.EmailAddress.Address
			}
			attendees = append(attendees, name)
		}

		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Calendar",
			Title:       item.Subject,
			Body:        item.BodyPreview,
			Metadata: map[string]string{
				"start":     item.Start.DateTime,
				"end":       item.End.DateTime,
				"attendees": strings.Join(attendees, ", "),
				"organizer": item.Organizer.EmailAddress.Address,
				"location":  item.Location.DisplayName,
			},
			OccurredAt: start,
		})
	}
	return signals, nil
}
