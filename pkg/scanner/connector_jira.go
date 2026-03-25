package scanner

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&jiraConnector{
		client: &http.Client{Timeout: 30 * time.Second},
	})
}

type jiraConnector struct{ client *http.Client }

func (j *jiraConnector) Vendor() string { return "jira" }

// DetectRegion infers the region from the Jira Cloud base URL.
// Atlassian's EU data residency uses a *.atlassian.net subdomain with an EU
// "site" setting; the canonical EU URL suffix is ".atlassian.net" routed via
// the EU pod — detectable via the "eu" sub-label in some enterprise URLs.
func (j *jiraConnector) DetectRegion(vc *VendorConnection) string {
	baseURL := strings.ToLower(credStr(vc, "base_url"))
	if strings.Contains(baseURL, ".eu.atlassian.net") || strings.HasPrefix(baseURL, "https://eu.") {
		return "EU"
	}
	if strings.Contains(baseURL, ".atlassian.net") {
		return "US"
	}
	return "" // self-hosted / Data Center: user must set manually
}

// TestConnection verifies credentials via the /rest/api/3/myself endpoint.
func (j *jiraConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	baseURL, email, token := credStr(vc, "base_url"), credStr(vc, "email"), credStr(vc, "api_token")
	if baseURL == "" || email == "" || token == "" {
		return ConnectionResult{Success: false, Message: "Jira requires base_url, email, and api_token"}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if err := j.get(baseURL, email, token, "/rest/api/3/myself", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Jira connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Jira credentials validated — cloud instance accessible"}
}

// FetchEvents retrieves recent issues and scans summaries and description text.
func (j *jiraConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	baseURL := strings.TrimRight(credStr(vc, "base_url"), "/")
	email := credStr(vc, "email")
	token := credStr(vc, "api_token")
	if baseURL == "" || email == "" || token == "" {
		return nil, fmt.Errorf("jira: base_url, email, and api_token are required")
	}

	issues, err := j.fetchIssues(baseURL, email, token)
	if err != nil {
		return nil, fmt.Errorf("jira: fetch issues: %w", err)
	}
	return j.normalise(baseURL, issues), nil
}

// ── Jira API types ────────────────────────────────────────────────────────────

type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description *jiraDocNode `json:"description"`
		Assignee *struct {
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
		} `json:"assignee"`
		Reporter *struct {
			DisplayName  string `json:"displayName"`
			EmailAddress string `json:"emailAddress"`
		} `json:"reporter"`
	} `json:"fields"`
}

// jiraDocNode is a simplified Atlassian Document Format node.
type jiraDocNode struct {
	Type    string         `json:"type"`
	Content []jiraDocNode  `json:"content"`
	Text    string         `json:"text"`
}

type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (j *jiraConnector) get(baseURL, email, token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return err
	}
	creds := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Accept", "application/json")

	resp, err := j.client.Do(req)
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

func (j *jiraConnector) fetchIssues(baseURL, email, token string) ([]jiraIssue, error) {
	path := `/rest/api/3/search?jql=ORDER+BY+updated+DESC&maxResults=100&fields=summary,description,assignee,reporter`
	var result jiraSearchResponse
	err := j.get(baseURL, email, token, path, &result)
	return result.Issues, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

// extractText recursively extracts plain text from an Atlassian Document Format node.
func extractJiraText(node *jiraDocNode) string {
	if node == nil {
		return ""
	}
	if node.Type == "text" {
		return node.Text
	}
	var sb strings.Builder
	for _, child := range node.Content {
		sb.WriteString(extractJiraText(&child))
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

func (j *jiraConnector) normalise(baseURL string, issues []jiraIssue) []NormalizedEvent {
	host := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
	endpoint := host + "/rest/api/3/search"
	var out []NormalizedEvent
	for _, issue := range issues {
		meta := map[string]string{"endpoint": endpoint, "key": issue.Key}

		if issue.Fields.Summary != "" {
			out = append(out, NormalizedEvent{
				VendorID: "jira", EventID: issue.ID, Source: "issue.summary",
				Text: issue.Fields.Summary, Metadata: meta,
			})
		}
		if desc := extractJiraText(issue.Fields.Description); desc != "" {
			out = append(out, NormalizedEvent{
				VendorID: "jira", EventID: issue.ID, Source: "issue.description",
				Text: desc, Metadata: meta,
			})
		}
		if issue.Fields.Assignee != nil {
			for _, f := range []struct{ s, v string }{
				{"issue.assignee.name", issue.Fields.Assignee.DisplayName},
				{"issue.assignee.email", issue.Fields.Assignee.EmailAddress},
			} {
				if f.v != "" {
					out = append(out, NormalizedEvent{
						VendorID: "jira", EventID: issue.ID, Source: f.s,
						Text: f.v, Metadata: meta,
					})
				}
			}
		}
		if issue.Fields.Reporter != nil {
			for _, f := range []struct{ s, v string }{
				{"issue.reporter.name", issue.Fields.Reporter.DisplayName},
				{"issue.reporter.email", issue.Fields.Reporter.EmailAddress},
			} {
				if f.v != "" {
					out = append(out, NormalizedEvent{
						VendorID: "jira", EventID: issue.ID, Source: f.s,
						Text: f.v, Metadata: meta,
					})
				}
			}
		}
	}
	return out
}
