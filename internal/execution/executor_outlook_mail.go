package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// OutlookMailExecutor moves an email to the Junk Email folder.
// Requires the Mail.ReadWrite scope on the connected OAuth token.
type OutlookMailExecutor struct{}

func (e *OutlookMailExecutor) Slug() string { return "outlook-mail" }

func (e *OutlookMailExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	if action.ResourceID == "" {
		return fmt.Errorf("outlook-mail: move: missing message_id")
	}

	url := fmt.Sprintf(
		"https://graph.microsoft.com/v1.0/me/messages/%s/move",
		action.ResourceID,
	)

	body, err := json.Marshal(map[string]string{
		"destinationId": "junkemail",
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
		return fmt.Errorf("outlook-mail: move %s: %w", action.ResourceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("outlook-mail: move %s: HTTP %d", action.ResourceID, resp.StatusCode)
	}
	return nil
}
