package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// OutlookCalendarExecutor declines a calendar event via Microsoft Graph.
// Requires the Calendars.ReadWrite scope on the connected OAuth token.
type OutlookCalendarExecutor struct{}

func (e *OutlookCalendarExecutor) Slug() string { return "outlook-calendar" }

func (e *OutlookCalendarExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	if action.ResourceID == "" {
		return fmt.Errorf("outlook-calendar: decline: missing event_id")
	}

	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/events/%s/decline",
		action.ResourceID,
	)

	body, err := json.Marshal(map[string]string{
		"comment":         fmt.Sprintf("Declined automatically: %s", action.Reason),
		"sendResponse":    "true",
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.OAuthAccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("outlook-calendar: decline %s: %w", action.ResourceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("outlook-calendar: decline %s: HTTP %d", action.ResourceID, resp.StatusCode)
	}
	return nil
}
