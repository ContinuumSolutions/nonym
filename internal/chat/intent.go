package chat

import "strings"

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
