package datasync

// whatsapp.go — Meta WhatsApp Business Cloud API adapter.
//
// WhatsApp does not expose a REST "list messages since X" endpoint.
// Instead, Meta pushes messages to a registered webhook URL.
// This adapter acts as the bridge:
//   - RegisterRoutes mounts GET /webhooks/whatsapp (hub verification) and
//     POST /webhooks/whatsapp (inbound message delivery) on the Fiber app.
//   - Incoming messages are buffered in memory.
//   - Pull() drains the buffer and returns signals to the pipeline.
//
// Credentials (APIKeyAuth):
//   APIKey = "PHONE_NUMBER_ID|PERMANENT_ACCESS_TOKEN"
//   APIEndpoint is unused; the webhook verify token comes from WHATSAPP_VERIFY_TOKEN env.

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// whatsappMsg is one inbound message buffered from the webhook.
type whatsappMsg struct {
	From      string    // sender's WhatsApp phone number (no +)
	Name      string    // sender's display name (from contacts array)
	Body      string    // plain-text message body
	MessageID string    // wamid — deduplicate across restarts if persisted later
	Received  time.Time // timestamp from the message payload
}

// WhatsAppAdapter receives messages via Meta's webhook and surfaces them as
// RawSignals during the next pipeline sync cycle.
type WhatsAppAdapter struct {
	mu          sync.Mutex
	inbox       []whatsappMsg
	verifyToken string // set by RegisterRoutes; validated on every GET verification
}

// NewWhatsAppAdapter creates an adapter instance. Call RegisterRoutes to mount
// the webhook endpoints, then pass the adapter to datasync.NewEngine.
func NewWhatsAppAdapter(verifyToken string) *WhatsAppAdapter {
	return &WhatsAppAdapter{verifyToken: verifyToken}
}

func (a *WhatsAppAdapter) Slug() string { return "whatsapp" }

// RegisterRoutes mounts:
//   GET  /webhooks/whatsapp  — Meta hub verification challenge
//   POST /webhooks/whatsapp  — inbound message delivery
func (a *WhatsAppAdapter) RegisterRoutes(r fiber.Router) {
	r.Get("/webhooks/whatsapp", a.verify)
	r.Post("/webhooks/whatsapp", a.receive)
}

// Pull drains the in-memory inbox and returns messages that arrived after
// `since`. Messages older than `since` are discarded (already processed in a
// previous cycle or arrived out of order).
func (a *WhatsAppAdapter) Pull(_ context.Context, _ Credentials, since time.Time) ([]RawSignal, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.inbox) == 0 {
		return nil, nil
	}

	var signals []RawSignal
	for _, m := range a.inbox {
		if !m.Received.After(since) {
			continue // already covered by a previous pipeline run
		}
		title := "WhatsApp from " + m.From
		if m.Name != "" {
			title = "WhatsApp from " + m.Name
		}
		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Communication",
			Title:       title,
			Body:        m.Body,
			Metadata: map[string]string{
				"from":       m.From,
				"name":       m.Name,
				"message_id": m.MessageID,
			},
			OccurredAt: m.Received,
		})
	}
	a.inbox = a.inbox[:0] // clear after draining
	return signals, nil
}

// ── Webhook handlers ──────────────────────────────────────────────────────────

// verify handles GET /webhooks/whatsapp — Meta's hub.verify_token challenge.
// Meta sends hub.mode=subscribe, hub.verify_token, hub.challenge.
// We echo hub.challenge when the token matches.
func (a *WhatsAppAdapter) verify(c *fiber.Ctx) error {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == a.verifyToken && challenge != "" {
		return c.SendString(challenge)
	}
	return c.Status(fiber.StatusForbidden).SendString("forbidden")
}

// receive handles POST /webhooks/whatsapp — Meta delivers message events here.
func (a *WhatsAppAdapter) receive(c *fiber.Ctx) error {
	// Always reply 200 immediately — Meta retries if we don't acknowledge fast.
	defer func() { _ = c.SendStatus(fiber.StatusOK) }()

	var payload waPayload
	if err := json.Unmarshal(c.Body(), &payload); err != nil {
		log.Printf("whatsapp: bad webhook payload: %v", err)
		return nil
	}

	if payload.Object != "whatsapp_business_account" {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			v := change.Value

			// Build a name lookup from the contacts array.
			names := make(map[string]string, len(v.Contacts))
			for _, ct := range v.Contacts {
				names[ct.WAID] = ct.Profile.Name
			}

			for _, msg := range v.Messages {
				// Only handle inbound text messages.
				if msg.Type != "text" {
					continue
				}
				ts := unixStringToTime(msg.Timestamp)
				a.inbox = append(a.inbox, whatsappMsg{
					From:      msg.From,
					Name:      names[msg.From],
					Body:      msg.Text.Body,
					MessageID: msg.ID,
					Received:  ts,
				})
			}
		}
	}
	return nil
}

// ── Meta webhook payload types ────────────────────────────────────────────────

type waPayload struct {
	Object string    `json:"object"`
	Entry  []waEntry `json:"entry"`
}

type waEntry struct {
	ID      string     `json:"id"`
	Changes []waChange `json:"changes"`
}

type waChange struct {
	Field string  `json:"field"`
	Value waValue `json:"value"`
}

type waValue struct {
	MessagingProduct string      `json:"messaging_product"`
	Contacts         []waContact `json:"contacts"`
	Messages         []waMessage `json:"messages"`
}

type waContact struct {
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	WAID string `json:"wa_id"`
}

type waMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"` // Unix seconds as string
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
}

// unixStringToTime converts a Unix-seconds string (as sent by Meta) to time.Time.
func unixStringToTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(secs, 0)
}
