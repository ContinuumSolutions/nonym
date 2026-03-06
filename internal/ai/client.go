// Package ai provides a client for the local Ollama LLM instance.
// It analyses RawSignal batches and returns structured AnalysedSignal values
// ready for the brain pipeline (step 6).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/datasync"
)

// Client talks to a locally running Ollama instance.
type Client struct {
	host        string
	model       string
	numCtx      int // context window size (tokens); 0 = Ollama default
	numPredict  int // max tokens to generate; 0 = Ollama default
	identityCtx func() string // optional: called per-request to inject user identity into prompts
}

// NewClient creates a client pointing at host with the given model.
// Defaults: host = "http://localhost:11434", model = "llama3.2".
func NewClient(host, model string) *Client {
	if host == "" {
		host = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	return &Client{host: host, model: model}
}

// WithNumCtx sets the context window size (in tokens).
// Smaller values reduce memory use and speed up inference on CPU.
// A good starting point is 2048; increase if conversations feel truncated.
func (c *Client) WithNumCtx(n int) *Client    { c.numCtx = n; return c }

// WithNumPredict caps the maximum tokens the model will generate per reply.
func (c *Client) WithNumPredict(n int) *Client { c.numPredict = n; return c }

// WithIdentityProvider registers a function that returns the current user identity context.
// It is called on every LLM analysis request; the returned string is prepended to the
// system prompt so the model knows who it is scoring signals for.
// Pass a closure that reads from your profile store: func() string { return store.GetIdentityContext() }
func (c *Client) WithIdentityProvider(fn func() string) *Client { c.identityCtx = fn; return c }

func (c *Client) getIdentityContext() string {
	if c.identityCtx == nil {
		return ""
	}
	return c.identityCtx()
}

// ollamaOptions builds the options map for the Ollama API request.
func (c *Client) ollamaOptions(temperature float64) map[string]any {
	opts := map[string]any{"temperature": temperature}
	if c.numCtx > 0 {
		opts["num_ctx"] = c.numCtx
	}
	if c.numPredict > 0 {
		opts["num_predict"] = c.numPredict
	}
	return opts
}


// AnalysedSignal is the fully structured output produced by the LLM for one RawSignal.
// It contains the relevance analysis, categorization, and optional reply draft.
type AnalysedSignal struct {
	Signal          datasync.RawSignal
	Category        string // relevant|newsletter|automated|notification
	Priority        string // high|medium|low
	NeedsReply      bool
	IsRelevant      bool
	Reasoning       string
	SuggestedAction string
	ReplyDraft      string
	ReplyTone       string
	Summary         string

	// Deprecated: Legacy fields for backward compatibility during transition
	Narrative string          `json:"narrative"`  // Use Summary instead
	Request   LegacyRequest   `json:"request"`    // Use new fields instead
	EventType LegacyEventType `json:"event_type"` // Use Category instead
	Importance LegacyImportance `json:"importance"` // Use Priority instead
	Gain      LegacyGain      `json:"gain"`       // Removed in simplified model
}

// Legacy types for backward compatibility during transition

type LegacyEventType = activities.EventType
type LegacyImportance = activities.Importance
type LegacyGainType = activities.GainType
type LegacyGainKind = activities.GainKind

type LegacyRequest struct {
	ID              string  `json:"id"`
	SenderID        string  `json:"sender_id"`
	Description     string  `json:"description"`
	EstimatedROI    float64 `json:"estimated_roi"`
	TimeCommitment  float64 `json:"time_commitment"`
	ManipulationPct float64 `json:"manipulation_pct"`
}

type LegacyGain = activities.Gain

// llmOutput is the JSON schema the LLM is instructed to produce.
type llmOutput struct {
	Category        string `json:"category"`         // relevant|newsletter|automated|notification
	Priority        string `json:"priority"`         // high|medium|low
	NeedsReply      bool   `json:"needs_reply"`      // true if requires a response
	IsRelevant      bool   `json:"is_relevant"`      // true if user should pay attention
	Reasoning       string `json:"reasoning"`        // why relevant/not relevant
	SuggestedAction string `json:"suggested_action"` // what user should do
	ReplyDraft      string `json:"reply_draft"`      // drafted reply if needs_reply
	ReplyTone       string `json:"reply_tone"`       // professional|casual|friendly|formal
	Summary         string `json:"summary"`          // one sentence summary
}

const baseSystemPrompt = `You are a smart email and signal analysis assistant.
Your job is to analyze incoming communications and determine:
1. Is this relevant to the user?
2. What category is it?
3. Does it need a reply?
4. Should the user focus on this now?

Respond ONLY with valid JSON matching this exact schema:
{
  "category": "relevant|newsletter|automated|notification",
  "priority": "high|medium|low",
  "needs_reply": true|false,
  "is_relevant": true|false,
  "reasoning": "<why this is/isn't relevant to the user>",
  "suggested_action": "<what the user should do, if anything>",
  "reply_draft": "<drafted reply if needs_reply is true, empty string otherwise>",
  "reply_tone": "professional|casual|friendly|formal",
  "summary": "<one sentence: what this is and why it matters>"
}

Categories explained:
- "relevant" → Needs user attention (personal emails, important notifications, deadlines)
- "newsletter" → Automated marketing/updates (can be archived)
- "automated" → System notifications, receipts, confirmations
- "notification" → Service alerts, status updates

Priority levels:
- "high" → Urgent, time-sensitive, requires immediate attention
- "medium" → Important but can wait a few hours
- "low" → Informational, no action needed

Reply drafting rules:
- Only draft if needs_reply is true AND it's from a person (not a system)
- Keep replies concise and match the original tone
- Use professional tone for business, casual for personal
- Include specific details from the original message when relevant

Use the USER CONTEXT (if present) to understand what's relevant to this specific user.`

// buildSystemPrompt returns the analysis system prompt, optionally prefixed with
// the user's identity context so the LLM can score signals relative to their profession.
func buildSystemPrompt(identity string) string {
	if identity == "" {
		return baseSystemPrompt
	}
	return identity + "\n\n" + baseSystemPrompt
}

// Analyse sends one RawSignal to Ollama and returns a structured AnalysedSignal.
func (c *Client) Analyse(ctx context.Context, signal datasync.RawSignal) (*AnalysedSignal, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": buildSystemPrompt(c.getIdentityContext())},
			{"role": "user", "content": buildUserMessage(signal)},
		},
		"stream":     false,
		"format":     "json",
		"keep_alive": -1,
		"options": map[string]interface{}{
			// Signal analysis JSON is compact; cap at 350 tokens to avoid runaway generation.
			"num_predict": 350,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai: ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ai: ollama HTTP %d: %s", resp.StatusCode, body)
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("ai: decode ollama envelope: %w", err)
	}

	var out llmOutput
	if err := json.Unmarshal([]byte(ollamaResp.Message.Content), &out); err != nil {
		return nil, fmt.Errorf("ai: decode LLM JSON: %w", err)
	}

	return toAnalysedSignal(signal, out), nil
}

// AnalyseBatch analyses a slice of signals concurrently.
// Errors are returned per-index; a nil entry means success.
func (c *Client) AnalyseBatch(ctx context.Context, signals []datasync.RawSignal) ([]*AnalysedSignal, []error) {
	results := make([]*AnalysedSignal, len(signals))
	errs := make([]error, len(signals))

	// Cap concurrency — Ollama is CPU-bound on the local machine.
	const maxConcurrency = 3
	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	for i, sig := range signals {
		wg.Add(1)
		go func(idx int, s datasync.RawSignal) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx], errs[idx] = c.Analyse(ctx, s)
		}(i, sig)
	}
	wg.Wait()
	return results, errs
}

// buildUserMessage formats a RawSignal into a readable prompt for the LLM.
func buildUserMessage(signal datasync.RawSignal) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Service: %s\n", signal.ServiceSlug)
	if signal.ServicePurpose != "" {
		fmt.Fprintf(&sb, "Service Purpose: %s\n", signal.ServicePurpose)
	}
	fmt.Fprintf(&sb, "Category: %s\n", signal.Category)
	fmt.Fprintf(&sb, "Title: %s\n", signal.Title)
	if signal.Body != "" {
		fmt.Fprintf(&sb, "Body: %s\n", signal.Body)
	}
	if len(signal.Metadata) > 0 {
		sb.WriteString("Metadata:\n")
		for k, v := range signal.Metadata {
			if v != "" {
				fmt.Fprintf(&sb, "  %s: %s\n", k, v)
			}
		}
	}
	fmt.Fprintf(&sb, "Timestamp: %s\n", signal.OccurredAt.Format("2006-01-02 15:04:05 UTC"))
	return sb.String()
}


// toAnalysedSignal maps the raw LLM output into the typed AnalysedSignal.
func toAnalysedSignal(signal datasync.RawSignal, out llmOutput) *AnalysedSignal {
	// Derive sender from metadata for backward compatibility
	sender := signal.Metadata["from"]
	if sender == "" {
		sender = signal.Metadata["user"]
	}
	if sender == "" {
		sender = signal.ServiceSlug
	}

	return &AnalysedSignal{
		Signal:          signal,
		Category:        out.Category,
		Priority:        out.Priority,
		NeedsReply:      out.NeedsReply,
		IsRelevant:      out.IsRelevant,
		Reasoning:       out.Reasoning,
		SuggestedAction: out.SuggestedAction,
		ReplyDraft:      out.ReplyDraft,
		ReplyTone:       out.ReplyTone,
		Summary:         out.Summary,

		// Backward compatibility fields
		Narrative: out.Summary, // Use summary as narrative
		Request: LegacyRequest{
			ID:              fmt.Sprintf("%s-%d", signal.ServiceSlug, signal.OccurredAt.Unix()),
			SenderID:        sender,
			Description:     out.Summary,
			EstimatedROI:    0,     // Simplified: no ROI calculation
			TimeCommitment:  0,     // Simplified: no time commitment
			ManipulationPct: 0,     // Simplified: no manipulation detection
		},
		EventType:  activities.Other, // Default to Other event type
		Importance: activities.Low,   // Default to Low importance
		Gain: activities.Gain{
			Type:    activities.Positive,
			Kind:    activities.Money,
			Value:   0,
			Symbol:  "",
			Details: "",
		},
	}
}

