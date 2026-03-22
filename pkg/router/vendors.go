package router

// VendorProfile describes a third-party monitoring or analytics vendor whose
// outbound traffic Nonym can intercept and redact.
type VendorProfile struct {
	// ID is the canonical lowercase identifier used in audit records (e.g. "sentry").
	ID string `json:"id"`
	// Name is the human-readable display name.
	Name string `json:"name"`
	// Description explains what data this vendor receives and why it matters.
	Description string `json:"description"`
	// Category groups vendors (e.g. "error-tracking", "apm", "analytics").
	Category string `json:"category"`
	// KnownHosts lists the hostnames that this vendor's SDK sends data to.
	KnownHosts []string `json:"known_hosts"`
	// DataTypes lists the categories of data this vendor typically receives.
	DataTypes []string `json:"data_types"`
	// ComplianceFrameworks lists frameworks whose requirements are implicated
	// when this vendor receives unredacted data.
	ComplianceFrameworks []string `json:"compliance_frameworks"`
	// IntegrationMethods lists how Nonym can intercept this vendor's traffic.
	IntegrationMethods []string `json:"integration_methods"`
	// SDKPackages are the npm/pip/gem package names for the vendor SDK.
	SDKPackages []string `json:"sdk_packages"`
	// NonymSDK is the @nonym/<vendor> package name (empty if not yet published).
	NonymSDK string `json:"nonym_sdk,omitempty"`
}

// VendorCatalog is the authoritative registry of supported vendor profiles.
var VendorCatalog = []VendorProfile{
	{
		ID:          "sentry",
		Name:        "Sentry",
		Description: "Error tracking and performance monitoring. Sentry captures exception messages, stack traces, breadcrumbs, and full request/response bodies — all of which can contain PII such as names, emails, and authentication tokens.",
		Category:    "error-tracking",
		KnownHosts: []string{
			"sentry.io",
			"o0.ingest.sentry.io",
			"o1.ingest.sentry.io",
			"o2.ingest.sentry.io",
		},
		DataTypes: []string{
			"email", "ip_address", "user_id", "username",
			"request_body", "stack_trace", "breadcrumbs", "api_key",
		},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"sdk_wrapper", "proxy"},
		SDKPackages:          []string{"@sentry/node", "@sentry/browser", "@sentry/react", "sentry-sdk"},
		NonymSDK:             "@nonym/sentry",
	},
	{
		ID:          "datadog",
		Name:        "Datadog",
		Description: "APM, infrastructure monitoring, and log management. Datadog log pipelines and APM traces routinely include user-identifying information embedded in log messages, HTTP headers, and database query parameters.",
		Category:    "apm",
		KnownHosts: []string{
			"datadoghq.com",
			"api.datadoghq.com",
			"logs.datadoghq.com",
			"trace.agent.datadoghq.com",
		},
		DataTypes: []string{
			"email", "ip_address", "user_id", "log_message",
			"http_headers", "query_params", "request_body",
		},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"proxy", "agent_config"},
		SDKPackages:          []string{"dd-trace", "datadog-lambda-js", "datadog-metrics"},
		NonymSDK:             "",
	},
	{
		ID:          "posthog",
		Name:        "PostHog",
		Description: "Product analytics and session recording. PostHog event properties and person profiles accumulate email addresses, names, and device fingerprints that must be stripped to comply with GDPR right-to-erasure requirements.",
		Category:    "analytics",
		KnownHosts: []string{
			"app.posthog.com",
			"eu.posthog.com",
			"us.posthog.com",
		},
		DataTypes: []string{
			"email", "ip_address", "user_id", "device_id",
			"session_recording", "event_properties", "person_properties",
		},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"sdk_wrapper", "proxy"},
		SDKPackages:          []string{"posthog-js", "posthog-node", "posthog-python"},
		NonymSDK:             "",
	},
	{
		ID:          "mixpanel",
		Name:        "Mixpanel",
		Description: "User analytics platform. Mixpanel event tracking captures user properties and behavioral data that often includes directly identifying information.",
		Category:    "analytics",
		KnownHosts: []string{
			"api.mixpanel.com",
			"api-eu.mixpanel.com",
		},
		DataTypes: []string{
			"email", "ip_address", "user_id", "event_properties",
		},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"mixpanel", "mixpanel-browser"},
		NonymSDK:             "",
	},
	{
		ID:          "newrelic",
		Name:        "New Relic",
		Description: "Observability platform covering APM, logs, and infrastructure. New Relic distributed tracing and log forwarding can expose PII embedded in application logs and HTTP payloads.",
		Category:    "apm",
		KnownHosts: []string{
			"collector.newrelic.com",
			"log-api.newrelic.com",
			"metric-api.newrelic.com",
			"trace-api.newrelic.com",
		},
		DataTypes: []string{
			"email", "ip_address", "user_id", "log_message",
			"http_headers", "request_body",
		},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"proxy", "agent_config"},
		SDKPackages:          []string{"newrelic", "@newrelic/apollo-server-plugin"},
		NonymSDK:             "",
	},
}

// catalogByID indexes VendorCatalog by vendor ID for fast lookup.
var catalogByID = func() map[string]*VendorProfile {
	m := make(map[string]*VendorProfile, len(VendorCatalog))
	for i := range VendorCatalog {
		m[VendorCatalog[i].ID] = &VendorCatalog[i]
	}
	return m
}()

// GetVendorProfile returns the VendorProfile for a given vendor ID, or nil if
// the vendor is not in the catalog.
func GetVendorProfile(id string) *VendorProfile {
	return catalogByID[id]
}

// DetectVendorFromHost returns the vendor ID for a given hostname, or an empty
// string if no vendor in the catalog matches.
func DetectVendorFromHost(host string) string {
	for i := range VendorCatalog {
		for _, known := range VendorCatalog[i].KnownHosts {
			if host == known || len(host) > len(known) && host[len(host)-len(known)-1] == '.' && host[len(host)-len(known):] == known {
				return VendorCatalog[i].ID
			}
		}
	}
	return ""
}
