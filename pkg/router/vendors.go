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
	// ── Error Tracking (additional) ────────────────────────────────────────────
	{
		ID:          "rollbar",
		Name:        "Rollbar",
		Description: "Error tracking and crash reporting. Captures full request context, query params, POST body, and person fields containing email addresses and user data.",
		Category:    "error-tracking",
		KnownHosts:  []string{"api.rollbar.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "request_body", "query_params"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"sdk_wrapper", "proxy"},
		SDKPackages:          []string{"rollbar", "rollbar-react"},
	},
	{
		ID:          "bugsnag",
		Name:        "Bugsnag",
		Description: "Crash monitoring platform. metaData objects, user fields, and breadcrumbs capture PII embedded in application state at the time of the error.",
		Category:    "error-tracking",
		KnownHosts:  []string{"notify.bugsnag.com", "sessions.bugsnag.com", "api.bugsnag.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "request_body"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"sdk_wrapper", "proxy"},
		SDKPackages:          []string{"@bugsnag/js", "bugsnag"},
	},
	// ── Analytics (additional) ─────────────────────────────────────────────────
	{
		ID:          "segment",
		Name:        "Segment",
		Description: "Customer Data Platform (CDP). The highest-leverage PII intercept point — identify() calls fan out to every connected destination with email, name, and phone.",
		Category:    "analytics",
		KnownHosts:  []string{"api.segment.io", "cdn.segment.com", "cdn.segment.io"},
		DataTypes:   []string{"email", "ip_address", "user_id", "name", "phone", "event_properties"},
		ComplianceFrameworks: []string{"GDPR", "CCPA", "SOC2"},
		IntegrationMethods:   []string{"sdk_wrapper", "proxy"},
		SDKPackages:          []string{"@segment/analytics-next", "analytics-node", "analytics-python"},
	},
	{
		ID:          "amplitude",
		Name:        "Amplitude",
		Description: "Product analytics. Autocapture can capture form field values and user properties including emails and names.",
		Category:    "analytics",
		KnownHosts:  []string{"api2.amplitude.com", "api.eu.amplitude.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "event_properties", "user_properties"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@amplitude/analytics-browser", "@amplitude/analytics-node"},
	},
	{
		ID:          "heap",
		Name:        "Heap",
		Description: "Product analytics with autocapture. Records every user interaction including form inputs without requiring code — highest risk of accidental PII capture.",
		Category:    "analytics",
		KnownHosts:  []string{"heapanalytics.com", "cdn.heapanalytics.com"},
		DataTypes:   []string{"email", "user_id", "event_properties", "form_inputs"},
		ComplianceFrameworks: []string{"GDPR", "CCPA"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"heap"},
	},
	{
		ID:          "google-analytics",
		Name:        "Google Analytics 4",
		Description: "Web analytics. Client-side JS tag; config misconfigurations silently send PII (names, emails) as custom dimensions to Google.",
		Category:    "analytics",
		KnownHosts:  []string{"analytics.google.com", "www.googletagmanager.com", "www.google-analytics.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "event_properties"},
		ComplianceFrameworks: []string{"GDPR", "CCPA"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"gtag.js", "@analytics/google-analytics"},
	},
	{
		ID:          "hotjar",
		Name:        "Hotjar",
		Description: "Session recording and heatmaps. Runs client-side — misconfigured masking is the #1 compliance risk for PII capture.",
		Category:    "analytics",
		KnownHosts:  []string{"in.hotjar.com", "static.hotjar.com", "insights.hotjar.com"},
		DataTypes:   []string{"session_recording", "form_inputs", "ip_address"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"hotjar"},
	},
	// ── Customer Engagement ────────────────────────────────────────────────────
	{
		ID:          "intercom",
		Name:        "Intercom",
		Description: "Customer messaging and support. Stores full conversation transcripts, user files, and custom attributes often containing PHI and financial info.",
		Category:    "customer-support",
		KnownHosts:  []string{"api.intercom.io", "api-iam.intercom.io"},
		DataTypes:   []string{"email", "name", "phone", "user_id", "conversation_content"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"proxy", "sdk_wrapper"},
		SDKPackages:          []string{"@intercom/messenger-js-sdk", "intercom-client"},
	},
	{
		ID:          "zendesk",
		Name:        "Zendesk",
		Description: "Customer support platform. Ticket bodies, user profiles, attachments, and custom fields routinely contain PHI, payment information, and personal context.",
		Category:    "customer-support",
		KnownHosts:  []string{"*.zendesk.com"},
		DataTypes:   []string{"email", "name", "phone", "address", "user_id"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"node-zendesk", "zendesk"},
	},
	{
		ID:          "hubspot",
		Name:        "HubSpot",
		Description: "CRM and marketing automation. Contact records, form submissions, and email campaigns contain email addresses, phone numbers, and company details.",
		Category:    "crm",
		KnownHosts:  []string{"api.hubapi.com", "js.hs-scripts.com", "js.hsforms.net"},
		DataTypes:   []string{"email", "name", "phone", "address", "user_id"},
		ComplianceFrameworks: []string{"GDPR", "CCPA", "SOC2"},
		IntegrationMethods:   []string{"proxy", "sdk_wrapper"},
		SDKPackages:          []string{"@hubspot/api-client"},
	},
	{
		ID:          "salesforce",
		Name:        "Salesforce",
		Description: "Enterprise CRM. Contact, Lead, Account, and Opportunity objects hold the most sensitive customer business data. Misconfigured profiles expose PII to all users.",
		Category:    "crm",
		KnownHosts:  []string{"*.salesforce.com", "*.force.com"},
		DataTypes:   []string{"email", "name", "phone", "address", "financial", "user_id"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"jsforce", "nforce"},
	},
	{
		ID:          "customer-io",
		Name:        "Customer.io",
		Description: "Marketing automation. Stores customer profiles, event streams, and personalized message content with email addresses and behavioral data.",
		Category:    "marketing",
		KnownHosts:  []string{"track.customer.io", "api.customer.io"},
		DataTypes:   []string{"email", "name", "phone", "event_properties"},
		ComplianceFrameworks: []string{"GDPR", "CAN-SPAM"},
		IntegrationMethods:   []string{"proxy", "sdk_wrapper"},
		SDKPackages:          []string{"customerio-node", "customerio-rails"},
	},
	// ── Payments ───────────────────────────────────────────────────────────────
	{
		ID:          "stripe",
		Name:        "Stripe",
		Description: "Payment processing. Stripe handles card data via client-side tokenization — do not proxy raw card data. Config misconfigurations are the primary risk.",
		Category:    "payments",
		KnownHosts:  []string{"api.stripe.com", "js.stripe.com"},
		DataTypes:   []string{"financial", "email", "name", "address"},
		ComplianceFrameworks: []string{"PCI-DSS", "GDPR"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"stripe", "@stripe/stripe-js"},
	},
	// ── Communication ─────────────────────────────────────────────────────────
	{
		ID:          "twilio",
		Name:        "Twilio",
		Description: "SMS, voice, and email platform. Message bodies contain OTPs, PII in notification templates, and call recordings with sensitive conversations.",
		Category:    "communication",
		KnownHosts:  []string{"api.twilio.com", "insights.twilio.com"},
		DataTypes:   []string{"phone", "email", "name", "sms_content"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"twilio", "twilio-node"},
	},
	{
		ID:          "sendgrid",
		Name:        "SendGrid",
		Description: "Transactional email service. Email sends contain recipient addresses and dynamic template data that may include personal information and order details.",
		Category:    "email",
		KnownHosts:  []string{"api.sendgrid.com", "smtp.sendgrid.net"},
		DataTypes:   []string{"email", "name", "address"},
		ComplianceFrameworks: []string{"GDPR", "CAN-SPAM", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@sendgrid/mail"},
	},
	{
		ID:          "mailgun",
		Name:        "Mailgun",
		Description: "Email API service. Message logs contain recipient addresses, stored email bodies, and route forwarding rules that can expose PII to unintended destinations.",
		Category:    "email",
		KnownHosts:  []string{"api.mailgun.net", "api.eu.mailgun.net"},
		DataTypes:   []string{"email", "name", "message_content"},
		ComplianceFrameworks: []string{"GDPR", "CAN-SPAM"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"mailgun-js", "mailgun.js"},
	},
	{
		ID:          "mailchimp",
		Name:        "Mailchimp",
		Description: "Email marketing platform. Audience segments and contact tags may store sensitive behavioral or health-related classifications alongside PII.",
		Category:    "marketing",
		KnownHosts:  []string{"api.mailchimp.com", "*.api.mailchimp.com"},
		DataTypes:   []string{"email", "name", "address", "phone"},
		ComplianceFrameworks: []string{"GDPR", "CAN-SPAM"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@mailchimp/mailchimp_marketing"},
	},
	{
		ID:          "slack",
		Name:        "Slack (Webhooks/Bots)",
		Description: "Team communication. Webhook notifications sent from apps often include customer names, error details with PII, and alert payloads with raw data.",
		Category:    "communication",
		KnownHosts:  []string{"hooks.slack.com", "slack.com", "api.slack.com"},
		DataTypes:   []string{"email", "name", "message_content"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@slack/webhook", "@slack/bolt"},
	},
	// ── Identity ───────────────────────────────────────────────────────────────
	{
		ID:          "auth0",
		Name:        "Auth0",
		Description: "Identity platform. Actions and Rules can inadvertently log full user profiles to external services. JWT tokens with unnecessary PII claims are common.",
		Category:    "identity",
		KnownHosts:  []string{"*.auth0.com", "*.us.auth0.com", "*.eu.auth0.com"},
		DataTypes:   []string{"email", "name", "ip_address", "user_id"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"auth0", "@auth0/auth0-react", "@auth0/auth0-spa-js"},
	},
	{
		ID:          "okta",
		Name:        "Okta",
		Description: "Enterprise identity and SSO. Event hooks and SCIM provisioning can expose user profile data including personal email, phone, and department information.",
		Category:    "identity",
		KnownHosts:  []string{"*.okta.com", "*.oktapreview.com"},
		DataTypes:   []string{"email", "name", "phone", "user_id"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"@okta/okta-sdk-nodejs", "@okta/okta-auth-js"},
	},
	// ── Feature Flags ──────────────────────────────────────────────────────────
	{
		ID:          "launchdarkly",
		Name:        "LaunchDarkly",
		Description: "Feature flag management. SDK user context objects often use email addresses as the user key — sending PII to analytics dashboards.",
		Category:    "feature-flags",
		KnownHosts:  []string{"app.launchdarkly.com", "sdk.launchdarkly.com", "clientsdk.launchdarkly.com"},
		DataTypes:   []string{"email", "user_id", "name"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"launchdarkly-node-server-sdk", "launchdarkly-js-client-sdk"},
	},
	{
		ID:          "split-io",
		Name:        "Split.io",
		Description: "Feature flag and experimentation platform. Treatment definitions and user attributes can expose PII in targeting rules and impressions data.",
		Category:    "feature-flags",
		KnownHosts:  []string{"sdk.split.io", "events.split.io", "auth.split.io"},
		DataTypes:   []string{"email", "user_id"},
		ComplianceFrameworks: []string{"GDPR"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@splitsoftware/splitio"},
	},
	// ── Search ─────────────────────────────────────────────────────────────────
	{
		ID:          "algolia",
		Name:        "Algolia",
		Description: "Search-as-a-service. Indices may contain user-generated content with PII. Search query logs capture what users searched for — itself personal data.",
		Category:    "search",
		KnownHosts:  []string{"*.algolia.net", "*.algolianet.com"},
		DataTypes:   []string{"email", "name", "user_id", "address"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"algoliasearch"},
	},
	// ── Incident Management ───────────────────────────────────────────────────
	{
		ID:          "pagerduty",
		Name:        "PagerDuty",
		Description: "Incident management and alerting. Alert payloads and incident details often include customer context and error messages with PII.",
		Category:    "incident-management",
		KnownHosts:  []string{"api.pagerduty.com", "events.pagerduty.com"},
		DataTypes:   []string{"email", "name", "phone"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"@pagerduty/pdjs", "pdpyras"},
	},
	{
		ID:          "opsgenie",
		Name:        "OpsGenie",
		Description: "Alert and incident management. Alert details can contain customer PII included by application teams in alert messages and custom properties.",
		Category:    "incident-management",
		KnownHosts:  []string{"api.opsgenie.com", "app.opsgenie.com"},
		DataTypes:   []string{"email", "name", "phone"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"proxy"},
		SDKPackages:          []string{"opsgenie-sdk"},
	},
	// ── Logging ───────────────────────────────────────────────────────────────
	{
		ID:          "elastic",
		Name:        "Elastic (ELK)",
		Description: "Full log ingestion with no default PII filtering. Often the final store for all application logs — a high-value PII repository.",
		Category:    "logging",
		KnownHosts:  []string{"*.es.io", "*.elastic-cloud.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message", "api_key"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"agent_config", "proxy"},
		SDKPackages:          []string{"@elastic/elasticsearch", "elasticsearch"},
	},
	{
		ID:          "splunk",
		Name:        "Splunk",
		Description: "SIEM and log management. Often stores the most comprehensive PII dataset in an organization — authentication events, application logs, and network logs.",
		Category:    "logging",
		KnownHosts:  []string{"*.splunkcloud.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message", "financial"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"agent_config", "proxy"},
		SDKPackages:          []string{"splunk-logging", "splunk-sdk"},
	},
	{
		ID:          "sumo-logic",
		Name:        "Sumo Logic",
		Description: "Cloud-native log management. Collectors ingest application and infrastructure logs that commonly contain PII in structured and unstructured fields.",
		Category:    "logging",
		KnownHosts:  []string{"*.sumologic.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"proxy", "agent_config"},
		SDKPackages:          []string{"sumologic-javascript-logging", "sumologic-winston"},
	},
	// ── Cloud Infrastructure ──────────────────────────────────────────────────
	{
		ID:          "aws",
		Name:        "AWS (CloudWatch/S3)",
		Description: "Amazon Web Services. CloudWatch Logs capture everything Lambda functions print. S3 buckets may store PII files. Most common source of large-scale PII exposure.",
		Category:    "cloud",
		KnownHosts:  []string{"*.amazonaws.com", "*.aws.amazon.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message", "financial", "api_key"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"aws-sdk", "@aws-sdk/client-cloudwatch-logs"},
	},
	{
		ID:          "gcp",
		Name:        "Google Cloud (GCP)",
		Description: "Google Cloud Platform. Cloud Logging captures application and infrastructure logs. BigQuery tables often store customer data in unencrypted text columns.",
		Category:    "cloud",
		KnownHosts:  []string{"*.googleapis.com", "*.google.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"@google-cloud/logging", "google-cloud"},
	},
	{
		ID:          "azure",
		Name:        "Azure Monitor",
		Description: "Microsoft Azure monitoring. Azure Monitor logs, Application Insights, and Log Analytics workspaces can aggregate sensitive data across all Azure services.",
		Category:    "cloud",
		KnownHosts:  []string{"*.azure.com", "*.microsoft.com"},
		DataTypes:   []string{"email", "ip_address", "user_id", "log_message"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"@azure/monitor-ingestion", "@azure/monitor-query"},
	},
	{
		ID:          "cloudflare",
		Name:        "Cloudflare",
		Description: "CDN, DNS, and WAF. Workers scripts and Logpush jobs can log full HTTP request bodies. WAF misconfigurations may allow PII leakage in URLs.",
		Category:    "infrastructure",
		KnownHosts:  []string{"api.cloudflare.com", "cloudflareinsights.com"},
		DataTypes:   []string{"ip_address", "log_message"},
		ComplianceFrameworks: []string{"GDPR", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"cloudflare"},
	},
	// ── Data Warehouse ────────────────────────────────────────────────────────
	{
		ID:          "snowflake",
		Name:        "Snowflake",
		Description: "Cloud data warehouse. Often the final destination of all PII — customer tables, transaction records, and support data. A critical compliance control point.",
		Category:    "data-warehouse",
		KnownHosts:  []string{"*.snowflakecomputing.com"},
		DataTypes:   []string{"email", "name", "phone", "address", "financial", "health"},
		ComplianceFrameworks: []string{"GDPR", "HIPAA", "PCI-DSS", "SOC2"},
		IntegrationMethods:   []string{"agent_config"},
		SDKPackages:          []string{"snowflake-sdk"},
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
