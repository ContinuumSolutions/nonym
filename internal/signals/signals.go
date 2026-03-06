// Package signals provides simplified signal analysis and management.
// Replaces the complex brain/activities system with a focus on:
// - Relevance detection
// - Smart categorization
// - Reply drafting
// - Actionable insights
package signals

import (
	"time"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
)

// Signal represents a processed communication with AI analysis.
type Signal struct {
	ID              int                   `json:"id"`
	ServiceSlug     string                `json:"service_slug"`
	OriginalSignal  datasync.RawSignal    `json:"original_signal"`
	Analysis        ai.AnalysedSignal     `json:"analysis"`
	Status          Status                `json:"status"`
	ReplyStatus     ReplyStatus           `json:"reply_status"`
	UserNotes       string                `json:"user_notes"`
	ProcessedAt     time.Time             `json:"processed_at"`
	LastUpdated     time.Time             `json:"last_updated"`
}

// Status tracks what the user has done with this signal.
type Status int

const (
	StatusPending   Status = 0 // Waiting for user review
	StatusDone      Status = 1 // User marked as completed
	StatusIgnored   Status = 2 // User marked as not important
	StatusSnoozed   Status = 3 // User postponed until later
)

func (s Status) String() string {
	switch s {
	case StatusDone: return "done"
	case StatusIgnored: return "ignored"
	case StatusSnoozed: return "snoozed"
	default: return "pending"
	}
}

// ReplyStatus tracks the state of drafted replies.
type ReplyStatus int

const (
	ReplyNone     ReplyStatus = 0 // No reply needed/generated
	ReplyDrafted  ReplyStatus = 1 // Reply generated, waiting for user
	ReplyEdited   ReplyStatus = 2 // User modified the draft
	ReplyApproved ReplyStatus = 3 // User approved, ready to send
	ReplyRejected ReplyStatus = 4 // User rejected the draft
	ReplySent     ReplyStatus = 5 // Reply was sent
)

func (r ReplyStatus) String() string {
	switch r {
	case ReplyDrafted: return "drafted"
	case ReplyEdited: return "edited"
	case ReplyApproved: return "approved"
	case ReplyRejected: return "rejected"
	case ReplySent: return "sent"
	default: return "none"
	}
}

// FilterCriteria for querying signals.
type FilterCriteria struct {
	Category    string    // relevant, newsletter, automated, notification
	Priority    string    // high, medium, low
	Status      *Status   // nil for all statuses
	NeedsReply  *bool     // nil for all, true/false to filter
	IsRelevant  *bool     // nil for all, true/false to filter
	Since       time.Time // only signals processed after this time
	ServiceSlug string    // filter by service
}

// SignalSummary provides counts for dashboard display.
type SignalSummary struct {
	TotalPending     int `json:"total_pending"`
	HighPriority     int `json:"high_priority"`
	NeedingReplies   int `json:"needing_replies"`
	RelevantToday    int `json:"relevant_today"`
	NewslettersToday int `json:"newsletters_today"`
}

// DraftReply represents a generated or edited email reply.
type DraftReply struct {
	ID           int       `json:"id"`
	SignalID     int       `json:"signal_id"`
	OriginalText string    `json:"original_text"`   // AI-generated draft
	EditedText   string    `json:"edited_text"`     // User-modified version
	Tone         string    `json:"tone"`            // professional, casual, friendly, formal
	Recipients   []string  `json:"recipients"`      // email addresses
	Subject      string    `json:"subject"`         // reply subject line
	Status       ReplyStatus `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProcessingResult summarizes a batch signal analysis run.
type ProcessingResult struct {
	TotalSignals     int       `json:"total_signals"`
	RelevantSignals  int       `json:"relevant_signals"`
	DraftsGenerated  int       `json:"drafts_generated"`
	NewslettersFound int       `json:"newsletters_found"`
	ProcessedAt      time.Time `json:"processed_at"`
	Errors           []string  `json:"errors,omitempty"`
}