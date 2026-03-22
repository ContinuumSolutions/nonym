# Compliance Shield — Backend Specification

## What the Frontend Does (Mockup State)

The Protection page has a **Compliance Shield** panel that maps detected PII to four regulatory
frameworks (GDPR, HIPAA, PCI-DSS, SOC2) and shows estimated fine exposure averted. Currently:

- Framework attribution is computed **client-side** from `metadata.redaction_details[].entity_type`
- Fine estimates use **hardcoded per-unit values** (€20K GDPR, $15K HIPAA, $5K PCI-DSS)
- PCI-DSS is detected by running a card PAN regex against `original_text` in the browser — because
  the NER engine mislabels card numbers as `phone number`
- There is no `compliance_frameworks` field on protection events; the frontend rederives it every
  render from raw redaction details
- No API exists for pre-aggregated compliance stats

The goal of these tasks is to move all of this logic server-side so the frontend can remove the
derivation code and consume authoritative, organization-configurable compliance data.

---

## Task 1 — Fix NER Entity Typing for Card PANs

**Priority: High — correctness blocker**

The NER engine currently classifies card numbers like `4532-1234-5678-9012` and
`4242424242424242` as `phone number`. This pollutes the phone number category and forces the
frontend to do secondary regex detection for PCI-DSS attribution.

### What to do

In the post-NER processing step (after the engine returns detections, before writing the
protection event), add a secondary pass over every detection whose `entity_type` is
`phone number` or `PHONE`:

```
1. Strip spaces, hyphens, and dots from original_text.
2. If length is 13–19 digits AND passes Luhn check → reclassify as CREDIT_CARD.
3. If length is 3–4 digits AND the surrounding context contains "cvv", "cvc", "security code"
   (case-insensitive) → reclassify as CARD_CVV.
```

Luhn check (standard algorithm — implement once, unit-test with known card numbers):
- Double every second digit from right to left.
- Sum all digits (doubled digits: if > 9, subtract 9).
- If total mod 10 == 0 → valid card PAN.

Use known test PANs to verify: `4242424242424242` (Visa), `5555555555554444` (Mastercard),
`378282246310005` (Amex).

**Canonical entity types to use going forward:**

| Type | Covers |
|---|---|
| `PERSON` | Names |
| `EMAIL` | Email addresses |
| `PHONE` | Phone numbers (after card PANs are removed) |
| `SSN` | Social Security Numbers (US) |
| `NIN` | National Insurance Numbers (UK) |
| `ADDRESS` | Physical addresses |
| `ORGANIZATION` | Company / org names |
| `DATE` | Dates (including DOB) |
| `CREDIT_CARD` | Card PANs (post-Luhn reclassification) |
| `CARD_CVV` | Card CVV/CVC |
| `IBAN` | Bank account numbers |
| `IP_ADDRESS` | IP addresses |

Normalise all entity types to `UPPER_SNAKE_CASE` (the current mix of `phone number`, `PHONE`,
`address`, `PERSON` is inconsistent and breaks client-side matching).

---

## Task 2 — Canonical Compliance Framework Mapping

**Priority: High**

Define the authoritative mapping from entity type to compliance framework. Store this in code
(not the database — it changes infrequently and is not per-org configurable at this stage).

### Mapping table

| Entity Type | GDPR | HIPAA | PCI-DSS | SOC2 |
|---|---|---|---|---|
| `PERSON` | ✓ | ✓ (18 PHI identifiers) | | ✓ |
| `EMAIL` | ✓ | ✓ | | ✓ |
| `PHONE` | ✓ | ✓ | | |
| `SSN` | ✓ | ✓ | | ✓ |
| `NIN` | ✓ | | | ✓ |
| `ADDRESS` | ✓ | ✓ | | |
| `ORGANIZATION` | ✓ | | | ✓ |
| `DATE` | ✓ | ✓ (DOB, admission/discharge dates) | | |
| `CREDIT_CARD` | ✓ | | ✓ | ✓ |
| `CARD_CVV` | ✓ | | ✓ | ✓ |
| `IBAN` | ✓ | | ✓ | ✓ |
| `IP_ADDRESS` | ✓ | | | ✓ |

**Regulatory citations to keep with the mapping (for audit trails):**

- GDPR: Article 5(1)(c) — Data Minimisation; Article 4(1) — Personal Data definition
- HIPAA: 45 CFR § 164.514(b)(2) — 18 PHI identifiers; § 164.312 — Technical Safeguards
- PCI-DSS v4.0: Requirement 3.3 — Sensitive Authentication Data; Req 3.4 — PAN protection
- SOC2 (Trust Services Criteria): CC6 — Logical and Physical Access Controls; CC9 — Risk Mitigation

### Implementation

Implement as a function/constant exported from a compliance package:

```
func FrameworksForEntityType(entityType string) []string
// returns e.g. ["GDPR", "HIPAA", "SOC2"] for "PERSON"

func FrameworksForEvent(redactionDetails []RedactionDetail) []string
// deduplicated union across all entity types in the event
```

---

## Task 3 — Tag Protection Events at Write Time

**Priority: High**

When a protection event is created (at proxy interception time), compute and store the applicable
compliance frameworks so queries can filter and aggregate by framework without re-deriving them.

### Schema change

Add a column to the `protection_events` table:

```sql
ALTER TABLE protection_events
  ADD COLUMN compliance_frameworks text[] NOT NULL DEFAULT '{}';

-- Index for filtering events by framework
CREATE INDEX idx_protection_events_frameworks
  ON protection_events USING GIN (compliance_frameworks);
```

### Write path

After the NER post-processing step (Task 1) and before `INSERT INTO protection_events`:

```
1. Collect all entity_type values from redaction_details.
2. Call FrameworksForEvent(redactionDetails) → []string.
3. Store as compliance_frameworks on the event row.
```

### API response — add to protection events

Include `compliance_frameworks` in `GET /api/v1/protection-events` responses:

```json
{
  "id": "evt_1774071444308062657",
  "timestamp": "2026-03-21T05:37:24.308064Z",
  "type": "pii_detected",
  "action": "anonymized",
  "provider": "openai",
  "status": "open",
  "protection": "multiple",
  "severity": "low",
  "redaction_count": 7,
  "compliance_frameworks": ["GDPR", "HIPAA", "SOC2"],
  "metadata": {
    "redaction_count": 7,
    "redaction_details": [...]
  }
}
```

This lets the frontend drop its client-side `complianceFrameworks()` derivation entirely and
display the field directly.

### Backfill

For existing events that predate this column, run a one-time migration:

```sql
-- Pseudocode: backfill by re-deriving from stored metadata JSONB
UPDATE protection_events
SET compliance_frameworks = derive_frameworks_from_metadata(metadata)
WHERE compliance_frameworks = '{}';
```

Implement `derive_frameworks_from_metadata` as a PL/pgSQL function that reads
`metadata->'redaction_details'->[*]->>'entity_type'` and applies the same mapping.
Run in batches of 1000 rows to avoid lock contention.

---

## Task 4 — Compliance Summary Widget Endpoint

**Priority: High**

Add `GET /api/v1/dashboard/widgets/compliance-summary` so the frontend can replace its
hardcoded mockup with live, server-aggregated data.

### Query parameters

| Param | Type | Default | Description |
|---|---|---|---|
| `timeRange` | string | `24h` | `1h`, `24h`, `7d`, `30d` |

### Response schema

```json
{
  "time_range": "24h",
  "generated_at": "2026-03-21T05:37:24Z",
  "frameworks": [
    {
      "key": "GDPR",
      "label": "GDPR",
      "events_protected": 32,
      "redactions": 87,
      "top_entity_types": ["PERSON", "EMAIL", "ADDRESS"],
      "citation": "Art. 5(1)(c) · Data Minimisation",
      "estimated_exposure_averted": {
        "amount": 640000,
        "currency": "EUR",
        "per_event_basis": 20000,
        "disclaimer": "estimate"
      }
    },
    {
      "key": "HIPAA",
      "label": "HIPAA",
      "events_protected": 9,
      "redactions": 27,
      "top_entity_types": ["SSN", "DATE", "PERSON"],
      "citation": "45 CFR § 164 · PHI Safeguards",
      "estimated_exposure_averted": {
        "amount": 135000,
        "currency": "USD",
        "per_event_basis": 15000,
        "disclaimer": "estimate"
      }
    },
    {
      "key": "PCI-DSS",
      "label": "PCI-DSS",
      "events_protected": 8,
      "redactions": 12,
      "top_entity_types": ["CREDIT_CARD"],
      "citation": "Req. 3.3 · Cardholder Data Protection",
      "estimated_exposure_averted": {
        "amount": 40000,
        "currency": "USD",
        "per_event_basis": 5000,
        "disclaimer": "estimate"
      }
    },
    {
      "key": "SOC2",
      "label": "SOC2",
      "events_protected": 32,
      "redactions": 87,
      "top_entity_types": ["PERSON", "EMAIL", "SSN"],
      "citation": "CC6 · Logical Access Controls",
      "estimated_exposure_averted": null
    }
  ],
  "total_estimated_exposure_averted_usd": 826200
}
```

Notes on the `estimated_exposure_averted` field:
- The `disclaimer: "estimate"` must always be present. The UI already shows this as "Preview";
  the backend must not imply these are legally binding calculations.
- `total_estimated_exposure_averted_usd` converts GDPR EUR to USD at a fixed rate (1.08). When
  a configurable rate API is added (Task 5), use that instead.
- SOC2 returns `null` for exposure because SOC2 violations don't carry direct statutory fines —
  the frontend renders "Audit-ready / Compliant" for null.

### SQL

```sql
SELECT
  unnest(compliance_frameworks) AS framework,
  COUNT(DISTINCT id)            AS events_protected,
  SUM(redaction_count)          AS redactions
FROM protection_events
WHERE
  organization_id = $org_id
  AND timestamp   >= now() - $time_range::interval
GROUP BY framework;
```

For `top_entity_types`, aggregate entity types across matching events:

```sql
SELECT
  unnest(compliance_frameworks) AS framework,
  rd->>'entity_type'           AS entity_type,
  COUNT(*)                      AS cnt
FROM protection_events,
  jsonb_array_elements(metadata->'redaction_details') AS rd
WHERE
  organization_id = $org_id
  AND timestamp   >= now() - $time_range::interval
GROUP BY framework, entity_type
ORDER BY framework, cnt DESC;
```

Take the top 3 entity types per framework in application code.

---

## Task 5 — Per-Organization Fine Rate Configuration

**Priority: Medium — needed before removing the "Preview" label from the UI**

The hardcoded per-event fine estimates (€20K GDPR, $15K HIPAA, $5K PCI-DSS) must be
configurable per organization because exposure varies by company size, industry, and prior
violations.

### Schema

```sql
CREATE TABLE compliance_fine_rates (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id  uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  framework        text NOT NULL,            -- 'GDPR', 'HIPAA', 'PCI-DSS'
  per_event_amount numeric(12,2) NOT NULL,
  currency         char(3) NOT NULL,         -- ISO 4217: 'EUR', 'USD'
  updated_at       timestamptz NOT NULL DEFAULT now(),
  updated_by       uuid REFERENCES users(id),
  UNIQUE (organization_id, framework)
);

-- Seed defaults for every new organization (run in org creation transaction)
INSERT INTO compliance_fine_rates (organization_id, framework, per_event_amount, currency)
VALUES
  ($org_id, 'GDPR',    20000, 'EUR'),
  ($org_id, 'HIPAA',   15000, 'USD'),
  ($org_id, 'PCI-DSS',  5000, 'USD');
```

### API endpoints

**`GET /api/v1/settings/compliance`**

Returns the organization's current fine rate configuration.

```json
{
  "fine_rates": [
    { "framework": "GDPR",    "per_event_amount": 20000, "currency": "EUR" },
    { "framework": "HIPAA",   "per_event_amount": 15000, "currency": "USD" },
    { "framework": "PCI-DSS", "per_event_amount": 5000,  "currency": "USD" }
  ],
  "disclaimer": "These estimates are for internal risk awareness only and do not constitute legal advice."
}
```

**`PUT /api/v1/settings/compliance`**

Updates fine rates. Requires `admin` role. Validates:
- `per_event_amount` > 0
- `currency` is a known ISO 4217 code
- `framework` is one of `GDPR`, `HIPAA`, `PCI-DSS`

```json
{
  "fine_rates": [
    { "framework": "GDPR", "per_event_amount": 50000, "currency": "EUR" }
  ]
}
```

Returns `200` with the full updated rate list.

When the compliance-summary widget (Task 4) is called, read from `compliance_fine_rates` instead
of hardcoded values. Once this is live, the "Preview" pill can be removed from the frontend.

---

## Task 6 — Add `compliance_frameworks` Filter to Protection Events API

**Priority: Medium**

Now that events store `compliance_frameworks`, expose it as a query filter.

**`GET /api/v1/protection-events`** — add query parameter:

| Param | Type | Example | Description |
|---|---|---|---|
| `framework` | string | `GDPR` | Return only events tagged with this framework |

Implementation:

```sql
-- Only add this clause when framework param is present
AND $framework = ANY(compliance_frameworks)
```

This lets a future "GDPR Events" view or export filter the list without client-side filtering.

---

## Task 7 — Add `compliance_summary` to Dashboard Widget System

**Priority: Low**

Register the new widget in the dashboard layout system so it can appear on the main dashboard
in addition to the Protection page.

Add to `dashboard-widget-schemas.json`:

```json
"compliance_summary": {
  "description": "Per-framework compliance coverage panel. Shows event counts and estimated fine exposure averted.",
  "endpoint": "/api/v1/dashboard/widgets/compliance-summary",
  "refresh_interval_seconds": 300
}
```

Add `"compliance_summary"` to the `WidgetType` enum.

---

## Removal Checklist (Frontend — after backend tasks ship)

Once the backend is live, remove the following from `Protection.vue`:

- [ ] `CARD_RE` regex and `isCardData()` helper — PCI-DSS now comes from `compliance_frameworks` field
- [ ] `GDPR_TYPES`, `HIPAA_TYPES`, `SOC2_TYPES` sets
- [ ] `complianceFrameworks()` function — replace with `ev.compliance_frameworks` direct field access
- [ ] `complianceStats` computed property — replace with `api.protection.summary()` → `/compliance-summary` endpoint
- [ ] `fmtTotalExposure()` helper — backend returns `total_estimated_exposure_averted_usd`
- [ ] Hardcoded per-unit fine values (20000, 15000, 5000) in template
- [ ] `mockup-pill` "Preview" badge — remove once Task 5 (configurable rates) ships
- [ ] Update `ProtectionEvent` type to add `compliance_frameworks: string[]`
- [ ] Update API service to call `/compliance-summary` with `timeRange` param

---

## Implementation Order

```
Task 1 (NER fix)  →  Task 2 (mapping)  →  Task 3 (event tagging + backfill)
     ↓
Task 4 (widget endpoint)  →  Task 6 (filter)  →  Task 7 (dashboard registration)
     ↓
Task 5 (configurable rates)  →  remove "Preview" label
```

Tasks 1–3 must ship together in one deploy (NER fix feeds the mapping, mapping feeds the tagging).
Tasks 4 and 6 can ship independently after Task 3. Task 5 is the last gate before the UI is
considered production-ready rather than a preview.
