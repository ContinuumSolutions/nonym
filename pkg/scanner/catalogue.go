package scanner

import "github.com/gofiber/fiber/v2"

// CatalogueEntry describes a vendor in the scanner catalogue returned to the
// frontend.  It extends VendorProfile with fields needed to render the
// "Connect a vendor" modal.
type CatalogueEntry struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Abbr            string      `json:"abbr"`
	Color           string      `json:"color"`
	Bg              string      `json:"bg"`
	Category        string      `json:"category"`
	Description     string      `json:"description"`
	AuthType        string      `json:"auth_type"`   // "api_key" | "oauth"
	AuthFields      []AuthField `json:"auth_fields"` // empty for proxy-only vendors
	ScanMode        string      `json:"scan_mode"`   // "data" | "config" | "both"
	ScanDescription string      `json:"scan_description"`
	DocsURL         string      `json:"docs_url"`
}

// AuthField is a single credential input for the connect modal.
type AuthField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Secret      bool   `json:"secret"`
	HelpText    string `json:"help_text,omitempty"`
}

// VendorCatalogue is the ordered, authoritative list of every vendor the
// scanner knows about.  Proxy vendors (openai, anthropic) are omitted here —
// they appear only in the router's VendorCatalog and are scanned via the AI
// proxy pipeline, not via a scanner connection.
var VendorCatalogue = []CatalogueEntry{
	// ── Error Tracking ─────────────────────────────────────────────────────────
	{
		ID:       "sentry",
		Name:     "Sentry",
		Abbr:     "SE",
		Color:    "#F55",
		Bg:       "rgba(255,85,85,0.12)",
		Category: "error-tracking",
		Description: "Error tracking & performance monitoring. Captures exception " +
			"messages, stack traces, breadcrumbs, request bodies, and user context " +
			"— all rich PII sources.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_token", Label: "Auth Token", Placeholder: "sntrys_...", Secret: true,
				HelpText: "User Auth Token from Settings → Account → API → Auth Tokens. Needs org:read, project:read, event:read scopes."},
			{Key: "org_slug", Label: "Organization Slug", Placeholder: "my-org", Secret: false,
				HelpText: "Found in the URL: sentry.io/organizations/<slug>/"},
		},
		ScanMode:        "data",
		ScanDescription: "Fetches recent error events across all projects and scans event payload, user context, tags, breadcrumbs, and exception values for PII.",
		DocsURL:         "https://docs.sentry.io/api/auth/",
	},
	{
		ID:       "rollbar",
		Name:     "Rollbar",
		Abbr:     "RB",
		Color:    "#1DA463",
		Bg:       "rgba(29,164,99,0.12)",
		Category: "error-tracking",
		Description: "Error tracking and crash reporting. Captures full request " +
			"context, query params, POST body, and person fields that commonly " +
			"contain email addresses and user data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_token", Label: "Project Access Token (read)", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Project Settings → Access Tokens → create a read scope token."},
		},
		ScanMode:        "data",
		ScanDescription: "Fetches recent items (errors) and scans request parameters, person data, and custom fields for PII.",
		DocsURL:         "https://docs.rollbar.com/reference/authentication",
	},
	{
		ID:       "bugsnag",
		Name:     "Bugsnag",
		Abbr:     "BG",
		Color:    "#4C35D9",
		Bg:       "rgba(76,53,217,0.12)",
		Category: "error-tracking",
		Description: "Crash monitoring platform. Bugsnag metaData objects, user " +
			"fields, and breadcrumbs capture PII embedded in application state at " +
			"the time of the error.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "auth_token", Label: "Personal Auth Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Account Settings → My Account → Personal Auth Token."},
			{Key: "organization_name", Label: "Organization Name", Placeholder: "my-company", Secret: false},
		},
		ScanMode:        "data",
		ScanDescription: "Fetches recent error events from all projects and scans user context, metaData, and request fields for PII.",
		DocsURL:         "https://bugsnagapi.docs.apiary.io/#introduction/authentication",
	},
	// ── APM & Observability ────────────────────────────────────────────────────
	{
		ID:       "datadog",
		Name:     "Datadog",
		Abbr:     "DD",
		Color:    "#632CA6",
		Bg:       "rgba(99,44,166,0.12)",
		Category: "apm",
		Description: "APM, infrastructure monitoring, and log management. Log " +
			"pipelines and APM traces routinely include user data embedded in " +
			"log messages, HTTP headers, and SQL query parameters.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Organization Settings → API Keys. Use a scoped key with logs_read permission."},
			{Key: "app_key", Label: "Application Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Organization Settings → Application Keys."},
			{Key: "site", Label: "Datadog Site (optional)", Placeholder: "datadoghq.com", Secret: false,
				HelpText: "Use datadoghq.eu for EU region or us3/us5.datadoghq.com for other regions."},
		},
		ScanMode:        "both",
		ScanDescription: "Queries recent log events and APM trace attributes for PII. Config audit checks scrubbing rules, sensitive data scanner status, and key scoping.",
		DocsURL:         "https://docs.datadoghq.com/api/latest/authentication/",
	},
	{
		ID:       "newrelic",
		Name:     "New Relic",
		Abbr:     "NR",
		Color:    "#008C99",
		Bg:       "rgba(0,140,153,0.12)",
		Category: "apm",
		Description: "Observability platform covering APM, browser monitoring, " +
			"logs, and infrastructure. Distributed tracing and log forwarding " +
			"can expose PII embedded in application logs and HTTP payloads.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "User API Key", Placeholder: "NRAK-...", Secret: true,
				HelpText: "API Key Management → Create a key → choose 'User' type. Prefix: NRAK-"},
			{Key: "account_id", Label: "Account ID", Placeholder: "1234567", Secret: false,
				HelpText: "Shown in the URL and in Account settings."},
		},
		ScanMode:        "both",
		ScanDescription: "Queries recent log events via NerdGraph API and scans log messages and custom attributes for PII.",
		DocsURL:         "https://docs.newrelic.com/docs/apis/intro-apis/new-relic-api-keys/",
	},
	{
		ID:       "elastic",
		Name:     "Elastic (ELK)",
		Abbr:     "EL",
		Color:    "#FEC514",
		Bg:       "rgba(254,197,20,0.12)",
		Category: "logging",
		Description: "Elasticsearch, Logstash, and Kibana. Full log ingestion " +
			"with no default PII filtering. Often the final store for all " +
			"application logs — a high-value PII repository.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "host", Label: "Elasticsearch Host", Placeholder: "https://my-cluster.es.io:9243", Secret: false},
			{Key: "api_key", Label: "API Key", Placeholder: "base64encodedkey==", Secret: true,
				HelpText: "Kibana → Stack Management → API Keys → Create API key. Needs read access to target indices."},
			{Key: "index_pattern", Label: "Index Pattern (optional)", Placeholder: "logs-*", Secret: false,
				HelpText: "Glob pattern for indices to scan. Defaults to 'logs-*'."},
		},
		ScanMode:        "both",
		ScanDescription: "Samples recent documents from log indices and scans message fields for PII. Config audit checks authentication, TLS, and index access controls.",
		DocsURL:         "https://www.elastic.co/guide/en/elasticsearch/reference/current/security-api-create-api-key.html",
	},
	{
		ID:       "splunk",
		Name:     "Splunk",
		Abbr:     "SP",
		Color:    "#65A637",
		Bg:       "rgba(101,166,55,0.12)",
		Category: "logging",
		Description: "SIEM and log management platform. Splunk often stores the " +
			"most comprehensive PII dataset in an organization — authentication " +
			"events, application logs, and network logs.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "host", Label: "Splunk Host", Placeholder: "https://splunk.company.com:8089", Secret: false},
			{Key: "token", Label: "Auth Token", Placeholder: "Splunk xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → Tokens → New Token (Splunk Web). Or use username/password with basic auth."},
		},
		ScanMode:        "both",
		ScanDescription: "Runs a Splunk search over recent events and scans message fields for PII. Config audit checks HEC token scope, index access restrictions, and masking rules.",
		DocsURL:         "https://docs.splunk.com/Documentation/Splunk/latest/Security/UseAuthTokens",
	},
	{
		ID:       "sumo-logic",
		Name:     "Sumo Logic",
		Abbr:     "SL",
		Color:    "#000099",
		Bg:       "rgba(0,0,153,0.12)",
		Category: "logging",
		Description: "Cloud-native log management and analytics. Sumo Logic " +
			"collectors ingest application and infrastructure logs that commonly " +
			"contain PII in structured and unstructured fields.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_id", Label: "Access ID", Placeholder: "suXXXXXXXXXXXX", Secret: false,
				HelpText: "Administration → Security → Access Keys → Add Access Key."},
			{Key: "access_key", Label: "Access Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true},
			{Key: "api_endpoint", Label: "API Endpoint (optional)", Placeholder: "https://api.sumologic.com/api", Secret: false,
				HelpText: "Varies by deployment. See: https://help.sumologic.com/docs/api/about-the-api/"},
		},
		ScanMode:        "both",
		ScanDescription: "Runs a search query over recent log messages and scans for PII. Config audit checks source masking rules and partition access controls.",
		DocsURL:         "https://help.sumologic.com/docs/manage/security/access-keys/",
	},
	// ── Product Analytics ──────────────────────────────────────────────────────
	{
		ID:       "segment",
		Name:     "Segment",
		Abbr:     "SG",
		Color:    "#52BD95",
		Bg:       "rgba(82,189,149,0.12)",
		Category: "analytics",
		Description: "Customer Data Platform (CDP). Segment is the highest-leverage " +
			"PII intercept point — data placed here fans out to every connected " +
			"destination. identify() calls commonly contain email, name, and phone.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_token", Label: "Public API Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Workspace Settings → Access Management → Tokens → Generate Token. Requires Source Read and Config API access."},
			{Key: "workspace_slug", Label: "Workspace Slug", Placeholder: "my-workspace", Secret: false,
				HelpText: "Found in the URL: app.segment.com/<workspace>/"},
		},
		ScanMode:        "both",
		ScanDescription: "Queries recent events via Segment's Profile API and scans traits/properties for PII. Config audit checks destination filters, IP suppression, and privacy settings.",
		DocsURL:         "https://segment.com/docs/config-api/authentication/",
	},
	{
		ID:       "mixpanel",
		Name:     "Mixpanel",
		Abbr:     "MX",
		Color:    "#7856FF",
		Bg:       "rgba(120,86,255,0.12)",
		Category: "analytics",
		Description: "User analytics platform. Mixpanel event tracking captures " +
			"user properties and behavioral data that often includes directly " +
			"identifying information via $email, $name, and $distinct_id.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "service_account_user", Label: "Service Account Username", Placeholder: "user.XXXXXX.mp-service-account", Secret: false,
				HelpText: "Organization Settings → Service Accounts → Create. Copy the username."},
			{Key: "service_account_secret", Label: "Service Account Secret", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true},
			{Key: "project_id", Label: "Project ID", Placeholder: "12345678", Secret: false,
				HelpText: "Found in Project Settings → Project ID."},
		},
		ScanMode:        "both",
		ScanDescription: "Exports recent events and scans event properties, user profiles, and $distinct_id for PII. Config audit checks EU residency, IP collection, and data retention.",
		DocsURL:         "https://developer.mixpanel.com/reference/service-accounts",
	},
	{
		ID:       "amplitude",
		Name:     "Amplitude",
		Abbr:     "AM",
		Color:    "#1B6EF3",
		Bg:       "rgba(27,110,243,0.12)",
		Category: "analytics",
		Description: "Product analytics platform. Amplitude's autocapture can " +
			"grab form field values and user properties including emails, names, " +
			"and user-defined attributes.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Project API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Project Settings → General → API Key."},
			{Key: "secret_key", Label: "Project Secret Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Project Settings → General → Secret Key. Required for export API."},
		},
		ScanMode:        "both",
		ScanDescription: "Exports recent event data and scans user properties and event properties for PII. Config audit checks blocked properties and EU residency.",
		DocsURL:         "https://www.docs.developers.amplitude.com/analytics/apis/authentication/",
	},
	{
		ID:       "posthog",
		Name:     "PostHog",
		Abbr:     "PH",
		Color:    "#F54E00",
		Bg:       "rgba(245,78,0,0.12)",
		Category: "analytics",
		Description: "Product analytics and session recording. PostHog event " +
			"properties and person profiles accumulate email addresses, names, " +
			"and device fingerprints. Session recordings can capture keystrokes.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Personal API Key", Placeholder: "phx_...", Secret: true,
				HelpText: "Settings → Personal API Keys → Create personal API key."},
			{Key: "project_id", Label: "Project ID", Placeholder: "12345", Secret: false,
				HelpText: "Found in Project Settings → Project ID."},
			{Key: "host", Label: "PostHog Host (optional)", Placeholder: "https://app.posthog.com", Secret: false,
				HelpText: "Required for self-hosted instances. Leave blank for PostHog Cloud."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent events and person records and scans properties and $set/$set_once calls for PII. Config audit checks session recording masking and autocapture scope.",
		DocsURL:         "https://posthog.com/docs/api",
	},
	{
		ID:       "heap",
		Name:     "Heap",
		Abbr:     "HP",
		Color:    "#6C47FF",
		Bg:       "rgba(108,71,255,0.12)",
		Category: "analytics",
		Description: "Product analytics with autocapture. Heap records every user " +
			"interaction including form inputs and text selections without requiring " +
			"code — the highest risk of accidental PII capture.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Account → Settings → API → Generate API Key."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks global data redaction rules, sensitive page exclusions, and user identifier settings. Full event export not available via API.",
		DocsURL:         "https://developers.heap.io/docs/authentication",
	},
	{
		ID:       "google-analytics",
		Name:     "Google Analytics 4",
		Abbr:     "GA",
		Color:    "#E37400",
		Bg:       "rgba(227,116,0,0.12)",
		Category: "analytics",
		Description: "Web analytics platform. GA4 uses client-side JavaScript, so " +
			"direct data scanning is not feasible. Config misconfigurations can " +
			"silently send PII (names, emails) as custom dimensions to Google.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "property_id", Label: "GA4 Property ID", Placeholder: "123456789", Secret: false,
				HelpText: "Admin → Property → Property Settings → Property ID."},
			{Key: "service_account_json", Label: "Service Account JSON (optional)", Placeholder: "{\"type\":\"service_account\",...}", Secret: true,
				HelpText: "Create a service account in Google Cloud, grant it Viewer access to the GA4 property, and paste the key JSON."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks custom dimensions/metrics for PII, consent mode implementation, data retention settings, and data sharing with Google ad partners.",
		DocsURL:         "https://developers.google.com/analytics/devguides/reporting/data/v1/quickstart-client-libraries",
	},
	{
		ID:       "hotjar",
		Name:     "Hotjar",
		Abbr:     "HJ",
		Color:    "#FF3C00",
		Bg:       "rgba(255,60,0,0.12)",
		Category: "analytics",
		Description: "Session recording and heatmaps. Hotjar runs client-side and " +
			"streams recording data directly — full PII interception is not " +
			"possible. Misconfigured masking is the #1 compliance risk.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "site_id", Label: "Site ID", Placeholder: "1234567", Secret: false,
				HelpText: "Shown in your Hotjar tracking code as hjid."},
			{Key: "api_key", Label: "Private API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → Private API Key (if using Hotjar's API)."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit validates that session recording masking is configured for all input fields, sensitive pages are excluded, and IP anonymization is enabled.",
		DocsURL:         "https://developer.hotjar.com/api/",
	},
	// ── Customer Engagement ────────────────────────────────────────────────────
	{
		ID:       "intercom",
		Name:     "Intercom",
		Abbr:     "IC",
		Color:    "#286EFA",
		Bg:       "rgba(40,110,250,0.12)",
		Category: "customer-support",
		Description: "Customer messaging and support. Intercom stores full " +
			"conversation transcripts, user-uploaded files, and custom attributes " +
			"that often contain PHI, financial information, and personal details.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_token", Label: "Access Token", Placeholder: "dG9rXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX==", Secret: true,
				HelpText: "Developer Hub → Your App → Authentication → Access Token. For testing: Settings → Developers → Developer Mode Access Token."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent conversations and contact records, scanning message bodies and custom attributes for PII. Config audit checks workspace data residency and attribute scope.",
		DocsURL:         "https://developers.intercom.com/docs/build-an-integration/getting-started/authentication/",
	},
	{
		ID:       "zendesk",
		Name:     "Zendesk",
		Abbr:     "ZD",
		Color:    "#03363D",
		Bg:       "rgba(3,54,61,0.12)",
		Category: "customer-support",
		Description: "Customer support platform. Zendesk ticket bodies, user " +
			"profiles, attachments, and custom fields routinely contain PHI, " +
			"payment information, and detailed personal context.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "subdomain", Label: "Subdomain", Placeholder: "mycompany", Secret: false,
				HelpText: "The part before .zendesk.com in your URL."},
			{Key: "email", Label: "Agent Email", Placeholder: "admin@mycompany.com", Secret: false},
			{Key: "api_token", Label: "API Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Admin Center → Apps and Integrations → Zendesk API → Add API Token."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent tickets and user records, scanning ticket descriptions, comments, and custom fields for PII. Config audit checks field encryption and HIPAA settings.",
		DocsURL:         "https://developer.zendesk.com/api-reference/introduction/security-and-auth/",
	},
	{
		ID:       "hubspot",
		Name:     "HubSpot",
		Abbr:     "HS",
		Color:    "#FF7A59",
		Bg:       "rgba(255,122,89,0.12)",
		Category: "crm",
		Description: "CRM and marketing automation platform. HubSpot contact " +
			"records, form submissions, and email campaigns routinely contain " +
			"PII including email addresses, phone numbers, and company details.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_token", Label: "Private App Access Token", Placeholder: "pat-eu1-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → Integrations → Private Apps → Create private app. Grant CRM read scopes."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent contact and deal records and scans properties for PII. Config audit checks tracking pixel scope, GDPR tools, and data sharing settings.",
		DocsURL:         "https://developers.hubspot.com/docs/api/private-apps",
	},
	{
		ID:       "salesforce",
		Name:     "Salesforce",
		Abbr:     "SF",
		Color:    "#00A1E0",
		Bg:       "rgba(0,161,224,0.12)",
		Category: "crm",
		Description: "Enterprise CRM. Salesforce Contact, Lead, Account, and " +
			"Opportunity objects hold the most sensitive customer business data " +
			"in most organizations. Misconfigured profiles expose PII to all users.",
		AuthType: "oauth",
		AuthFields: []AuthField{
			{Key: "instance_url", Label: "Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Secret: false},
			{Key: "access_token", Label: "Connected App Access Token", Placeholder: "00D...", Secret: true,
				HelpText: "Create a Connected App → OAuth Settings → Use JWT Bearer Flow or Username-Password Flow for server-to-server."},
		},
		ScanMode:        "both",
		ScanDescription: "Queries recent Contact and Lead records for PII in standard and custom fields. Config audit checks Shield encryption, field-level security, and Event Monitoring.",
		DocsURL:         "https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/quickstart_oauth.htm",
	},
	{
		ID:       "customer-io",
		Name:     "Customer.io",
		Abbr:     "CI",
		Color:    "#6C3EFF",
		Bg:       "rgba(108,62,255,0.12)",
		Category: "marketing",
		Description: "Marketing automation platform. Customer.io stores customer " +
			"profiles, event streams, and personalized message content that " +
			"includes email addresses, names, and behavioral data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "App API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → API Credentials → App API Keys → Create API Key."},
			{Key: "site_id", Label: "Site ID", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: false},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent customer profiles and scans attributes and segment memberships for PII. Config audit checks data expiry and message content scope.",
		DocsURL:         "https://customer.io/docs/api/app/",
	},
	// ── Payments ───────────────────────────────────────────────────────────────
	{
		ID:       "stripe",
		Name:     "Stripe",
		Abbr:     "ST",
		Color:    "#635BFF",
		Bg:       "rgba(99,91,255,0.12)",
		Category: "payments",
		Description: "Payment processing platform. Stripe handles card data via " +
			"client-side tokenization — do NOT proxy raw card data. Config " +
			"misconfigurations (webhook logging, API key scope) are the primary risk.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "restricted_key", Label: "Restricted API Key", Placeholder: "rk_live_...", Secret: true,
				HelpText: "Developers → API Keys → Create restricted key with read-only access to Charges, Customers, PaymentIntents."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks webhook signature enforcement, API key scoping, Stripe.js usage (not raw card data), and Radar fraud rules.",
		DocsURL:         "https://stripe.com/docs/keys#limiting-access-with-restricted-api-keys",
	},
	// ── Communication ─────────────────────────────────────────────────────────
	{
		ID:       "twilio",
		Name:     "Twilio",
		Abbr:     "TW",
		Color:    "#F22F46",
		Bg:       "rgba(242,47,70,0.12)",
		Category: "communication",
		Description: "SMS, voice, and email platform. Twilio message bodies can " +
			"contain OTPs, PII in notification templates, phone numbers, and " +
			"call recordings that may include sensitive conversations.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "account_sid", Label: "Account SID", Placeholder: "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: false,
				HelpText: "Console Dashboard → Account Info → Account SID."},
			{Key: "auth_token", Label: "Auth Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Console Dashboard → Account Info → Auth Token. Consider using API Keys instead."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent SMS/MMS messages and scans message body for PII embedded in notification templates. Config audit checks call recording retention and API key scoping.",
		DocsURL:         "https://www.twilio.com/docs/iam/api-keys",
	},
	{
		ID:       "sendgrid",
		Name:     "SendGrid",
		Abbr:     "SG",
		Color:    "#1A82E2",
		Bg:       "rgba(26,130,226,0.12)",
		Category: "email",
		Description: "Transactional email service. SendGrid email sends contain " +
			"recipient addresses and dynamic template data that may include " +
			"personal information, order details, and healthcare context.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key", Placeholder: "SG.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → API Keys → Create API Key → Restricted Access → Mail Send + Stats + Templates (read)."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent email stats and template configurations, scanning dynamic template fields for PII. Config audit checks API key scoping and unsubscribe mechanisms.",
		DocsURL:         "https://docs.sendgrid.com/ui/account-and-settings/api-keys",
	},
	{
		ID:       "mailgun",
		Name:     "Mailgun",
		Abbr:     "MG",
		Color:    "#C62136",
		Bg:       "rgba(198,33,54,0.12)",
		Category: "email",
		Description: "Email API service. Mailgun message logs contain recipient " +
			"addresses, email bodies (stored for 3 days by default), and route " +
			"forwarding rules that can expose PII to unintended destinations.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Private API Key", Placeholder: "key-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Account Settings → API Security → API Keys → Private API Key."},
			{Key: "domain", Label: "Sending Domain", Placeholder: "mail.mycompany.com", Secret: false},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent stored messages and event logs, scanning message bodies and recipient lists for PII. Config audit checks route forwarding rules and message retention.",
		DocsURL:         "https://documentation.mailgun.com/docs/mailgun/api-reference/authentication/",
	},
	{
		ID:       "mailchimp",
		Name:     "Mailchimp",
		Abbr:     "MC",
		Color:    "#FFE01B",
		Bg:       "rgba(255,224,27,0.12)",
		Category: "marketing",
		Description: "Email marketing platform. Mailchimp audience segments and " +
			"contact tags may store sensitive behavioral or health-related " +
			"classifications alongside PII like email, name, and address.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-us1", Secret: true,
				HelpText: "Profile → Extras → API Keys → Create A Key. The suffix (e.g., -us1) indicates your data center."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches audience members and scans contact fields, tags, and merge field values for PII. Config audit checks audience data classification and tag taxonomy.",
		DocsURL:         "https://mailchimp.com/developer/marketing/docs/fundamentals/",
	},
	{
		ID:       "slack",
		Name:     "Slack (Webhooks/Bots)",
		Abbr:     "SK",
		Color:    "#4A154B",
		Bg:       "rgba(74,21,75,0.12)",
		Category: "communication",
		Description: "Team communication platform. Slack webhook notifications " +
			"sent from your app often include customer names, error details with " +
			"PII, alert payloads with raw data, and operational context.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "bot_token", Label: "Bot OAuth Token", Placeholder: "xoxb-...", Secret: true,
				HelpText: "App Settings → OAuth & Permissions → Bot User OAuth Token. Grant channels:read, chat:write scopes."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks bot token scoping, webhook URL exposure, channel privacy settings, and message retention policies.",
		DocsURL:         "https://api.slack.com/authentication/token-types",
	},
	// ── Identity & Auth ────────────────────────────────────────────────────────
	{
		ID:       "auth0",
		Name:     "Auth0",
		Abbr:     "A0",
		Color:    "#EB5424",
		Bg:       "rgba(235,84,36,0.12)",
		Category: "identity",
		Description: "Identity platform. Auth0 Actions and Rules can inadvertently " +
			"log full user profiles to external services. JWT tokens with " +
			"unnecessary PII claims are a common misconfiguration.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "domain", Label: "Auth0 Domain", Placeholder: "myapp.auth0.com", Secret: false,
				HelpText: "Your Auth0 tenant domain. Found in Applications → Settings."},
			{Key: "management_token", Label: "Management API Token", Placeholder: "eyJhbGciOiJSUzI1NiJ9...", Secret: true,
				HelpText: "Applications → Auth0 Management API → Machine to Machine → Generate token. Needs read:logs and read:clients scopes only."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks Actions/Rules for PII logging, JWT custom claims for unnecessary personal data, legacy grant types, and log retention settings.",
		DocsURL:         "https://auth0.com/docs/secure/tokens/access-tokens/management-api-access-tokens",
	},
	{
		ID:       "okta",
		Name:     "Okta",
		Abbr:     "OK",
		Color:    "#007DC1",
		Bg:       "rgba(0,125,193,0.12)",
		Category: "identity",
		Description: "Enterprise identity and SSO platform. Okta event hooks and " +
			"SCIM provisioning can expose user profile data including personal " +
			"email, phone, and department information to third-party applications.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "org_url", Label: "Okta Organization URL", Placeholder: "https://mycompany.okta.com", Secret: false},
			{Key: "api_token", Label: "API Token", Placeholder: "00x...", Secret: true,
				HelpText: "Admin Console → Security → API → Tokens → Create Token."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks event hook destinations for PII exposure, SCIM attribute scope, System Log forwarding, and API token rotation.",
		DocsURL:         "https://developer.okta.com/docs/reference/core-okta-api/",
	},
	// ── Feature Flags ──────────────────────────────────────────────────────────
	{
		ID:       "launchdarkly",
		Name:     "LaunchDarkly",
		Abbr:     "LD",
		Color:    "#405BFF",
		Bg:       "rgba(64,91,255,0.12)",
		Category: "feature-flags",
		Description: "Feature flag management. SDK user context objects often " +
			"use email addresses as the user key — sending PII to LaunchDarkly " +
			"analytics dashboards and making GDPR deletion non-trivial.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Access Token", Placeholder: "api-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true,
				HelpText: "Account Settings → Authorization → Access tokens → Create token. Reader role is sufficient."},
			{Key: "project_key", Label: "Project Key", Placeholder: "my-project", Secret: false,
				HelpText: "Account Settings → Projects → your project's key."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches user segments and targeting rules, scanning user context attributes for PII. Config audit checks user key format and analytics data retention.",
		DocsURL:         "https://docs.launchdarkly.com/home/account-security/api-access-tokens",
	},
	{
		ID:       "split-io",
		Name:     "Split.io",
		Abbr:     "SP",
		Color:    "#D94045",
		Bg:       "rgba(217,64,69,0.12)",
		Category: "feature-flags",
		Description: "Feature flag and experimentation platform. Split treatment " +
			"definitions and user attributes can expose PII in targeting rules " +
			"and impressions data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Admin API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Admin → API Keys → Add API Key (Admin type)."},
			{Key: "workspace_id", Label: "Workspace ID", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: false},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks user key format in split definitions, attribute cardinality for PII fields, and impressions data retention.",
		DocsURL:         "https://docs.split.io/reference/authentication",
	},
	// ── Search ─────────────────────────────────────────────────────────────────
	{
		ID:       "algolia",
		Name:     "Algolia",
		Abbr:     "AL",
		Color:    "#003DFF",
		Bg:       "rgba(0,61,255,0.12)",
		Category: "search",
		Description: "Search-as-a-service. Algolia indices may contain user-generated " +
			"content, user profiles, and order records with PII. Search query " +
			"logs capture what users searched for — itself personal data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "app_id", Label: "Application ID", Placeholder: "XXXXXXXXXX", Secret: false,
				HelpText: "Settings → API Keys → Application ID."},
			{Key: "api_key", Label: "Admin API Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → API Keys → Admin API Key. Or create a key with browse and search scopes only."},
		},
		ScanMode:        "both",
		ScanDescription: "Browses index records and scans searchable attributes for PII. Config audit checks API key scoping, unretrievableAttributes, and query log anonymization.",
		DocsURL:         "https://www.algolia.com/doc/api-client/getting-started/what-is-the-api-client/javascript/?client=javascript",
	},
	// ── Incident Management ───────────────────────────────────────────────────
	{
		ID:       "pagerduty",
		Name:     "PagerDuty",
		Abbr:     "PD",
		Color:    "#25C151",
		Bg:       "rgba(37,193,81,0.12)",
		Category: "incident-management",
		Description: "Incident management and alerting. Alert payloads and " +
			"incident details often include customer context, error messages " +
			"with PII, and on-call engineer contact information.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key (v2)", Placeholder: "u+xxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Integrations → API Access Keys → Create New API Key (Read-only)."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent incidents and scans incident summaries and custom details for PII. Config audit checks alert payload templates and notification channel security.",
		DocsURL:         "https://developer.pagerduty.com/docs/rest-api/auth/",
	},
	{
		ID:       "opsgenie",
		Name:     "OpsGenie",
		Abbr:     "OG",
		Color:    "#3E78B5",
		Bg:       "rgba(62,120,181,0.12)",
		Category: "incident-management",
		Description: "Alert and incident management platform. OpsGenie alert " +
			"details can contain customer PII included by application teams in " +
			"alert messages and custom properties.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "API Key", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true,
				HelpText: "Settings → API Key Management → Add new API key (Read access only)."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent alerts and scans alert messages and details for PII. Config audit checks alert routing and notification content templates.",
		DocsURL:         "https://docs.opsgenie.com/docs/api-key-management",
	},
	// ── Cloud Infrastructure ──────────────────────────────────────────────────
	{
		ID:       "aws",
		Name:     "AWS (CloudWatch/S3)",
		Abbr:     "AW",
		Color:    "#FF9900",
		Bg:       "rgba(255,153,0,0.12)",
		Category: "cloud",
		Description: "Amazon Web Services. CloudWatch Logs capture everything " +
			"Lambda functions print. S3 buckets may store PII files. The most " +
			"common source of large-scale PII exposure via misconfiguration.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "access_key_id", Label: "AWS Access Key ID", Placeholder: "AKIAIOSFODNN7EXAMPLE", Secret: false,
				HelpText: "IAM → Users → Security credentials → Create access key. Use a read-only IAM role with CloudWatch Logs and S3 ListObject/GetObject scopes."},
			{Key: "secret_access_key", Label: "AWS Secret Access Key", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true},
			{Key: "region", Label: "AWS Region", Placeholder: "us-east-1", Secret: false},
			{Key: "log_group", Label: "CloudWatch Log Group (optional)", Placeholder: "/aws/lambda/my-function", Secret: false,
				HelpText: "Specific log group to scan. Leave blank to scan all accessible groups."},
		},
		ScanMode:        "both",
		ScanDescription: "Samples recent CloudWatch log events and scans message fields for PII. Config audit checks S3 public access, CloudTrail, log retention, and IAM least-privilege.",
		DocsURL:         "https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html",
	},
	{
		ID:       "gcp",
		Name:     "Google Cloud (GCP)",
		Abbr:     "GC",
		Color:    "#4285F4",
		Bg:       "rgba(66,133,244,0.12)",
		Category: "cloud",
		Description: "Google Cloud Platform. Cloud Logging captures application " +
			"and infrastructure logs. BigQuery tables often store customer data " +
			"in unencrypted text columns without column-level access control.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "project_id", Label: "GCP Project ID", Placeholder: "my-project-123456", Secret: false},
			{Key: "service_account_json", Label: "Service Account Key JSON", Placeholder: "{\"type\":\"service_account\",...}", Secret: true,
				HelpText: "IAM → Service Accounts → Create Service Account → JSON key. Grant roles/logging.viewer and roles/storage.objectViewer."},
		},
		ScanMode:        "both",
		ScanDescription: "Queries Cloud Logging for recent log entries and scans text payloads for PII. Config audit checks DLP configuration, service account key usage, and VPC Service Controls.",
		DocsURL:         "https://cloud.google.com/docs/authentication/getting-started",
	},
	{
		ID:       "azure",
		Name:     "Azure Monitor",
		Abbr:     "AZ",
		Color:    "#0078D4",
		Bg:       "rgba(0,120,212,0.12)",
		Category: "cloud",
		Description: "Microsoft Azure monitoring and logging. Azure Monitor logs, " +
			"Application Insights, and Log Analytics workspaces can aggregate " +
			"sensitive data across all Azure services.",
		AuthType: "oauth",
		AuthFields: []AuthField{
			{Key: "tenant_id", Label: "Tenant ID", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: false,
				HelpText: "Azure Active Directory → Properties → Tenant ID."},
			{Key: "client_id", Label: "App (Client) ID", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: false,
				HelpText: "App registrations → your app → Application (client) ID. Grant Log Analytics Reader role."},
			{Key: "client_secret", Label: "Client Secret", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "App registrations → Certificates & secrets → New client secret."},
			{Key: "workspace_id", Label: "Log Analytics Workspace ID", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: false},
		},
		ScanMode:        "both",
		ScanDescription: "Queries Azure Log Analytics for recent log entries and scans messages for PII. Config audit checks Application Insights sampling, workspace access controls, and Purview classification.",
		DocsURL:         "https://docs.microsoft.com/en-us/azure/active-directory/develop/quickstart-register-app",
	},
	{
		ID:       "digitalocean",
		Name:     "DigitalOcean",
		Abbr:     "DO",
		Color:    "#0080FF",
		Bg:       "rgba(0,128,255,0.12)",
		Category: "cloud",
		Description: "Cloud infrastructure provider. DigitalOcean Spaces object " +
			"storage buckets may be publicly accessible. App Platform environment " +
			"variables and Droplet user-data scripts can contain credentials and " +
			"personal configuration data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_token", Label: "Personal Access Token", Placeholder: "dop_v1_...", Secret: true,
				HelpText: "API → Tokens → Generate New Token (Read scope is sufficient)."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks Spaces bucket ACLs for public access, App Platform env vars for exposed secrets, and Droplet user-data for credential leakage.",
		DocsURL:         "https://docs.digitalocean.com/reference/api/create-personal-access-token/",
	},
	{
		ID:       "cloudflare",
		Name:     "Cloudflare",
		Abbr:     "CF",
		Color:    "#F48120",
		Bg:       "rgba(244,129,32,0.12)",
		Category: "infrastructure",
		Description: "CDN, DNS, and WAF. Cloudflare Workers scripts and Logpush " +
			"jobs can log full HTTP request bodies containing PII. WAF " +
			"misconfigurations may allow PII leakage in URLs and parameters.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_token", Label: "API Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "My Profile → API Tokens → Create Token. Use 'Read All Resources' template or scope to Zone:Analytics:Read and Zone:Workers Scripts:Read."},
			{Key: "account_id", Label: "Account ID", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: false,
				HelpText: "Found in the sidebar on any zone overview page."},
		},
		ScanMode:        "config",
		ScanDescription: "Config audit checks Workers scripts for body logging, Logpush job configurations for PII export, WAF custom rules, and Zero Trust network policies.",
		DocsURL:         "https://developers.cloudflare.com/fundamentals/api/get-started/create-token/",
	},
	// ── Email Delivery ────────────────────────────────────────────────────────
	{
		ID:       "postmark",
		Name:     "Postmark",
		Abbr:     "PM",
		Color:    "#FFDE00",
		Bg:       "rgba(255,222,0,0.12)",
		Category: "email",
		Description: "Transactional email delivery. Postmark stores sent message " +
			"bodies, recipients, and metadata. Outbound emails frequently contain " +
			"names, addresses, order details, and other personal information.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "server_token", Label: "Server API Token", Placeholder: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", Secret: true,
				HelpText: "Postmark account → Servers → your server → API Tokens → Server API Token."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent outbound messages and scans To/CC/Subject/body fields for PII. Config audit checks bounce and suppression list retention settings.",
		DocsURL:         "https://postmarkapp.com/developer/api/overview",
	},
	// ── Developer Tools ───────────────────────────────────────────────────────
	{
		ID:       "github",
		Name:     "GitHub",
		Abbr:     "GH",
		Color:    "#24292F",
		Bg:       "rgba(36,41,47,0.12)",
		Category: "developer-tools",
		Description: "Code hosting and collaboration. GitHub issues, pull request " +
			"descriptions, commit messages, and Actions logs can contain " +
			"credentials, PII, and internal system details committed by mistake.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "token", Label: "Personal Access Token", Placeholder: "ghp_... or github_pat_...", Secret: true,
				HelpText: "Settings → Developer settings → Personal access tokens → Tokens (classic). Needs repo and read:org scopes."},
			{Key: "org", Label: "Organization or Username", Placeholder: "my-org", Secret: false,
				HelpText: "The GitHub org or user whose repositories will be scanned."},
		},
		ScanMode:        "both",
		ScanDescription: "Scans recent issues and pull request bodies across org repositories for PII. Config audit checks secret scanning enablement, branch protection, and Actions secrets scope.",
		DocsURL:         "https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api",
	},
	{
		ID:       "dockerhub",
		Name:     "Docker Hub",
		Abbr:     "DH",
		Color:    "#2496ED",
		Bg:       "rgba(36,150,237,0.12)",
		Category: "developer-tools",
		Description: "Container image registry. Docker Hub repository descriptions " +
			"and image labels can expose internal URLs, credentials baked into " +
			"images, and build metadata referencing personal developer accounts.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "username", Label: "Docker Hub Username", Placeholder: "myusername", Secret: false},
			{Key: "pat", Label: "Personal Access Token", Placeholder: "dckr_pat_...", Secret: true,
				HelpText: "Account Settings → Security → Access Tokens → Generate New Token (Read-only)."},
		},
		ScanMode:        "config",
		ScanDescription: "Scans repository descriptions and image labels for hardcoded secrets, internal URLs, and PII in build metadata. Config audit checks public visibility and team access scope.",
		DocsURL:         "https://docs.docker.com/docker-hub/access-tokens/",
	},
	// ── Project Management ────────────────────────────────────────────────────
	{
		ID:       "jira",
		Name:     "Jira",
		Abbr:     "JR",
		Color:    "#0052CC",
		Bg:       "rgba(0,82,204,0.12)",
		Category: "project-management",
		Description: "Issue and project tracking. Jira ticket descriptions, " +
			"comments, and custom fields routinely contain customer names, " +
			"support case details, and personal data referenced in bug reports.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "base_url", Label: "Jira Cloud URL", Placeholder: "https://mycompany.atlassian.net", Secret: false,
				HelpText: "Your Atlassian Cloud URL (include https://)."},
			{Key: "email", Label: "Account Email", Placeholder: "admin@mycompany.com", Secret: false},
			{Key: "api_token", Label: "API Token", Placeholder: "ATATxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Atlassian account settings → Security → API tokens → Create API token."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent issues and scans summaries, descriptions, and custom field values for PII. Config audit checks project visibility, issue security schemes, and user directory exposure.",
		DocsURL:         "https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/#authentication",
	},
	// ── Productivity ──────────────────────────────────────────────────────────
	{
		ID:       "airtable",
		Name:     "Airtable",
		Abbr:     "AT",
		Color:    "#18BFFF",
		Bg:       "rgba(24,191,255,0.12)",
		Category: "productivity",
		Description: "Flexible database and collaboration platform. Airtable bases " +
			"are commonly used as lightweight CRMs and data stores that accumulate " +
			"contact information, customer data, and operational records.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Personal Access Token", Placeholder: "patXXXXXXXXXXXXXX.XXXXXXXX", Secret: true,
				HelpText: "Account → Developer hub → Personal access tokens → Create token. Grant data.records:read and schema.bases:read."},
		},
		ScanMode:        "both",
		ScanDescription: "Lists bases and scans table records for PII in text, email, phone, and URL fields. Config audit checks base sharing links and workspace member permissions.",
		DocsURL:         "https://airtable.com/developers/web/api/introduction",
	},
	{
		ID:       "notion",
		Name:     "Notion",
		Abbr:     "NT",
		Color:    "#000000",
		Bg:       "rgba(0,0,0,0.08)",
		Category: "productivity",
		Description: "Note-taking and wiki platform. Notion pages and databases " +
			"accumulate meeting notes, customer research, user interview " +
			"transcripts, and internal docs that often contain personal data.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "api_key", Label: "Internal Integration Token", Placeholder: "secret_...", Secret: true,
				HelpText: "notion.so/my-integrations → New integration → Internal. Share relevant pages with the integration."},
		},
		ScanMode:        "both",
		ScanDescription: "Searches pages and databases accessible to the integration and scans block content for PII in text, rich_text, email, and phone fields.",
		DocsURL:         "https://developers.notion.com/docs/authorization",
	},
	// ── Customer Support (additional) ─────────────────────────────────────────
	{
		ID:       "zoho-desk",
		Name:     "Zoho Desk",
		Abbr:     "ZH",
		Color:    "#E42527",
		Bg:       "rgba(228,37,39,0.12)",
		Category: "customer-support",
		Description: "Customer support and ticketing. Zoho Desk tickets contain " +
			"customer contact details, conversation history, and attachments that " +
			"accumulate PII across all support interactions.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "org_id", Label: "Organization ID", Placeholder: "123456789", Secret: false,
				HelpText: "Setup → Developer Space → API (OrgId shown on the page)."},
			{Key: "access_token", Label: "OAuth Access Token", Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", Secret: true,
				HelpText: "Use the Self Client flow: Setup → Developer Space → Self Client → Generate Code → exchange for access token with Desk.tickets.READ scope."},
		},
		ScanMode:        "both",
		ScanDescription: "Fetches recent tickets and scans subject, description, and custom field values for PII. Config audit checks department-level data sharing and ticket visibility rules.",
		DocsURL:         "https://desk.zoho.com/DeskAPIDocument#OauthTokens",
	},
	// ── Data Warehouse ────────────────────────────────────────────────────────
	{
		ID:       "snowflake",
		Name:     "Snowflake",
		Abbr:     "SN",
		Color:    "#29B5E8",
		Bg:       "rgba(41,181,232,0.12)",
		Category: "data-warehouse",
		Description: "Cloud data warehouse. Snowflake is often the final " +
			"destination of all PII — customer tables, transaction records, and " +
			"support data. A misconfigured Snowflake instance is a catastrophic " +
			"breach waiting to happen.",
		AuthType: "api_key",
		AuthFields: []AuthField{
			{Key: "account", Label: "Account Identifier", Placeholder: "myorg-myaccount", Secret: false,
				HelpText: "Account settings → Account URL. Use the format: org-account (without .snowflakecomputing.com)."},
			{Key: "username", Label: "Username", Placeholder: "NONYM_SCANNER", Secret: false,
				HelpText: "Create a dedicated read-only user and role."},
			{Key: "password", Label: "Password", Placeholder: "••••••••••••", Secret: true},
			{Key: "warehouse", Label: "Warehouse (optional)", Placeholder: "COMPUTE_WH", Secret: false},
			{Key: "database", Label: "Database (optional)", Placeholder: "PROD_DB", Secret: false},
		},
		ScanMode:        "both",
		ScanDescription: "Samples rows from tables to detect PII in column values. Config audit checks network policies, MFA enforcement, Dynamic Data Masking policies, and role privilege scope.",
		DocsURL:         "https://docs.snowflake.com/en/developer-guide/sql-api/index",
	},
}

// vendorCatalogueByID indexes VendorCatalogue for fast lookup.
var vendorCatalogueByID = func() map[string]*CatalogueEntry {
	m := make(map[string]*CatalogueEntry, len(VendorCatalogue))
	for i := range VendorCatalogue {
		m[VendorCatalogue[i].ID] = &VendorCatalogue[i]
	}
	return m
}()

// GetCatalogueEntry returns the CatalogueEntry for a vendor ID, or nil.
func GetCatalogueEntry(id string) *CatalogueEntry {
	return vendorCatalogueByID[id]
}

// HandleGetCatalogue serves the full scanner vendor catalogue.
// GET /api/v1/scanner/vendors/catalogue
func HandleGetCatalogue(c *fiber.Ctx) error {
	_, ok := c.Locals("organization_id").(int)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Authentication required"})
	}
	return c.JSON(fiber.Map{
		"vendors": VendorCatalogue,
		"total":   len(VendorCatalogue),
	})
}
