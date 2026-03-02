package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ChatTurn is a single message in a multi-turn conversation.
type ChatTurn struct {
	Role    string // "user" or "assistant"
	Content string
}

// Chat sends a multi-turn conversation to Ollama and returns the model's reply.
// The systemPrompt is prepended as a system message; turns are sent in order.
func (c *Client) Chat(ctx context.Context, systemPrompt string, turns []ChatTurn) (string, error) {
	msgs := make([]map[string]string, 0, len(turns)+1)
	if systemPrompt != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": systemPrompt})
	}
	for _, t := range turns {
		msgs = append(msgs, map[string]string{"role": t.Role, "content": t.Content})
	}

	payload, err := json.Marshal(map[string]interface{}{
		"model":    c.model,
		"messages": msgs,
		"stream":   false,
		// Low temperature keeps the model grounded on the data briefing and
		// prevents it from drifting into generic AI assistant behaviours.
		"options": map[string]interface{}{
			"temperature": 0.2,
		},
	})
	if err != nil {
		return "", fmt.Errorf("ai: chat marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: chat build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: chat read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai: ollama HTTP %d: %s", resp.StatusCode, body)
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("ai: chat decode response: %w", err)
	}
	return ollamaResp.Message.Content, nil
}
