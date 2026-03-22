Wi# Nonym V2 — Backend Engineering Plan

> **Audience:** Backend team
> **Date:** 2026-03-22
> **Status:** Planning — ready for review

---

## 1. Overview

V2 introduces a second product pillar: **Vendor Scanner**. The existing AI Proxy backend is unchanged. New backend work creates a parallel pipeline for:

1. **Vendor connectors** — authenticated clients for SaaS APIs (Sentry, Datadog, Mixpanel, Stripe, etc.)
2. **Scanner service** — fetches data from vendor APIs, normalizes it, and runs detection
3. **Detection engine** — finds PII and sensitive data patterns in vendor data
4. **Findings store** — persists results, manages status (open / resolved / suppressed)
5. **Report generator** — produces compliance snapshots (GDPR, SOC2, HIPAA) as PDF/JSON
6. **New REST API** — endpoints consumed by the V2 frontend

Everything is additive. Existing proxy endpoints (`/api/v1/statistics`, `/api/v1/transactions`, `/api/v1/protection-events`, etc.) are untouched.

---

## 2. New Data Models

### `vendor_connections`

```sql
CREATE TABLE vendor_connections (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  vendor        TEXT NOT NULL,                    -- 'sentry', 'datadog', 'openai', etc.
  display_name  TEXT,
  status        TEXT NOT NULL DEFAULT 'disconnected',  -- connected | disconnected | error | scanning
  auth_type     TEXT NOT NULL,                    -- 'api_key' | 'oauth'
  credentials   JSONB NOT NULL,                  -- encrypted, see §4
  settings      JSONB NOT NULL DEFAULT '{}',     -- scan_frequency, etc.
  connected_at  TIMESTAMPTZ,
  last_scan_at  TIMESTAMPTZ,
  error_message TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, vendor)
);
```

### `scans`

```sql
CREATE TABLE scans (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  vendor_ids      TEXT[],                         -- which vendors were scanned
  status          TEXT NOT NULL DEFAULT 'pending', -- pending | running | done | failed
  started_at      TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,
  findings_count  INT NOT NULL DEFAULT 0,
  error_message   TEXT,
  triggered_by    TEXT,                           -- 'manual' | 'scheduled' | 'onboarding'
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### `findings`

```sql
CREATE TABLE findings (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id            UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  scan_id           UUID NOT NULL REFERENCES scans(id),
  vendor_connection_id UUID NOT NULL REFERENCES vendor_connections(id),
  vendor            TEXT NOT NULL,
  data_type         TEXT NOT NULL,               -- 'email' | 'phone' | 'name' | 'ip_address' | 'api_key' | 'token' | 'financial' | 'health'
  risk_level        TEXT NOT NULL,               -- 'high' | 'medium' | 'low'
  title             TEXT NOT NULL,
  description       TEXT NOT NULL,
  location          TEXT,                        -- e.g. 'event.user.email'
  endpoint          TEXT,                        -- e.g. 'POST /api/login'
  occurrences       INT NOT NULL DEFAULT 1,
  sample_masked     TEXT,                        -- PII removed, e.g. 'j***@example.com'
  status            TEXT NOT NULL DEFAULT 'open', -- 'open' | 'resolved' | 'suppressed'
  compliance_impact JSONB,                       -- [{framework, article, risk_level}]
  fixes             JSONB,                       -- [{language, code, description}]
  first_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at       TIMESTAMPTZ,
  resolved_by       UUID REFERENCES users(id),
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### `reports`

```sql
CREATE TABLE reports (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  framework    TEXT NOT NULL,                   -- 'GDPR' | 'SOC2' | 'HIPAA' | 'Custom'
  time_range   TEXT NOT NULL,                   -- 'last_30_days' | 'last_90_days' | custom
  options      JSONB NOT NULL DEFAULT '{}',     -- include_raw_samples, etc.
  status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'generating' | 'done' | 'failed'
  file_url     TEXT,                            -- S3/object-storage URL
  share_token  TEXT UNIQUE,                     -- for share links
  generated_at TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ,                     -- share link expiry
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 3. New REST API Endpoints

All endpoints require `Authorization: Bearer <jwt>`. All paths prefixed `/api/v1/`.

### Vendors

```
GET    /vendors
       → VendorConnection[]
       Query: ?status=connected

POST   /vendors
       Body: { vendor: string, auth_type: string, credentials: object, settings?: object }
       → VendorConnection

DELETE /vendors/:id
       → 204

POST   /vendors/:id/test
       → { success: bool, message: string, events_accessible?: number }

POST   /vendors/:id/scan
       → { scan_id: string }
```

### Scans

```
GET    /scans
       → Scan[]
       Query: ?limit=20&offset=0

POST   /scans
       Body: { vendor_ids?: string[] }    -- empty = all connected vendors
       → { scan_id: string }

GET    /scans/:id
       → Scan

GET    /scans/:id/status
       → SSE stream: { event: 'progress' | 'finding' | 'done' | 'error', data: ... }
```

SSE progress events:

```json
// progress
{ "event": "progress", "vendor": "sentry", "phase": "fetching", "percent": 30 }

// finding discovered during scan
{ "event": "finding", "finding_id": "...", "vendor": "sentry", "risk_level": "high" }

// scan complete
{ "event": "done", "findings_count": 19, "scan_id": "..." }
```

### Findings

```
GET    /findings
       → Finding[]
       Query: ?vendor=sentry&risk_level=high&data_type=email&status=open&limit=50&offset=0

GET    /findings/:id
       → Finding

PATCH  /findings/:id
       Body: { status: 'resolved' | 'suppressed' }
       → Finding
```

### Reports

```
GET    /reports
       → Report[]

POST   /reports/generate
       Body: {
         framework: 'GDPR' | 'SOC2' | 'HIPAA' | 'Custom',
         time_range: string,
         options: { include_findings, include_vendors, include_flows, include_fixes, include_raw_samples }
       }
       → { report_id: string }   -- async; poll /reports/:id or use webhook

GET    /reports/:id
       → Report

GET    /reports/:id/download
       → 302 redirect to signed file URL

GET    /reports/share/:token
       → Report (public, no auth required)
```

### Scanner Overview

```
GET    /scanner/overview
       → {
           vendors_connected: number,
           findings: { high: number, medium: number, low: number, total: number },
           risk_score: number,
           compliance: ComplianceSnapshot,
           last_scan_at: string | null
         }

GET    /scanner/flows
       → {
           nodes: { id, label, type: 'app' | 'vendor' }[],
           edges: { from, to, findings_count, risk_level }[]
         }
```

---

## 4. Credential Storage (Security)

Vendor credentials (API keys, OAuth tokens) must be stored encrypted at rest.

**Approach:**
- Use envelope encryption: each credential encrypted with an org-specific DEK (Data Encryption Key)
- DEK itself encrypted with a master key stored in a KMS (AWS KMS / HashiCorp Vault)
- `vendor_connections.credentials` stores the encrypted ciphertext (JSONB)
- Decryption only happens inside the scanner service at scan time
- Credentials never returned to frontend in plaintext — only a masked representation

**Masked format:**
```json
{ "type": "api_key", "masked": "sk-proj-...xxxx" }
```

**OAuth tokens:**
- Store `access_token` + `refresh_token` encrypted
- Auto-refresh before scans
- Revoke on vendor disconnect

---

## 5. Vendor Connector Architecture

Each vendor is implemented as a **Connector** — a struct/interface with a standard contract:

```go
type Connector interface {
    // Test whether the credentials work and return metadata
    TestConnection(ctx context.Context, creds Credentials) (ConnectionResult, error)

    // Fetch raw events/logs for scanning
    Fetch(ctx context.Context, creds Credentials, opts FetchOptions) (<-chan RawEvent, error)

    // Return vendor metadata (name, logo, supported data types, docs URL)
    Meta() VendorMeta
}

type RawEvent struct {
    ID       string
    Source   string          // e.g. "event.user.email"
    Payload  map[string]any
    Endpoint string
    OccurredAt time.Time
}
```

### Sentry Connector

- **Auth:** `Authorization: Bearer <sentry_token>`
- **Fetch:** `GET /api/0/projects/{org}/{project}/issues/` + `GET /api/0/issues/{id}/events/`
- **Fields to extract:**
  - `event.user.{email, username, ip_address}`
  - `event.request.data.*`
  - `event.request.headers.Authorization`
  - `event.extra.*`
  - `event.breadcrumbs[].message`
- **Rate limits:** 100 req/s (respect `Retry-After` header)
- **Pagination:** cursor-based (`?cursor=`)

### Datadog Connector

- **Auth:** `DD-API-KEY` + `DD-APPLICATION-KEY` headers
- **Fetch:** `POST /api/v2/logs/events/search`
- **Fields to extract:**
  - `attributes.message` (primary leak source — full-text scan)
  - `attributes.http.*`
  - `attributes.network.client.ip`
  - Flatten all `attributes.*` recursively
- **Rate limits:** 300 req/min
- **Time window:** Default last 24h per scan

### OpenAI Connector

OpenAI does not expose prompt logs via API. Two modes:

**Mode A (via Nonym proxy — preferred):**
- All OpenAI requests routed through existing proxy
- Proxy already captures prompt + response
- Scanner reads from proxy's existing transaction log
- No additional connector needed — query `transactions` table

**Mode B (indirect — from other vendors):**
- Scan Sentry/Datadog for logged OpenAI prompt content
- Tag findings with vendor = "openai" when prompt data found

### Mixpanel Connector

- **Auth:** Service account credentials (Basic auth)
- **Fetch:** `POST /api/2.0/export/` (raw event export)
- **Fields:** Event properties, `$email`, `$name`, `distinct_id`
- **Rate limits:** 100 concurrent exports

### Stripe Connector

- **Auth:** Restricted API key (read-only: `logs:read`)
- **Fetch:** `GET /v1/events` (webhook event log)
- **Fields:** `data.object.email`, `data.object.customer`, billing info
- **Note:** Stripe data rarely contains PII leaks — mostly metadata; lower priority

---

## 6. Detection Engine

### Phase 1 (MVP): Leverage the current NER + Regex + keyword rules

Fast, no dependencies, good precision for structured PII.

```go
type Detector interface {
    Detect(text string) []Detection
}

type Detection struct {
    DataType   string    // 'email', 'phone', 'api_key', etc.
    Value      string    // raw match (to be masked before storage)
    Masked     string    // e.g. "j***@example.com"
    Confidence float64   // 0.0–1.0
    RuleID     string
}
```

**Detection rules:**

| Rule ID | Pattern | Data Type | Risk |
|---|---|---|---|
| `email` | `\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b` | email | high |
| `phone_e164` | `\+\d{10,15}` | phone | high |
| `phone_us` | `\(?\d{3}\)?[-.\s]\d{3}[-.\s]\d{4}` | phone | high |
| `api_key_openai` | `sk-[a-zA-Z0-9]{20,}` | api_key | high |
| `api_key_anthropic` | `sk-ant-[a-zA-Z0-9-]{20,}` | api_key | high |
| `jwt` | `eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+` | token | high |
| `ssn` | `\d{3}-\d{2}-\d{4}` | financial | high |
| `credit_card` | Luhn-valid 13–19 digit numbers | financial | high |
| `ip_address` | `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b` | ip_address | medium |
| `keyword_password` | `password\s*[=:]\s*\S+` | token | high |
| `keyword_secret` | `secret\s*[=:]\s*\S+` | token | high |
| `keyword_health` | `diagnosis\|prescription\|patient\|medical` | health | high |

**Keyword field names** (flag any field named):
- `password`, `passwd`, `secret`, `token`, `auth`, `authorization`, `ssn`, `dob`, `birthday`

### Tighten Identification with: Named Entity Recognition (NER)

Add a lightweight NER model for names detection:
- Use `GLiNER` (already mentioned in research) — fast, runs locally
- Detects: `PER` (person name), `ORG`, `LOC`
- Run as a sidecar service called over gRPC/HTTP
- Only called for `message` / `content` fields (expensive — not for every field)

### Phase 2: Context-aware LLM classification (V2.1)

- For ambiguous cases, call an internal LLM to classify risk
- Rate-limited, only for medium-confidence findings

### Normalization

All vendor-specific events are normalized to a common format before detection:

```go
type NormalizedEvent struct {
    VendorID   string
    EventID    string
    Source     string            // field path, e.g. "event.user.email"
    Text       string            // value to scan
    Metadata   map[string]string // endpoint, timestamp, etc.
    RawPayload []byte            // for audit trail
}
```

### Risk scoring per finding

| Condition | Risk |
|---|---|
| PII + external vendor + high-confidence detection | High |
| PII + external vendor + medium-confidence | Medium |
| Sensitive keywords + internal | Medium |
| IP addresses only | Medium |
| Metadata / low-sensitivity fields | Low |

**Org risk score** = weighted average of open finding risks:
```
score = 100 - (high * 15 + medium * 5 + low * 1), clamped to [0, 100]
```

---

## 7. Fix Suggestion Engine

Each rule has associated fix templates per vendor and language.

```go
type FixTemplate struct {
    VendorID string
    RuleID   string
    Language string
    Template string  // Go template with {{.FieldPath}} etc.
}
```

Fixes are pre-authored for the MVP launch vendors:

**Sentry × email × JavaScript:**
```javascript
Sentry.init({
  beforeSend(event) {
    if (event.user) {
      delete event.user.email;
      delete event.user.ip_address;
    }
    return event;
  }
});
```

**Datadog × email × config:**
```bash
DD_LOGS_CONFIG_REDACTION_RULES='[
  {
    "pattern": "[A-Z0-9._%+-]+@[A-Z0-9.-]+\\.[A-Z]{2,}",
    "replace_placeholder": "[REDACTED_EMAIL]"
  }
]'
```

Store fixes in the `findings.fixes` JSONB column, generated at detection time.

---

## 8. Scan Orchestration

### Scan pipeline (per vendor)

```
1. ValidateCredentials
2. FetchEvents (paginated, concurrent workers)
3. NormalizeEvents
4. DetectPII (parallel, worker pool)
5. DeduplicateFindings (same location + data_type = same finding, increment occurrences)
6. PersistFindings
7. UpdateScanStatus
8. BroadcastSSE (progress events)
```

### Deduplication logic

A finding is considered a duplicate if:
- Same `org_id` + `vendor` + `data_type` + `location` + `endpoint`
- In status `open`

On duplicate: increment `occurrences`, update `last_seen_at`.

### Scheduled scans

Use a cron worker (existing scheduler if available, or add one):

| Frequency | Schedule |
|---|---|
| hourly | `0 * * * *` |
| daily | `0 2 * * *` |
| weekly | `0 2 * * 1` |

Each org's connected vendors are scanned per their configured frequency.

### Concurrency

- Max 3 concurrent vendor fetches per org (avoid overloading vendor APIs)
- Each fetch uses bounded worker pools (configurable, default 5 workers per vendor)
- Scan timeout: 5 minutes per vendor, 15 minutes total per scan

---

## 9. Report Generation

Reports are generated asynchronously:

```
1. POST /reports/generate → create report record (status: pending), return report_id
2. Worker picks up report job
3. Query findings, vendor connections, compliance rules
4. Render PDF (using headless Chrome / wkhtmltopdf / Go PDF library)
5. Upload to object storage (S3 / equivalent)
6. Update report record (status: done, file_url, share_token)
7. Optionally: send email notification
```

**PDF template sections:**
1. Cover page (org name, framework, date range, generated by Nonym)
2. Executive summary (risk score gauge, key metrics)
3. Vendor inventory table
4. Findings by severity
5. Data flow summary
6. Recommended actions
7. Scan log (timestamps, evidence)

**Share links:**
- `share_token` is a random 32-byte hex string
- Public endpoint: `GET /api/v1/reports/share/:token` — no auth required
- TTL: configurable, default 30 days
- Returns report metadata + download URL

---

## 10. Compliance Mapping

Pre-built mapping of data types to compliance frameworks:

```go
var ComplianceRules = map[DataType][]ComplianceImpact{
    "email": {
        { Framework: "GDPR", Article: "Art. 4(1) — Personal Data Definition", Risk: "high" },
        { Framework: "CCPA", Article: "§1798.140(o) — Personal Information", Risk: "high" },
    },
    "ip_address": {
        { Framework: "GDPR", Article: "Recital 30 — Online Identifiers", Risk: "medium" },
    },
    "health": {
        { Framework: "HIPAA", Article: "45 CFR §164.514 — PHI", Risk: "high" },
        { Framework: "GDPR", Article: "Art. 9 — Special Category Data", Risk: "high" },
    },
    // ...
}
```

---

## 11. Security Requirements

- **Credentials:** Encrypted at rest (envelope encryption, KMS-backed)
- **Credentials in transit:** HTTPS only, never logged
- **Finding samples:** PII masked before storage — never store raw PII values
- **Report share links:** Time-limited, signed tokens
- **Vendor API access:** Read-only scopes only; document exact permissions required per vendor
- **Audit log:** All credential changes, scan triggers, finding status changes logged
- **Rate limiting:** Scanner respects vendor API rate limits; backs off on 429s
- **Org isolation:** All queries filter by `org_id`; no cross-tenant data access

---

## 12. Build Phases

### Phase 1 — Data models + auth (Sprint 1–2)
- [ ] DB migrations for `vendor_connections`, `scans`, `findings`, `reports`
- [ ] Credential encryption service (KMS integration)
- [ ] Basic CRUD API for vendors (connect, list, delete, test)
- [ ] Auth + org isolation middleware applied to all new routes

### Phase 2 — Sentry + Datadog connectors (Sprint 3–4)
- [ ] Sentry connector (fetch + normalize)
- [ ] Datadog connector (fetch + normalize)
- [ ] Detection engine Phase 1 (regex + keyword rules)
- [ ] Fix template engine
- [ ] Findings persistence + deduplication
- [ ] `POST /scans` + synchronous scan flow (async in Phase 3)

### Phase 3 — Async scans + SSE (Sprint 5)
- [ ] Scan job queue (use existing queue or add one)
- [ ] SSE endpoint for live scan progress
- [ ] Scheduled scan cron worker
- [ ] Scan timeout + error handling

### Phase 4 — Overview + Reports (Sprint 6–7)
- [ ] `GET /scanner/overview` endpoint
- [ ] `GET /scanner/flows` endpoint
- [ ] Report generation pipeline (PDF + object storage)
- [ ] Share link system

### Phase 5 — NER + additional vendors (Sprint 8+)
- [ ] GLiNER NER sidecar service
- [ ] Mixpanel connector
- [ ] Stripe connector
- [ ] OpenAI proxy-based scanning (read from transactions table)

---

## 13. Monitoring + Observability

Add metrics for:
- Scan duration per vendor (histogram)
- Findings count per scan (gauge)
- Detection rule hit rates (counter per rule_id)
- Credential decryption errors (counter)
- Vendor API error rates (counter per vendor)
- Report generation duration (histogram)

Alerts:
- Scan failure rate > 10% in 1h
- Credential decryption failures (any)
- Vendor API sustained 429s (throttled > 5 min)
