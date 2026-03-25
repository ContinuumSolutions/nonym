# Nonym Providers V2 — Top 40 Services: PII Scanning & Config Audit Strategy

> **Goal**: Ensure businesses are set up safely with their vendor stack. For each provider this document covers:
> 1. **PII Data Scanning** — what sensitive data flows through and how to intercept/redact it
> 2. **Config Audit** — misconfigurations that expose PII or create compliance risk
> 3. **Best Practice Recommendations** — actionable guidance for security teams

---

## Implementation Status

All 40 providers are now implemented in the backend:

| Layer | File | What it does |
|---|---|---|
| **Scanner Catalogue** | `pkg/scanner/catalogue.go` | `GET /api/v1/scanner/vendors/catalogue` — serves all 40 vendors with auth fields for the frontend connect modal |
| **VendorCatalog** | `pkg/router/vendors.go` | Extended from 5 → 40 vendors for the proxy/SDK-integration system |
| **Connector: Sentry** | `pkg/scanner/connector_sentry.go` | Existing — full data scan |
| **Connector: Datadog** | `pkg/scanner/connector_datadog.go` | Logs API v2 — real data scan |
| **Connector: PostHog** | `pkg/scanner/connector_posthog.go` | Events + Persons API — real data scan |
| **Connector: Mixpanel** | `pkg/scanner/connector_mixpanel.go` | Engage API (profiles) — real data scan |
| **Connector: New Relic** | `pkg/scanner/connector_newrelic.go` | NerdGraph NRQL — real data scan |
| **Connector: Rollbar** | `pkg/scanner/connector_rollbar.go` | Items API — real data scan |
| **Connector: Bugsnag** | `pkg/scanner/connector_bugsnag.go` | Errors API — real data scan |
| **Connector: Intercom** | `pkg/scanner/connector_intercom.go` | Conversations + Contacts — real data scan |
| **Connector: Zendesk** | `pkg/scanner/connector_zendesk.go` | Tickets + Users API — real data scan |
| **Connector: HubSpot** | `pkg/scanner/connector_hubspot.go` | CRM Contacts API — real data scan |
| **Connector: Stripe** | `pkg/scanner/connector_stripe.go` | Webhook config audit (no raw card data) |
| **Connector: SendGrid** | `pkg/scanner/connector_sendgrid.go` | Templates + suppression lists — data scan |
| **Connector: Twilio** | `pkg/scanner/connector_twilio.go` | Messages API — real data scan |
| **Connector: PagerDuty** | `pkg/scanner/connector_pagerduty.go` | Incidents API — real data scan |
| **Connector: Algolia** | `pkg/scanner/connector_algolia.go` | Index browse — real data scan |
| **Credential validation** | `pkg/scanner/vendor_connections.go` | `testConnection()` updated for all 40 vendors |
| **Route** | `cmd/gateway/main.go` | `GET /api/v1/scanner/vendors/catalogue` registered |

Vendors without dedicated connectors (Segment, Amplitude, Elastic, Splunk, Snowflake, AWS, GCP, Azure, Auth0, Okta, etc.) use `credentialEvents` fallback scanning and the live credential validation in `testConnection()`. Full connectors for these are Phase 2–4 work per the implementation roadmap.

---

## API Reference for Frontend

### Catalogue Endpoint

```
GET /api/v1/scanner/vendors/catalogue
Authorization: Bearer <jwt>
```

Response shape (matches `todo/vendor-catalogue.md`):
```json
{
  "vendors": [
    {
      "id": "sentry",
      "name": "Sentry",
      "abbr": "SE",
      "color": "#F55",
      "bg": "rgba(255,85,85,0.12)",
      "category": "error-tracking",
      "description": "...",
      "auth_type": "api_key",
      "auth_fields": [
        { "key": "api_token", "label": "Auth Token", "placeholder": "sntrys_...", "secret": true, "help_text": "..." },
        { "key": "org_slug", "label": "Organization Slug", "placeholder": "my-org", "secret": false }
      ],
      "scan_mode": "data",
      "scan_description": "...",
      "docs_url": "https://docs.sentry.io/api/auth/"
    }
  ],
  "total": 38
}
```

`scan_mode` values:
- `"data"` — Nonym can fetch and scan real vendor data for PII
- `"config"` — Config audit only (client-side/PCI-restricted vendors)
- `"both"` — Data scan + config audit

---

## Provider Setup Reference

### How to Get API Credentials

| Provider | Where to get credentials | Key field(s) | Scope needed |
|---|---|---|---|
| **Sentry** | Settings → Account → API → Auth Tokens | `api_token` | `org:read`, `project:read`, `event:read` |
| **Rollbar** | Project Settings → Access Tokens | `access_token` | `read` scope only |
| **Bugsnag** | Account Settings → Personal Auth Token | `auth_token` | Read-only |
| **Datadog** | Org Settings → API Keys + App Keys | `api_key`, `app_key` | `logs_read` |
| **New Relic** | API Key Mgmt → User key (NRAK-) | `api_key` + `account_id` | Read |
| **Elastic** | Stack Mgmt → API Keys | `api_key` + `host` | `read` on target indices |
| **Splunk** | Settings → Tokens | `token` + `host` | Search index access |
| **Sumo Logic** | Admin → Security → Access Keys | `access_id` + `access_key` | `Manage Collectors` |
| **Segment** | Workspace → Access Mgmt → Tokens | `access_token` + `workspace_slug` | Source Read |
| **Mixpanel** | Org Settings → Service Accounts | `service_account_user` + `service_account_secret` + `project_id` | Read |
| **Amplitude** | Project Settings → API Key + Secret | `api_key` + `secret_key` | Export API |
| **PostHog** | Settings → Personal API Keys | `api_key` + `project_id` | Project read |
| **Heap** | Account → API → Generate | `api_key` | Read |
| **Google Analytics 4** | GCP → Service Account | `property_id` + optional `service_account_json` | GA4 Viewer |
| **Hotjar** | Settings → Private API Key | `site_id` + `api_key` | Site read |
| **Intercom** | Developer Hub → Access Token | `access_token` | `read_conversations`, `read_contacts` |
| **Zendesk** | Admin Center → API → Token | `subdomain` + `email` + `api_token` | `tickets:read`, `users:read` |
| **HubSpot** | Settings → Private Apps | `access_token` | CRM Objects Read |
| **Salesforce** | Setup → Connected Apps → OAuth | `instance_url` + `access_token` | `api`, `read_only` |
| **Customer.io** | Settings → API Credentials | `api_key` + `site_id` | App API |
| **Stripe** | Developers → API Keys → Restricted | `restricted_key` | `read` on account, webhooks |
| **Twilio** | Console Dashboard | `account_sid` + `auth_token` | Read SMS logs |
| **SendGrid** | Settings → API Keys → Restricted | `api_key` | `Mail Send Read`, `Stats`, `Templates` |
| **Mailgun** | Account Settings → API Keys | `api_key` + `domain` | Private Key (read) |
| **Mailchimp** | Profile → Extras → API Keys | `api_key` | Read |
| **Slack** | App Settings → OAuth & Permissions | `bot_token` | `channels:read`, `chat:write` |
| **Auth0** | Applications → Management API token | `domain` + `management_token` | `read:logs`, `read:clients` |
| **Okta** | Admin → Security → API → Tokens | `org_url` + `api_token` | Read-only admin |
| **LaunchDarkly** | Account Settings → Access Tokens | `api_key` + `project_key` | Reader role |
| **Split.io** | Admin → API Keys → Admin type | `api_key` + `workspace_id` | Read |
| **Algolia** | Settings → API Keys | `app_id` + `api_key` | Search + Browse |
| **PagerDuty** | Integrations → API Access Keys | `api_key` | Read-only |
| **OpsGenie** | Settings → API Key Mgmt | `api_key` | Read |
| **AWS** | IAM → Users → Access Keys | `access_key_id` + `secret_access_key` + `region` | CloudWatch Logs Read, S3 Read |
| **GCP** | IAM → Service Accounts → JSON key | `project_id` + `service_account_json` | `logging.viewer` |
| **Azure** | App Registrations → Client Secret | `tenant_id` + `client_id` + `client_secret` + `workspace_id` | Log Analytics Reader |
| **Cloudflare** | My Profile → API Tokens | `api_token` + `account_id` | Zone Read, Workers Read |
| **Snowflake** | Dedicated read-only user | `account` + `username` + `password` | SELECT on target schemas |

---

## What Nonym Consumes Per Provider

| Provider | Scan Mode | What Nonym reads | Findings produced |
|---|---|---|---|
| Sentry | Data | Error events: title, message, user context, tags, breadcrumbs, exception values | PII in error payload |
| Datadog | Data + Config | Log events (Logs v2 API), custom attributes | PII in log messages |
| PostHog | Data + Config | Events (PII props only), person properties | PII in event/person data |
| Mixpanel | Data + Config | User profiles (`$distinct_id`, `$email`, `$name`, etc.) | PII in profiles |
| New Relic | Data + Config | Log messages via NerdGraph NRQL | PII in logs |
| Rollbar | Data | Error items: title | PII in error titles |
| Bugsnag | Data | Error messages and context fields | PII in errors |
| Intercom | Data + Config | Conversation subjects/bodies, contact email/name/phone | PII in support conversations |
| Zendesk | Data + Config | Ticket subjects/descriptions, user name/email/phone | PII in tickets |
| HubSpot | Data + Config | Contact properties: email, name, phone, address | PII in CRM contacts |
| Stripe | Config only | Webhook endpoint URLs, enabled events | Insecure webhook config |
| Twilio | Data + Config | SMS message from/to/body | PII in SMS messages |
| SendGrid | Data + Config | Template subjects, suppression list emails | PII in email templates |
| PagerDuty | Data + Config | Incident titles and summaries | PII in alert messages |
| Algolia | Data + Config | Index records (PII fields: email, name, phone, address) | PII in search index |
| Others | Config | Credential format + stored settings | Config issues via fallback scan |

---

## Documentation Notes

When writing user-facing documentation for each provider, include:

1. **Why connect this vendor** — what PII risk it poses, which regulations are affected
2. **How to get credentials** — step-by-step with screenshots, exact field names in the vendor UI
3. **Required permissions** — the minimum scope needed (principle of least privilege)
4. **What Nonym scans** — specific fields/endpoints inspected, data types detected
5. **Config checks performed** — which misconfigurations Nonym will flag and why they matter
6. **Quick remediation** — how to fix the most common issues Nonym finds
7. **Compliance coverage** — which framework articles are satisfied by connecting this vendor

---

## Coverage Summary

| # | Vendor | Category | PII Scanning | Config Audit | Risk Level |
|---|--------|----------|-------------|--------------|------------|
| 1 | Sentry | Error Tracking | ✅ | ✅ | Critical |
| 2 | Datadog | APM / Logging | ✅ | ✅ | Critical |
| 3 | Segment | CDP / Analytics | ✅ | ✅ | Critical |
| 4 | Mixpanel | Product Analytics | ✅ | ✅ | High |
| 5 | Amplitude | Product Analytics | ✅ | ✅ | High |
| 6 | PostHog | Product Analytics | ✅ | ✅ | High |
| 7 | Google Analytics 4 | Web Analytics | ⚠️ Config only | ✅ | High |
| 8 | Hotjar | Session Recording | ⚠️ Config only | ✅ | High |
| 9 | Intercom | Customer Messaging | ✅ | ✅ | High |
| 10 | Zendesk | Customer Support | ✅ | ✅ | High |
| 11 | HubSpot | CRM / Marketing | ✅ | ✅ | High |
| 12 | Salesforce | CRM | ✅ | ✅ | Critical |
| 13 | Stripe | Payments | ⚠️ Config only | ✅ | Critical |
| 14 | Twilio | SMS / Voice | ✅ | ✅ | High |
| 15 | SendGrid | Transactional Email | ✅ | ✅ | High |
| 16 | Mailchimp | Email Marketing | ✅ | ✅ | Medium |
| 17 | Mailgun | Transactional Email | ✅ | ✅ | Medium |
| 18 | New Relic | APM / Observability | ✅ | ✅ | Critical |
| 19 | Elastic (ELK) | Logging / Search | ✅ | ✅ | Critical |
| 20 | Splunk | SIEM / Logging | ✅ | ✅ | Critical |
| 21 | PagerDuty | Incident Management | ✅ | ✅ | Medium |
| 22 | OpsGenie | Incident Management | ✅ | ✅ | Medium |
| 23 | Rollbar | Error Tracking | ✅ | ✅ | High |
| 24 | Bugsnag | Error Tracking | ✅ | ✅ | High |
| 25 | Auth0 | Identity / AuthN | ⚠️ Config only | ✅ | Critical |
| 26 | Okta | Identity / SSO | ⚠️ Config only | ✅ | Critical |
| 27 | LaunchDarkly | Feature Flags | ✅ | ✅ | Medium |
| 28 | Split.io | Feature Flags | ✅ | ✅ | Medium |
| 29 | Algolia | Search | ✅ | ✅ | High |
| 30 | OpenAI | AI / LLM | ✅ | ✅ | Critical |
| 31 | Anthropic | AI / LLM | ✅ | ✅ | Critical |
| 32 | AWS (CloudWatch/S3) | Cloud Infrastructure | ✅ | ✅ | Critical |
| 33 | Google Cloud (GCP) | Cloud Infrastructure | ✅ | ✅ | Critical |
| 34 | Azure Monitor | Cloud Infrastructure | ✅ | ✅ | Critical |
| 35 | Cloudflare | CDN / WAF | ⚠️ Config only | ✅ | High |
| 36 | Snowflake | Data Warehouse | ✅ | ✅ | Critical |
| 37 | Heap | Product Analytics | ✅ | ✅ | High |
| 38 | Customer.io | Marketing Automation | ✅ | ✅ | High |
| 39 | Slack (Webhooks/Bots) | Team Communication | ✅ | ✅ | High |
| 40 | Sumo Logic | Log Management | ✅ | ✅ | High |

> **Legend**: ✅ = full data scanning possible | ⚠️ Config only = data scanning not feasible (client-side/encrypted at source), but config auditing delivers high value

---

## Risk Levels Explained

- **Critical** — Direct PII/PHI/PCI data commonly seen; regulatory fines imminent if not controlled
- **High** — PII often present; significant GDPR/CCPA exposure
- **Medium** — PII possible but less common; configuration risk primary concern

---

## Detailed Provider Profiles

---

### 1. Sentry — Error Tracking

**Why it's risky**: Sentry captures the full exception context: request headers, query params, POST bodies, user objects, breadcrumbs, and local variables. Developers routinely send full user context without scrubbing.

**Common PII in payloads**:
- User email/name/ID in `user` context
- Auth tokens and session cookies in request headers
- SSNs, DOBs, card numbers in form-body stack traces
- SQL queries with raw user data in breadcrumbs

**PII Scanning Strategy**:
- Intercept via Sentry's `beforeSend` hook or forward-proxy the DSN endpoint
- Scan: `event.user`, `event.request.headers`, `event.request.data`, `event.request.query_string`, `event.breadcrumbs`, `event.extra`, `event.contexts`
- Redact PII fields before forwarding to `https://sentry.io`

**Config Audit Checks**:
- [ ] Is `sendDefaultPii` set to `false`? (default is false, but many enable it)
- [ ] Are `denyUrls` / `allowUrls` configured to exclude internal admin paths?
- [ ] Is the DSN exposed client-side (browser JS bundle)? A public DSN allows anyone to POST to your project
- [ ] Is `attachStacktrace` paired with local variable capture (`includeLocalVariables`)? This captures every variable in scope
- [ ] Are Sentry data scrubbing rules configured in project settings (Settings → Security & Privacy → Data Scrubbing)?
- [ ] Is there a Data Processing Agreement (DPA) signed with Sentry?
- [ ] For EU orgs: is the Sentry instance on the EU region (`sentry.io/region/eu`)?

**Best Practice**:
```javascript
Sentry.init({
  dsn: "...",
  sendDefaultPii: false, // NEVER set true without a scrubbing layer
  beforeSend(event) {
    // Strip all user data at source
    delete event.user;
    delete event.request?.headers?.cookie;
    delete event.request?.headers?.authorization;
    return event;
  }
});
```

---

### 2. Datadog — APM & Log Management

**Why it's risky**: Datadog receives logs, traces, and metrics from every service. A single `logger.info(req)` can dump an entire HTTP request including auth tokens and bodies.

**Common PII in payloads**:
- Full HTTP request/response bodies in APM traces
- Database queries with raw data (SQL, MongoDB queries)
- Log lines with email addresses, phone numbers, card numbers
- Custom tags that include user identifiers

**PII Scanning Strategy**:
- Proxy the Datadog agent endpoint: `DD_PROXY_HTTPS`, `DD_PROXY_HTTP`
- Alternatively intercept via the Datadog Agent's log processing pipeline
- Target endpoints: `https://http-intake.logs.datadoghq.com`, `https://trace.agent.datadoghq.com`
- Scan: log `message` fields, APM span `resource`, `meta`, `metrics` attributes, custom tags

**Config Audit Checks**:
- [ ] Are log scrubbing rules configured in `datadog.yaml` (`replace_rules`)?
- [ ] Is `logs_config.container_collect_all: false`? (collecting all container logs is a common PII leak)
- [ ] Is APM `analytics_enabled` capturing all traces at 100% (cost and PII risk)?
- [ ] Are `sensitive_data_scanner` rules enabled in Datadog's UI for PCI/HIPAA patterns?
- [ ] Is the API key scoped minimally (not using Admin API key in production agents)?
- [ ] For EU: is the intake endpoint set to `datadoghq.eu`?
- [ ] Are tags like `usr.email`, `usr.id` being set carelessly on all traces?

**Best Practice**:
```yaml
# datadog.yaml
logs_config:
  processing_rules:
    - type: mask_sequences
      name: mask_credit_cards
      pattern: '\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b'
      replace_placeholder: "[CREDIT_CARD_REDACTED]"
    - type: mask_sequences
      name: mask_emails
      pattern: '[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+'
      replace_placeholder: "[EMAIL_REDACTED]"
```

---

### 3. Segment — Customer Data Platform (CDP)

**Why it's risky**: Segment is designed to receive and relay user events to dozens of downstream tools. It is the highest-leverage interception point — PII placed here fans out to every connected destination.

**Common PII in payloads**:
- `identify()` calls: email, name, phone, address, DOB
- `track()` calls: can contain arbitrary properties with PII
- `page()` calls: URL may contain user identifiers
- Cross-device linking creates a full user profile

**PII Scanning Strategy**:
- Intercept via Segment's `middleware` (source middleware runs before events leave)
- Or proxy the Segment API endpoint: `https://api.segment.io/v1/`
- Scan: `traits` object in `identify`, `properties` in `track`, `context.page.url`
- Apply field-level masking before data reaches destinations

**Config Audit Checks**:
- [ ] Are Destination Filters configured per-destination to block PII fields going to analytics tools?
- [ ] Is `context.ip` being suppressed (`integrations.All.ip: false`)?
- [ ] Are write keys exposed in client-side code (JavaScript source)?
- [ ] Is the Privacy Portal configured with a data inventory and field classification?
- [ ] Are GDPR delete/suppress requests flowing through Segment's Privacy API?
- [ ] Is there a DPA in place with Segment (Twilio)?
- [ ] Are `anonymousId` and `userId` truly anonymous (not email addresses used as userId)?

---

### 4. Mixpanel — Product Analytics

**Common PII in payloads**: User properties (`$email`, `$name`, `$phone`), event properties with form inputs, `$distinct_id` set to email or username.

**PII Scanning Strategy**:
- Intercept via Mixpanel's JavaScript `before_send` hook or server-side SDK middleware
- Proxy: `https://api.mixpanel.com/track`, `/engage`
- Scan: `$set` operations in `/engage`, all custom event properties in `/track`

**Config Audit Checks**:
- [ ] Is `$distinct_id` set to a UUID (not email/username)?
- [ ] Is IP collection disabled (`ip: 0` in API calls)?
- [ ] Is `$email` property only set in user profiles (not in events)?
- [ ] Is EU residency option enabled (for EU companies)?
- [ ] Are Data Views configured to restrict access to PII properties per-role?

---

### 5. Amplitude — Product Analytics

**Common PII**: Same as Mixpanel. Additionally, Amplitude's autocapture can grab form field values.

**PII Scanning Strategy**: Proxy `https://api2.amplitude.com/2/httpapi`. Scan `user_properties`, `event_properties`, `groups`.

**Config Audit Checks**:
- [ ] Is `optOut` available to users and functioning?
- [ ] Are blocked properties configured (Settings → Govern → Block Properties) for known PII fields?
- [ ] Is Amplitude's EU data residency enabled if required?
- [ ] Is the API key exposed in client-side bundles (obfuscation is not security)?

---

### 6. PostHog — Product Analytics (Self-hosted & Cloud)

**Common PII**: Session recordings capture full keystrokes including passwords. Autocapture grabs all DOM elements by default.

**PII Scanning Strategy**:
- Server-side: proxy `/capture/` and `/batch/` endpoints
- Scan `properties`, `$set`, `$set_once` fields
- For session recordings: flag that PostHog ingests them directly (no proxy feasible for recording streams — must use config controls)

**Config Audit Checks**:
- [ ] Is `mask_all_inputs: true` set in session recording config? (Critical — prevents password capture)
- [ ] Is `mask_all_element_attributes: true` enabled?
- [ ] Are specific input selectors blocked via `mask_input_options`?
- [ ] Is autocapture scoped with `autocapture_opt_out` on sensitive pages?
- [ ] For self-hosted: is the instance on a private network with access controls?
- [ ] Is `disable_session_recording` true on pages with PCI/HIPAA data (checkout, patient portals)?

---

### 7. Google Analytics 4 — Web Analytics

**Note**: Data scanning is not feasible — GA4 uses a first-party JavaScript tag. The risk is misconfiguration sending PII to Google.

**Config Audit Checks**:
- [ ] Are custom dimensions/metrics sending PII (names, emails, user IDs that resolve to real people)?
- [ ] Is the `anonymize_ip` configuration active (GA4 anonymizes by default but verify)?
- [ ] Are URL query parameters that contain PII (e.g., `?email=`, `?token=`) excluded via URL parameter exclusion?
- [ ] Is Google Signals disabled if you don't need cross-device measurement (reduces data sharing with Google)?
- [ ] Is data retention set to the minimum needed (not "Do not automatically expire")?
- [ ] Is a Consent Mode (v2) implementation in place for EU users?
- [ ] Is the Measurement ID (`G-XXXXXXXX`) appropriately domain-restricted?

---

### 8. Hotjar — Session Recording & Heatmaps

**Note**: Hotjar runs client-side; server-side proxy is not applicable. Config controls are the primary defense.

**Config Audit Checks** (Critical — session recorders are the highest PII risk of any tool):
- [ ] Is "Suppress All Text" enabled on pages with PCI/HIPAA data?
- [ ] Are all form inputs masked (Settings → Sites → Suppress Text)?
- [ ] Is the recording script excluded from `/checkout`, `/account`, `/profile`, `/admin`, `/health`, `/payment` pages?
- [ ] Is IP anonymization enabled (Site Settings → Privacy)?
- [ ] Is a DPA signed with Hotjar?
- [ ] Is there a cookie consent mechanism gating Hotjar's activation for EU users?

---

### 9. Intercom — Customer Messaging & Support

**Common PII**: Full conversation transcripts, user-sent files, `company` and `user` attributes, custom attributes storing PII.

**PII Scanning Strategy**:
- Intercept server-side API calls to `https://api.intercom.io/` (notes, tags, conversations)
- Scan: `body` of conversation messages, `custom_attributes` on user/lead objects
- For inbound messages from customers: flag that scanning must happen on receipt (webhook payload)

**Config Audit Checks**:
- [ ] Is the Intercom Messenger excluded from pages that show PCI/PHI data?
- [ ] Are custom user attributes storing sensitive fields (SSN, DOB, medical record numbers)?
- [ ] Is the workspace on Intercom's EU data hosting if required?
- [ ] Is team inbox access controlled by role (not all agents can see all conversations)?
- [ ] Are Intercom webhooks sent to secure endpoints with signature verification?

---

### 10. Zendesk — Customer Support Platform

**Common PII**: Ticket bodies, user profiles, attachments, custom fields, macros that expose PII.

**PII Scanning Strategy**:
- Intercept via Zendesk API (`/api/v2/tickets`, `/api/v2/users`) when programmatically creating tickets
- Proxy ticket creation webhooks from your app to Zendesk
- Scan: `description`, `comment.body`, `user.email`, `user.phone`, custom ticket fields

**Config Audit Checks**:
- [ ] Are custom ticket fields storing sensitive data (PHI, financial info) classified?
- [ ] Is sensitive ticket data redacted before tickets are shared externally?
- [ ] Is end-user PII being included in ticket subject lines (leaked in email notifications)?
- [ ] Is HIPAA BAA in place if healthcare data is in tickets?
- [ ] Is SSO enforced for all agents (no password-based access)?
- [ ] Is field-level data encryption enabled for sensitive custom fields?

---

### 11. HubSpot — CRM & Marketing

**Common PII**: Contact records (full name, email, phone, company, deal value), form submissions, email campaign data.

**PII Scanning Strategy**:
- Intercept HubSpot API calls from your application: `https://api.hubapi.com/crm/v3/objects/contacts`
- Scan `properties` object in contact/deal creation and update calls
- For forms: intercept form submission endpoint before HubSpot receives it

**Config Audit Checks**:
- [ ] Is the HubSpot tracking pixel excluded from pages with sensitive data?
- [ ] Are custom contact properties storing fields that exceed what marketing needs (SSN, DOB, medical)?
- [ ] Is data sharing with HubSpot's ad partners (`Allow HubSpot to use contact data for ads`) disabled?
- [ ] Are private app tokens scoped minimally (not using full-access tokens for read-only integrations)?
- [ ] Is GDPR Tools enabled (Settings → Privacy & Consent)?

---

### 12. Salesforce — CRM

**Common PII**: Contact, Lead, Account, Opportunity data. Often stores the most sensitive customer business data.

**PII Scanning Strategy**:
- Intercept Salesforce REST API calls from your backend
- Scan: `Contact`, `Lead`, `Account`, `Opportunity`, `Case` object payloads
- Flag custom objects that store PII in unencrypted text fields

**Config Audit Checks**:
- [ ] Are Salesforce Shield (Platform Encryption) features enabled for sensitive fields?
- [ ] Is Field Audit Trail enabled to track who accessed PII fields?
- [ ] Are profiles and permission sets scoped per-role (principle of least privilege)?
- [ ] Is Event Monitoring enabled to detect bulk data exports?
- [ ] Are connected apps (OAuth integrations) reviewed for excessive scopes?
- [ ] Is multi-factor authentication enforced for all users?
- [ ] Is IP restriction configured to limit access to known office/VPN IPs?

---

### 13. Stripe — Payments

**Note**: Stripe's client libraries handle card data directly and are PCI-compliant by design. DO NOT proxy raw card data — this breaks PCI-DSS scope reduction.

**Config Audit Checks** (Config is the only safe vector here):
- [ ] Is Stripe.js used (client-side tokenization) rather than sending raw card data to your server?
- [ ] Is the webhook endpoint validating Stripe signatures (`stripe-signature` header)?
- [ ] Are Stripe webhook payloads logged in your own systems? (They contain customer details)
- [ ] Are Stripe API keys rotated and stored in a secrets manager (not env var in code)?
- [ ] Is the Stripe publishable key domain-restricted in the Stripe Dashboard?
- [ ] Are restricted API keys used for each service (not the full secret key)?
- [ ] Is Stripe Radar configured to enforce risk rules?
- [ ] Is Stripe's Customer Portal used for self-service billing (avoids your app storing billing context)?

---

### 14. Twilio — SMS, Voice & Email

**Common PII**: Phone numbers, SMS message bodies (may contain OTPs, PII), call recordings, WhatsApp messages.

**PII Scanning Strategy**:
- Intercept outbound Twilio REST API calls: `https://api.twilio.com/2010-04-01/Accounts/{SID}/Messages`
- Scan: `Body` field of SMS messages for PII being sent to users (e.g., messages templated with name/address)
- For inbound: scan webhook payloads `Body`, `From` fields

**Config Audit Checks**:
- [ ] Are call recordings enabled? If so, is auto-delete configured?
- [ ] Are call recording URLs (signed S3 URLs) being logged in your system?
- [ ] Is SMS content logged in your application logs? (OTPs, verification codes)
- [ ] Is your Twilio account using subaccounts to isolate production data?
- [ ] Is MFA enabled on the Twilio console account?
- [ ] Are API keys (not account SID/auth token) used in application code?

---

### 15. SendGrid — Transactional Email

**Common PII**: Email recipient addresses, email bodies (may contain order details, personal info, links with PII in query params).

**PII Scanning Strategy**:
- Intercept: `POST https://api.sendgrid.com/v3/mail/send`
- Scan: `personalizations[].to`, `personalizations[].dynamic_template_data`, `content[].value`
- Flag: URLs in email bodies that contain PII query parameters

**Config Audit Checks**:
- [ ] Are email templates storing PII in dynamic data beyond what's needed?
- [ ] Is the SendGrid API key scoped to Mail Send only (not Full Access)?
- [ ] Is a suppression list/unsubscribe mechanism active (GDPR/CAN-SPAM)?
- [ ] Are dedicated IP addresses used (shared IPs can cause reputation issues)?
- [ ] Is click tracking embedding PII in redirect URLs?

---

### 16–17. Mailchimp & Mailgun

**Similar to SendGrid**. Additionally:
- Mailchimp: Audience segments may store sensitive behavioral or health-related tags — audit tag taxonomy
- Mailgun: Route rules that forward emails to third parties are a common data leak vector

---

### 18. New Relic — APM & Observability

**Common PII**: Same profile as Datadog. New Relic's browser agent also captures frontend JS errors with full context.

**PII Scanning Strategy**: Proxy `https://collector.newrelic.com` for APM data. Scan span attributes, log messages, error traces.

**Config Audit Checks**:
- [ ] Is `high_security: true` enabled in `newrelic.yml`? (This prevents custom attributes from being sent — consider if too restrictive)
- [ ] Are `strip_exception_messages` settings configured for sensitive exception types?
- [ ] Is the Browser agent excluded from pages with sensitive forms?
- [ ] Are custom attributes (`addCustomAttribute`) sending user PII to New Relic?
- [ ] Is the license key stored securely (not hardcoded in repo)?

---

### 19. Elastic (ELK Stack) — Logging & Search

**Why it's risky**: Often self-hosted but connected to cloud. Full log ingestion with no default PII filtering. Elasticsearch indices are frequently misconfigured to be publicly accessible.

**PII Scanning Strategy**:
- Intercept at the Logstash/Filebeat pipeline level via Nonym filter plugin
- Or proxy the Elasticsearch bulk ingest endpoint: `POST /_bulk`
- Scan: all log `message` fields, structured log attributes

**Config Audit Checks**:
- [ ] Is Elasticsearch on a public IP without authentication? (This has caused numerous high-profile data breaches)
- [ ] Is X-Pack Security (TLS + authentication) enabled?
- [ ] Are index templates using field-level encryption for PII fields?
- [ ] Is ILM (Index Lifecycle Management) configured to delete old indices automatically?
- [ ] Are Kibana dashboards access-controlled per team?
- [ ] Is audit logging enabled to track who queries PII data?

---

### 20. Splunk — SIEM & Log Management

**Common PII**: Everything from application logs, network logs, authentication events. Splunk often stores the most comprehensive PII dataset in an organization.

**PII Scanning Strategy**:
- Intercept at the Splunk Universal Forwarder before data leaves the host
- Or scan HEC (HTTP Event Collector) endpoint: `POST /services/collector`
- Use Splunk's built-in `sed` command to mask in search-time (not ideal — mask at ingest time)

**Config Audit Checks**:
- [ ] Are `props.conf` SEDCMD rules configured for masking PII at index time?
- [ ] Is access to PII-containing indexes role-restricted?
- [ ] Is the HEC token scoped to specific indexes (not the default catch-all)?
- [ ] Is data retention configured per index with appropriate PII data lifetimes?
- [ ] Is Splunk Cloud in the correct data residency region?

---

### 21–22. PagerDuty & OpsGenie — Incident Management

**Common PII**: Alert payloads that include customer data (e.g., "User john@example.com cannot log in"), on-call engineer phone numbers.

**PII Scanning Strategy**:
- Intercept outbound alert creation API calls
- Scan: `payload.summary`, `payload.custom_details`, `body` fields before sending to paging services

**Config Audit Checks**:
- [ ] Are alert payloads stripping PII before inclusion in page body (phone/SMS delivery)?
- [ ] Are on-call schedules and personal phone numbers access-controlled?
- [ ] Are runbook links in alerts pointing to secure, authenticated documentation?

---

### 23–24. Rollbar & Bugsnag — Error Tracking

**Same profile as Sentry** with slight differences:
- Rollbar: `person.email`, `request.params` are common PII sources. `scrub_fields` config list should include all PII fields
- Bugsnag: `user.email`, `metaData` objects. Use `addOnError` callback to strip PII. Enable "On-premise" for HIPAA

---

### 25. Auth0 — Identity Platform

**Note**: Auth0 handles authentication tokens — intercepting token flows would break security. Config auditing is the primary value.

**Config Audit Checks** (High value):
- [ ] Are Rules/Actions logging full user profiles to external services (a common misconfiguration)?
- [ ] Is the Management API token scoped minimally? Management API tokens are extremely powerful
- [ ] Are refresh token rotation and absolute expiry configured?
- [ ] Is anomaly detection (brute force protection, breached password detection) enabled?
- [ ] Are legacy grant types (Implicit Flow, Resource Owner Password) disabled?
- [ ] Is the `allowed_origins` for CORS locked down (not `*`)?
- [ ] Are log streams configured to send Auth0 logs to a SIEM? (Auth0 logs contain login history with IPs — PII)
- [ ] Are custom claims in JWT tokens including unnecessary PII?
- [ ] Is MFA enforced for admin users of the Auth0 tenant?

---

### 26. Okta — Enterprise Identity & SSO

**Config Audit Checks**:
- [ ] Are Okta event hooks (webhooks) sending user profile data to third parties?
- [ ] Are SCIM provisioning integrations scoped to only necessary user attributes?
- [ ] Is the Okta System Log forwarded to a SIEM? (Contains PII in login events)
- [ ] Are API tokens scoped and rotated regularly?
- [ ] Is phishing-resistant MFA (WebAuthn/FIDO2) enforced for privileged users?
- [ ] Are inactive users/accounts automatically deprovisioned?

---

### 27–28. LaunchDarkly & Split.io — Feature Flags

**Common PII**: Feature flag evaluation contexts often include `user.key` set to email, `user.email`, `user.country`, leading to PII stored in flag analytics dashboards.

**PII Scanning Strategy**:
- Intercept SDK initialization and flag evaluation calls
- Scan: user context objects passed to `variation()` calls
- Ensure `user.key` is a UUID, not email

**Config Audit Checks**:
- [ ] Is `user.key` set to an opaque ID (UUID) rather than email/username?
- [ ] Are custom user attributes storing PII beyond what's needed for targeting?
- [ ] Are flag evaluation analytics retained longer than necessary?
- [ ] Is anonymous users' data segregated from identified user data?

---

### 29. Algolia — Search-as-a-Service

**Common PII**: Search indices may contain user-generated content with PII. Query logs capture what users searched for (which can be PII itself).

**PII Scanning Strategy**:
- Intercept `POST /1/indexes/{indexName}/batch` calls to scan records being indexed
- Scan indexed object fields for embedded PII before they reach Algolia's servers

**Config Audit Checks**:
- [ ] Are search indices containing personal information (user profiles, orders, messages) audited?
- [ ] Are `attributesForFaceting` exposing PII as filterable facets?
- [ ] Is query logging disabled or anonymized for GDPR compliance?
- [ ] Are Algolia API keys scoped per-application with `restrictIndices`?
- [ ] Is the Search-Only API key used client-side (not the Admin API key)?
- [ ] Are `unretrievableAttributes` configured for sensitive fields that must exist in the index but not be returned?

---

### 30–31. OpenAI & Anthropic — AI / LLM Providers

**Why it's critical**: LLM prompts often contain unstructured text with the highest concentration of PII — customer support threads, medical queries, legal documents.

**PII Scanning Strategy** *(Nonym's core capability — extend to all models)*:
- Intercept: `POST /v1/chat/completions`, `/v1/completions`, `/v1/embeddings`
- Also intercept: `/v1/files` (training data uploads), `/v1/fine-tuning/jobs`
- Scan: `messages[].content`, `input` field for embeddings, file uploads for fine-tuning
- De-anonymize responses before returning to client

**Config Audit Checks**:
- [ ] Is data-opt-out enabled (OpenAI: `X-OpenAI-Data-Usage: false` header, or API usage terms)?
- [ ] Are API keys stored in a secrets manager (not in code or `.env` files committed to repos)?
- [ ] Are API keys rotated and scoped per service?
- [ ] Is fine-tuning dataset reviewed for PII before upload?
- [ ] Are assistants/threads storing conversation history with PII longer than necessary?

---

### 32. AWS — Cloud Infrastructure (CloudWatch, S3, Lambda Logs)

**Common PII**: CloudWatch Logs capture everything Lambda functions print. S3 buckets may store PII files. CloudTrail contains user actions.

**PII Scanning Strategy**:
- Intercept CloudWatch Logs using Lambda subscription filters → route through Nonym scanner before archiving
- For S3: scan objects on `s3:ObjectCreated:*` events via Lambda trigger
- For SQS/SNS: scan message bodies in transit

**Config Audit Checks**:
- [ ] Are S3 buckets blocking public access (`Block All Public Access` = ON)?
- [ ] Is S3 server-side encryption enabled for buckets containing PII?
- [ ] Is CloudTrail enabled in all regions and log file validation active?
- [ ] Are CloudWatch Log groups encrypted with KMS?
- [ ] Are CloudWatch Log retention policies set (not "Never expire")?
- [ ] Is AWS Macie enabled to automatically discover PII in S3?
- [ ] Are Lambda environment variables encrypted and not containing raw secrets?
- [ ] Are IAM roles following least privilege (no `*` actions on `*` resources)?
- [ ] Is VPC Flow Logs enabled and stored securely?
- [ ] Are RDS instances not publicly accessible?

---

### 33. Google Cloud Platform (GCP) — Cloud Infrastructure

**Config Audit Checks**:
- [ ] Is Cloud Logging configured to exclude sensitive request payloads?
- [ ] Is Cloud DLP (Data Loss Prevention) enabled to scan Cloud Storage for PII?
- [ ] Are service account keys avoided in favor of Workload Identity Federation?
- [ ] Is VPC Service Controls configured to prevent data exfiltration?
- [ ] Are BigQuery datasets encrypted with CMEK for PII tables?
- [ ] Are Cloud Audit Logs retained per compliance requirements?

---

### 34. Azure Monitor — Cloud Infrastructure

**Config Audit Checks**:
- [ ] Are Log Analytics workspaces access-controlled?
- [ ] Is Azure Purview deployed for data classification?
- [ ] Are Application Insights configured to anonymize user IDs?
- [ ] Is diagnostic settings shipping logs to a SIEM?
- [ ] Are Azure Key Vault access policies reviewed and scoped?

---

### 35. Cloudflare — CDN, DNS & WAF

**Note**: Cloudflare terminates TLS and sees all traffic. It is a configuration-risk vendor, not a data-scanning target via Nonym.

**Config Audit Checks**:
- [ ] Are Cloudflare Workers scripts logging request bodies to storage?
- [ ] Is Cloudflare Logs (Logpush) shipping full HTTP request/response bodies somewhere?
- [ ] Is Zero Trust (ZTNA) configured to restrict access to internal applications?
- [ ] Is the WAF configured with OWASP ruleset and PII exfiltration rules?
- [ ] Are Page Rules or Transform Rules masking PII from URL parameters before logging?
- [ ] Is Cloudflare's `privacy_pass` or bot management logging user fingerprints?
- [ ] Are DNS records protected against zone transfer and subdomain takeover?

---

### 36. Snowflake — Data Warehouse

**Why it's critical**: Snowflake is often the final destination of all PII — customer tables, transaction records, support data, analytics. A misconfigured Snowflake instance is a catastrophic breach.

**PII Scanning Strategy**:
- Scan data at ingestion time (before it lands in Snowflake) via pipeline proxying
- Use Snowflake's Data Classification feature + Dynamic Data Masking policies to enforce at query time

**Config Audit Checks**:
- [ ] Is network policy configured to restrict access to known IPs (VPN, data center)?
- [ ] Is MFA enforced for all users?
- [ ] Are Column-Level Security (Dynamic Data Masking) policies applied to PII columns?
- [ ] Is Row Access Policy enforcing data segregation by team?
- [ ] Is Tri-Secret Secure (customer-managed key) enabled for highly sensitive data?
- [ ] Is query history audit enabled and stored securely?
- [ ] Are `ACCOUNTADMIN` and `SECURITYADMIN` roles used minimally and not for day-to-day work?
- [ ] Is time travel retention set appropriately (not excessively long for PII tables)?
- [ ] Are external stages (S3/GCS) using private storage integrations (not public URLs)?

---

### 37. Heap — Product Analytics (Autocapture)

**Why it's high risk**: Heap's autocapture records every user interaction including form inputs, text selections, and clicked elements without requiring code. This is the most likely tool to accidentally capture passwords, SSNs, and health data.

**Config Audit Checks**:
- [ ] Is `rewrite` configured to sanitize user-typed values before Heap captures them?
- [ ] Are sensitive pages (checkout, health forms, authentication) excluded via `heap.resetIdentity()`?
- [ ] Is `heap.clearEventProperties()` called between sessions to prevent PII carryover?
- [ ] Are global data redaction rules set in Heap's Privacy Settings?
- [ ] Is the Heap project ID domain-restricted?

---

### 38. Customer.io — Marketing Automation

**Common PII**: Customer profiles, event attributes, email/SMS content with personalization tokens.

**PII Scanning Strategy**:
- Intercept: `POST https://track.customer.io/api/v1/customers/{id}` (identify), `/events` (track)
- Scan: customer attributes, event data properties

**Config Audit Checks**:
- [ ] Are customer attributes storing more PII than needed for campaigns?
- [ ] Is data expiry configured for inactive customer profiles?
- [ ] Are transactional messages (which can contain full order details) audited?
- [ ] Is the site ID and API key stored securely?

---

### 39. Slack (Webhooks, Bots & Apps) — Team Communication

**Common PII**: Webhook notifications often include customer names, errors with PII in message text, alerts with raw data.

**PII Scanning Strategy**:
- Intercept outbound webhook calls: `POST https://hooks.slack.com/services/...`
- Scan: `text`, `blocks[].text`, `attachments[].text` fields before sending

**Config Audit Checks**:
- [ ] Are Slack incoming webhook URLs stored securely (not in public repos)?
- [ ] Are bot tokens scoped minimally (not requesting `channels:history` if not needed)?
- [ ] Are Slack apps using OAuth 2.0 (not legacy tokens)?
- [ ] Is Slack Enterprise Grid configured with DLP integrations?
- [ ] Are channels containing customer PII private (not public workspace channels)?
- [ ] Is message retention policy set to comply with data residency requirements?

---

### 40. Sumo Logic — Log Management

**Common PII**: Application logs, security logs, cloud logs — all potentially containing PII.

**PII Scanning Strategy**: Same as Splunk/Datadog. Intercept at the Sumo Logic collector before data leaves the host using Processing Rules.

**Config Audit Checks**:
- [ ] Are Sumo Logic Processing Rules configured to mask PII at collection time?
- [ ] Are Field Extraction Rules creating structured fields that expose PII as searchable attributes?
- [ ] Is access to PII-containing partitions role-restricted?
- [ ] Is the Sumo Logic deployment in the correct geographic region for data residency?

---

## Strategic Framework: Scanning vs Config Audit

### When Full Data Scanning is Possible (Proxy/SDK Intercept)

These providers accept server-side API calls or can be proxied:

| Approach | Providers | How to Intercept |
|---|---|---|
| **HTTPS Forward Proxy** | Sentry, Datadog, New Relic, Rollbar, Bugsnag, Sumo Logic | `HTTPS_PROXY` env var or SDK transport override |
| **SDK Middleware** | Segment, Mixpanel, Amplitude, PostHog, LaunchDarkly, OpenAI, Anthropic | Wrap SDK before/after hooks |
| **API Interception** | HubSpot, Intercom, Zendesk, SendGrid, Mailgun, Twilio, Salesforce, Algolia, Customer.io | Route app's API calls through Nonym gateway |
| **Webhook Scanning** | Slack, PagerDuty, OpsGenie | Proxy webhook `POST` calls |
| **Pipeline Filter** | Elastic/ELK, Splunk, Sumo Logic | Filebeat/Logstash/UF filter plugin |

### When Config Audit is the Primary Defense

These providers use client-side JavaScript, certificate pinning, or handle sensitive data natively:

| Provider | Why No Data Scanning | Config Audit Focus |
|---|---|---|
| Google Analytics 4 | Client-side JS tag, data goes directly to Google | PII in custom dimensions, consent mode |
| Hotjar | Client-side recording, streams directly to Hotjar | Masking rules, page exclusions |
| Stripe | PCI-mandated client-side tokenization | Webhook security, key scoping |
| Auth0 / Okta | Auth flows must be direct (MITM breaks security) | JWT claims, log streaming, Actions/Rules |
| Cloudflare | Sits in front of all traffic, terminates TLS before app | Workers logging, Logpush config |

---

## PII Data Types to Scan (Universal)

```
TIER 1 — CRITICAL (always redact):
├── Financial: credit card numbers (Luhn), CVV, bank account numbers, routing numbers
├── Government ID: SSN (US), NIN (UK), SIN (CA), passport numbers, driver's license
├── Healthcare: ICD codes, medication names in context, insurance member IDs
└── Authentication: passwords, API keys, bearer tokens, session tokens, JWTs, private keys

TIER 2 — HIGH (redact in most contexts):
├── Direct identifiers: email addresses, full names, phone numbers
├── Location: street addresses, GPS coordinates, IP addresses (GDPR: personal data)
└── Financial (less critical): IBAN, BIC/SWIFT codes, transaction IDs

TIER 3 — CONTEXTUAL (redact based on vendor type):
├── Quasi-identifiers: DOB, postal code, gender, age, nationality
├── Device identifiers: device ID, MAC address, advertising ID
└── Account identifiers: usernames, internal user IDs (if linkable to real person)
```

---

## Config Audit Checklist (Universal — Apply to Every Vendor)

Every vendor integration should be audited against these universal controls:

### Access & Authentication
- [ ] API keys/secrets stored in a secrets manager (AWS Secrets Manager, HashiCorp Vault, GCP Secret Manager)
- [ ] API keys scoped to minimum required permissions (not Admin/Full Access keys in production)
- [ ] API keys rotated at least annually (or on team member departure)
- [ ] MFA enforced for vendor console/dashboard access
- [ ] SSO (SAML/OIDC) enabled for team access where available

### Data Minimization
- [ ] Only fields required for the vendor's function are sent (not full object serialization)
- [ ] User identifiers are opaque UUIDs (not email addresses used as IDs)
- [ ] IP address collection is disabled or anonymized where not needed
- [ ] Data retention is configured to auto-delete after the minimum required period

### Data Residency & Legal
- [ ] Vendor data residency region matches your compliance requirements (EU, US)
- [ ] Data Processing Agreement (DPA) signed with vendor
- [ ] Vendor is listed in your GDPR Article 30 Records of Processing Activities (RoPA)
- [ ] Vendor is listed in your privacy policy

### Network & Transport
- [ ] All connections to vendor use TLS 1.2+ (no HTTP)
- [ ] Webhook endpoints validate vendor signatures (prevent spoofed events)
- [ ] SDK versions are up-to-date (old SDKs may have known data leaks)

### Logging & Audit Trail
- [ ] Vendor activity logs are forwarded to your central SIEM
- [ ] Access to PII in vendor dashboards is role-controlled and logged
- [ ] Data subject deletion/export requests can be fulfilled via vendor API

---

## Implementation Roadmap for Nonym V2

### Phase 1 — High-Impact, Low-Effort (Months 1–2)
Focus on vendors where Nonym's existing proxy model extends naturally:

1. **Sentry** — SDK `beforeSend` integration + forward proxy for all SDK transports
2. **Datadog** — Agent proxy config + log pipeline filter
3. **Segment** — Source middleware SDK wrappers for JS, Python, Go, Ruby
4. **New Relic / Rollbar / Bugsnag** — Forward proxy (same codebase as Sentry extension)

### Phase 2 — API Interception (Months 3–4)
Build per-vendor API gateway rules:

5. **Intercom / Zendesk / HubSpot** — REST API proxy rules with field-level scanning
6. **Slack webhooks** — Webhook proxy module
7. **SendGrid / Mailgun** — Email send API scanning
8. **OpenAI / Anthropic** *(extend existing)* — Add embeddings and fine-tuning endpoint coverage

### Phase 3 — Config Audit Engine (Months 5–6)
Build an automated config audit scanner:

9. **Google Analytics 4** — Detect PII in GA4 custom dimensions via API + tag audit
10. **Auth0 / Okta** — Audit rules/actions for PII logging, check JWT claims
11. **AWS / GCP / Azure** — Cloud config scanning (S3 public access, CloudWatch retention)
12. **Stripe** — Check webhook secret presence, key scope, CSP headers
13. **Snowflake** — Column classification + masking policy audit

### Phase 4 — Advanced Coverage (Months 7+)
14. Elastic/ELK — Logstash/Filebeat plugin
15. Splunk — HEC proxy + UF filter
16. Salesforce — API proxy + Shield configuration audit
17. Session recorders (Hotjar, FullStory) — Config audit + CSP-based exclusion enforcement

---

## Nonym Value Proposition Per Vendor

> For each vendor, Nonym provides a **Vendor Safety Score** composed of:
> 1. **Data Exposure Score** — PII detected in outbound traffic to this vendor (0–100)
> 2. **Config Risk Score** — Misconfigurations found in vendor settings (0–100)
> 3. **Compliance Coverage** — Which frameworks are satisfied by Nonym's controls (GDPR, HIPAA, PCI-DSS, SOC 2)

This positions Nonym not just as a proxy but as a **vendor risk management platform** — giving security teams a real-time view of their entire vendor PII exposure surface.
