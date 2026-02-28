package harvest

import (
	"testing"
	"time"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/datasync"
)

// ── normaliseSender ───────────────────────────────────────────────────────────

func TestNormaliseSender_DisplayNameWithEmail(t *testing.T) {
	got := normaliseSender("Alice Smith <alice@example.com>")
	if got != "Alice Smith" {
		t.Errorf("want %q, got %q", "Alice Smith", got)
	}
}

func TestNormaliseSender_EmailOnly(t *testing.T) {
	got := normaliseSender("alice@example.com")
	if got != "alice@example.com" {
		t.Errorf("want email as-is, got %q", got)
	}
}

func TestNormaliseSender_LeadingBracket(t *testing.T) {
	// No content before '<' — fall through to raw string
	got := normaliseSender("<alice@example.com>")
	if got != "<alice@example.com>" {
		t.Errorf("want raw string, got %q", got)
	}
}

func TestNormaliseSender_WhitespaceTrimmed(t *testing.T) {
	got := normaliseSender("  Bob Jones <bob@example.com>  ")
	if got != "Bob Jones" {
		t.Errorf("want %q, got %q", "Bob Jones", got)
	}
}

// ── extractSender ─────────────────────────────────────────────────────────────

func makeSignal(metadata map[string]string) datasync.RawSignal {
	return datasync.RawSignal{
		ServiceSlug: "test",
		Category:    "Communication",
		OccurredAt:  time.Now(),
		Metadata:    metadata,
	}
}

func TestExtractSender_FromKey(t *testing.T) {
	sig := makeSignal(map[string]string{"from": "Carol <carol@example.com>", "user": "other"})
	got := extractSender(sig)
	if got != "Carol" {
		t.Errorf("want %q (from key takes priority), got %q", "Carol", got)
	}
}

func TestExtractSender_UserKeyFallback(t *testing.T) {
	sig := makeSignal(map[string]string{"user": "Dave"})
	got := extractSender(sig)
	if got != "Dave" {
		t.Errorf("want %q, got %q", "Dave", got)
	}
}

func TestExtractSender_OrganizerKeyFallback(t *testing.T) {
	sig := makeSignal(map[string]string{"organizer": "Eve"})
	got := extractSender(sig)
	if got != "Eve" {
		t.Errorf("want %q, got %q", "Eve", got)
	}
}

func TestExtractSender_EmptyMetadata(t *testing.T) {
	sig := makeSignal(map[string]string{})
	if got := extractSender(sig); got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

func TestExtractSender_NilMetadata(t *testing.T) {
	sig := datasync.RawSignal{ServiceSlug: "test", OccurredAt: time.Now()}
	if got := extractSender(sig); got != "" {
		t.Errorf("want empty string for nil metadata, got %q", got)
	}
}

// ── debtImportance ────────────────────────────────────────────────────────────

func TestDebtImportance(t *testing.T) {
	cases := []struct {
		net  int
		want activities.Importance
	}{
		{1, activities.Low},
		{2, activities.Medium},
		{4, activities.Medium},
		{5, activities.High},
		{10, activities.High},
	}
	for _, tc := range cases {
		got := debtImportance(tc.net)
		if got != tc.want {
			t.Errorf("debtImportance(%d) = %v, want %v", tc.net, got, tc.want)
		}
	}
}
