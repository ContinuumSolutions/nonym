package scanner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&notionConnector{
		baseURL: "https://api.notion.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type notionConnector struct {
	baseURL string
	client  *http.Client
}

func (n *notionConnector) Vendor() string { return "notion" }

// TestConnection verifies by fetching the bot user.
func (n *notionConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "api_key", "token")
	if !strings.HasPrefix(token, "secret_") && !strings.HasPrefix(token, "ntn_") {
		return ConnectionResult{Success: false, Message: "Notion internal integration token must start with 'secret_' or 'ntn_'"}
	}
	if err := n.get(token, "/v1/users/me", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Notion connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Notion token validated — integration accessible"}
}

// FetchEvents searches accessible pages/databases and scans block content for PII.
func (n *notionConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "api_key", "token")
	if token == "" {
		return nil, fmt.Errorf("notion: api_key is required")
	}

	pages, err := n.searchPages(token)
	if err != nil {
		return nil, fmt.Errorf("notion: search: %w", err)
	}

	var all []NormalizedEvent
	for _, page := range pages {
		blocks, err := n.fetchBlocks(token, page.ID)
		if err != nil {
			continue
		}
		all = append(all, n.normalise(page, blocks)...)
		if len(all) >= 500 {
			break
		}
	}
	return all, nil
}

// ── Notion API types ──────────────────────────────────────────────────────────

type notionPage struct {
	ID         string `json:"id"`
	Object     string `json:"object"` // "page" | "database"
	Properties map[string]notionProperty `json:"properties"`
}

type notionProperty struct {
	Type  string `json:"type"`
	Title []struct {
		PlainText string `json:"plain_text"`
	} `json:"title"`
	RichText []struct {
		PlainText string `json:"plain_text"`
	} `json:"rich_text"`
	Email  string `json:"email"`
	Phone  string `json:"phone_number"`
	URL    string `json:"url"`
}

type notionSearchResponse struct {
	Results []notionPage `json:"results"`
}

type notionBlocksResponse struct {
	Results []json.RawMessage `json:"results"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (n *notionConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", n.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (n *notionConnector) post(token, path string, payload, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", n.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	return json.Unmarshal(respBody, out)
}

func (n *notionConnector) searchPages(token string) ([]notionPage, error) {
	payload := map[string]interface{}{
		"page_size": 30,
		"sort":      map[string]string{"direction": "descending", "timestamp": "last_edited_time"},
	}
	var result notionSearchResponse
	err := n.post(token, "/v1/search", payload, &result)
	return result.Results, err
}

func (n *notionConnector) fetchBlocks(token, pageID string) ([]json.RawMessage, error) {
	var result notionBlocksResponse
	err := n.get(token, fmt.Sprintf("/v1/blocks/%s/children?page_size=50", pageID), &result)
	return result.Results, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (n *notionConnector) normalise(page notionPage, blocks []json.RawMessage) []NormalizedEvent {
	endpoint := "api.notion.com/v1/blocks/" + page.ID + "/children"
	var out []NormalizedEvent

	// Scan page properties (title, rich_text, email, phone, url).
	for propName, prop := range page.Properties {
		var texts []string
		switch prop.Type {
		case "title":
			for _, rt := range prop.Title {
				if rt.PlainText != "" {
					texts = append(texts, rt.PlainText)
				}
			}
		case "rich_text":
			for _, rt := range prop.RichText {
				if rt.PlainText != "" {
					texts = append(texts, rt.PlainText)
				}
			}
		case "email":
			if prop.Email != "" {
				texts = append(texts, prop.Email)
			}
		case "phone_number":
			if prop.Phone != "" {
				texts = append(texts, prop.Phone)
			}
		case "url":
			if prop.URL != "" {
				texts = append(texts, prop.URL)
			}
		}
		for _, t := range texts {
			out = append(out, NormalizedEvent{
				VendorID: "notion", EventID: page.ID,
				Source:   "page.property." + propName,
				Text:     t,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}

	// Scan block content (paragraph, heading, bulleted_list_item, etc.).
	for _, rawBlock := range blocks {
		var block map[string]json.RawMessage
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			continue
		}
		var blockType string
		if t, ok := block["type"]; ok {
			json.Unmarshal(t, &blockType) //nolint:errcheck
		}
		var blockID string
		if id, ok := block["id"]; ok {
			json.Unmarshal(id, &blockID) //nolint:errcheck
		}
		// Blocks with rich_text arrays (paragraph, heading_1/2/3, bulleted/numbered list, etc.)
		if content, ok := block[blockType]; ok {
			var contentMap map[string]json.RawMessage
			if err := json.Unmarshal(content, &contentMap); err == nil {
				if rawRT, ok := contentMap["rich_text"]; ok {
					var richTexts []struct {
						PlainText string `json:"plain_text"`
					}
					if err := json.Unmarshal(rawRT, &richTexts); err == nil {
						var sb strings.Builder
						for _, rt := range richTexts {
							sb.WriteString(rt.PlainText)
						}
						if text := sb.String(); text != "" {
							out = append(out, NormalizedEvent{
								VendorID: "notion", EventID: blockID,
								Source:   "block." + blockType,
								Text:     text,
								Metadata: map[string]string{"endpoint": endpoint, "page_id": page.ID},
							})
						}
					}
				}
			}
		}
	}
	return out
}
