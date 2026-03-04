package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Tool describes one callable function the LLM can invoke during a chat turn.
type Tool struct {
	Name        string
	Description string
	Parameters  ToolParameters
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

// ToolParameters is the JSON Schema subset that Ollama accepts.
type ToolParameters struct {
	Properties map[string]ToolParam
	Required   []string
}

// ToolParam is one property in the tool's parameter schema.
type ToolParam struct {
	Type        string
	Description string
	Enum        []string // optional; omitted when empty
}

// ollamaMessage is the full Ollama message struct, including the tool_calls
// field that only appears in assistant messages when the model calls a tool.
type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolCallFn `json:"function"`
}

type ollamaToolCallFn struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// toWire converts a Tool to the JSON format Ollama's /api/chat endpoint expects.
func (t Tool) toWire() map[string]any {
	props := make(map[string]any, len(t.Parameters.Properties))
	for name, p := range t.Parameters.Properties {
		prop := map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		props[name] = prop
	}
	required := t.Parameters.Required
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters": map[string]any{
				"type":       "object",
				"properties": props,
				"required":   required,
			},
		},
	}
}

// ChatWithTools sends a conversation to Ollama with tool calling support.
// When the model requests a tool, the tool is executed and the result is fed
// back. The loop repeats until the model replies without requesting further
// tools, or until maxIterations is reached to prevent runaway calls.
func (c *Client) ChatWithTools(ctx context.Context, systemPrompt string, turns []ChatTurn, tools []Tool) (string, error) {
	// Build the initial message list.
	msgs := make([]ollamaMessage, 0, len(turns)+1)
	if systemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: systemPrompt})
	}
	for _, t := range turns {
		msgs = append(msgs, ollamaMessage{Role: t.Role, Content: t.Content})
	}

	// Index tools by name for fast lookup during execution.
	index := make(map[string]Tool, len(tools))
	for _, t := range tools {
		index[t.Name] = t
	}

	// Convert tools to Ollama wire format.
	wireTools := make([]map[string]any, len(tools))
	for i, t := range tools {
		wireTools[i] = t.toWire()
	}

	const maxIterations = 5
	for range maxIterations {
		content, toolCalls, err := c.chatRound(ctx, msgs, wireTools)
		if err != nil {
			return "", err
		}

		// No tool calls — model produced a final answer.
		if len(toolCalls) == 0 {
			return content, nil
		}

		// Append the assistant's tool-call message.
		msgs = append(msgs, ollamaMessage{
			Role:      "assistant",
			Content:   content,
			ToolCalls: toolCalls,
		})

		// Execute each requested tool and append the result.
		for _, tc := range toolCalls {
			result := executeToolCall(ctx, index, tc)
			msgs = append(msgs, ollamaMessage{Role: "tool", Content: result})
		}
	}

	// Max iterations reached — do one final call without tools so the model
	// summarises what it has gathered into a text reply.
	content, _, err := c.chatRound(ctx, msgs, nil)
	return content, err
}

// chatRound posts one request to Ollama and returns the response text and any
// tool calls the model requested. wireTools may be nil to disable tool use.
func (c *Client) chatRound(ctx context.Context, msgs []ollamaMessage, wireTools []map[string]any) (string, []ollamaToolCall, error) {
	body := map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"stream":     false,
		"keep_alive": -1,
		"options": map[string]any{
			"temperature": 0.2,
			"num_predict": 800,
		},
	}
	if len(wireTools) > 0 {
		body["tools"] = wireTools
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("ai: tools marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return "", nil, fmt.Errorf("ai: tools build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("ai: ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("ai: tools read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("ai: ollama HTTP %d: %s", resp.StatusCode, raw)
	}

	var ollamaResp struct {
		Message struct {
			Content   string           `json:"content"`
			ToolCalls []ollamaToolCall `json:"tool_calls"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		return "", nil, fmt.Errorf("ai: tools decode response: %w", err)
	}

	return ollamaResp.Message.Content, ollamaResp.Message.ToolCalls, nil
}

// executeToolCall looks up the named tool and runs it, returning a JSON string.
// Errors are encoded as JSON so the model can read them.
func executeToolCall(ctx context.Context, index map[string]Tool, tc ollamaToolCall) string {
	tool, ok := index[tc.Function.Name]
	if !ok {
		return fmt.Sprintf(`{"error":"unknown tool %q"}`, tc.Function.Name)
	}
	result, err := tool.Execute(ctx, tc.Function.Arguments)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return result
}
