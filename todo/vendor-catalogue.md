# Backend TODO: Vendor Catalogue Endpoint

The `VENDOR_CATALOGUE` array in `src/stores/scanner.ts:18` is currently hardcoded on the frontend. It should be served by the backend so that supported vendors can be added, removed, or updated without a frontend deploy.

---

## Endpoint

```
GET /api/v1/scanner/vendors/catalogue
```

- **Auth**: Bearer JWT (same as all dashboard endpoints)
- **Purpose**: Returns the list of all *supported* vendor integrations — i.e. the vendors Nonym knows how to scan. This is distinct from `/api/v1/scanner/vendors` which returns the user's *configured connections*.

---

## Response Schema

```json
{
  "vendors": [
    {
      "id": "sentry",
      "name": "Sentry",
      "abbr": "SE",
      "color": "#F55",
      "bg": "rgba(255, 85, 85, 0.12)",
      "description": "Error tracking & performance monitoring",
      "auth_type": "api_key",
      "auth_fields": [
        {
          "key": "api_token",
          "label": "Auth Token",
          "placeholder": "sntrys_...",
          "secret": true
        },
        {
          "key": "org_slug",
          "label": "Organization Slug",
          "placeholder": "my-org",
          "secret": false
        }
      ],
      "scan_description": "Scans error events for user PII, request data, headers, and breadcrumbs",
      "docs_url": "https://docs.sentry.io/api/auth/"
    }
  ]
}
```

### Top-level fields

| Field | Type | Description |
|---|---|---|
| `vendors` | `VendorCatalogueItem[]` | Ordered list of supported vendors |

### `VendorCatalogueItem` fields

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | yes | Stable machine identifier (e.g. `"sentry"`). Used as a foreign key in `VendorConnection.vendor`. |
| `name` | `string` | yes | Human-readable display name (e.g. `"Sentry"`) |
| `abbr` | `string` | yes | 2–3 char abbreviation shown in avatar chips (e.g. `"SE"`) |
| `color` | `string` | yes | CSS colour for the vendor accent (hex or `rgba(...)`) |
| `bg` | `string` | yes | CSS colour for the vendor chip background (typically low-opacity rgba of `color`) |
| `description` | `string` | yes | One-line description of the vendor shown in the connect modal |
| `auth_type` | `"api_key" \| "oauth"` | yes | How the user authenticates with this vendor |
| `auth_fields` | `AuthField[]` | yes | Ordered list of credential fields to collect from the user. Empty array for proxy-only vendors (OpenAI, Anthropic). |
| `scan_description` | `string` | yes | One-sentence description of what Nonym scans for this vendor |
| `docs_url` | `string` | yes | Link to the vendor's auth/API key documentation |

### `AuthField` fields

| Field | Type | Required | Description |
|---|---|---|---|
| `key` | `string` | yes | Machine key sent back as the credential payload key when connecting |
| `label` | `string` | yes | Human-readable field label |
| `placeholder` | `string` | yes | Input placeholder text |
| `secret` | `boolean` | no | If `true`, render as a password input. Defaults to `false`. |

---

## Current vendors to seed

The backend should initially return the following six vendors (matching what is currently hardcoded):

| id | name | auth_type | Notes |
|---|---|---|---|
| `sentry` | Sentry | `api_key` | fields: `api_token`, `org_slug` |
| `datadog` | Datadog | `api_key` | fields: `api_key`, `app_key` |
| `openai` | OpenAI | `api_key` | no auth fields — scanned via Nonym AI Proxy |
| `anthropic` | Anthropic | `api_key` | no auth fields — scanned via Nonym AI Proxy |
| `mixpanel` | Mixpanel | `api_key` | fields: `service_account_user`, `service_account_secret`, `project_id` |
| `stripe` | Stripe | `api_key` | fields: `restricted_key` |

---

## Frontend integration plan

Once the endpoint exists, `scanner.ts` will:

1. Call `GET /api/v1/scanner/vendors/catalogue` in `init()`.
2. Map the snake_case response back to the camelCase `VendorMeta` interface.
3. Store the result in a new `catalogue` ref (replacing the hardcoded `VENDOR_CATALOGUE` export).
4. Fall back to the hardcoded constant if the endpoint is unavailable.

The `VendorId` union type in `src/types/index.ts:409` should eventually be derived dynamically rather than kept in sync by hand — that can happen once the endpoint is stable.
