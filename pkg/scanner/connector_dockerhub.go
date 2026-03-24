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
	Register(&dockerhubConnector{
		baseURL: "https://hub.docker.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type dockerhubConnector struct {
	baseURL string
	client  *http.Client
}

func (d *dockerhubConnector) Vendor() string { return "dockerhub" }

// TestConnection authenticates and fetches the user profile.
func (d *dockerhubConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	username := credStr(vc, "username")
	pat := credStr(vc, "pat", "api_key")
	if username == "" || pat == "" {
		return ConnectionResult{Success: false, Message: "Docker Hub requires username and pat (personal access token)"}
	}
	if !strings.HasPrefix(pat, "dckr_pat_") {
		return ConnectionResult{Success: false, Message: "Docker Hub personal access token must start with dckr_pat_"}
	}
	token, err := d.login(username, pat)
	if err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Docker Hub login failed: %v", err)}
	}
	if err := d.get(token, fmt.Sprintf("/v2/users/%s/", username), new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Docker Hub connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "Docker Hub credentials validated — account accessible"}
}

// FetchEvents lists repositories and scans descriptions and labels for PII.
func (d *dockerhubConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	username := credStr(vc, "username")
	pat := credStr(vc, "pat", "api_key")
	if username == "" || pat == "" {
		return nil, fmt.Errorf("dockerhub: username and pat are required")
	}
	token, err := d.login(username, pat)
	if err != nil {
		return nil, fmt.Errorf("dockerhub: login: %w", err)
	}
	repos, err := d.listRepos(token, username)
	if err != nil {
		return nil, fmt.Errorf("dockerhub: list repos: %w", err)
	}
	return d.normalise(repos), nil
}

// ── Docker Hub API types ──────────────────────────────────────────────────────

type dockerhubLoginResponse struct {
	Token string `json:"token"`
}

type dockerhubRepo struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
	FullDescription string `json:"full_description"`
}

type dockerhubReposResponse struct {
	Results []dockerhubRepo `json:"results"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (d *dockerhubConnector) login(username, pat string) (string, error) {
	payload := strings.NewReader(
		fmt.Sprintf(`{"username":%q,"password":%q}`, username, pat),
	)
	req, err := http.NewRequest("POST", d.baseURL+"/v2/users/login", payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "", fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var result dockerhubLoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Token, nil
}

func (d *dockerhubConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", d.baseURL+path, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := d.client.Do(req)
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

func (d *dockerhubConnector) listRepos(token, username string) ([]dockerhubRepo, error) {
	var result dockerhubReposResponse
	err := d.get(token, fmt.Sprintf("/v2/repositories/%s/?page_size=50&ordering=last_updated", username), &result)
	return result.Results, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (d *dockerhubConnector) normalise(repos []dockerhubRepo) []NormalizedEvent {
	endpoint := "hub.docker.com/v2/repositories"
	var out []NormalizedEvent
	for _, r := range repos {
		id := r.Namespace + "/" + r.Name
		if r.Description != "" {
			out = append(out, NormalizedEvent{
				VendorID: "dockerhub", EventID: id, Source: "repo.description",
				Text:     r.Description,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
		if r.FullDescription != "" {
			out = append(out, NormalizedEvent{
				VendorID: "dockerhub", EventID: id, Source: "repo.full_description",
				Text:     r.FullDescription,
				Metadata: map[string]string{"endpoint": endpoint},
			})
		}
	}
	return out
}
