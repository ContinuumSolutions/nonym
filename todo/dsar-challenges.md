
# DSAR & Right to Erasure — Product Opportunity

Research into r/gdpr and EDPB enforcement reports (2024–2025) on startup pain points
around Data Subject Access Requests (GDPR Art. 15) and Right to Erasure (Art. 17).

---

## Why Now

The **2025 EDPB Coordinated Enforcement Framework** specifically targets the right to
erasure, with 30 DPAs across Europe actively auditing erasure handling. This is live
regulatory pressure hitting Nonym's target customer (startups using AI APIs) hardest.

---

## Core Customer Pain Points

| Pain Point | Severity | Root Cause |
|---|---|---|
| No data map — "we don't know where it lives" | Critical | PII sprawl across 50–300 SaaS tools; never inventoried |
| Manual DSAR = spreadsheets + email | Critical | ~98% of DPOs use no automation; $1,400–$10,000 per request |
| No formal intake — clock starts from a tweet | High | Requests arrive via any channel; no system of record |
| AI providers that "can't forget" | High | LLMs encode PII in weights; model unlearning is unsolved |
| Sub-processor cascade failure | High | Deletion must propagate to every vendor; most have no tooling |
| Token/pseudonym orphaning | Medium | Tokenized data is still personal data; reverse mapping is lost |
| No audit trail = no proof | Medium | Regulators ask for evidence; spreadsheets don't satisfy audits |
| Vexatious / bulk DSAR abuse | Low | Bad actors weaponise DSARs to create operational burden |

---

## Nonym's Structural Advantage

Generic DSAR tools (OneTrust, Transcend, DataGrail) track *where* data might be and
ask humans to go find it. **Nonym is already at the interception layer** — it holds:

- The **token registry** (every PII field ever substituted)
- The **transaction log** (every AI request/response with metadata)
- The **provider map** (which provider received which tokens)
- The **protection events** (PII detection and redaction history)

Erasure = revoke the token encryption key. The data becomes cryptographically
inaccessible everywhere it was ever sent, without touching downstream systems.

---

## Features to Build

### Phase 1 — Subject Identity Graph (foundation)

Every time Nonym tokenizes a PII field, write a bidirectional entry:

```
Subject (email/ID) ←→ Token(s) ←→ Provider(s) ←→ Transaction(s)
```

This reverse mapping is the prerequisite for all DSAR features. Without it,
tokenization creates a compliance liability: pseudonymized data is still personal
data (EDPB Jan 2025 guidelines), but now it can't be located for a rights request.

**Backend:** Extend token store with `subject_map` index. Key on subject email/ID.

---

### Phase 2 — DSAR Intake Portal + Dashboard

**Public intake portal** at a configurable URL (linked from privacy policy):

```
Subject submits email + request type
    → Nonym sends email OTP for identity verification
    → Request enters queue with 30-day SLA timer (extendable to 90 with notice)
    → Assigned to team member
    → Subject receives automated acknowledgement
```

Request types: Access (Art. 15), Erasure (Art. 17), Rectification (Art. 16),
Portability (Art. 20), Objection (Art. 21).

**New sidebar nav item: "Data Subjects"** (between Protection and Providers):

```
┌─────────────────────────────────────────────────────────────┐
│  DATA SUBJECTS                                    + New Request │
├─────────────────────────────────────────────────────────────┤
│  OPEN   PENDING   COMPLETED   OVERDUE                        │
│   12      4          89         1                            │
├─────────────────────────────────────────────────────────────┤
│  Request          Type      Subject         SLA      Status  │
│  DSAR-2026-047    Access    alice@co.io    18d left  In Review│
│  DSAR-2026-048    Erasure   bob@acme.com   27d left  Pending │
│  DSAR-2026-046    Erasure   jane@corp.io   2d left   ⚠ Urgent│
└─────────────────────────────────────────────────────────────┘
```

SLA timer: `--red` when <5 days, `--orange` when <10 days.

---

### Phase 3 — One-Click Subject Data Export (Art. 15)

When a team member opens a DSAR, Nonym resolves all tokens for that subject and
generates a structured subject report:

```
DSAR-2026-047 — Access Request — alice@corp.io

  Subject Identity
  ├── Email: alice@corp.io
  ├── Tokens issued: 14
  └── First seen: 2025-11-03

  AI Interactions via Nonym
  ├── 47 requests routed
  ├── Providers: OpenAI (31), Anthropic (16)
  ├── PII fields detected: name(12), email(47), phone(3)
  └── Date range: 2025-11-03 → 2026-03-20

  [ Generate Subject Report PDF ]   [ Send to Subject ]
```

The PDF is the Art. 15 response — machine-generated from the transaction log. No
manual data hunting required.

**Backend:**
```
GET /api/v1/subjects/{subject_id}/export
Authorization: Bearer <token>
Response: PDF or JSON export of all transactions for that subject
```

---

### Phase 4 — Cryptographic Token Revocation (Art. 17)

Core GDPR differentiator. Erasure flow:

```
Team member clicks "Execute Erasure"
    → Nonym invalidates all tokens for subject (revokes encryption key)
    → All future proxy requests for that subject return null at gateway layer
    → Historical transactions become cryptographically orphaned (non-re-identifiable)
    → Subject record wiped from identity graph
    → Erasure event logged with timestamp + hash proof
    → Article 17 compliance certificate generated
    → Subject notified automatically
```

This is stronger than database deletion — no record can re-emerge from a backup
because the key no longer exists. Satisfies the "put beyond use" standard for
backup archives (ICO guidance).

**Backend:**
```
POST /api/v1/subjects/{subject_id}/erase
Authorization: Bearer <token>
Response: { erasure_id, tokens_revoked, timestamp, certificate_url }
```

---

### Phase 5 — Sub-Processor Propagation

When erasure is triggered, Nonym knows which providers received which tokens.
Automatically:

1. Call provider deletion APIs (OpenAI, Anthropic, Google) with relevant request IDs
2. Mark each provider: `✓ Deletion confirmed` / `⚠ Manual action required` /
   `✓ Data was tokenized — no raw PII transmitted`
3. For providers without a deletion API: pre-generate a templated deletion request
   email the team member can send in one click

**Backend:**
```
POST /api/v1/subjects/{subject_id}/propagate-erasure
Authorization: Bearer <token>
Response: array of { provider, status, method, confirmedAt }
```

---

## UI/UX Changes to Existing Views

### Sidebar
- Add **"Data Subjects"** nav item between Protection and Providers
- Red badge dot on nav item when any DSAR is within 5 days of deadline
- Today's Stats panel: add `Active DSARs` card

### Transactions view
- Add **Subject** column (email resolved from token map)
- Add filter: `Filter by subject email` — shows all AI transactions for a person
- Row expand: banner when subject has an active DSAR

### Protection view
- Add subject identity tag to each protection event
- New stat card: `Subjects with active erasure` count

### Providers view
- Add **DSAR API** status per provider: green = deletion API available (Nonym can
  propagate automatically), yellow = manual action required
- Informs erasure propagation capability at a glance

### Settings — new "Compliance" tab
- DSAR intake portal URL configuration
- Auto-assignment rules (round-robin or by request type)
- Notification templates (acknowledgement, completion, Art. 12(3) extension notice)
- Retention policy settings (backup window for "beyond use" documentation)
- DPA upload per provider (feeds into sub-processor register)

---

## Backend API Contracts

All endpoints require `Authorization: Bearer <token>`.

---

### 1. List DSARs

```
GET /api/v1/dsars
```

**Query params** (all optional):
- `status`: `pending` | `in_review` | `completed` | `rejected` | `extended`
- `type`: `access` | `erasure` | `rectification` | `portability` | `objection`
- `limit`: integer (default 100)
- `offset`: integer (default 0)

**Response**:
```json
{
  "requests": [ ...DsarRequest ],
  "total": 89,
  "open_count": 12,
  "pending_count": 4,
  "completed_count": 71,
  "overdue_count": 2
}
```

**`DsarRequest` shape**:
```json
{
  "id": "dsar_abc123",
  "reference": "DSAR-2026-047",
  "type": "erasure",
  "subject_email": "alice@example.com",
  "subject_name": "Alice Martin",
  "status": "in_review",
  "assigned_to": "sarah@company.io",
  "created_at": "2026-03-10T09:00:00Z",
  "deadline_at": "2026-04-09T09:00:00Z",
  "extended_deadline_at": null,
  "completed_at": null,
  "notes": "Subject referenced GDPR Art. 17.",
  "token_count": 14,
  "providers_involved": ["openai", "anthropic"],
  "erasure_certificate_url": null
}
```

---

### 2. Create DSAR

```
POST /api/v1/dsars
Content-Type: application/json
```

**Request body**:
```json
{
  "type": "erasure",
  "subject_email": "alice@example.com",
  "subject_name": "Alice Martin",
  "notes": "Optional context from the subject's request"
}
```

**Response**: `DsarRequest` (status defaults to `pending`, `deadline_at` = now + 30 days,
`reference` auto-generated as `DSAR-{year}-{seq}`)

---

### 3. Get DSAR

```
GET /api/v1/dsars/{id}
```

**Response**: `DsarRequest`

---

### 4. Update DSAR

```
PATCH /api/v1/dsars/{id}
Content-Type: application/json
```

**Request body** (all optional):
```json
{
  "status": "in_review",
  "assigned_to": "mark@company.io",
  "notes": "Updated context",
  "extended_deadline_at": "2026-06-08T09:00:00Z"
}
```

**Response**: updated `DsarRequest`

**Usage**: Used for assignment, status transitions, and Art. 12(3) deadline extension.

---

### 5. Subject Lookup

```
GET /api/v1/subjects/lookup?email=alice@example.com
```

**Response**:
```json
{
  "id": "subj_abc123",
  "email": "alice@example.com",
  "name": "Alice Martin",
  "first_seen": "2025-11-03T14:22:00Z",
  "token_count": 14,
  "transaction_count": 47,
  "providers": ["openai", "anthropic"],
  "active_dsar": { ...DsarRequest } | null
}
```

**Derivation hint**: Resolve from the token registry — look up all tokens whose
`subject_identity` key matches the email, aggregate metadata. If no tokens exist,
`token_count` = 0 (subject may still submit a request).

---

### 6. Export Subject Data (Art. 15)

```
POST /api/v1/subjects/{id}/export
```

**Response**:
```json
{ "download_url": "https://..." }
```

**Usage**: Called when a team member clicks "Export Data (Art. 15)" on an access
request. The backend generates a structured PDF/JSON report of all transactions
associated with the subject's tokens. `download_url` is a short-lived signed URL.

**Derivation hint**: Join `subject_map` (token → subject) with `transactions` (token
appearances in request/response bodies) and `protection_events`. Include: request
timestamps, providers, PII entity types detected, redaction actions taken.

---

### 7. Erase Subject (Art. 17)

```
POST /api/v1/subjects/{id}/erase
```

**Response**:
```json
{
  "erasure_id": "erasure_xyz789",
  "tokens_revoked": 14,
  "timestamp": "2026-03-25T10:15:00Z",
  "certificate_url": "https://...",
  "sub_processor_results": [
    {
      "provider": "OpenAI",
      "status": "confirmed",
      "method": "api",
      "confirmed_at": "2026-03-25T10:15:03Z"
    },
    {
      "provider": "Anthropic",
      "status": "not_applicable",
      "method": "tokenized",
      "confirmed_at": null
    }
  ]
}
```

**Behaviour**:
1. Revoke encryption keys for all tokens associated with `subject_id`
2. For each provider in `providers_involved`:
   - If provider has a deletion API: call it with the relevant request IDs → `status: confirmed`
   - If data was tokenized (no raw PII sent): `status: not_applicable`, `method: tokenized`
   - If provider has no deletion API: `status: manual_required`, `method: manual`
3. Generate Art. 17 compliance certificate (signed PDF with hash proof)
4. Return `certificate_url` (short-lived signed URL or permanent storage URL)

**`sub_processor_results[].status` values**:
- `confirmed` — deletion API called and acknowledged
- `not_applicable` — Nonym never sent raw PII to this provider (tokenized only)
- `pending` — deletion request sent but awaiting confirmation
- `manual_required` — no deletion API; team must act manually

---

### Summary of new endpoints

| Method | Path | Phase | Purpose |
|--------|------|-------|---------|
| GET | `/api/v1/dsars` | 2 | List DSARs with SLA counts |
| POST | `/api/v1/dsars` | 2 | Create DSAR, auto-start 30-day clock |
| GET | `/api/v1/dsars/{id}` | 2 | DSAR detail |
| PATCH | `/api/v1/dsars/{id}` | 2 | Assign, change status, extend deadline |
| GET | `/api/v1/subjects/lookup` | 2 | Resolve subject by email → tokens + profile |
| POST | `/api/v1/subjects/{id}/export` | 3 | Art. 15 — generate subject data report |
| POST | `/api/v1/subjects/{id}/erase` | 4 | Art. 17 — revoke tokens + cascade + certificate |

---

## Frontend Implementation (completed)

The following frontend files were created/modified against this spec:

| File | Change |
|------|--------|
| `src/types/index.ts` | Added `DsarRequest`, `DsarListResponse`, `SubjectProfile`, `ErasureResult`, `SubProcessorErasureResult`, `DsarRequestType`, `DsarStatus` |
| `src/services/api.ts` | Added `api.dsar` namespace with all 7 methods + mappers |
| `src/stores/dsar.ts` | New Pinia store — state, mock data, SLA helpers, actions |
| `src/views/DataSubjects.vue` | New full-page view — stats bar, DSAR table, expandable rows, erasure modal, new request modal |
| `src/router/index.ts` | Added `/data-subjects` route |
| `src/components/Sidebar.vue` | Added "Data Subjects" nav item with reactive urgent-count badge |

---

## Messaging

> **"GDPR Article 17 in one click — because Nonym already has the token map."**

For startups: *"Your lawyers charge $300/hr to hunt down DSAR data manually.
Nonym already logged it."*

---

## References

- EDPB CEF 2025: Right to Erasure coordinated enforcement launch
- EDPB Pseudonymization Guidelines (Jan 2025) — pseudonymized data retains full GDPR rights
- ICO: "putting beyond use" standard for backup archives
- DSAR cost baseline: Gartner $1,524/request; range $1,400–$10,000
- DSAR volume growth: 72% increase since 2021; 246% over two years in some sectors
