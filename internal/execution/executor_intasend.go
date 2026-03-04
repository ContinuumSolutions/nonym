package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// IntaSendExecutor sends money via the IntaSend send-money API.
// Step 12 — Micro-Wallet (IntaSend side).
//
// The flow is two steps:
//  1. POST /api/v1/send-money/initiate/ → returns tracking_id
//  2. POST /api/v1/send-money/approve/  → completes the transfer
//
// Execute() performs both steps sequentially (used for auto-execute path).
// Initiate() performs only the first step and returns the tracking_id
// (used when amount is above threshold — approve step happens via the queue).
type IntaSendExecutor struct{}

func (e *IntaSendExecutor) Slug() string { return "intasend" }

func (e *IntaSendExecutor) Execute(ctx context.Context, creds datasync.Credentials, action Action) error {
	trackingID, err := e.Initiate(ctx, creds, action)
	if err != nil {
		return err
	}
	return e.ApproveByTrackingID(ctx, creds, trackingID)
}

// Initiate calls the IntaSend send-money initiate endpoint and returns the tracking_id.
func (e *IntaSendExecutor) Initiate(ctx context.Context, creds datasync.Credentials, action Action) (string, error) {
	if action.ResourceID == "" {
		return "", fmt.Errorf("intasend: send-money: missing account (counterparty)")
	}

	name := action.ResourceMeta["name"]
	if name == "" {
		name = action.ResourceID
	}
	currency := action.ResourceMeta["currency"]
	if currency == "" {
		currency = "KES"
	}

	payload := map[string]any{
		"currency": currency,
		"transactions": []map[string]any{
			{
				"name":      name,
				"account":   action.ResourceID,
				"amount":    action.EstimatedCost,
				"narrative": action.Reason,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://api.intasend.com/api/v1/send-money/initiate/",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("intasend: initiate: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("intasend: initiate: HTTP %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		TrackingID string `json:"tracking_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("intasend: initiate: decode response: %w", err)
	}
	if result.TrackingID == "" {
		return "", fmt.Errorf("intasend: initiate: empty tracking_id in response")
	}
	return result.TrackingID, nil
}

// ApproveByTrackingID calls the IntaSend approve endpoint with a tracking_id.
func (e *IntaSendExecutor) ApproveByTrackingID(ctx context.Context, creds datasync.Credentials, trackingID string) error {
	payload := map[string]string{"tracking_id": trackingID}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://api.intasend.com/api/v1/send-money/approve/",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+creds.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("intasend: approve %s: %w", trackingID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("intasend: approve %s: HTTP %d: %s", trackingID, resp.StatusCode, respBody)
	}
	return nil
}
