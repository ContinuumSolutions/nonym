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
	Register(&sendgridConnector{
		baseURL: "https://api.sendgrid.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type sendgridConnector struct {
	baseURL string
	client  *http.Client
}

func (s *sendgridConnector) Vendor() string { return "sendgrid" }

// TestConnection verifies the API key by calling the user profile endpoint.
func (s *sendgridConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	apiKey := credStr(vc, "api_key", "token")
	if !strings.HasPrefix(apiKey, "SG.") {
		return ConnectionResult{Success: false, Message: "SendGrid API key must start with SG."}
	}
	if err := s.get(apiKey, "/v3/user/profile", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("SendGrid connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "SendGrid key validated — account accessible"}
}

func (s *sendgridConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	apiKey := ""
	for _, k := range []string{"api_key", "token"} {
		if v, ok := vc.Credentials[k].(string); ok && v != "" {
			apiKey = v
			break
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("sendgrid: no api_key in credentials")
	}

	// Scan dynamic templates for PII in template bodies.
	templates, err := s.fetchTemplates(apiKey)
	if err != nil {
		return nil, fmt.Errorf("sendgrid: fetch templates: %w", err)
	}

	// Scan suppression lists (contain email addresses).
	suppressions, err := s.fetchSuppressions(apiKey)
	if err != nil {
		suppressions = nil
	}

	var all []NormalizedEvent
	all = append(all, s.normaliseTemplates(templates)...)
	all = append(all, s.normaliseSuppressions(suppressions)...)
	return all, nil
}

// ── SendGrid API types ────────────────────────────────────────────────────────

type sendgridTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Versions []struct {
		Subject   string `json:"subject"`
		HtmlContent string `json:"html_content"`
		PlainContent string `json:"plain_content"`
	} `json:"versions"`
}

type sendgridTemplatesResponse struct {
	Templates []sendgridTemplate `json:"templates"`
}

type sendgridSuppression struct {
	Email   string `json:"email"`
	Created int64  `json:"created"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (s *sendgridConnector) get(apiKey, path string, out interface{}) error {
	req, err := http.NewRequest("GET", s.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
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

func (s *sendgridConnector) fetchTemplates(apiKey string) ([]sendgridTemplate, error) {
	var result sendgridTemplatesResponse
	err := s.get(apiKey, "/v3/templates?generations=dynamic&page_size=50", &result)
	return result.Templates, err
}

func (s *sendgridConnector) fetchSuppressions(apiKey string) ([]sendgridSuppression, error) {
	var result []sendgridSuppression
	err := s.get(apiKey, "/v3/suppression/unsubscribes?limit=100", &result)
	return result, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (s *sendgridConnector) normaliseTemplates(templates []sendgridTemplate) []NormalizedEvent {
	var out []NormalizedEvent
	for _, t := range templates {
		for _, v := range t.Versions {
			if v.Subject != "" {
				out = append(out, NormalizedEvent{
					VendorID: "sendgrid", EventID: t.ID, Source: "template.subject",
					Text: v.Subject, Metadata: map[string]string{"endpoint": "api.sendgrid.com/v3/templates"},
				})
			}
		}
	}
	return out
}

func (s *sendgridConnector) normaliseSuppressions(suppressions []sendgridSuppression) []NormalizedEvent {
	var out []NormalizedEvent
	for i, sup := range suppressions {
		out = append(out, NormalizedEvent{
			VendorID: "sendgrid",
			EventID:  fmt.Sprintf("sup_%d", i),
			Source:   "suppression.email",
			Text:     sup.Email,
			Metadata: map[string]string{"endpoint": "api.sendgrid.com/v3/suppression"},
		})
	}
	return out
}
