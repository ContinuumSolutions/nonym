# Nonym Integration Examples

How to route each vendor through the Nonym gateway so PII is redacted before
it reaches third-party servers.  Every example assumes:

- Nonym gateway is running at `https://gateway.yourdomain.com`
- Your Nonym API key is in the `NONYM_API_KEY` environment variable

---

## Sentry

**Risk:** exception messages, breadcrumbs, user context, and full request bodies
routinely contain names, emails, and auth tokens.
**Compliance:** GDPR Art. 25, HIPAA, SOC2

### Option A — Drop-in SDK wrapper (recommended)

Replace your `@sentry/node` import with `@nonym/sentry`.  No other code changes
are required.

```bash
npm install @nonym/sentry
```

```js
// Before
import * as Sentry from "@sentry/node";

// After — one line change
import * as Sentry from "@nonym/sentry";

Sentry.init({
  dsn: process.env.SENTRY_DSN,
  environment: process.env.NODE_ENV,
  // All your existing options stay exactly the same.
});
```

Set the environment variables:

```bash
NONYM_API_KEY=your_nonym_api_key
NONYM_HOST=https://gateway.yourdomain.com
SENTRY_DSN=https://your_key@o123456.ingest.sentry.io/789
```

`@nonym/sentry` intercepts every event in the `beforeSend` hook, sends it to
the Nonym gateway for NER redaction, and only then forwards the clean event to
Sentry.  If the gateway is unreachable it falls back to local field-name-based
redaction so your error reporting is never interrupted.

### Option B — HTTP proxy

Point the Sentry SDK at your Nonym gateway instead of `sentry.io`:

```js
Sentry.init({
  dsn: process.env.SENTRY_DSN,
  tunnel: "https://gateway.yourdomain.com/v1/proxy/sentry",
});
```

Add `X-Nonym-Vendor: sentry` to your gateway routing rules so Nonym knows to
strip PII before forwarding to the real Sentry ingest endpoint.

---

## Datadog

**Risk:** log pipelines and APM traces carry user-identifying strings in log
messages, HTTP headers, and DB query parameters.
**Compliance:** GDPR, HIPAA, PCI-DSS, SOC2

### Option A — Proxy via Nonym gateway (Node.js / `dd-trace`)

```bash
npm install dd-trace
```

```js
// datadog.js — initialise before anything else
const tracer = require("dd-trace").init({
  // Route all trace and log traffic through Nonym
  url: "https://gateway.yourdomain.com/v1/proxy/datadog",
  logInjection: true,
});

module.exports = tracer;
```

Set the environment variables:

```bash
DD_API_KEY=your_datadog_api_key
DD_SITE=datadoghq.com
NONYM_API_KEY=your_nonym_api_key
```

### Option B — Agent-level proxy (any language)

Configure the Datadog Agent to send to Nonym instead of `datadoghq.com`:

```yaml
# /etc/datadog-agent/datadog.yaml
dd_url: https://gateway.yourdomain.com/v1/proxy/datadog
logs_config:
  logs_dd_url: https://gateway.yourdomain.com/v1/proxy/datadog-logs
apm_config:
  apm_dd_url: https://gateway.yourdomain.com/v1/proxy/datadog-apm
```

Nonym redacts PII in log lines and trace metadata before forwarding to
`api.datadoghq.com`.

### Option C — Log scrubbing with `pino` / `winston`

If you use a structured logger, pipe output through the Nonym scrub endpoint
before shipping to Datadog:

```js
const pino = require("pino");

const transport = pino.transport({
  target: "pino-datadog-transport",
  options: {
    ddClientConf: {
      authMethods: { apiKeyAuthInstance: { apiKey: process.env.DD_API_KEY } },
    },
    // Use Nonym as the HTTP endpoint
    ddServerConf: { site: "gateway.yourdomain.com/v1/proxy/datadog-logs" },
  },
});

const logger = pino(transport);
```

---

## PostHog

**Risk:** event properties and person profiles accumulate emails, names, and
device fingerprints — directly relevant to GDPR right-to-erasure.
**Compliance:** GDPR, SOC2

### Option A — SDK wrapper with `beforeCapture` hook (Node.js)

```bash
npm install posthog-node
```

```js
const { PostHog } = require("posthog-node");
const fetch = require("node-fetch");

const NONYM_HOST = process.env.NONYM_HOST || "https://gateway.yourdomain.com";
const NONYM_API_KEY = process.env.NONYM_API_KEY;

async function scrub(properties) {
  try {
    const res = await fetch(`${NONYM_HOST}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${NONYM_API_KEY}`,
        "X-Nonym-Vendor": "posthog",
        "X-Nonym-Mode": "scrub-only",
      },
      body: JSON.stringify({ nonym_scrub_payload: properties }),
    });
    if (res.ok) {
      const body = await res.json();
      return body.nonym_scrubbed || properties;
    }
  } catch (_) {}
  return properties;
}

const posthog = new PostHog(process.env.POSTHOG_API_KEY, {
  host: process.env.POSTHOG_HOST || "https://app.posthog.com",
});

// Wrap capture to scrub before sending
const _capture = posthog.capture.bind(posthog);
posthog.capture = async ({ distinctId, event, properties = {} }) => {
  const clean = await scrub(properties);
  return _capture({ distinctId, event, properties: clean });
};

module.exports = posthog;
```

Usage:

```js
const posthog = require("./posthog");

posthog.capture({
  distinctId: userId,
  event: "user_signed_up",
  properties: {
    plan: "pro",
    email: user.email, // will be redacted before reaching PostHog
  },
});
```

### Option B — Reverse proxy host

Point the PostHog SDK at your Nonym gateway:

```js
const { PostHog } = require("posthog-node");

const posthog = new PostHog(process.env.POSTHOG_API_KEY, {
  // Nonym forwards to app.posthog.com after redaction
  host: "https://gateway.yourdomain.com/v1/proxy/posthog",
});
```

### Option C — Browser (posthog-js)

```js
import posthog from "posthog-js";

posthog.init(process.env.NEXT_PUBLIC_POSTHOG_KEY, {
  api_host: "https://gateway.yourdomain.com/v1/proxy/posthog",
  // Sanitise before the event leaves the browser
  before_send: (event) => {
    if (event?.properties?.email) {
      event.properties.email = "[redacted]";
    }
    return event;
  },
});
```

---

## Mixpanel

**Risk:** event properties and user profiles capture emails, IPs, and
behavioural data used for direct identification.
**Compliance:** GDPR, SOC2

### Option A — Proxy via Nonym gateway (Node.js)

```bash
npm install mixpanel
```

```js
const Mixpanel = require("mixpanel");

// Point to Nonym instead of api.mixpanel.com
const mixpanel = Mixpanel.init(process.env.MIXPANEL_TOKEN, {
  host: "gateway.yourdomain.com",
  path: "/v1/proxy/mixpanel",
  protocol: "https",
});

module.exports = mixpanel;
```

Set the environment variables:

```bash
MIXPANEL_TOKEN=your_mixpanel_token
NONYM_API_KEY=your_nonym_api_key
```

### Option B — Manual scrub wrapper

```js
const Mixpanel = require("mixpanel");
const fetch = require("node-fetch");

const NONYM_HOST = process.env.NONYM_HOST || "https://gateway.yourdomain.com";

async function scrub(properties) {
  try {
    const res = await fetch(`${NONYM_HOST}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${process.env.NONYM_API_KEY}`,
        "X-Nonym-Vendor": "mixpanel",
        "X-Nonym-Mode": "scrub-only",
      },
      body: JSON.stringify({ nonym_scrub_payload: properties }),
    });
    if (res.ok) {
      const body = await res.json();
      return body.nonym_scrubbed || properties;
    }
  } catch (_) {}
  return properties;
}

const _mixpanel = Mixpanel.init(process.env.MIXPANEL_TOKEN);

module.exports = {
  track: async (event, properties = {}) => {
    const clean = await scrub(properties);
    _mixpanel.track(event, clean);
  },
  people: {
    set: async (distinctId, properties = {}) => {
      const clean = await scrub(properties);
      _mixpanel.people.set(distinctId, clean);
    },
  },
};
```

Usage:

```js
const analytics = require("./mixpanel");

analytics.track("purchase_completed", {
  plan: "enterprise",
  email: customer.email, // stripped before reaching Mixpanel
  amount: 299,
});

analytics.people.set(userId, {
  $email: customer.email,  // stripped
  $name: customer.name,    // stripped
  plan: "enterprise",
});
```

### Option C — Browser (mixpanel-browser)

```js
import mixpanel from "mixpanel-browser";

mixpanel.init(process.env.NEXT_PUBLIC_MIXPANEL_TOKEN, {
  // Route through Nonym gateway
  api_host: "https://gateway.yourdomain.com/v1/proxy/mixpanel",
});
```

---

## Scanning your environment for unprotected vendor keys

Use the Nonym scanner endpoint to audit your `.env` or config files:

```bash
curl -X POST https://gateway.yourdomain.com/api/v1/scanner \
  -H "Authorization: Bearer $NONYM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "env": {
      "SENTRY_DSN": "https://abc@o123.ingest.sentry.io/456",
      "DD_API_KEY": "abc123",
      "POSTHOG_KEY": "phc_abc",
      "MIXPANEL_TOKEN": "abc123"
    }
  }'
```

The response lists which keys are already routed through Nonym (`protected: true`)
and which are sending data directly to the vendor (`protected: false`), with a
risk rating and a quick-start instruction for each.

---

## Setting up a webhook to monitor vendor events

Get notified whenever Nonym detects or redacts PII destined for a vendor:

```bash
curl -X POST https://gateway.yourdomain.com/api/v1/webhooks \
  -H "Authorization: Bearer $NONYM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://yourdomain.com/hooks/nonym",
    "events": ["pii_detected", "pii_redacted", "vendor_request_blocked"],
    "secret": "your_webhook_secret"
  }'
```
