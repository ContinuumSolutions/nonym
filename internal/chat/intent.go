package chat

import "strings"

// Intent classifies what kind of data the user is asking about so we can
// fetch only the relevant records and embed them directly in the user turn.
type Intent int

const (
	IntentGeneral       Intent = iota // no data injection needed
	IntentFocusToday                  // "what should I focus on", "priorities"
	IntentDecisions                   // "recent decisions", "what did you do"
	IntentFinancial                   // "money", "spending", "gains"
	IntentHealth                      // "health", "stress", "sleep", "mood"
	IntentSocialDebt                  // "owe", "favour", "harvest"
	IntentReputation                  // "reputation", "score", "tier"
	IntentNotifications               // "notifications", "alerts"
	IntentKernelStatus                // "status", "sync", "kernel"
)

// detectIntent classifies a user message into the most specific data intent.
// More specific patterns are checked first to avoid false matches.
func detectIntent(message string) Intent {
	msg := strings.ToLower(message)

	// Focus / priorities — most actionable intent
	if matchesAny(msg, []string{
		"what should i focus", "focus on today", "my priorities",
		"what to do today", "what to work on", "where should i",
		"most important", "top priority", "what matters",
		"what's important", "what is important",
		"what do i need to do", "today's tasks", "my tasks",
		"anything urgent", "what's pending", "what is pending",
	}) {
		return IntentFocusToday
	}

	// Health — check before financial (avoids overlap on "stress cost" etc.)
	if matchesAny(msg, []string{
		"my stress", "my sleep", "my mood", "my energy",
		"how am i feeling", "how do i feel", "biometric",
		"health check", "my health", "feeling today",
		"how tired", "my wellbeing", "decision shield",
		"am i shielded", "stress level", "sleep quality",
	}) {
		return IntentHealth
	}

	// Financial
	if matchesAny(msg, []string{
		"spending", "spent", "expense", "expenses",
		"how much money", "my finances", "financial",
		"money saved", "money earned", "income", "revenue",
		"my gains", "time saved", "hours saved",
		"what did i earn", "what did i spend", "transactions",
		"how much have you saved", "total savings",
	}) {
		return IntentFinancial
	}

	// Social debt
	if matchesAny(msg, []string{
		"who owes", "social debt", "favour", "favor",
		"harvest", "unreciprocated", "reciprocat",
		"owe me", "owes me", "social scan",
	}) {
		return IntentSocialDebt
	}

	// Reputation
	if matchesAny(msg, []string{
		"my reputation", "reputation score", "my score",
		"my tier", "trust tax", "ledger", "am i exiled",
		"trust score",
	}) {
		return IntentReputation
	}

	// Notifications
	if matchesAny(msg, []string{
		"my notifications", "unread notifications",
		"any alerts", "any notifications", "what alerts",
		"show alerts", "show notifications", "new notifications",
	}) {
		return IntentNotifications
	}

	// Decisions / activity
	if matchesAny(msg, []string{
		"decisions", "what did you decide", "what have you decided",
		"what did you do", "what have you done", "recent activity",
		"accepted events", "declined events", "automated events",
		"show decisions", "list decisions", "what actions",
		"what have you handled",
	}) {
		return IntentDecisions
	}

	// Kernel status
	if matchesAny(msg, []string{
		"kernel status", "last sync", "last run",
		"scheduler status", "sync status", "how are you doing",
		"how is the kernel", "kernel health",
	}) {
		return IntentKernelStatus
	}

	return IntentGeneral
}

// dataKeywords are lower-cased substrings that signal the user is asking about
// live kernel data. When any match, we pre-fetch and inject fresh tool results
// directly into the system prompt before calling the LLM.
var dataKeywords = []string{
	// financial / spending
	"spend", "spending", "spent", "expense", "expenses",
	"transaction", "transactions", "purchase", "purchases",
	"cost", "costs", "bill", "bills", "payment", "payments",
	"charge", "charges", "invoice", "invoices",
	"balance", "bank", "bank account",
	"how much", "how many", "total", "sum",
	"money", "earned", "saved me", "made me",
	"income", "revenue", "profit", "loss",
	// time savings
	"time saved", "time have you", "hours saved",
	// events / activity
	"events", "recent", "activity", "decisions",
	"accepted", "declined", "automated", "ghosted",
	// health
	"biometrics", "stress", "sleep", "mood", "energy", "check-in", "health",
	// notifications
	"notification", "alert", "unread",
	// social
	"harvest", "favour", "favor", "debt", "owe",
	// reputation
	"reputation", "score", "tier", "ledger", "trust",
	// scheduler / sync
	"sync", "scheduler", "last run",
	// generic data requests
	"status", "stats", "statistics", "numbers", "data",
	"what have you done", "what did you do",
	"show me", "give me", "tell me my",
	"this week", "this month", "today", "yesterday",
	"last week", "last month",
}

// needsLiveData returns true when the message appears to be asking about
// live kernel data. Used to decide whether to pre-fetch and inject tool results.
func needsLiveData(message string) bool {
	msg := strings.ToLower(message)
	for _, kw := range dataKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
