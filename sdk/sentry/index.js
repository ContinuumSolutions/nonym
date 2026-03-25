/**
 * @nonym/sentry — Drop-in replacement for @sentry/node / @sentry/browser.
 *
 * Usage:
 *   // Before:
 *   import * as Sentry from "@sentry/node";
 *
 *   // After:
 *   import * as Sentry from "@nonym/sentry";
 *
 * Everything else stays the same.  Nonym intercepts error payloads via the
 * `beforeSend` hook and strips PII before forwarding to sentry.io.
 *
 * Environment variables:
 *   NONYM_API_KEY   — Your Nonym API key (required).
 *   NONYM_HOST      — Your Nonym gateway host (default: http://localhost:8080).
 *   NONYM_VENDOR    — Vendor tag sent in X-Nonym-Vendor header (default: sentry).
 */

"use strict";

const SentrySDK = require("@sentry/node");

const NONYM_HOST = process.env.NONYM_HOST || "http://localhost:8080";
const NONYM_API_KEY = process.env.NONYM_API_KEY || "";
const NONYM_VENDOR = process.env.NONYM_VENDOR || "sentry";

// ─── PII field names we strip from Sentry payloads ───────────────────────────

const PII_FIELDS = [
  "email",
  "username",
  "ip_address",
  "name",
  "phone",
  "address",
  "credit_card",
  "ssn",
  "password",
  "token",
  "secret",
  "api_key",
  "authorization",
];

/**
 * Recursively walk an object and redact string values whose key matches a
 * known PII field name.  Values are replaced with "[Redacted by Nonym]".
 *
 * @param {any} obj
 * @returns {any} sanitised deep-clone
 */
function redactPII(obj) {
  if (obj === null || obj === undefined) return obj;
  if (typeof obj === "string") return obj;
  if (Array.isArray(obj)) return obj.map(redactPII);
  if (typeof obj === "object") {
    const out = {};
    for (const [k, v] of Object.entries(obj)) {
      const keyLower = k.toLowerCase();
      const isPIIKey = PII_FIELDS.some(
        (f) => keyLower === f || keyLower.includes(f)
      );
      out[k] = isPIIKey && typeof v === "string" ? "[Redacted by Nonym]" : redactPII(v);
    }
    return out;
  }
  return obj;
}

/**
 * Send a Sentry event payload to the Nonym gateway for server-side NER
 * redaction, then return the sanitised event so Sentry can forward it.
 *
 * Falls back to local redactPII() if the Nonym gateway is unreachable.
 *
 * @param {object} event  Sentry event object
 * @returns {Promise<object>} sanitised event
 */
async function scrubViaGateway(event) {
  if (!NONYM_API_KEY) {
    // No API key configured — fall back to local regex redaction.
    return redactPII(event);
  }

  try {
    const res = await fetch(`${NONYM_HOST}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${NONYM_API_KEY}`,
        "X-Nonym-Vendor": NONYM_VENDOR,
        "X-Nonym-Mode": "scrub-only", // tells Nonym not to forward to an LLM
      },
      body: JSON.stringify({ nonym_scrub_payload: event }),
    });

    if (res.ok) {
      const body = await res.json();
      // Nonym returns the redacted payload in the `nonym_scrubbed` field.
      return body.nonym_scrubbed || redactPII(event);
    }
  } catch (_err) {
    // Gateway unreachable — fall back silently.
  }

  return redactPII(event);
}

// ─── Nonym-aware Sentry init ──────────────────────────────────────────────────

/**
 * @nonym/sentry init — wraps the standard Sentry.init() call and injects the
 * Nonym beforeSend hook.
 *
 * @param {import("@sentry/node").NodeOptions} options
 */
function init(options = {}) {
  const userBeforeSend = options.beforeSend;

  const nonymBeforeSend = async (event, hint) => {
    // 1. Run the user's own beforeSend first (if any).
    let processed = userBeforeSend ? await userBeforeSend(event, hint) : event;
    if (!processed) return null; // user dropped the event

    // 2. Strip PII via Nonym gateway (or local fallback).
    return scrubViaGateway(processed);
  };

  return SentrySDK.init({
    ...options,
    beforeSend: nonymBeforeSend,
  });
}

// ─── Re-export everything from the underlying Sentry SDK ─────────────────────

module.exports = {
  ...SentrySDK,
  init,
  // Expose Nonym-specific helpers for power users.
  _nonym: {
    redactPII,
    scrubViaGateway,
    NONYM_HOST,
    NONYM_VENDOR,
  },
};
