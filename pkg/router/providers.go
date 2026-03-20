package router

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Provider represents an AI service provider
type Provider struct {
	Name        string            `json:"name"`
	BaseURL     string            `json:"base_url"`
	Enabled     bool              `json:"enabled"`
	HealthCheck string            `json:"health_check"`
	Priority    int               `json:"priority"`
	Headers     map[string]string `json:"headers"`
	LastCheck   time.Time         `json:"last_check"`
	Healthy     bool              `json:"healthy"`
}

// ProviderConfig holds provider configuration
type ProviderConfig struct {
	BaseURL string `yaml:"base_url"`
	Enabled bool   `yaml:"enabled"`
}

// RoutingRule defines how to route requests
type RoutingRule struct {
	PathPattern    string   `json:"path_pattern"`
	Provider       string   `json:"provider"`
	Conditions     []string `json:"conditions"`
	Priority       int      `json:"priority"`
	SecurityLevel  string   `json:"security_level"`
}

// Router handles provider selection and routing logic
type Router struct {
	providers map[string]*Provider
	rules     []RoutingRule
	mutex     sync.RWMutex
}

var (
	globalRouter *Router
	initOnce     sync.Once
)

// Reset clears router state so Initialize can be called again. For use in tests only.
func Reset() {
	globalRouter = nil
	initOnce = sync.Once{}
}

// Initialize sets up the router with provider configurations
func Initialize(configs map[string]ProviderConfig) error {
	var err error
	initOnce.Do(func() {
		globalRouter = &Router{
			providers: make(map[string]*Provider),
			rules:     []RoutingRule{},
		}
		err = globalRouter.loadProviders(configs)
		globalRouter.setupDefaultRules()
	})
	return err
}

// DetermineProvider selects the appropriate provider for a request
func DetermineProvider(path string, headers map[string]string) (string, *url.URL, error) {
	if globalRouter == nil {
		return "", nil, fmt.Errorf("router not initialized")
	}

	return globalRouter.determineProvider(path, headers)
}

// GetProviderStatus returns the status of all providers
func GetProviderStatus() map[string]interface{} {
	if globalRouter == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	globalRouter.mutex.RLock()
	defer globalRouter.mutex.RUnlock()

	status := make(map[string]interface{})
	for name, provider := range globalRouter.providers {
		status[name] = map[string]interface{}{
			"enabled":     provider.Enabled,
			"healthy":     provider.Healthy,
			"last_check":  provider.LastCheck,
			"priority":    provider.Priority,
		}
	}

	return status
}

func (r *Router) loadProviders(configs map[string]ProviderConfig) error {
	// SPG Configuration
	if config, exists := configs["spg"]; exists && config.Enabled {
		r.providers["spg"] = &Provider{
			Name:        "spg",
			BaseURL:     config.BaseURL,
			Enabled:     true,
			HealthCheck: "/gateway/status",
			Priority:    0, // Highest priority for SPG
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Healthy: true,
		}
	}

	// OpenAI Configuration
	if config, exists := configs["openai"]; exists && config.Enabled {
		r.providers["openai"] = &Provider{
			Name:        "openai",
			BaseURL:     config.BaseURL,
			Enabled:     true,
			HealthCheck: "/v1/models",
			Priority:    1,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Healthy: true,
		}
	}

	// Anthropic Configuration
	if config, exists := configs["anthropic"]; exists && config.Enabled {
		r.providers["anthropic"] = &Provider{
			Name:        "anthropic",
			BaseURL:     config.BaseURL,
			Enabled:     true,
			HealthCheck: "/v1/messages",
			Priority:    2,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Healthy: true,
		}
	}

	// Google/Gemini Configuration
	if config, exists := configs["google"]; exists && config.Enabled {
		r.providers["google"] = &Provider{
			Name:        "google",
			BaseURL:     config.BaseURL,
			Enabled:     true,
			HealthCheck: "/v1/models",
			Priority:    3,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Healthy: true,
		}
	}

	// Local LLM Configuration (Optional)
	if config, exists := configs["local"]; exists && config.Enabled {
		r.providers["local"] = &Provider{
			Name:        "local",
			BaseURL:     config.BaseURL,
			Enabled:     true,
			HealthCheck: "/api/tags",
			Priority:    4,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Healthy: true,
		}
	}

	return nil
}

func (r *Router) setupDefaultRules() {
	r.rules = []RoutingRule{
		// SPG Instance: Route all requests through SPG if available
		{
			PathPattern:   "/v1/",
			Provider:      "spg",
			Conditions:    []string{},
			Priority:      0,
			SecurityLevel: "highest",
		},
		// High security: Financial data goes to local LLM
		{
			PathPattern:   "/v1/chat/completions",
			Provider:      "local",
			Conditions:    []string{"contains:financial", "contains:payment", "contains:credit"},
			Priority:      1,
			SecurityLevel: "high",
		},
		// Medium security: Personal data to trusted providers
		{
			PathPattern:   "/v1/chat/completions",
			Provider:      "anthropic",
			Conditions:    []string{"contains:personal", "contains:healthcare"},
			Priority:      2,
			SecurityLevel: "medium",
		},
		// Standard routing: OpenAI for general queries
		{
			PathPattern:   "/v1/chat/completions",
			Provider:      "openai",
			Conditions:    []string{},
			Priority:      3,
			SecurityLevel: "standard",
		},
		// Embeddings: Google for efficiency
		{
			PathPattern:   "/v1/embeddings",
			Provider:      "google",
			Conditions:    []string{},
			Priority:      1,
			SecurityLevel: "standard",
		},
		// Anthropic specific endpoints
		{
			PathPattern:   "/v1/messages",
			Provider:      "anthropic",
			Conditions:    []string{},
			Priority:      1,
			SecurityLevel: "standard",
		},
		// Local LLM endpoints
		{
			PathPattern:   "/api/",
			Provider:      "local",
			Conditions:    []string{},
			Priority:      1,
			SecurityLevel: "standard",
		},
	}
}

func (r *Router) determineProvider(path string, headers map[string]string) (string, *url.URL, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Extract content from headers for content-based routing
	contentHint := extractContentHint(headers)

	// Find matching rule
	var selectedRule *RoutingRule
	for _, rule := range r.rules {
		if r.matchesRule(rule, path, contentHint) {
			// Check if provider is available
			if provider, exists := r.providers[rule.Provider]; exists && provider.Enabled && provider.Healthy {
				selectedRule = &rule
				break
			}
		}
	}

	if selectedRule == nil {
		return "", nil, fmt.Errorf("no suitable provider found for path: %s", path)
	}

	// Get provider
	provider := r.providers[selectedRule.Provider]

	// Construct target URL
	targetURL, err := url.Parse(provider.BaseURL + path)
	if err != nil {
		return "", nil, fmt.Errorf("invalid provider URL: %w", err)
	}

	return provider.Name, targetURL, nil
}

func (r *Router) matchesRule(rule RoutingRule, path string, contentHint string) bool {
	// Check path pattern
	if !strings.Contains(path, strings.TrimPrefix(rule.PathPattern, "/")) {
		return false
	}

	// Check conditions
	if len(rule.Conditions) == 0 {
		return true // No conditions means it matches
	}

	for _, condition := range rule.Conditions {
		if strings.HasPrefix(condition, "contains:") {
			keyword := strings.TrimPrefix(condition, "contains:")
			if strings.Contains(strings.ToLower(contentHint), strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

func extractContentHint(headers map[string]string) string {
	// Look for content hints in headers or other metadata
	// This is a simplified version - in practice, you might parse request body
	var hints []string

	for key, value := range headers {
		key = strings.ToLower(key)
		if strings.Contains(key, "content") || strings.Contains(key, "hint") {
			hints = append(hints, value)
		}
	}

	return strings.Join(hints, " ")
}

// HealthCheck performs health checks on all providers
func HealthCheck() {
	if globalRouter == nil {
		return
	}

	globalRouter.mutex.Lock()
	defer globalRouter.mutex.Unlock()

	for name, provider := range globalRouter.providers {
		healthy := performHealthCheck(provider)
		provider.Healthy = healthy
		provider.LastCheck = time.Now()

		if !healthy {
			fmt.Printf("Provider %s is unhealthy\n", name)
		}
	}
}

func performHealthCheck(provider *Provider) bool {
	// Simple HTTP GET to health check endpoint
	// In a real implementation, this would make an actual HTTP request
	// For now, we'll simulate it
	return provider.Enabled // Simplified check
}

// UpdateProviderStatus manually updates a provider's status
func UpdateProviderStatus(name string, enabled bool) error {
	if globalRouter == nil {
		return fmt.Errorf("router not initialized")
	}

	globalRouter.mutex.Lock()
	defer globalRouter.mutex.Unlock()

	provider, exists := globalRouter.providers[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	provider.Enabled = enabled
	return nil
}

// AddRoutingRule adds a new routing rule
func AddRoutingRule(rule RoutingRule) error {
	if globalRouter == nil {
		return fmt.Errorf("router not initialized")
	}

	globalRouter.mutex.Lock()
	defer globalRouter.mutex.Unlock()

	globalRouter.rules = append(globalRouter.rules, rule)
	return nil
}

// GetRoutingRules returns all current routing rules
func GetRoutingRules() []RoutingRule {
	if globalRouter == nil {
		return nil
	}

	globalRouter.mutex.RLock()
	defer globalRouter.mutex.RUnlock()

	return append([]RoutingRule(nil), globalRouter.rules...)
}