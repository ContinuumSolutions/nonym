package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/egokernel/ek1/internal/datasync"
)

// InteractionClass is the harvest-specific LLM output for a single communication signal.
type InteractionClass struct {
	// Kind classifies the interaction from the user's perspective.
	// Values: "favour_given" | "favour_received" | "request" | "neutral"
	Kind string `json:"kind"`

	// Overlap is the estimated probability (0–1) that this contact's needs
	// match the user's skills or solution set. Used for ghost-agreement detection.
	Overlap float64 `json:"overlap"`
}

const classifySystemPrompt = `You analyse communication signals for a personal AI agent.
Classify each interaction strictly from the USER'S perspective.

Respond ONLY with valid JSON — no markdown, no explanation:
{
  "kind": "favour_given|favour_received|request|neutral",
  "overlap": <float 0.0-1.0>
}

Definitions:
- favour_given:    the USER helped, introduced, advised, or gave value to the other person
- favour_received: the other person helped, introduced, advised, or gave value to the USER
- request:         the other person is asking for something with no indication of reciprocity
- neutral:         informational, transactional, scheduling, or otherwise unclassifiable

overlap: probability that this contact's problem domain matches the user's apparent expertise.
Use context clues (industry, topics discussed, requests made). Default to 0.1 if unclear.`

// ClassifyInteraction asks the LLM to classify a single communication or calendar
// signal for social graph analysis. Used exclusively by the harvest engine.
func (c *Client) ClassifyInteraction(ctx context.Context, signal datasync.RawSignal) (InteractionClass, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": classifySystemPrompt},
			{"role": "user", "content": buildUserMessage(signal)},
		},
		"stream":     false,
		"format":     "json",
		"keep_alive": -1,
		"options": map[string]interface{}{
			// Classify output is tiny: {"kind":"...","overlap":0.x} — 80 tokens is plenty.
			"num_predict": 80,
		},
	})
	if err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: ollama: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: read: %w", err)
	}
	if resp.StatusCode >= 400 {
		return InteractionClass{}, fmt.Errorf("ai: classify: HTTP %d", resp.StatusCode)
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: decode envelope: %w", err)
	}

	var out InteractionClass
	if err := json.Unmarshal([]byte(ollamaResp.Message.Content), &out); err != nil {
		return InteractionClass{}, fmt.Errorf("ai: classify: decode JSON: %w", err)
	}
	return out, nil
}
