// Integration example: Replace line 394 in cmd/ek1/main.go with:

// OLD:
// chat.NewSimpleHandler(aiClient, profileStore, checkInStore, notifsStore, signalsStore, chatHistory).RegisterRoutes(api)

// NEW:
baseHandler := chat.NewSimpleHandler(aiClient, profileStore, checkInStore, notifsStore, signalsStore, chatHistory)
analysisHandler := chat.NewAnalysisHandler(baseHandler, db)
analysisHandler.RegisterRoutes(api)

// This enables:
// 1. Regular chat at POST /api/v1/chat (with auto-detection)
// 2. Data analysis at POST /api/v1/chat/analyze
// 3. Chat history at GET /api/v1/chat/history

// The analysis system will automatically:
// - Query your SQLite database for real historical data
// - Prevent hallucinations by grounding AI responses in actual records
// - Provide actionable insights based on patterns in your signals, biometrics, and notifications
// - Support time frames: "week" (default), "month", "quarter"
// - Allow focusing on specific data types: ["signals", "biometrics", "notifications"]
