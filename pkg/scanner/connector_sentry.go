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
	Register(&sentryConnector{
		baseURL: "https://sentry.io",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

// ── Connector ─────────────────────────────────────────────────────────────────

type sentryConnector struct {
	baseURL string
	client  *http.Client
}

func (s *sentryConnector) Vendor() string { return "sentry" }

// FetchEvents lists all accessible projects then fetches up to 100 recent
// events per project (capped at 500 total).
func (s *sentryConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := s.token(vc)
	if token == "" {
		return nil, fmt.Errorf("sentry: no token/api_key found in credentials")
	}

	orgs, err := s.listOrganizations(token)
	if err != nil {
		return nil, fmt.Errorf("sentry: list organizations: %w", err)
	}
	if len(orgs) == 0 {
		return nil, fmt.Errorf("sentry: token has access to 0 organizations — check scopes (need org:read, project:read, event:read)")
	}

	const maxEvents = 500
	var all []NormalizedEvent

	for _, org := range orgs {
		projects, err := s.listProjects(token, org.Slug)
		if err != nil {
			log.Printf("sentry connector: list projects for org %q: %v", org.Slug, err)
			continue
		}

		for _, proj := range projects {
			if len(all) >= maxEvents {
				break
			}
			limit := 100
			if len(all)+limit > maxEvents {
				limit = maxEvents - len(all)
			}

			events, err := s.fetchEvents(token, org.Slug, proj.Slug, limit)
			if err != nil {
				log.Printf("sentry connector: fetch events for %s/%s: %v", org.Slug, proj.Slug, err)
				continue
			}
			all = append(all, s.normalise(org.Slug, proj.Slug, events)...)
		}
	}

	return all, nil
}

// ── Sentry API types ──────────────────────────────────────────────────────────

type sentryOrg struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type sentryProject struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type sentryUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	IPAddress string `json:"ip_address"`
	Username  string `json:"username"`
	Name      string `json:"name"`
}

type sentryTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type sentryBreadcrumb struct {
	Message  string                 `json:"message"`
	Category string                 `json:"category"`
	Data     map[string]interface{} `json:"data"`
}

type sentryBreadcrumbs struct {
	Values []sentryBreadcrumb `json:"values"`
}

type sentryException struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type sentryExceptionEntry struct {
	Values []sentryException `json:"values"`
}

type sentryEntry struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type sentryEvent struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	User        sentryUser             `json:"user"`
	Tags        []sentryTag            `json:"tags"`
	Extra       map[string]interface{} `json:"extra"`
	Breadcrumbs sentryBreadcrumbs      `json:"breadcrumbs"`
	Entries     []sentryEntry          `json:"entries"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (s *sentryConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP %d) — check token scopes", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (s *sentryConnector) listOrganizations(token string) ([]sentryOrg, error) {
	var orgs []sentryOrg
	err := s.get(token, "/api/0/organizations/?member=1", &orgs)
	return orgs, err
}

func (s *sentryConnector) listProjects(token, orgSlug string) ([]sentryProject, error) {
	var projects []sentryProject
	err := s.get(token, fmt.Sprintf("/api/0/organizations/%s/projects/", orgSlug), &projects)
	return projects, err
}

func (s *sentryConnector) fetchEvents(token, orgSlug, projectSlug string, limit int) ([]sentryEvent, error) {
	var events []sentryEvent
	path := fmt.Sprintf("/api/0/projects/%s/%s/events/?limit=%d&full=true", orgSlug, projectSlug, limit)
	err := s.get(token, path, &events)
	return events, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

// normalise converts raw Sentry events into NormalizedEvents, one per
// scannable text fragment so detection is maximally granular.
func (s *sentryConnector) normalise(orgSlug, projectSlug string, events []sentryEvent) []NormalizedEvent {
	endpoint := fmt.Sprintf("sentry.io/%s/%s", orgSlug, projectSlug)
	var out []NormalizedEvent

	emit := func(eventID, field, text string) {
		if text == "" {
			return
		}
		out = append(out, NormalizedEvent{
			VendorID: "sentry",
			EventID:  eventID,
			Source:   field,
			Text:     text,
			Metadata: map[string]string{
				"endpoint": endpoint,
				"org":      orgSlug,
				"project":  projectSlug,
			},
		})
	}

	for _, ev := range events {
		id := ev.ID

		// Core fields
		emit(id, "event.title", ev.Title)
		emit(id, "event.message", ev.Message)

		// User context
		emit(id, "event.user.email", ev.User.Email)
		emit(id, "event.user.ip_address", ev.User.IPAddress)
		emit(id, "event.user.username", ev.User.Username)
		emit(id, "event.user.name", ev.User.Name)

		// Tags
		for _, tag := range ev.Tags {
			emit(id, "event.tags."+tag.Key, tag.Value)
		}

		// Extra (arbitrary key-value blob)
		for k, v := range ev.Extra {
			if str, ok := stringify(v); ok {
				emit(id, "event.extra."+k, str)
			}
		}

		// Breadcrumbs
		for i, bc := range ev.Breadcrumbs.Values {
			src := fmt.Sprintf("event.breadcrumbs[%d]", i)
			emit(id, src+".message", bc.Message)
			for k, v := range bc.Data {
				if str, ok := stringify(v); ok {
					emit(id, src+".data."+k, str)
				}
			}
		}

		// Entries (exception values, message entries)
		for _, entry := range ev.Entries {
			switch entry.Type {
			case "exception":
				var exc sentryExceptionEntry
				if json.Unmarshal(entry.Data, &exc) == nil {
					for i, ex := range exc.Values {
						emit(id, fmt.Sprintf("event.exception[%d].value", i), ex.Value)
					}
				}
			case "message":
				var msg struct {
					Formatted string `json:"formatted"`
				}
				if json.Unmarshal(entry.Data, &msg) == nil {
					emit(id, "event.entry.message", msg.Formatted)
				}
			}
		}
	}

	return out
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (s *sentryConnector) token(vc *VendorConnection) string {
	for _, key := range []string{"token", "api_token", "api_key", "auth_token"} {
		if v, ok := vc.Credentials[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func stringify(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, t != ""
	case fmt.Stringer:
		return t.String(), true
	}
	return "", false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
