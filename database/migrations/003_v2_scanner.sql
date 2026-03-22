-- Migration 003: V2 Scanner — vendor_connections, scans, findings, reports
-- Run after 001 (auth schema) and 002 (vendor_support).

-- ── vendor_connections ────────────────────────────────────────────────────────
-- Stores authenticated connections to external SaaS APIs for scanning.
-- Different from vendor_integrations (SDK-proxy setup).
CREATE TABLE IF NOT EXISTS vendor_connections (
  id            TEXT PRIMARY KEY,
  org_id        INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  vendor        TEXT NOT NULL,
  display_name  TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'disconnected',   -- connected | disconnected | error | scanning
  auth_type     TEXT NOT NULL DEFAULT 'api_key',        -- api_key | oauth
  credentials   TEXT NOT NULL DEFAULT '{}',             -- JSON, masked before returning to client
  settings      TEXT NOT NULL DEFAULT '{}',             -- scan_frequency etc.
  connected_at  TIMESTAMPTZ,
  last_scan_at  TIMESTAMPTZ,
  error_message TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, vendor)
);

CREATE INDEX IF NOT EXISTS idx_vendor_connections_org_id ON vendor_connections(org_id);
CREATE INDEX IF NOT EXISTS idx_vendor_connections_status ON vendor_connections(status);

-- ── scans ─────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS scans (
  id             TEXT PRIMARY KEY,
  org_id         INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  vendor_ids     TEXT NOT NULL DEFAULT '[]',            -- JSON array of vendor names
  status         TEXT NOT NULL DEFAULT 'pending',       -- pending | running | done | failed
  started_at     TIMESTAMPTZ,
  completed_at   TIMESTAMPTZ,
  findings_count INTEGER NOT NULL DEFAULT 0,
  error_message  TEXT,
  triggered_by   TEXT NOT NULL DEFAULT 'manual',        -- manual | scheduled | onboarding
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scans_org_id ON scans(org_id);
CREATE INDEX IF NOT EXISTS idx_scans_status ON scans(status);
CREATE INDEX IF NOT EXISTS idx_scans_created_at ON scans(created_at);

-- ── findings ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS findings (
  id                   TEXT PRIMARY KEY,
  org_id               INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  scan_id              TEXT NOT NULL REFERENCES scans(id),
  vendor_connection_id TEXT NOT NULL REFERENCES vendor_connections(id),
  vendor               TEXT NOT NULL,
  data_type            TEXT NOT NULL,   -- email | phone | name | ip_address | api_key | token | financial | health
  risk_level           TEXT NOT NULL,   -- high | medium | low
  title                TEXT NOT NULL,
  description          TEXT NOT NULL,
  location             TEXT,
  endpoint             TEXT,
  occurrences          INTEGER NOT NULL DEFAULT 1,
  sample_masked        TEXT,
  status               TEXT NOT NULL DEFAULT 'open',   -- open | resolved | suppressed
  compliance_impact    TEXT NOT NULL DEFAULT '[]',     -- JSON [{framework, article, risk_level}]
  fixes                TEXT NOT NULL DEFAULT '[]',     -- JSON [{language, code, description}]
  first_seen_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at          TIMESTAMPTZ,
  resolved_by          INTEGER REFERENCES users(id),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_findings_org_id ON findings(org_id);
CREATE INDEX IF NOT EXISTS idx_findings_scan_id ON findings(scan_id);
CREATE INDEX IF NOT EXISTS idx_findings_vendor ON findings(vendor);
CREATE INDEX IF NOT EXISTS idx_findings_status ON findings(status);
CREATE INDEX IF NOT EXISTS idx_findings_risk_level ON findings(risk_level);

-- ── reports ──────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS reports (
  id           TEXT PRIMARY KEY,
  org_id       INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  framework    TEXT NOT NULL,          -- GDPR | SOC2 | HIPAA | Custom
  time_range   TEXT NOT NULL,
  options      TEXT NOT NULL DEFAULT '{}',
  status       TEXT NOT NULL DEFAULT 'pending',  -- pending | generating | done | failed
  file_url     TEXT,
  share_token  TEXT UNIQUE,
  generated_at TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reports_org_id ON reports(org_id);
CREATE INDEX IF NOT EXISTS idx_reports_share_token ON reports(share_token);
