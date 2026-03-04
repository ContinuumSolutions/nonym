package execution

import (
	"strconv"
	"strings"

	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/datasync"
)

// ActionType identifies what real-world operation the kernel will perform.
type ActionType string

const (
	ActionNone               ActionType = "none"
	ActionArchiveEmail       ActionType = "archive_email"       // manipulative email → archive
	ActionDeclineCalendar    ActionType = "decline_calendar"    // low-value invite → decline
	ActionRequestRefund      ActionType = "request_refund"      // late delivery/dispute → refund
	ActionCancelSubscription ActionType = "cancel_subscription" // price increase → cancel
)

// Action describes a concrete operation to perform against a service API.
type Action struct {
	Type           ActionType
	ServiceSlug    string
	ResourceID     string            // message_id / event_id / charge_id / account
	ResourceMeta   map[string]string // additional context for executors
	Reason         string
	EstimatedCost  float64 // used for auto-exec threshold check
	ReputationRisk float64 // 0–1; above 0.3 → queued for review
}

// ClassifyAction derives the action from the signal metadata and LLM scores.
// No extra LLM call is needed — all inputs come from the pipeline's existing analysis.
//
// Rules:
//   - Communication + manipulation_pct > 0.10        → ActionArchiveEmail
//   - Calendar + time_commitment ≤ 0.5 AND roi < 100 → ActionDeclineCalendar
//   - Finance/Billing + narrative contains late|delay|refund → ActionRequestRefund
//   - Finance/Billing + narrative contains subscription|price increase → ActionCancelSubscription
func ClassifyAction(signal datasync.RawSignal, as *ai.AnalysedSignal) Action {
	narrative := strings.ToLower(as.Narrative)
	category := signal.Category
	slug := signal.ServiceSlug

	switch {
	case category == "Communication" && as.Request.ManipulationPct > 0.10:
		return Action{
			Type:           ActionArchiveEmail,
			ServiceSlug:    slug,
			ResourceID:     signal.Metadata["message_id"],
			ResourceMeta:   copyMeta(signal.Metadata),
			Reason:         "manipulation detected",
			EstimatedCost:  0,
			ReputationRisk: as.Request.ManipulationPct,
		}

	case category == "Calendar" && as.Request.TimeCommitment <= 0.5 && as.Request.EstimatedROI < 100:
		return Action{
			Type:           ActionDeclineCalendar,
			ServiceSlug:    slug,
			ResourceID:     signal.Metadata["event_id"],
			ResourceMeta:   copyMeta(signal.Metadata),
			Reason:         "low-value time commitment",
			EstimatedCost:  0,
			ReputationRisk: 0.1,
		}

	case (category == "Finance" || category == "Billing") &&
		(strings.Contains(narrative, "late") ||
			strings.Contains(narrative, "delay") ||
			strings.Contains(narrative, "refund")):
		resourceID := signal.Metadata["charge_id"]
		if resourceID == "" {
			resourceID = signal.Metadata["transaction_id"]
		}
		return Action{
			Type:           ActionRequestRefund,
			ServiceSlug:    slug,
			ResourceID:     resourceID,
			ResourceMeta:   copyMeta(signal.Metadata),
			Reason:         "late/delayed delivery — refund requested",
			EstimatedCost:  parseAmount(signal.Metadata["amount"]),
			ReputationRisk: 0.0,
		}

	case (category == "Finance" || category == "Billing") &&
		(strings.Contains(narrative, "subscription") ||
			strings.Contains(narrative, "price increase")):
		resourceID := signal.Metadata["account"]
		if resourceID == "" {
			resourceID = signal.Metadata["charge_id"]
		}
		return Action{
			Type:           ActionCancelSubscription,
			ServiceSlug:    slug,
			ResourceID:     resourceID,
			ResourceMeta:   copyMeta(signal.Metadata),
			Reason:         "subscription price increase — cancellation initiated",
			EstimatedCost:  parseAmount(signal.Metadata["amount"]),
			ReputationRisk: 0.05,
		}
	}

	return Action{Type: ActionNone}
}

// IsAutoExecutable returns true when the action is safe to run without human approval:
// cost below threshold AND reputation risk below 30%.
func IsAutoExecutable(a Action, threshold float64) bool {
	return a.EstimatedCost < threshold && a.ReputationRisk < 0.3
}

func parseAmount(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func copyMeta(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
