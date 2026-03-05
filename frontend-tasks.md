# Frontend Tasks & API Contract Notes

This document captures API changes and UI tasks the frontend must implement or fix.
Updated as the backend evolves — check this file after every backend session.

---

## 1. Preferences — New `base_hourly_rate` Field

**Endpoint:** `PUT /profile/preferences`
**Endpoint:** `GET /profile` (returned inside `preferences`)

### What changed
`base_hourly_rate` (float, USD/hr) is now a first-class preference field.
It was previously hardcoded at $500 internally and invisible to the user.
It now drives the signal triage gate — any email/calendar item whose estimated ROI
does not justify the user's hourly rate is automatically rejected.

### API shape
```json
// GET /profile → preferences object
{
  "preferences": {
    "time_sovereignty": 5,
    "financial_growth": 5,
    "health_recovery": 5,
    "reputation_building": 5,
    "privacy_protection": 5,
    "autonomy": 5,
    "base_hourly_rate": 85.0
  }
}

// PUT /profile/preferences — full object required, all fields
{
  "time_sovereignty": 7,
  "financial_growth": 6,
  "health_recovery": 5,
  "reputation_building": 8,
  "privacy_protection": 5,
  "autonomy": 5,
  "base_hourly_rate": 150.0
}
```

### Validation rules
- `time_sovereignty`, `financial_growth`, `health_recovery`, `reputation_building`,
  `privacy_protection`, `autonomy`: integer, 1–10 inclusive, all required.
- `base_hourly_rate`: positive float (> 0). Zero or omitted falls back to a computed
  default (`10 + financial_growth × 15`). Negative values are rejected with 400.

### UI guidance
- Show as a labelled currency input: **"Your hourly rate (USD/hr)"**
- Place it prominently in the preferences screen — it is the single most impactful
  setting. A user on $50/hr will see completely different signal filtering than one
  on $500/hr.
- Suggested range hint: $25–$1000. Allow free-text entry; do not cap it.
- Explain in tooltip: *"Signals whose estimated value does not justify this rate are
  automatically filtered out. Set this to what one hour of your attention is worth."*
- Default shown on first load: 85 (the DB default for new users).

### How it affects signal filtering (show this to users)
The kernel rejects a signal when:
```
estimated_roi  <  base_hourly_rate × time_commitment_hours × time_sovereignty_factor × 1.5
```
Example at $85/hr, `time_sovereignty=5`, 0.5 hr email:
- threshold = 85 × 0.5 × 0.5 × 1.5 = **$31.88**
- LLM estimates ROI = $200 → signal **passes**

---

## 2. Events — New `raw_data` Field

**Endpoints:** `GET /activities/events`, `GET /activities/events/:id`

### What changed
Each event now includes `raw_data`: the original signal payload (email, calendar item,
Slack message, transaction, etc.) that the kernel analysed. This gives the user full
context for every decision without leaving the app.

### API shape
```json
{
  "id": 42,
  "event_type": 2,
  "decision": 2,
  "importance": 1,
  "narrative": "Client confirmed the meeting for tomorrow at 12:30.",
  "analysis": { ... },
  "gain": { ... },
  "source_service": "zoho-mail",
  "raw_data": {
    "ServiceSlug": "zoho-mail",
    "ServicePurpose": "Business email inbox...",
    "Category": "Communication",
    "Title": "Re: PROSPECTIVE BUSINESS",
    "Body": "Hi Christine, Thank you for the email...",
    "Metadata": {
      "from": "moses@example.com",
      "message_id": "1234567890"
    },
    "OccurredAt": "2026-03-05T12:50:31Z"
  },
  "read": false,
  "created_at": "2026-03-05T12:50:32Z",
  "updated_at": "2026-03-05T12:50:32Z"
}
```

`raw_data` is `null` / omitted for events created before this change (existing rows).

### UI guidance
- On the event detail page, render a collapsible **"Source Signal"** panel showing:
  - `Title` as the subject/headline
  - `Body` as the message preview (truncate to ~300 chars, expand on tap)
  - `Metadata.from` as sender
  - `OccurredAt` as the signal timestamp (distinct from `created_at` which is when
    the kernel processed it)
  - `source_service` badge (already exists in events list — keep it)
- Do not show `raw_data` panel at all when the field is absent/null.
- `raw_data` fields are PascalCase (Go JSON serialisation of `datasync.RawSignal`).

---

## 3. Events — Existing `analysis` Field (must display)

**Endpoints:** `GET /activities/events`, `GET /activities/events/:id`

The `analysis` object has always been returned but the frontend is not currently
rendering it. This is the most important transparency feature.

### Shape
```json
"analysis": {
  "service_slug": "zoho-mail",
  "signal_title": "Re: PROSPECTIVE BUSINESS",
  "estimated_roi": 200.00,
  "time_commitment": 0.5,
  "manipulation_pct": 0.02,
  "roi_threshold": 31.88,
  "triage_gate": "accepted",
  "decide_utility": 178.75,
  "decide_threshold": 127.50
}
```

### `triage_gate` values and what to show
| Value | Meaning | UI label |
|-------|---------|----------|
| `accepted` | Passed all gates | "Accepted" (green) |
| `financial_insignificance` | ROI below hourly-rate threshold | "Low ROI" (amber) |
| `manipulation` | Detected guilt/urgency language | "Manipulation detected" (red) |
| `decide_utility` | Passed triage but utility score too low | "Below utility bar" (amber) |
| `decide_risk` | Utility ok but reputation risk too high | "High risk" (red) |
| `unknown` | Unexpected triage result | "Unknown" (grey) |

### UI guidance
- Show a **"Why this decision?"** section on the event detail page.
- Display `estimated_roi` vs `roi_threshold` as a bar or two-line comparison:
  *"Estimated value: $200 / Required: $31.88"*
- Show `manipulation_pct` as a percentage badge if > 0 (e.g. "2% manipulation").
- Show `decide_utility` vs `decide_threshold` when present.
- Show `time_commitment` as hours (e.g. "0.5 hr estimated").

---

## 4. Events — `decision` Enum Values

The `decision` integer maps to:

| Value | Label | Colour |
|-------|-------|--------|
| 0 | Pending | grey |
| 1 | Accepted | green |
| 2 | Declined | red |
| 3 | Negotiated | blue |
| 4 | Automated | purple |
| 5 | Cancelled | grey |

`Declined` covers both rejected (low ROI) and ghosted (manipulation) events.
Use `analysis.triage_gate` to distinguish them for display.

---

## 5. Preferences — `financial_growth` Effect on Utility Threshold

The `utility_threshold` is now derived as:
```
utility_threshold = base_hourly_rate × (2.5 − financial_growth × 0.2)
```
At `base_hourly_rate=$85`:
| financial_growth | utility_threshold |
|---|---|
| 1 | $195.50 (very strict) |
| 5 | $127.50 (balanced) |
| 10 | $42.50 (permissive) |

Show this as a tooltip or live preview when the user adjusts `financial_growth`
alongside `base_hourly_rate`.

---

## 6. OAuth Callback Error Improvements

The 403 error from OAuth services now includes the provider's actual error message
rather than a generic "insufficient scopes" label. The callback popup's
`reason` field in the `postMessage` will still be `token_exchange_failed`, but
the server log contains the full provider message.

No UI change needed. If you show the `reason` string in the UI, keep the current
mapping from `friendlyOAuthError()`.

---

## 7. Zoho Mail — Sort Order Fixed

Zoho Mail now returns the **50 most recent** messages (was returning oldest 50).
No UI change — this is a backend fix. Mention in release notes if relevant.

---

## 8. Gmail — API Disabled Error Surfacing

If the Gmail API is not enabled in the user's Google Cloud project, the sync
engine now logs the exact Google error message instead of "insufficient OAuth scopes".
No UI change needed. If you display sync errors per-service, the `last_error` field
in `GET /scheduler/status` will contain the provider's message verbatim.
