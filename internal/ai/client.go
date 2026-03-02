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
	host  string
	model string
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

// SignalRequest carries the triage inputs derived from the LLM analysis.
// It mirrors brain.IncomingRequest but lives here to avoid an import cycle
// (brain/pipeline.go imports ai; ai must not import brain).
type SignalRequest struct {
	ID              string
	SenderID        string
	Description     string
	EstimatedROI    float64
	TimeCommitment  float64
	ManipulationPct float64
}

// AnalysedSignal is the fully structured output produced by the LLM for one RawSignal.
// It contains everything the brain pipeline needs: triage input and a partial Event.
type AnalysedSignal struct {
	Signal     datasync.RawSignal
	Request    SignalRequest         // for EgoKernel.Triage() — convert in brain/pipeline.go
	EventType  activities.EventType
	Importance activities.Importance
	Narrative  string
	Gain       activities.Gain
}

// llmOutput is the JSON schema the LLM is instructed to produce.
type llmOutput struct {
	EventType       string  `json:"event_type"`       // Finance|Calendar|Communication|Billing|Health
	Importance      string  `json:"importance"`       // Low|Medium|High
	EstimatedROI    float64 `json:"estimated_roi"`    // USD value of engaging with the signal
	TimeCommitment  float64 `json:"time_commitment"`  // hours needed to respond or act
	ManipulationPct float64 `json:"manipulation_pct"` // 0.0–1.0
	Narrative       string  `json:"narrative"`        // one sentence
	Gain            struct {
		Type    string  `json:"type"`    // Positive|Negative
		Value   float64 `json:"value"`
		Symbol  string  `json:"symbol"`  // e.g. "$" or "hrs"
		Details string  `json:"details"`
	} `json:"gain"`
}

const systemPrompt = `You are the analysis engine for a personal autonomous AI agent.
You receive raw signals from the user's connected services (email, calendar, finance, health) and must analyse each one.

Respond ONLY with valid JSON matching this exact schema — no markdown, no explanation:
{
  "event_type": "Finance|Calendar|Communication|Billing|Health",
  "importance": "Low|Medium|High",
  "estimated_roi": <float — USD value of engaging with this signal, 0 if none>,
  "time_commitment": <float — hours needed to respond or act, 0 if passive>,
  "manipulation_pct": <float 0.0–1.0 — detect guilt language, urgency traps, false scarcity>,
  "narrative": "<one factual sentence: what happened and why it matters to the user>",
  "gain": {
    "type": "Positive|Negative",
    "value": <float — magnitude of the gain or loss, 0 if none>,
    "symbol": "<currency or unit, e.g. $ or hrs, empty string if none>",
    "details": "<brief description of the gain/loss, empty string if none>"
  }
}

Rules:
- event_type must match the signal category where possible
- manipulation_pct > 0.15 causes the signal to be ghosted automatically
- estimated_roi and time_commitment drive the financial significance filter
- narrative is a single sentence, factual, no filler words
- gain.value is 0 and gain.details is "" when there is no meaningful gain`

// Analyse sends one RawSignal to Ollama and returns a structured AnalysedSignal.
func (c *Client) Analyse(ctx context.Context, signal datasync.RawSignal) (*AnalysedSignal, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
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
	gainType := activities.Positive
	if strings.EqualFold(out.Gain.Type, "Negative") {
		gainType = activities.Negative
	}
	gainKind := parseGainKind(out.Gain.Symbol)

	// Derive sender from whatever metadata key the adapter populates.
	sender := signal.Metadata["from"]
	if sender == "" {
		sender = signal.Metadata["user"]
	}
	if sender == "" {
		sender = signal.ServiceSlug
	}

	return &AnalysedSignal{
		Signal: signal,
		Request: SignalRequest{
			ID:              fmt.Sprintf("%s-%d", signal.ServiceSlug, signal.OccurredAt.Unix()),
			SenderID:        sender,
			Description:     out.Narrative,
			EstimatedROI:    out.EstimatedROI,
			TimeCommitment:  out.TimeCommitment,
			ManipulationPct: out.ManipulationPct,
		},
		EventType:  parseEventType(out.EventType, signal.Category),
		Importance: parseImportance(out.Importance),
		Narrative:  out.Narrative,
		Gain: activities.Gain{
			Type:    gainType,
			Kind:    gainKind,
			Value:   float32(out.Gain.Value),
			Symbol:  out.Gain.Symbol,
			Details: out.Gain.Details,
		},
	}
}

func parseEventType(s, fallback string) activities.EventType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "finance":
		return activities.Finance
	case "calendar":
		return activities.Calendar
	case "communication":
		return activities.Communication
	case "billing":
		return activities.Billing
	case "health":
		return activities.Health
	case "other":
		return activities.Other
	}
	// Derive from signal category if LLM returned an unrecognised value.
	if fallback != "" && fallback != s {
		return parseEventType(fallback, "")
	}
	return activities.Other // unrecognised — explicit fallback value
}

// parseGainKind derives the gain kind from the symbol the LLM returned.
// Time-unit symbols map to Time; everything else (including "$") maps to Money.
func parseGainKind(symbol string) activities.GainKind {
	switch strings.ToLower(strings.TrimSpace(symbol)) {
	case "h", "hr", "hrs", "hours", "min", "mins", "minutes":
		return activities.Time
	}
	return activities.Money
}

func parseImportance(s string) activities.Importance {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return activities.High
	case "medium":
		return activities.Medium
	default:
		return activities.Low
	}
}
