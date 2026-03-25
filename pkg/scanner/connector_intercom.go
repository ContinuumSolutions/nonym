package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func init() {
	Register(&intercomConnector{
		baseURL: "https://api.intercom.io",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type intercomConnector struct {
	baseURL string
	client  *http.Client
}

func (ic *intercomConnector) Vendor() string { return "intercom" }

// TestConnection verifies the access token by calling /me.
func (ic *intercomConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "access_token", "token", "api_key")
	if len(token) < 8 {
		return ConnectionResult{Success: false, Message: "Intercom access token is missing or too short"}
	}
	if err := ic.get(token, "/me", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Intercom connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Intercom token validated — admin account accessible"}
}

func (ic *intercomConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "access_token", "token", "api_key")
	if token == "" {
		return nil, fmt.Errorf("intercom: no access_token in credentials")
	}

	convos, err := ic.fetchConversations(token)
	if err != nil {
		return nil, fmt.Errorf("intercom: fetch conversations: %w", err)
	}

	contacts, err := ic.fetchContacts(token)
	if err != nil {
		log.Printf("intercom connector: fetch contacts: %v", err)
	}

	var all []NormalizedEvent
	all = append(all, ic.normaliseConversations(convos)...)
	all = append(all, ic.normaliseContacts(contacts)...)
	return all, nil
}

// ── Intercom API types ────────────────────────────────────────────────────────

type intercomConversation struct {
	ID                 string `json:"id"`
	Subject            string `json:"subject"`
	ConversationMessage struct {
		Body string `json:"body"`
	} `json:"conversation_message"`
}

type intercomConversationsResponse struct {
	Conversations []intercomConversation `json:"conversations"`
}

type intercomContact struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type intercomContactsResponse struct {
	Data []intercomContact `json:"data"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (ic *intercomConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", ic.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Intercom-Version", "2.10")

	resp, err := ic.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — check access token scopes", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (ic *intercomConnector) fetchConversations(token string) ([]intercomConversation, error) {
	var result intercomConversationsResponse
	err := ic.get(token, "/conversations?per_page=50&order=desc&sort=updated_at", &result)
	return result.Conversations, err
}

func (ic *intercomConnector) fetchContacts(token string) ([]intercomContact, error) {
	var result intercomContactsResponse
	err := ic.get(token, "/contacts?per_page=50", &result)
	return result.Data, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (ic *intercomConnector) normaliseConversations(convos []intercomConversation) []NormalizedEvent {
	var out []NormalizedEvent
	for _, c := range convos {
		if c.Subject != "" {
			out = append(out, NormalizedEvent{
				VendorID: "intercom", EventID: c.ID, Source: "conversation.subject",
				Text: c.Subject, Metadata: map[string]string{"endpoint": "api.intercom.io/conversations"},
			})
		}
		if c.ConversationMessage.Body != "" {
			out = append(out, NormalizedEvent{
				VendorID: "intercom", EventID: c.ID, Source: "conversation.message.body",
				Text: c.ConversationMessage.Body, Metadata: map[string]string{"endpoint": "api.intercom.io/conversations"},
			})
		}
	}
	return out
}

func (ic *intercomConnector) normaliseContacts(contacts []intercomContact) []NormalizedEvent {
	var out []NormalizedEvent
	emit := func(id, field, text string) {
		if text == "" {
			return
		}
		out = append(out, NormalizedEvent{
			VendorID: "intercom", EventID: id, Source: field,
			Text: text, Metadata: map[string]string{"endpoint": "api.intercom.io/contacts"},
		})
	}
	for _, c := range contacts {
		emit(c.ID, "contact.email", c.Email)
		emit(c.ID, "contact.name", c.Name)
		emit(c.ID, "contact.phone", c.Phone)
	}
	return out
}
