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
	Register(&bugsnagConnector{
		baseURL: "https://api.bugsnag.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type bugsnagConnector struct {
	baseURL string
	client  *http.Client
}

func (b *bugsnagConnector) Vendor() string { return "bugsnag" }

// TestConnection verifies the auth token by listing organizations.
func (b *bugsnagConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "auth_token", "api_key", "token")
	if len(token) < 8 {
		return ConnectionResult{Success: false, Message: "Bugsnag auth token is missing or too short"}
	}
	var orgs []struct {
		ID string `json:"id"`
	}
	if err := b.get(token, "/user/organizations", &orgs); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("Bugsnag connection failed: %v", err)}
	}
	n := len(orgs)
	return ConnectionResult{
		Success:          true,
		Message:          fmt.Sprintf("Bugsnag token validated — %d organization(s) accessible", n),
		EventsAccessible: &n,
	}
}

func (b *bugsnagConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "auth_token", "api_key", "token")
	if token == "" {
		return nil, fmt.Errorf("bugsnag: no auth_token in credentials")
	}

	orgName, _ := vc.Credentials["organization_name"].(string)

	projects, err := b.listProjects(token, orgName)
	if err != nil {
		return nil, fmt.Errorf("bugsnag: list projects: %w", err)
	}

	var all []NormalizedEvent
	const maxTotal = 500
	for _, proj := range projects {
		if len(all) >= maxTotal {
			break
		}
		errors, err := b.fetchErrors(token, proj.ID)
		if err != nil {
			log.Printf("bugsnag connector: fetch errors for project %s: %v", proj.ID, err)
			continue
		}
		all = append(all, b.normalise(proj.ID, errors)...)
	}
	return all, nil
}

// ── Bugsnag API types ─────────────────────────────────────────────────────────

type bugsnagProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type bugsnagError struct {
	ID          string `json:"id"`
	ErrorClass  string `json:"error_class"`
	Message     string `json:"message"`
	Context     string `json:"context"`
	Severity    string `json:"severity"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (b *bugsnagConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", b.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("X-Version", "2")

	resp, err := b.client.Do(req)
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

func (b *bugsnagConnector) listProjects(token, orgName string) ([]bugsnagProject, error) {
	// First get the organization list.
	var orgs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := b.get(token, "/user/organizations", &orgs); err != nil {
		return nil, err
	}

	var projects []bugsnagProject
	for _, org := range orgs {
		if orgName != "" && org.Name != orgName {
			continue
		}
		var projs []bugsnagProject
		if err := b.get(token, "/organizations/"+org.ID+"/projects?per_page=50", &projs); err != nil {
			log.Printf("bugsnag: list projects for org %s: %v", org.ID, err)
			continue
		}
		projects = append(projects, projs...)
	}
	return projects, nil
}

func (b *bugsnagConnector) fetchErrors(token, projectID string) ([]bugsnagError, error) {
	var errs []bugsnagError
	err := b.get(token, "/projects/"+projectID+"/errors?sort=last_seen&direction=desc&per_page=100", &errs)
	return errs, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (b *bugsnagConnector) normalise(projectID string, errors []bugsnagError) []NormalizedEvent {
	var out []NormalizedEvent
	endpoint := "api.bugsnag.com/projects/" + projectID

	emit := func(id, field, text string) {
		if text == "" {
			return
		}
		out = append(out, NormalizedEvent{
			VendorID: "bugsnag",
			EventID:  id,
			Source:   field,
			Text:     text,
			Metadata: map[string]string{"endpoint": endpoint, "project_id": projectID},
		})
	}

	for _, e := range errors {
		emit(e.ID, "error.message", e.Message)
		emit(e.ID, "error.context", e.Context)
	}
	return out
}
