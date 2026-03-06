package signals

import (
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
)

func TestSignalCreation(t *testing.T) {
	signal := &Signal{
		ID:          1,
		ServiceSlug: "gmail",
		OriginalSignal: datasync.RawSignal{
			Title: "Contract renewal",
			Body:  "Please review the attached contract",
		},
		Analysis: ai.AnalysedSignal{
			Category:        "relevant",
			Priority:        "high",
			NeedsReply:      true,
			IsRelevant:      true,
			Reasoning:       "Contract deadline approaching",
			SuggestedAction: "Review and respond by Thursday",
			ReplyDraft:      "Hi, I'll review the terms and respond by Thursday.",
			Summary:         "Contract renewal needs attention",
		},
		Status:      StatusPending,
		ReplyStatus: ReplyDrafted,
		ProcessedAt: time.Now(),
	}

	if signal.Analysis.Category != "relevant" {
		t.Errorf("Expected category 'relevant', got %s", signal.Analysis.Category)
	}

	if signal.Status != StatusPending {
		t.Errorf("Expected status pending, got %v", signal.Status)
	}

	if !signal.Analysis.NeedsReply {
		t.Error("Expected signal to need reply")
	}
}

func TestStatusEnumStrings(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusPending, "pending"},
		{StatusDone, "done"},
		{StatusIgnored, "ignored"},
		{StatusSnoozed, "snoozed"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("Status.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestReplyStatusEnumStrings(t *testing.T) {
	tests := []struct {
		status ReplyStatus
		want   string
	}{
		{ReplyNone, "none"},
		{ReplyDrafted, "drafted"},
		{ReplyEdited, "edited"},
		{ReplyApproved, "approved"},
		{ReplyRejected, "rejected"},
		{ReplySent, "sent"},
	}

	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("ReplyStatus.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestFilterCriteria(t *testing.T) {
	filter := FilterCriteria{
		Category:    "relevant",
		Priority:    "high",
		ServiceSlug: "gmail",
	}

	if filter.Category != "relevant" {
		t.Error("Filter category not set correctly")
	}

	// Test with pointer fields
	needsReply := true
	filter.NeedsReply = &needsReply

	if filter.NeedsReply == nil || *filter.NeedsReply != true {
		t.Error("NeedsReply filter not set correctly")
	}
}

func TestDraftReply(t *testing.T) {
	draft := &DraftReply{
		ID:           1,
		SignalID:     123,
		OriginalText: "Hi, I'll review the contract and get back to you.",
		Tone:         "professional",
		Recipients:   []string{"alice@company.com"},
		Subject:      "Re: Contract renewal",
		Status:       ReplyDrafted,
		CreatedAt:    time.Now(),
	}

	if len(draft.Recipients) != 1 {
		t.Error("Expected one recipient")
	}

	if draft.Recipients[0] != "alice@company.com" {
		t.Error("Recipient not set correctly")
	}

	if draft.Status != ReplyDrafted {
		t.Error("Draft status not set correctly")
	}
}