package datasync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NotionAdapter pulls recently edited pages from Notion via the Search API.
// OAuth scope is determined by the Notion integration settings (not the request).
// API version: 2022-06-28
type NotionAdapter struct{}

func (a *NotionAdapter) Slug() string { return "notion" }

func (a *NotionAdapter) Pull(ctx context.Context, creds Credentials, since time.Time) ([]RawSignal, error) {
	payload := map[string]interface{}{
		"filter": map[string]string{
			"property": "object",
			"value":    "page",
		},
		"sort": map[string]string{
			"direction": "descending",
			"timestamp": "last_edited_time",
		},
		"page_size": 50,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("notion: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.notion.com/v1/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("notion: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.OAuthAccessToken)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("notion: search: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("notion: read response: %w", err)
	}

	var result notionSearchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("notion: decode response: %w", err)
	}

	var signals []RawSignal
	for _, page := range result.Results {
		editedAt, err := time.Parse(time.RFC3339, page.LastEditedTime)
		if err != nil || editedAt.Before(since) {
			continue
		}
		title := notionPageTitle(page.Properties)
		if title == "" {
			title = "Untitled"
		}
		signals = append(signals, RawSignal{
			ServiceSlug: a.Slug(),
			Category:    "Productivity",
			Title:       title,
			Body:        fmt.Sprintf("Notion page updated. URL: %s", page.URL),
			Metadata: map[string]string{
				"page_id": page.ID,
				"url":     page.URL,
			},
			OccurredAt: editedAt,
		})
	}
	return signals, nil
}

// notionSearchResult mirrors the Notion /v1/search response shape we need.
type notionSearchResult struct {
	Results []notionPage `json:"results"`
}

type notionPage struct {
	ID             string                        `json:"id"`
	LastEditedTime string                        `json:"last_edited_time"`
	URL            string                        `json:"url"`
	Properties     map[string]notionProperty     `json:"properties"`
}

type notionProperty struct {
	Type  string        `json:"type"`
	Title []notionRichText `json:"title"`
}

type notionRichText struct {
	PlainText string `json:"plain_text"`
}

// notionPageTitle extracts the plain-text title from the page's "title"-type property.
func notionPageTitle(props map[string]notionProperty) string {
	for _, prop := range props {
		if prop.Type == "title" {
			var parts []string
			for _, rt := range prop.Title {
				if rt.PlainText != "" {
					parts = append(parts, rt.PlainText)
				}
			}
			if len(parts) > 0 {
				return join(parts)
			}
		}
	}
	return ""
}

// join concatenates string slices without importing strings.
func join(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s
	}
	return out
}
