# Nonym: Stop Leaking Sensitive Data to Your Vendors

## Strategic Repositioning & Universal Data Protection Platform

---

## 1. The Problem We Are Solving

Every organization uses 10–40 SaaS vendors. Data flows to these vendors constantly:
- **Errors & stack traces** → Sentry (may contain PII in request params, headers, user context)
- **Application logs** → Datadog, Splunk, Logtail (may contain session tokens, card numbers, emails)
- **Product analytics** → PostHog, Mixpanel, Segment (may contain user identifiers, sensitive events)
- **APM & tracing** → New Relic, Datadog APM (may contain SQL queries with raw data, HTTP bodies)
- **Customer support** → Zendesk, Intercom (may receive unredacted conversation context)
- **AI/LLM providers** → OpenAI, Anthropic (may receive raw user content with PII) ← *current*
- **Custom internal APIs** → Third-party integrations, webhooks

### Why Organizations Over-Share

1. **Developer convenience** — Logging full objects is faster than sanitizing
2. **SDK defaults** — Vendor SDKs capture everything by default
3. **Speed-to-market pressure** — No one adds PII scrubbing during a 2AM bug fix
4. **Blind spots** — Engineers don't know what is sensitive per PCI/HIPAA/GDPR
5. **No tooling** — Until now, there was no universal scrubbing layer for vendor traffic

### Who Gets Hurt

| Customer Segment | Their Pain | Regulatory Risk |
|---|---|---|
| **PCI Teams** | Card numbers in Sentry errors, Datadog logs, API call bodies | PCI-DSS 3.2.1 violation, fines up to $100k/month |
| **GDPR/DPO Teams** | EU resident data sent to US vendors without proper DPA/SCCs | GDPR Article 46 violation, up to 4% global revenue |
| **HIPAA Teams** | PHI (diagnosis, SSN, DOB) leaking into monitoring tools | HIPAA breach, $100–$50k per violation |
| **SOC 2 Teams** | No audit trail proving data never reached third-party | SOC 2 Type II failure |
| **Security Teams** | API keys, tokens, secrets in logs | Credential exposure, supply chain risk |

---

## 2. Repositioned Value Proposition

> **"Nonym is the data firewall between your application and every vendor you use. It automatically detects and strips sensitive data before it reaches Datadog, Sentry, PostHog, or any external service — giving you vendor receipts, not data exposure."**

### Tagline Options
- "Stop leaking sensitive data to your vendors."
- "Every API call your app makes, under control."
- "The last line of defense before your data reaches SaaS."
- "Compliance-first vendor traffic control."

---

## 3. Current Architecture vs. Proposed Universal Architecture

### 3.1 Current Architecture (AI Proxy Only)

```
┌─────────────────┐        ┌──────────────────────┐       ┌───────────────────┐
│   Client App    │──────▶│  Nonym Gateway :8000  │──────▶│  AI Provider      │
│                 │        │  - Anonymize PII       │       │  (OpenAI/Anthropic│
│  X-API-Key:spg_│        │  - De-anonymize resp  │       │   /Google/Local)  │
└─────────────────┘        └──────────────────────┘       └───────────────────┘
```

**Coverage today**: AI LLM calls only
**Detection**: Regex + GLiNER ML model
**Integration**: SDK replacement (change base URL)

---

### 3.2 Proposed Architecture: Universal Vendor Data Firewall

```
┌────────────────────────────────────────────────────────────────────────────┐
│                        APPLICATION LAYER                                    │
│                                                                              │
│   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│   │  Sentry  │  │ Datadog  │  │ PostHog  │  │ OpenAI   │  │ Custom   │  │
│   │  SDK     │  │  Agent   │  │  SDK     │  │  SDK     │  │  APIs    │  │
│   └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│        │              │              │              │              │          │
└────────┼──────────────┼──────────────┼──────────────┼──────────────┼────────┘
         │              │              │              │              │
         ▼              ▼              ▼              ▼              ▼
┌────────────────────────────────────────────────────────────────────────────┐
│                   NONYM VENDOR DATA FIREWALL                                │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │  INGESTION LAYER                                                      │  │
│  │  ┌───────────────┐ ┌────────────────┐ ┌────────────────────────────┐ │  │
│  │  │ Forward Proxy │ │ SDK Interceptor│ │ Network Sidecar (Envoy)    │ │  │
│  │  │ (HTTPS/HTTP)  │ │ (JS/Python/Go) │ │ (Kubernetes/Docker)        │ │  │
│  │  └───────────────┘ └────────────────┘ └────────────────────────────┘ │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                  │                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │  DETECTION ENGINE (existing NER + expanded)                          │  │
│  │  ┌────────────┐ ┌────────────┐ ┌──────────────┐ ┌───────────────┐  │  │
│  │  │ Regex NER  │ │ GLiNER ML  │ │ Rule Engine  │ │ Custom Rules  │  │  │
│  │  │ (PII types)│ │ (entities) │ │ (per-vendor) │ │ (org-defined) │  │  │
│  │  └────────────┘ └────────────┘ └──────────────┘ └───────────────┘  │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                  │                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │  ACTION LAYER                                                         │  │
│  │  ┌────────────┐ ┌────────────┐ ┌──────────────┐ ┌───────────────┐  │  │
│  │  │ Redact     │ │ Anonymize  │ │ Block        │ │ Alert Only    │  │  │
│  │  │ (remove)   │ │ (tokenize) │ │ (block req)  │ │ (audit only)  │  │  │
│  │  └────────────┘ └────────────┘ └──────────────┘ └───────────────┘  │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                  │                                           │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │  AUDIT & COMPLIANCE                                                   │  │
│  │  Full audit trail • Per-vendor receipts • Framework mapping          │  │
│  │  GDPR data inventory • HIPAA BAA evidence • PCI-DSS scope reduction  │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
         │              │              │              │              │
         ▼              ▼              ▼              ▼              ▼
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│  Sentry  │    │ Datadog  │    │ PostHog  │    │ OpenAI   │    │ Custom   │
│ (clean)  │    │ (clean)  │    │ (clean)  │    │ (clean)  │    │ Vendor   │
└──────────┘    └──────────┘    └──────────┘    └──────────┘    └──────────┘
```

---

## 4. Integration Approaches

### 4.1 Approach A: Forward Proxy (Highest Coverage)

**How it works**: Configure vendor SDKs to send traffic through Nonym's HTTPS proxy. Nonym intercepts, scrubs, and forwards to the real vendor endpoint.

**Implementation**:
```bash
# Environment variable approach (works for most SDKs)
HTTPS_PROXY=https://nonym.yourdomain.com:8443
HTTP_PROXY=https://nonym.yourdomain.com:8443

# SDK-specific:
# Sentry
Sentry.init({ dsn: "...", transport: nonymTransport })

# Datadog
DD_PROXY_HTTPS=https://nonym.yourdomain.com:8443
```

**Pros**:
- Single deployment, covers all vendors automatically
- No per-vendor SDK work
- Works across languages (Go, Python, JS, Ruby, Java)
- Captures raw HTTP — can detect PII in any payload format

**Cons**:
- Requires TLS termination and re-encryption (adds latency ~2–5ms)
- Root CA certificate must be trusted by services
- Some SDKs hardcode endpoints or pin certificates

**Gateway Changes Required**:
- Add HTTPS CONNECT proxy handler (MITM proxy capability)
- TLS inspection with per-org CA certificates
- SNI-based routing to identify the target vendor

---

### 4.2 Approach B: SDK Interceptors / Native Libraries (Best DX)

**How it works**: Drop-in SDK wrappers for popular vendors that intercept before data is sent.

**Libraries to build**:

```javascript
// JavaScript/TypeScript
import { nonymSentry } from '@nonym/sentry'
import * as Sentry from '@sentry/browser'

Sentry.init({
  dsn: process.env.SENTRY_DSN,
  integrations: [nonymSentry({ apiKey: 'spg_...' })]
})

// PostHog
import { nonymPosthog } from '@nonym/posthog'
posthog.init('phc_...', {
  before_send: nonymPosthog.beforeSend({ apiKey: 'spg_...' })
})
```

```python
# Python
from nonym.integrations.datadog import NonymDatadogLogger
import logging

handler = NonymDatadogLogger(api_key="spg_...", datadog_api_key="dd_...")
logging.getLogger().addHandler(handler)
```

**Pros**:
- Best developer experience — feels native
- No network-level changes, no proxy config
- Per-vendor rules built-in (knows Sentry's event schema, PostHog's event structure)
- Works in serverless / edge environments where proxy is difficult

**Cons**:
- Must build and maintain per-vendor SDK integrations
- Coverage limited to vendors with an SDK wrapper
- Language-specific — need JS, Python, Go, Ruby, Java versions

---

### 4.3 Approach C: Network Sidecar (Best for k8s/Cloud)

**How it works**: Nonym runs as an Envoy/sidecar proxy in a Kubernetes pod, intercepting all egress traffic transparently.

**Implementation**:
```yaml
# kubernetes/sidecar.yaml
spec:
  containers:
  - name: app
    image: myapp:latest
  - name: nonym-sidecar
    image: nonym/sidecar:latest
    env:
    - name: NONYM_API_KEY
      value: spg_...
    - name: NONYM_INTERCEPT_EGRESS
      value: "true"
  initContainers:
  - name: nonym-init
    image: nonym/init:latest
    # Sets up iptables rules to route egress through sidecar
```

**Pros**:
- Completely transparent — no application code changes
- Covers all outbound traffic from the pod
- Perfect for organizations adopting service mesh

**Cons**:
- Complex setup, requires k8s expertise
- Not suitable for non-containerized applications
- iptables manipulation requires privileged containers

---

### 4.4 Approach D: Log/Event Processor (Async, Low Risk)

**How it works**: Nonym processes logs and events after they're written but before they leave the network (log shipper integration).

**Implementation**:
```yaml
# Fluent Bit / Fluentd filter plugin
[FILTER]
    Name   nonym
    Match  *
    api_key spg_...
    action redact

# Logstash filter
filter {
  nonym {
    api_key => "spg_..."
    fields => ["message", "exception.values"]
  }
}
```

**Pros**:
- Works on existing log pipelines without code changes
- Can process historical data
- Low latency impact (async)

**Cons**:
- Data is logged before scrubbing — brief exposure window
- Does not help with blocking, only detection and post-processing
- Does not cover API calls (only logs)

---

### 4.5 Approach E: Environment Scanner (Audit Mode)

**How it works**: Nonym connects to vendor accounts via API keys and scans data already stored for PII exposure. Also scans environment configs and code for patterns that will lead to over-sharing.

**Scan targets**:
- Sentry: Pull error events via Sentry API, scan for PII
- Datadog: Pull log samples via Datadog API, analyze
- PostHog: Pull event properties, detect user identifiers
- Source code: Scan `.env`, config files, SDK initializations

**Use case**: "Discovery mode" — understand your current exposure before implementing prevention.

**Pros**:
- Zero deployment risk — read-only audit
- Shows organizations their *current* exposure immediately
- Strong sales motion (show the problem before asking to fix it)
- Works as a free tier product feature

**Cons**:
- Requires vendor API access
- Retroactive — doesn't prevent future leaks
- Vendor API rate limits

---

## 5. Vendor-Specific Coverage Plan

### Priority 1: Immediate (Q1)

#### Sentry
- **What leaks**: Request parameters, user context (email, IP), breadcrumbs, extra data, tags
- **Integration**: `beforeSend` hook (JS), Django/Rails middleware, Sentry SDK `before_send`
- **PII at risk**: Email, IP, user ID, session tokens, request body, cookies
- **Action**: Build `@nonym/sentry` JS package + Python `nonym-sentry` package

#### Datadog
- **What leaks**: Log messages (raw application logs), APM spans (HTTP bodies, SQL queries), RUM session data
- **Integration**: Datadog Agent log filter pipeline, custom log handler
- **PII at risk**: Credit cards in SQL queries, emails in logs, SSNs in request bodies
- **Action**: Build Fluent Bit filter plugin + custom Datadog log handler for Python/Go

#### PostHog / Mixpanel / Amplitude
- **What leaks**: Event properties (email used as identifier), user profile properties, URL parameters
- **Integration**: `before_send` / `capture_sanitizer` hooks
- **PII at risk**: Email as distinct_id, phone in event properties, purchase amounts
- **Action**: Build JS SDK wrapper with property scrubber

### Priority 2: Short-term (Q2)

#### Splunk / Elastic / OpenSearch
- **Integration**: HEC (HTTP Event Collector) proxy, Logstash filter plugin
- **PII at risk**: Full log payloads with raw SQL, stack traces, session data

#### New Relic / Dynatrace
- **Integration**: Agent filter plugin, custom exporter
- **PII at risk**: Transaction traces, error parameters, custom attributes

#### Segment
- **Integration**: Segment destination filter, source function wrapper
- **PII at risk**: All user traits, track event properties, identify calls

#### Slack / Microsoft Teams (webhook alerts)
- **Integration**: Webhook proxy
- **PII at risk**: Alert messages containing customer data, incident data

### Priority 3: Medium-term (Q3–Q4)

#### Zendesk / Intercom / Freshdesk
- **Integration**: API proxy for ticket creation
- **PII at risk**: Customer messages, contact info, order details in tickets

#### GitHub / GitLab (error reporting, webhooks)
- **Integration**: Webhook filter
- **PII at risk**: Issue descriptions, PR comments with log dumps

#### AWS CloudWatch / GCP Cloud Logging
- **Integration**: Lambda/Cloud Function log processor
- **PII at risk**: Lambda invocation logs, API Gateway access logs

#### Custom Vendor APIs (Generic)
- **Integration**: Forward proxy with vendor domain allowlist + rule sets
- **PII at risk**: Arbitrary payloads — organization defines what to protect

---

## 6. Detection Engine Expansion

### Current Coverage
| Entity | Method |
|---|---|
| Email | Regex |
| Phone | Regex |
| SSN | Regex |
| Credit Card (PAN) | Regex |
| CVV | Regex |
| IBAN | Regex |
| IP Address | Regex |
| API Key / Password | Regex |
| Person Name | GLiNER ML |
| Location | GLiNER ML |
| Organization | GLiNER ML |

### Expansion Needed for Vendor Coverage

| New Entity | Detection Method | Compliance Driver |
|---|---|---|
| **Card PAN in JSON fields** | Luhn algorithm + field-name heuristics | PCI-DSS |
| **JWT / Bearer tokens** | Pattern: `eyJ...` | Security |
| **Database connection strings** | Pattern: `postgresql://`, `mysql://` | Security |
| **Private keys / certs** | Pattern: `-----BEGIN` | Security |
| **Session cookies** | Context-aware (header analysis) | GDPR |
| **IP + User-Agent combos** | Correlation detection | GDPR |
| **IBAN / SWIFT** | Regex + checksum | PCI, GDPR |
| **Passport number** | Country-specific regex | GDPR, HIPAA |
| **Driver's license** | Country-specific regex | GDPR |
| **NHS / Health ID numbers** | Country-specific regex | HIPAA |
| **Diagnosis codes (ICD-10)** | Codelist matching | HIPAA |
| **Medication names** | Dictionary matching | HIPAA |
| **Date of Birth patterns** | Contextual regex | HIPAA, GDPR |
| **Biometric descriptors** | ML | GDPR |
| **Ethnic origin indicators** | ML (special category) | GDPR Article 9 |
| **URL parameters with PII** | Parameter name heuristics | GDPR |

### Detection Enhancement: Context-Aware Field Rules

Add a **Field Rule Engine** where organizations define rules based on:
- JSON key names (e.g., `"email"`, `"card_number"`, `"user.ssn"`)
- HTTP header names (e.g., `X-User-Email`, `Authorization`)
- Query string parameter names (e.g., `?email=`, `?phone=`)

This allows coverage without ML — pure rule-based for high-performance environments.

---

## 7. Target Customer Deep Dive

### 7.1 PCI-DSS Teams

**Objective**: Reduce "cardholder data environment" (CDE) scope by preventing card data from reaching out-of-scope vendors.

**What they need from Nonym**:
- Block/redact Primary Account Numbers (PAN) before any non-PCI vendor
- Mask CVV completely (never stored, never forwarded)
- Automated PCI scope reports showing what was intercepted and where
- Evidence that monitoring tools (Datadog, Sentry) are NOT receiving card data

**Nonym Features to Highlight**:
- PCI-DSS framework tagging on all detection events
- "PCI Scope Report" — downloadable compliance artifact showing zero card data reached vendors
- Strict mode: block any request containing card-like data to non-approved endpoints

**Dashboard view**: PCI Compliance tab with Luhn-positive interceptions, vendor breakdown, scope attestation export

---

### 7.2 GDPR / DPO Teams

**Objective**: Data minimization (Article 5), lawful basis for transfers (Article 46), ROPA accuracy.

**What they need from Nonym**:
- Know exactly what personal data categories go to which vendor
- Evidence of pseudonymization before cross-border transfer
- Automated ROPA (Record of Processing Activities) generation
- Right-to-erasure support: find which vendors received a specific person's data

**Nonym Features to Highlight**:
- Data Flow Map: visual diagram of personal data → vendor flows
- Per-vendor data category inventory: "PostHog received: email (anonymized), IP (hashed)"
- Subject Access Request (SAR) helper: search all vendor traffic for a data subject
- Transfer receipts: proof that data was pseudonymized before reaching US vendor

**Dashboard view**: GDPR tab with data flow map, transfer log, erasure request tracker

---

### 7.3 HIPAA Covered Entities / Business Associates

**Objective**: Prevent PHI (Protected Health Information) from leaving the HIPAA-controlled environment.

**What they need from Nonym**:
- Detect PHI: names, dates, phone, SSN, medical record numbers, diagnosis codes, geographic data, device identifiers
- BAA (Business Associate Agreement) — Nonym must be able to sign BAAs
- Demonstrate technical safeguards for PHI in transit
- Audit logs retained for 6 years (HIPAA requirement)

**Nonym Features to Highlight**:
- HIPAA PHI entity set (all 18 Safe Harbor identifiers)
- BAA template available
- 6-year immutable audit log retention option
- PHI transfer risk report: show which vendor calls carried PHI risk

**Dashboard view**: HIPAA tab with PHI detection count, 18-identifier coverage, BAA status, audit log download

---

### 7.4 Security Teams (CISO / AppSec)

**Objective**: Prevent credential and secret leakage to monitoring vendors.

**What they need from Nonym**:
- Detect API keys, tokens, private keys, DB connection strings in logs
- Alert on credential patterns reaching Sentry, Datadog
- Integration with SIEM/SOAR for incident response

**Nonym Features to Highlight**:
- Secrets detection module (dedicated entity types)
- Webhook alerts when high-severity detection occurs
- SIEM integration (send audit events to Splunk, Elastic via webhook)

---

## 8. Architecture Changes Required

### 8.1 Immediate Changes (No Breaking Changes)

#### A. Multi-Destination Router Expansion
**File**: `pkg/router/providers.go`

Current router only routes to AI providers. Expand `DetermineProvider()` to support arbitrary vendor endpoints:

```go
// Add VendorProfile struct
type VendorProfile struct {
    Name          string
    BaseURL       string
    ContentTypes  []string // json, multipart, text
    PathRules     []VendorPathRule
    PIIRiskLevel  string // high, medium, low
    Frameworks    []string // PCI, GDPR, HIPAA
}

// Vendor registry (built-in profiles + org-defined)
var BuiltinVendors = map[string]VendorProfile{
    "sentry":   {...},
    "datadog":  {...},
    "posthog":  {...},
    "segment":  {...},
}
```

#### B. Field-Level Rule Engine
**New file**: `pkg/rules/engine.go`

```go
type FieldRule struct {
    OrgID       string
    VendorName  string   // optional: vendor-specific
    FieldPath   string   // JSONPath: "$.user.email", "$.properties.*"
    HeaderName  string   // HTTP header to scan
    QueryParam  string   // URL query param name
    Action      string   // redact | anonymize | block
    EntityType  string   // maps to compliance framework
}
```

#### C. Vendor Configuration UI
Add vendor management to the dashboard:
- Vendor catalog (Sentry, Datadog, etc.) with one-click setup
- Custom vendor definition (name + base URL + rules)
- Per-vendor action policy (allow | scrub | block)
- Per-vendor framework tagging

#### D. Expanded Audit Schema

```sql
ALTER TABLE transactions ADD COLUMN vendor_name VARCHAR(100);
ALTER TABLE transactions ADD COLUMN vendor_category VARCHAR(50); -- monitoring, analytics, ai, support
ALTER TABLE events ADD COLUMN vendor_name VARCHAR(100);

CREATE TABLE vendor_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id),
    vendor_name VARCHAR(100) NOT NULL,
    vendor_url VARCHAR(500),
    action VARCHAR(20) DEFAULT 'scrub', -- allow | scrub | block | audit_only
    enabled BOOLEAN DEFAULT true,
    frameworks TEXT[], -- GDPR, HIPAA, PCI
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE field_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(id),
    vendor_name VARCHAR(100),
    field_path VARCHAR(500),
    header_name VARCHAR(200),
    action VARCHAR(20) DEFAULT 'redact',
    entity_type VARCHAR(100),
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### 8.2 Medium-term Changes (New Capabilities)

#### A. HTTPS Forward Proxy (MITM capability)
Add CONNECT method handling to Fiber gateway:
- TLS termination with org-specific CA
- Per-org certificate generation using `crypto/x509`
- SNI-based vendor identification
- Upstream TLS re-establishment to real vendor

This is the highest-coverage approach and requires the most work (~4 weeks).

#### B. SDK Packages (separate repos)
- `npm: @nonym/sentry` — Sentry `beforeSend` wrapper
- `npm: @nonym/posthog` — PostHog `before_send` wrapper
- `pip: nonym-datadog` — Datadog log handler
- `pip: nonym-sentry` — Sentry Python `before_send`

Each SDK:
1. Intercepts the vendor SDK hook
2. Sends payload to Nonym gateway for scanning (or uses embedded lite scanner)
3. Returns scrubbed payload to vendor SDK
4. Logs detection events to Nonym audit trail

#### C. Scanner / Discovery Mode
New gateway endpoint: `POST /api/v1/scan/discover`
- Accept vendor API credentials
- Pull sample data from Sentry/Datadog/PostHog APIs
- Run NER over samples
- Return PII exposure report without storing vendor data

#### D. Compliance Report Generator
New endpoint: `GET /api/v1/compliance/report?framework=PCI&format=pdf`
- Pull audit data filtered by framework
- Generate attestation document
- Include: detection counts, vendor breakdown, action taken, date range

---

## 9. Frontend Guidance

### 9.1 Vendor Management Screen

```
┌──────────────────────────────────────────────────────────────────┐
│  Vendors                                          + Add Vendor   │
├──────────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  🔴 Sentry          errors.sentry.io      ACTIVE  [Edit]  │   │
│  │     Action: Scrub   Frameworks: GDPR, PCI               │   │
│  │     Last 24h: 142 events scrubbed, 0 blocked             │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  🟡 Datadog         api.datadoghq.com     ACTIVE  [Edit]  │   │
│  │     Action: Audit Only  Frameworks: HIPAA, GDPR          │   │
│  │     Last 24h: 89 detections (not yet scrubbing)          │   │
│  └──────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  🟢 PostHog         app.posthog.com       ACTIVE  [Edit]  │   │
│  │     Action: Anonymize  Frameworks: GDPR                  │   │
│  │     Last 24h: 23 identifiers anonymized                  │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

### 9.2 Data Flow Map (GDPR view)

```
YOUR APP
   │
   ├──[Email ✓ anonymized]──────▶  PostHog
   │
   ├──[IP ✓ redacted, Email ✓]───▶  Sentry
   │
   ├──[Logs ✓ scrubbed]───────────▶  Datadog
   │
   └──[Content ✓ anonymized]──────▶  OpenAI
```

Interactive: click on an edge to see what was detected and what action was taken.

### 9.3 Compliance Dashboard Tabs

```
[Overview] [PCI-DSS] [GDPR] [HIPAA] [SOC 2] [Custom]

── PCI-DSS Tab ─────────────────────────────────────────
  Scope Reduction
  ┌─────────────────────────────────────────────────┐
  │  Card data intercepted this month: 1,847 events │
  │  Vendors protected: Sentry, Datadog, PostHog    │
  │  Zero card data confirmed sent to any vendor ✓  │
  └─────────────────────────────────────────────────┘

  [Download PCI Attestation Report] [Export Evidence]
```

### 9.4 Vendor Setup Wizard

Step 1: Choose Vendor (catalog grid)
```
[Sentry] [Datadog] [PostHog] [Mixpanel] [Segment] [Custom...]
```

Step 2: Choose Integration Method
```
( ) Forward Proxy    — Route traffic through Nonym (all languages)
( ) SDK Package      — Install @nonym/sentry npm package
( ) Scanner Only     — Audit existing data (no prevention yet)
```

Step 3: Choose Action Policy
```
( ) Scrub & Forward  — Remove PII, send clean data to vendor
( ) Anonymize        — Replace PII with tokens (reversible)
( ) Block if PII     — Reject events containing sensitive data
( ) Audit Only       — Log detections, send data as-is
```

Step 4: Choose Compliance Frameworks
```
[x] GDPR   [x] PCI-DSS   [ ] HIPAA   [ ] SOC 2
```

Step 5: Copy integration snippet
```bash
# For Forward Proxy:
HTTPS_PROXY=https://your-org.nonym.io:8443
NONYM_API_KEY=spg_xxxxx

# For SDK:
npm install @nonym/sentry
```

### 9.5 Live Event Stream (Scanner View)

```
┌─────────────────────────────────────────────────────────────────┐
│  Live Event Stream                            [Pause] [Filter]  │
├─────────────────────────────────────────────────────────────────┤
│  14:32:01  SENTRY    email detected          → REDACTED  🟠    │
│  14:32:01  SENTRY    ip_address detected     → REDACTED  🟡    │
│  14:31:58  DATADOG   api_key detected        → BLOCKED   🔴    │
│  14:31:55  POSTHOG   phone_number detected   → ANONYMIZED 🟠   │
│  14:31:52  OPENAI    credit_card detected    → BLOCKED   🔴    │
│  14:31:50  CUSTOM    person_name detected    → REDACTED  🟠    │
└─────────────────────────────────────────────────────────────────┘
```

---

## 10. Pros and Cons of Each Architecture Approach

| Approach | Coverage | Complexity | Latency | Works Without Code Change | Best For |
|---|---|---|---|---|---|
| **Forward Proxy (MITM)** | Universal | High | Low (2–5ms) | Yes | Enterprise, k8s-heavy orgs |
| **SDK Interceptors** | Vendor-specific | Medium | Minimal | No | Developer-friendly orgs |
| **Network Sidecar** | Universal | Very High | Negligible | Yes | k8s/cloud-native |
| **Log Processor** | Logs only | Low | None (async) | Yes (existing pipeline) | Legacy systems |
| **Scanner/Discovery** | Audit only | Low | N/A | Yes | Sales motion, free tier |

**Recommendation**: Lead with **SDK Interceptors** for developer experience and early traction. Build the **Forward Proxy** as the enterprise offering for universal coverage. Offer **Scanner/Discovery** as a free audit tool to demonstrate value upfront.

---

## 11. Competitive Landscape

| Competitor | Approach | Gap Nonym Fills |
|---|---|---|
| **Nightfall AI** | DLP for SaaS (Slack, GitHub, Drive) | Nonym focuses on application-layer vendor traffic, not file storage |
| **Prisma Cloud DSPM** | Cloud data scanning | Nonym is real-time prevention, not posture management |
| **Skyflow** | Data vault / tokenization | Nonym is a proxy layer, not a data warehouse — simpler to adopt |
| **Transcend** | Data mapping / consent | Nonym is technical enforcement, not governance UI only |
| **BigID** | Data discovery | Nonym prevents, not just discovers |
| **AWS Macie** | S3 data classification | Nonym is real-time in-transit, not stored-data |

**Nonym's Position**: The only **real-time, in-transit, vendor-agnostic** PII firewall that works across all your vendor integrations with a 15-minute setup.

---

## 12. Execution Plan

### Phase 0: Foundation (Weeks 1–2) — No breaking changes
- [ ] Add `vendor_name` and `vendor_category` columns to `transactions` and `events` tables
- [ ] Create `vendor_policies` and `field_rules` tables
- [ ] Expand router to support arbitrary vendor URLs (not just AI providers)
- [ ] Add vendor-aware detection logging
- [ ] Update dashboard to show "Vendor" column in transaction list

### Phase 1: Vendor Catalog + Manual Config (Weeks 3–4)
- [ ] Build vendor catalog (Sentry, Datadog, PostHog, Segment, Mixpanel, Splunk)
- [ ] Vendor setup wizard UI (4 steps above)
- [ ] Per-vendor action policies (scrub/anonymize/block/audit)
- [ ] Per-vendor framework tagging
- [ ] Vendor-specific PII field rules (field path rules for known schemas)
- [ ] "Data Flow Map" visualization on dashboard

### Phase 2: SDK Packages (Weeks 5–8)
- [ ] `@nonym/sentry` (JavaScript/TypeScript)
- [ ] `@nonym/posthog` (JavaScript/TypeScript)
- [ ] `nonym-datadog` (Python)
- [ ] `nonym-sentry` (Python)
- [ ] Documentation and quickstart for each
- [ ] SDK event reporting to Nonym audit trail

### Phase 3: Scanner / Discovery Mode (Weeks 9–10)
- [ ] Sentry API scanner (pull & analyze recent error events)
- [ ] Datadog API scanner (pull log samples)
- [ ] PostHog API scanner (pull event property samples)
- [ ] Discovery report UI: "Your current exposure"
- [ ] Free tier: run a scan with API key, get report (sales motion)

### Phase 4: Compliance Reports (Weeks 11–12)
- [ ] PCI-DSS attestation report generator
- [ ] GDPR data flow inventory export
- [ ] HIPAA safeguard evidence report
- [ ] SOC 2 audit log export
- [ ] PDF + JSON export formats

### Phase 5: Forward Proxy / Enterprise (Weeks 13–20)
- [ ] HTTPS CONNECT proxy (MITM)
- [ ] Per-org CA certificate generation
- [ ] TLS inspection pipeline
- [ ] Kubernetes sidecar deployment guide
- [ ] Docker Compose egress-filter pattern
- [ ] Enterprise onboarding docs

---

## 13. How to Win

### Go-to-Market Strategy

1. **Free Discovery Tool**: "Scan your Sentry account for PII leaks — free, takes 2 minutes."
   - Instant value: shows the problem before asking for money
   - Converts to paid when they see real leaks in their production data

2. **Developer-Led Growth**: SDK packages on npm/PyPI with zero-friction setup
   - `npm install @nonym/sentry` → working in 10 minutes
   - GitHub README badge: "PII-protected by Nonym"

3. **Compliance-Led Sales**: Approach compliance teams with "GDPR transfer receipts" and "PCI scope reduction evidence"
   - Annual compliance audits are painful — offer audit-ready reports
   - PCI QSAs love evidence artifacts; generate them automatically

4. **Channel Partners**: Partner with PCI QSAs, GDPR consultants, HIPAA consultants
   - They recommend Nonym to clients as technical safeguard
   - Commission or referral model

5. **Case Study Pattern**: "Company X reduced their PCI scope from 47 systems to 3 by routing vendor traffic through Nonym"

### Pricing Model

| Tier | Price | What's Included |
|---|---|---|
| **Free / Discovery** | $0 | Scanner only, 1 vendor, 1k events/month |
| **Starter** | $99/mo | 3 vendors, 100k events/month, GDPR + PCI reports |
| **Growth** | $499/mo | Unlimited vendors, 1M events/month, all compliance reports, SDK packages |
| **Enterprise** | Custom | Forward proxy, k8s sidecar, BAA for HIPAA, SLA, on-prem option |

### Why Nonym Wins

1. **Universal** — one tool for all vendors, not Sentry-specific or Datadog-specific
2. **Real-time** — prevention in transit, not post-hoc discovery
3. **Compliance-mapped** — every detection event tagged to GDPR/HIPAA/PCI article
4. **Low friction** — 15-minute setup with SDK packages
5. **Evidence-generating** — downloadable compliance artifacts for auditors
6. **Cross-stack** — works with Go, Python, Node.js, Ruby, Java applications
7. **Existing foundation** — battle-tested NER engine, multi-tenant architecture, audit trail already built

---

## 14. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| **Latency concerns (proxy approach)** | Benchmark and publish; offer async/SDK mode for latency-sensitive paths |
| **Trust concerns (MITM)** | Open-source the proxy inspection code; offer on-prem deployment |
| **Vendor SDK API changes break integrations** | Pin SDK versions; automated compatibility tests in CI |
| **False positives scrub legitimate data** | Per-entity confidence threshold; allow-lists; audit-only mode to test before enforcing |
| **HIPAA BAA requirement** | Engage legal early; structure as Business Associate Agreement |
| **ML model accuracy on new entity types** | Hybrid approach: ML + regex + rule engine; rule engine as fallback |
| **Scaling (high-volume logs)** | Async processing queue; sampling mode for very high volume |

---

## 15. Immediate Next Steps

1. **Update README and landing page** with new positioning ("Stop leaking data to vendors")
2. **Build vendor catalog** with Sentry, Datadog, PostHog profiles in the router
3. **Add `vendor_name` to audit schema** so every transaction is vendor-attributed
4. **Build vendor setup wizard** in dashboard
5. **Create `@nonym/sentry` SDK** as proof-of-concept for SDK approach
6. **Build Sentry scanner** as free-tier discovery feature
7. **Add PCI/GDPR/HIPAA compliance report endpoints**
8. **Publish benchmarks** to address latency concerns proactively

---

*Document created: 2026-03-21*
*Status: Working draft — open for team review and prioritization*
