package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/datasync"
)

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

func TestToAnalysedSignal_MapsSimplifiedFields(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "gmail",
		Category:    "Communication",
		Title:       "Contract renewal reminder",
		OccurredAt:  time.Now(),
		Metadata:    map[string]string{"from": "alice@example.com"},
	}
	out := llmOutput{
		Category:        "relevant",
		Priority:        "high",
		NeedsReply:      true,
		IsRelevant:      true,
		Reasoning:       "Contract deadline approaching, needs attention",
		SuggestedAction: "Review terms and respond by Thursday",
		ReplyDraft:      "Hi Alice, thanks for the reminder. I'll review the terms and get back to you by Thursday.",
		ReplyTone:       "professional",
		Summary:         "Contract renewal deadline approaching",
	}

	as := toAnalysedSignal(sig, out)

	if as.Category != "relevant" {
		t.Errorf("Category: want 'relevant', got %q", as.Category)
	}
	if as.Priority != "high" {
		t.Errorf("Priority: want 'high', got %q", as.Priority)
	}
	if !as.NeedsReply {
		t.Error("NeedsReply: want true, got false")
	}
	if !as.IsRelevant {
		t.Error("IsRelevant: want true, got false")
	}
	if as.Reasoning != "Contract deadline approaching, needs attention" {
		t.Errorf("Reasoning: want specific text, got %q", as.Reasoning)
	}
	if as.ReplyTone != "professional" {
		t.Errorf("ReplyTone: want 'professional', got %q", as.ReplyTone)
	}
}

func TestToAnalysedSignal_NewsletterCategory(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "gmail",
		Title:       "Weekly Newsletter",
		OccurredAt:  time.Now(),
	}
	out := llmOutput{
		Category:    "newsletter",
		Priority:    "low",
		NeedsReply:  false,
		IsRelevant:  false,
		Reasoning:   "Automated newsletter, can be archived",
		Summary:     "Weekly marketing newsletter",
		ReplyDraft:  "", // No reply needed
	}

	as := toAnalysedSignal(sig, out)

	if as.Category != "newsletter" {
		t.Errorf("Category: want 'newsletter', got %q", as.Category)
	}
	if as.NeedsReply {
		t.Error("NeedsReply: newsletters should not need replies")
	}
	if as.IsRelevant {
		t.Error("IsRelevant: newsletters should not be relevant")
	}
	if as.ReplyDraft != "" {
		t.Error("ReplyDraft: should be empty for newsletters")
	}
}

func TestToAnalysedSignal_AutomatedNotification(t *testing.T) {
	sig := datasync.RawSignal{
		ServiceSlug: "github",
		Title:       "CI workflow failed",
		OccurredAt:  time.Now(),
	}
	out := llmOutput{
		Category:        "notification",
		Priority:        "medium",
		NeedsReply:      false,
		IsRelevant:      true, // CI failures can be relevant
		Reasoning:       "Build failure needs investigation",
		SuggestedAction: "Check logs and fix failing tests",
		Summary:         "CI pipeline failure notification",
	}

	as := toAnalysedSignal(sig, out)

	if as.Category != "notification" {
		t.Errorf("Category: want 'notification', got %q", as.Category)
	}
	if as.SuggestedAction == "" {
		t.Error("SuggestedAction: should have action for relevant notifications")
	}
}

// ── Category validation ──────────────────────────────────────────────────────────

func TestValidCategories(t *testing.T) {
	validCategories := []string{"relevant", "newsletter", "automated", "notification"}

	for _, category := range validCategories {
		out := llmOutput{Category: category}
		as := toAnalysedSignal(datasync.RawSignal{}, out)
		if as.Category != category {
			t.Errorf("Category %q not preserved correctly", category)
		}
	}
}

func TestValidPriorities(t *testing.T) {
	validPriorities := []string{"high", "medium", "low"}

	for _, priority := range validPriorities {
		out := llmOutput{Priority: priority}
		as := toAnalysedSignal(datasync.RawSignal{}, out)
		if as.Priority != priority {
			t.Errorf("Priority %q not preserved correctly", priority)
		}
	}
}