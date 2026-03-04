package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// GmailExecutor archives an email by removing it from INBOX.
// Requires the gmail.modify scope on the connected OAuth token.
type GmailExecutor struct{}

func (e *GmailExecutor) Slug() string { return "gmail" }

func (e *GmailExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	if action.ResourceID == "" {
		return fmt.Errorf("gmail: archive: missing message_id")
	}

	url := fmt.Sprintf(
		"https://gmail.googleapis.com/gmail/v1/users/me/messages/%s/modify",
		action.ResourceID,
	)

	body, err := json.Marshal(map[string]interface{}{
		"removeLabelIds": []string{"INBOX"},
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
		return fmt.Errorf("gmail: archive %s: %w", action.ResourceID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("gmail: archive %s: HTTP %d", action.ResourceID, resp.StatusCode)
	}
	return nil
}
