package ai

import (
	"bufio"
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

	opts := c.ollamaOptions(0.2)
	if _, set := opts["num_predict"]; !set {
		opts["num_predict"] = 400 // chat replies are rarely >400 tokens
	}
	payload, err := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     false,
		"keep_alive": -1,
		"options":    opts,
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

// ChatStream sends a multi-turn conversation to Ollama with streaming enabled.
// fn is called for each token as it arrives; the call blocks until Ollama signals done.
// Use this from SSE handlers to deliver first-token latency under one second.
func (c *Client) ChatStream(ctx context.Context, systemPrompt string, turns []ChatTurn, fn func(token string)) error {
	msgs := make([]map[string]string, 0, len(turns)+1)
	if systemPrompt != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": systemPrompt})
	}
	for _, t := range turns {
		msgs = append(msgs, map[string]string{"role": t.Role, "content": t.Content})
	}

	opts := c.ollamaOptions(0.2)
	if _, set := opts["num_predict"]; !set {
		opts["num_predict"] = 400
	}
	payload, err := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     true,
		"keep_alive": -1,
		"options":    opts,
	})
	if err != nil {
		return fmt.Errorf("ai: chat stream marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("ai: chat stream build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ai: chat stream ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ai: chat stream HTTP %d: %s", resp.StatusCode, body)
	}

	// Ollama streams newline-delimited JSON objects.
	// Each line: {"message":{"content":"<token>"},"done":false}
	// Last line:  {"done":true,...}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue // skip malformed lines
		}
		if chunk.Message.Content != "" {
			fn(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}
