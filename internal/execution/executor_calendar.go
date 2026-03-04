package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// GoogleCalendarExecutor declines a calendar event on behalf of the user.
// Requires the calendar.events scope on the connected OAuth token.
type GoogleCalendarExecutor struct{}

func (e *GoogleCalendarExecutor) Slug() string { return "google-calendar" }

func (e *GoogleCalendarExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	if action.ResourceID == "" {
		return fmt.Errorf("google-calendar: decline: missing event_id")
	}

	url := fmt.Sprintf(
		"https://www.googleapis.com/calendar/v3/calendars/primary/events/%s",
		action.ResourceID,
	)

	// Patch the user's own attendee entry to responseStatus=declined.
	body, err := json.Marshal(map[string]any{
		"attendees": []map[string]string{
			{"responseStatus": "declined"},
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.OAuthAccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("google-calendar: decline %s: %w", action.ResourceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("google-calendar: decline %s: HTTP %d", action.ResourceID, resp.StatusCode)
	}
	return nil
}
