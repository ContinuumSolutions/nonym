// Package chat serves the conversational interface between the user and their EK-1 kernel.
// POST /chat accepts a message and conversation history, injects a live data system prompt,
// and returns the kernel's reply from the local Ollama LLM.
package chat

import "time"

// Message is one turn in a conversation.
// Role is "user" or "kernel" (frontend convention; mapped to "assistant" before sending to Ollama).
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Request is the body for POST /chat.
type Request struct {
	Message string    `json:"message"`
	History []Message `json:"history"`
}

// Response is returned by POST /chat.
type Response struct {
	Reply     string    `json:"reply"`
	Timestamp time.Time `json:"timestamp"`
}
