package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&githubConnector{
		baseURL: "https://api.github.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type githubConnector struct {
	baseURL string
	client  *http.Client
}

func (g *githubConnector) Vendor() string { return "github" }

// DetectRegion returns "US" for github.com (all data is US-hosted).
// GitHub Enterprise Server is self-hosted; region is unknown from credentials alone.
func (g *githubConnector) DetectRegion(vc *VendorConnection) string {
	return "US"
}

// TestConnection verifies the token via the /user endpoint.
func (g *githubConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "token", "api_key")
	if !strings.HasPrefix(token, "ghp_") && !strings.HasPrefix(token, "github_pat_") && !strings.HasPrefix(token, "ghs_") {
		return ConnectionResult{Success: false, Message: "GitHub token must start with ghp_, github_pat_, or ghs_"}
	}
	if err := g.get(token, "/user", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("GitHub connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "GitHub token validated — account accessible"}
}

// FetchEvents scans recent issue and PR bodies across the org's repositories.
func (g *githubConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "token", "api_key")
	if token == "" {
		return nil, fmt.Errorf("github: token is required")
	}
	org := credStr(vc, "org")

	repos, err := g.listRepos(token, org)
	if err != nil {
		return nil, fmt.Errorf("github: list repos: %w", err)
	}

	var all []NormalizedEvent
	for _, repo := range repos {
		issues, err := g.fetchIssues(token, repo.FullName)
		if err != nil {
			continue
		}
		all = append(all, g.normaliseIssues(repo.FullName, issues)...)
		if len(all) >= 500 {
			break
		}
	}
	return all, nil
}

// ── GitHub API types ──────────────────────────────────────────────────────────

type githubRepo struct {
	FullName string `json:"full_name"`
}

type githubIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (g *githubConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", g.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := g.client.Do(req)
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

func (g *githubConnector) listRepos(token, org string) ([]githubRepo, error) {
	var path string
	if org != "" {
		path = fmt.Sprintf("/orgs/%s/repos?per_page=20&sort=updated&type=all", org)
	} else {
		path = "/user/repos?per_page=20&sort=updated&affiliation=owner,organization_member"
	}
	var repos []githubRepo
	err := g.get(token, path, &repos)
	return repos, err
}

func (g *githubConnector) fetchIssues(token, repoFullName string) ([]githubIssue, error) {
	path := fmt.Sprintf("/repos/%s/issues?state=all&per_page=30&sort=updated", repoFullName)
	var issues []githubIssue
	err := g.get(token, path, &issues)
	return issues, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (g *githubConnector) normaliseIssues(repo string, issues []githubIssue) []NormalizedEvent {
	endpoint := "api.github.com/repos/" + repo + "/issues"
	var out []NormalizedEvent
	for _, issue := range issues {
		id := fmt.Sprintf("%s#%d", repo, issue.Number)
		if issue.Title != "" {
			out = append(out, NormalizedEvent{
				VendorID: "github", EventID: id, Source: "issue.title",
				Text:     issue.Title,
				Metadata: map[string]string{"endpoint": endpoint, "repo": repo},
			})
		}
		if issue.Body != "" {
			out = append(out, NormalizedEvent{
				VendorID: "github", EventID: id, Source: "issue.body",
				Text:     issue.Body,
				Metadata: map[string]string{"endpoint": endpoint, "repo": repo},
			})
		}
	}
	return out
}
