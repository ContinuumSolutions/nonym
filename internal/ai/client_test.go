package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/datasync"
)

// ── parseEventType ────────────────────────────────────────────────────────────

func TestParseEventType(t *testing.T) {
	cases := []struct {
		s        string
		fallback string
		want     activities.EventType
	}{
		{"finance", "", activities.Finance},
		{"Finance", "", activities.Finance},
		{"FINANCE", "", activities.Finance},
		{"calendar", "", activities.Calendar},
		{"communication", "", activities.Communication},
		{"billing", "", activities.Billing},
		{"health", "", activities.Health},
		{"other", "", activities.Other},
		// Unknown but valid fallback
		{"unknown", "Calendar", activities.Calendar},
		// Both unknown → explicit Other fallback
		{"", "", activities.Other},
	}
	for _, tc := range cases {
		got := parseEventType(tc.s, tc.fallback)
		if got != tc.want {
			t.Errorf("parseEventType(%q, %q) = %v, want %v", tc.s, tc.fallback, got, tc.want)
		}
	}
}

// ── parseImportance ───────────────────────────────────────────────────────────

func TestParseImportance(t *testing.T) {
	cases := []struct {
		s    string
		want activities.Importance
	}{
		{"high", activities.High},
		{"High", activities.High},
		{"HIGH", activities.High},
		{"medium", activities.Medium},
		{"Medium", activities.Medium},
		{"low", activities.Low},
		{"", activities.Low},
		{"unknown", activities.Low},
	}
	for _, tc := range cases {
		got := parseImportance(tc.s)
		if got != tc.want {
			t.Errorf("parseImportance(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

// ── buildUserMessage ──────────────────────────────────────────────────────────

func TestBuildUserMessage_ContainsKeyFields(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "gmail",
		Category:    "Communication",
		Title:       "Invoice #123",
		Body:        "Please pay $500 by Friday.",
		OccurredAt:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Metadata:    map[string]string{"from": "vendor@acme.com"},
	}
	msg := buildUserMessage(sig)

	required := []string{"gmail", "Communication", "Invoice #123", "Please pay $500", "vendor@acme.com"}
	for _, r := range required {
		if !strings.Contains(msg, r) {
			t.Errorf("buildUserMessage missing %q in output:\n%s", r, msg)
		}
	}
}

func TestBuildUserMessage_OmitsEmptyBody(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "slack",
		Category:    "Communication",
		Title:       "Ping",
		OccurredAt:  time.Now(),
	}
	msg := buildUserMessage(sig)
	if strings.Contains(msg, "Body:") {
		t.Error("empty body should not produce a Body: line")
	}
}

func TestBuildUserMessage_OmitsEmptyMetadataValues(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "slack",
		Category:    "Communication",
		Title:       "Test",
		OccurredAt:  time.Now(),
		Metadata:    map[string]string{"from": "", "user": "alice"},
	}
	msg := buildUserMessage(sig)
	// "from" has empty value — should be omitted
	if strings.Contains(msg, "from:") {
		t.Error("empty metadata value should be omitted from the message")
	}
	if !strings.Contains(msg, "alice") {
		t.Error("non-empty metadata value should appear in message")
	}
}

// ── toAnalysedSignal ──────────────────────────────────────────────────────────

func TestToAnalysedSignal_MapsFields(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "gmail",
		Category:    "Communication",
		Title:       "Meeting request",
		OccurredAt:  time.Now(),
		Metadata:    map[string]string{"from": "alice@example.com"},
	}
	out := llmOutput{
		EventType:       "Calendar",
		Importance:      "High",
		EstimatedROI:    1500.0,
		TimeCommitment:  2.0,
		ManipulationPct: 0.05,
		Narrative:       "Alice wants a meeting.",
		Gain: struct {
			Type    string  `json:"type"`
			Value   float64 `json:"value"`
			Symbol  string  `json:"symbol"`
			Details string  `json:"details"`
		}{Type: "Positive", Value: 1500.0, Symbol: "$", Details: "Paid consulting"},
	}

	as := toAnalysedSignal(sig, out)

	if as.EventType != activities.Calendar {
		t.Errorf("EventType: want Calendar, got %v", as.EventType)
	}
	if as.Importance != activities.High {
		t.Errorf("Importance: want High, got %v", as.Importance)
	}
	if as.Request.EstimatedROI != 1500.0 {
		t.Errorf("EstimatedROI: want 1500.0, got %.1f", as.Request.EstimatedROI)
	}
	if as.Request.SenderID != "alice@example.com" {
		t.Errorf("SenderID: want alice@example.com, got %q", as.Request.SenderID)
	}
	if as.Gain.Type != activities.Positive {
		t.Errorf("Gain.Type: want Positive, got %v", as.Gain.Type)
	}
	if as.Gain.Value != 1500.0 {
		t.Errorf("Gain.Value: want 1500.0, got %.1f", as.Gain.Value)
	}
}

func TestToAnalysedSignal_NegativeGain(t *testing.T) {
	sig := datasync.RawSignal{ServiceSlug: "stripe", OccurredAt: time.Now()}
	out := llmOutput{
		EventType: "Billing",
		Gain: struct {
			Type    string  `json:"type"`
			Value   float64 `json:"value"`
			Symbol  string  `json:"symbol"`
			Details string  `json:"details"`
		}{Type: "Negative", Value: 200.0, Symbol: "$"},
	}
	as := toAnalysedSignal(sig, out)
	if as.Gain.Type != activities.Negative {
		t.Errorf("want Negative gain type, got %v", as.Gain.Type)
	}
}

func TestToAnalysedSignal_SenderFallsBackToServiceSlug(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "plaid",
		OccurredAt:  time.Now(),
		// No from/user metadata
	}
	as := toAnalysedSignal(sig, llmOutput{})
	if as.Request.SenderID != "plaid" {
		t.Errorf("SenderID should fall back to service slug, got %q", as.Request.SenderID)
	}
}
