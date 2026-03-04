package chat

import "strings"

// dataKeywords are lower-cased substrings that signal the user is asking about
// live kernel data. When none match, we skip the tool-calling round trip and
// rely solely on the pre-loaded system prompt data briefing.
var dataKeywords = []string{
	// gains / savings
	"how much", "how many", "total", "sum",
	"time saved", "time have you", "hours saved",
	"money", "earned", "saved me", "made me",
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
}

// needsTools returns true when the message appears to be asking for live
// kernel data that may not be fully covered by the static system prompt.
// False positives are fine — they just add one extra round trip.
// False negatives skip tool calling for data questions, falling back to the
// system prompt which already contains recent aggregates.
func needsTools(message string) bool {
	msg := strings.ToLower(message)
	for _, kw := range dataKeywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
