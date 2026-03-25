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
	Register(&digitaloceanConnector{
		baseURL: "https://api.digitalocean.com",
		client:  &http.Client{Timeout: 30 * time.Second},
	})
}

type digitaloceanConnector struct {
	baseURL string
	client  *http.Client
}

func (d *digitaloceanConnector) Vendor() string { return "digitalocean" }

// TestConnection verifies the API token via the /v2/account endpoint.
func (d *digitaloceanConnector) TestConnection(vc *VendorConnection) ConnectionResult {
	token := credStr(vc, "api_token", "token")
	if !strings.HasPrefix(token, "dop_v1_") {
		return ConnectionResult{Success: false, Message: "DigitalOcean personal access token must start with dop_v1_"}
	}
	if err := d.get(token, "/v2/account", new(struct{})); err != nil {
		return ConnectionResult{Success: false, Message: fmt.Sprintf("DigitalOcean connection failed: %v", err)}
	}
	return ConnectionResult{Success: true, Message: "DigitalOcean token validated — account accessible"}
}

// FetchEvents scans App Platform app specs for exposed environment variables.
func (d *digitaloceanConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	token := credStr(vc, "api_token", "token")
	if token == "" {
		return nil, fmt.Errorf("digitalocean: api_token is required")
	}

	apps, err := d.listApps(token)
	if err != nil {
		return nil, fmt.Errorf("digitalocean: list apps: %w", err)
	}

	var all []NormalizedEvent
	all = append(all, d.normaliseApps(apps)...)

	spaces, err := d.listSpaces(token)
	if err == nil {
		all = append(all, d.normaliseSpaces(spaces)...)
	}

	return all, nil
}

// ── DigitalOcean API types ────────────────────────────────────────────────────

type doApp struct {
	ID   string `json:"id"`
	Spec struct {
		Name    string `json:"name"`
		Envs    []doEnvVar `json:"envs"`
		Services []struct {
			Name string     `json:"name"`
			Envs []doEnvVar `json:"envs"`
		} `json:"services"`
	} `json:"spec"`
}

type doEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"` // "GENERAL" | "SECRET"
}

type doAppsResponse struct {
	Apps []doApp `json:"apps"`
}

type doSpace struct {
	Name   string `json:"Name"`
	Region string `json:"region"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (d *digitaloceanConnector) get(token, path string, out interface{}) error {
	req, err := http.NewRequest("GET", d.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

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

func (d *digitaloceanConnector) listApps(token string) ([]doApp, error) {
	var result doAppsResponse
	err := d.get(token, "/v2/apps?page=1&per_page=50", &result)
	return result.Apps, err
}

func (d *digitaloceanConnector) listSpaces(token string) ([]doSpace, error) {
	// Spaces uses the DO Spaces API (S3-compatible); metadata is available via DO API.
	var result struct {
		Buckets []doSpace `json:"buckets"`
	}
	err := d.get(token, "/v2/spaces", &result)
	return result.Buckets, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

func (d *digitaloceanConnector) normaliseApps(apps []doApp) []NormalizedEvent {
	endpoint := "api.digitalocean.com/v2/apps"
	var out []NormalizedEvent
	for _, app := range apps {
		// Scan top-level env vars.
		for _, env := range app.Spec.Envs {
			if env.Value != "" {
				out = append(out, NormalizedEvent{
					VendorID: "digitalocean", EventID: app.ID,
					Source: "app.env." + env.Key,
					Text:   env.Value,
					Metadata: map[string]string{"endpoint": endpoint, "app": app.Spec.Name},
				})
			}
		}
		// Scan service-level env vars.
		for _, svc := range app.Spec.Services {
			for _, env := range svc.Envs {
				if env.Value != "" {
					out = append(out, NormalizedEvent{
						VendorID: "digitalocean", EventID: app.ID,
						Source: fmt.Sprintf("app.service.%s.env.%s", svc.Name, env.Key),
						Text:   env.Value,
						Metadata: map[string]string{"endpoint": endpoint, "app": app.Spec.Name},
					})
				}
			}
		}
	}
	return out
}

func (d *digitaloceanConnector) normaliseSpaces(spaces []doSpace) []NormalizedEvent {
	endpoint := "api.digitalocean.com/v2/spaces"
	var out []NormalizedEvent
	for _, s := range spaces {
		// Record the space name and region as an event — the PII detector will
		// flag if the bucket name itself looks like a customer identifier.
		out = append(out, NormalizedEvent{
			VendorID: "digitalocean", EventID: "space:" + s.Name,
			Source: "spaces.bucket.name",
			Text:   s.Name,
			Metadata: map[string]string{"endpoint": endpoint, "region": s.Region},
		})
	}
	return out
}
