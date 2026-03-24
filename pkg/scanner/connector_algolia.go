package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func init() {
	Register(&algoliaConnector{client: &http.Client{Timeout: 30 * time.Second}})
}

type algoliaConnector struct{ client *http.Client }

func (a *algoliaConnector) Vendor() string { return "algolia" }

func (a *algoliaConnector) FetchEvents(vc *VendorConnection) ([]NormalizedEvent, error) {
	appID, _ := vc.Credentials["app_id"].(string)
	apiKey, _ := vc.Credentials["api_key"].(string)
	if appID == "" || apiKey == "" {
		return nil, fmt.Errorf("algolia: requires app_id and api_key")
	}

	indices, err := a.listIndices(appID, apiKey)
	if err != nil {
		return nil, fmt.Errorf("algolia: list indices: %w", err)
	}

	var all []NormalizedEvent
	const maxTotal = 500
	for _, idx := range indices {
		if len(all) >= maxTotal {
			break
		}
		records, err := a.browseIndex(appID, apiKey, idx.Name)
		if err != nil {
			log.Printf("algolia connector: browse index %q: %v", idx.Name, err)
			continue
		}
		all = append(all, a.normalise(idx.Name, records)...)
	}
	return all, nil
}

// ── Algolia API types ─────────────────────────────────────────────────────────

type algoliaIndex struct {
	Name    string `json:"name"`
	Entries int    `json:"entries"`
}

type algoliaIndicesResponse struct {
	Items []algoliaIndex `json:"items"`
}

type algoliaBrowseResponse struct {
	Hits []map[string]interface{} `json:"hits"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (a *algoliaConnector) dsn(appID string) string {
	return fmt.Sprintf("https://%s.algolia.net", appID)
}

func (a *algoliaConnector) get(appID, apiKey, path string, out interface{}) error {
	req, err := http.NewRequest("GET", a.dsn(appID)+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Algolia-Application-Id", appID)
	req.Header.Set("X-Algolia-API-Key", apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (HTTP 403)")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func (a *algoliaConnector) listIndices(appID, apiKey string) ([]algoliaIndex, error) {
	var result algoliaIndicesResponse
	err := a.get(appID, apiKey, "/1/indexes", &result)
	return result.Items, err
}

func (a *algoliaConnector) browseIndex(appID, apiKey, indexName string) ([]map[string]interface{}, error) {
	var result algoliaBrowseResponse
	path := fmt.Sprintf("/1/indexes/%s/browse?hitsPerPage=50", indexName)
	err := a.get(appID, apiKey, path, &result)
	return result.Hits, err
}

// ── Normalisation ─────────────────────────────────────────────────────────────

var algoliaPIIFields = map[string]bool{
	"email": true, "phone": true, "name": true, "address": true,
	"firstname": true, "lastname": true, "user_email": true,
}

func (a *algoliaConnector) normalise(indexName string, records []map[string]interface{}) []NormalizedEvent {
	endpoint := "algolia.net/1/indexes/" + indexName
	var out []NormalizedEvent
	for i, rec := range records {
		objectID, _ := rec["objectID"].(string)
		if objectID == "" {
			objectID = fmt.Sprintf("rec_%d", i)
		}
		for k, v := range rec {
			key := strings.ToLower(k)
			if !algoliaPIIFields[key] {
				continue
			}
			if s, ok := stringify(v); ok {
				out = append(out, NormalizedEvent{
					VendorID: "algolia",
					EventID:  objectID,
					Source:   "index." + indexName + "." + k,
					Text:     s,
					Metadata: map[string]string{"endpoint": endpoint},
				})
			}
		}
	}
	return out
}
