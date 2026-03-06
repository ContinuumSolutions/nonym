package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const inferSystemPrompt = `You are analysing the processed signal history of a personal AI agent user.
Based on the following event summaries from their email, calendar, finance, and health accounts,
infer who this person is professionally.

Respond ONLY with plain text — 2 to 4 sentences. Describe their profession, apparent industry,
key skill areas, and what kinds of signals they deal with most. Be specific and factual.
Do not use lists or markdown. Do not introduce yourself or explain what you are doing.`

// InferIdentity analyses a sample of recent event narratives and returns a plain-text
// description of who the user appears to be. Used by POST /profile/infer.
func (c *Client) InferIdentity(ctx context.Context, narratives []string) (string, error) {
	if len(narratives) == 0 {
		return "", fmt.Errorf("ai: infer: no narratives provided")
	}

	var sb strings.Builder
	sb.WriteString("Recent signal summaries:\n")
	for i, n := range narratives {
		if i >= 40 {
			break
		}
		fmt.Fprintf(&sb, "%d. %s\n", i+1, n)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": inferSystemPrompt},
			{"role": "user", "content": sb.String()},
		},
		"stream":     false,
		"keep_alive": -1,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 200,
		},
	})
	if err != nil {
		return "", fmt.Errorf("ai: infer: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("ai: infer: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: infer: ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ai: infer: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("ai: infer: ollama HTTP %d: %s", resp.StatusCode, body)
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("ai: infer: decode envelope: %w", err)
	}

	summary := strings.TrimSpace(ollamaResp.Message.Content)
	if summary == "" {
		return "", fmt.Errorf("ai: infer: empty response from LLM")
	}
	return summary, nil
}
