# Positioning Alignment — Backend Endpoints Needed

This document tracks new backend endpoints and data structures required to support
the compliance-positioning features built in the frontend (see `positioning.md`).

---

## 1. Vendor Catalogue

> Already documented in `vendor-catalogue.md`. Reproduced here for completeness.

```
GET /api/v1/scanner/vendors/catalogue
```

Returns the list of supported vendors with metadata needed to render the scanner UI.
See `vendor-catalogue.md` for full schema.

---

## 2. DPA Registry

### Get registry
```
GET /api/v1/scanner/dpa-registry
Authorization: Bearer <token>
```

**Response** — array of `DpaRecord`:
```json
[
  {
    "vendorId": "openai",
    "status": "signed",          // "signed" | "missing" | "expired" | "review_needed"
    "region": "US",
    "lastReviewed": "2024-11-01T00:00:00Z",
    "expiresAt": "2025-11-01T00:00:00Z"
  }
]
```

### Upsert a record
```
PUT /api/v1/scanner/dpa-registry/{vendor_id}
Authorization: Bearer <token>
Content-Type: application/json
```

**Request body**:
```json
{
  "status": "signed",
  "region": "EU",
  "lastReviewed": "2025-03-01T00:00:00Z",
  "expiresAt": "2026-03-01T00:00:00Z"
}
```

**Response**: updated `DpaRecord`.

**Usage**: The frontend shows a DPA coverage gap alert banner on the Scanner Overview
page when any connected vendor has `status: "missing"` or `"expired"`. This drives
daily-active-use by compliance teams who need to track GDPR Art. 28 obligations.

---

## 3. AI Traffic Summary

```
GET /api/v1/scanner/ai-traffic?period=30d
Authorization: Bearer <token>
```

**Query params**:
- `period`: `7d` | `30d` | `90d` (default: `30d`)

**Response** — array of `AiTrafficEntry`:
```json
[
  {
    "vendorId": "openai",
    "vendorName": "OpenAI",
    "promptCount": 14823,
    "piiDetectedCount": 312,
    "piiRedactedCount": 309,
    "baaStatus": "not_applicable",   // "signed" | "missing" | "not_applicable"
    "period": "30d"
  }
]
```

**Usage**: Rendered on Scanner Overview as the "AI Traffic & PII Exposure" section.
Highlights which AI vendors are receiving the most prompts and whether PII is leaking
through. The `baaStatus` field is relevant for HIPAA compliance — if PII is sent to
an AI vendor without a signed BAA, it is a violation.

**Derivation hint**: This data can be sourced from the existing proxy transaction log
(`/api/v1/transactions`) aggregated per vendor per period. A dedicated endpoint avoids
the frontend having to pull and aggregate thousands of raw transactions.

---

## 4. Shadow SaaS / Detected Vendors

```
GET /api/v1/scanner/detected-vendors
Authorization: Bearer <token>
```

**Response** — array of `ShadowVendor`:
```json
[
  {
    "host": "api.someunknownai.com",
    "firstSeen": "2025-03-10T14:22:00Z",
    "requestCount": 47,
    "piiDetected": true
  }
]
```

**Usage**: Rendered on Scanner Overview as a red alert banner when hosts are present
that are not in the approved vendor list (`/api/v1/scanner/vendors/catalogue`).
The user can dismiss entries (approve or block) via UI — those actions currently
only affect local state; see §6 below for future block-list management.

**Derivation hint**: Compare outbound proxy request hostnames against the known vendor
catalogue. Any host not in the catalogue (and not on an internal ignore list) should
appear here.

---

## 5. Proxy Rules (One-Click Fix / Apply via Nonym)

```
POST /api/v1/scanner/proxy-rules
Authorization: Bearer <token>
Content-Type: application/json
```

**Request body**:
```json
{
  "vendorId": "openai",
  "dataType": "email",
  "action": "redact"             // "redact" | "block" | "mask"
}
```

**Response**:
```json
{
  "rule_id": "rule_abc123",
  "vendorId": "openai",
  "dataType": "email",
  "action": "redact",
  "createdAt": "2025-03-24T10:00:00Z"
}
```

**Usage**: Called from `FindingDrawer.vue` when the user clicks "Apply via Nonym" on a
finding. Creates a proxy-level redaction rule so the fix is applied at the gateway
without requiring code changes in the customer's application.

---

## 6. Shadow Vendor Block / Approve (Future)

These are not yet wired to backend calls — the frontend currently handles them with
local state mutation. Wire up when ready:

```
POST /api/v1/scanner/vendor-allowlist
{ "host": "api.someai.com", "action": "approve" | "block" }
```

---

## 7. Evidence Package Generation

When the report `include` options include `evidence_package: true`, the backend should
return a downloadable ZIP file (or a presigned URL to one) containing:

- PDF compliance report
- Machine-readable findings export (JSON)
- Subprocessor register (CSV)
- Framework control mapping (CSV)
- Data flow diagram (PNG or SVG)
- Redaction log sample (last 30 days)

The existing `POST /api/v1/scanner/reports` endpoint should accept and honour the
`evidence_package` include flag and set `downloadUrl` on the resulting `ScannerReport`
to the ZIP file.

---

## 8. Subprocessor Register

The subprocessor register is **fully derived on the frontend** from:
- `GET /api/v1/scanner/vendors/catalogue` — vendor metadata (name, abbr, color, category)
- `GET /api/v1/scanner/dpa-registry` — DPA status per vendor
- Open findings — data types exposed per vendor

No separate backend endpoint is needed unless the organisation wants to persist
manual overrides (e.g., custom data-type assignments) in the future.

---

## 9. Framework Control Mapping

Also **fully derived on the frontend** from `finding.complianceImpact[]` (the array of
compliance impacts already returned on each finding object). No new endpoint needed.

---

## Summary of new endpoints

| Method | Path | Priority | Purpose |
|--------|------|----------|---------|
| GET | `/api/v1/scanner/vendors/catalogue` | P0 | Vendor metadata |
| GET | `/api/v1/scanner/dpa-registry` | P1 | DPA status per vendor |
| PUT | `/api/v1/scanner/dpa-registry/{vendor_id}` | P1 | Update DPA status |
| GET | `/api/v1/scanner/ai-traffic` | P1 | AI traffic + PII exposure summary |
| GET | `/api/v1/scanner/detected-vendors` | P1 | Shadow SaaS detection |
| POST | `/api/v1/scanner/proxy-rules` | P2 | One-click fix / apply via Nonym |
| POST | `/api/v1/scanner/vendor-allowlist` | P3 | Approve/block shadow vendors |
