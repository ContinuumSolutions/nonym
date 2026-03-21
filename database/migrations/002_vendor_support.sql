-- Migration 002: Vendor support + session security
-- Run this against existing databases that were created before these features.
-- Adds vendor_name to transactions, creates vendor_integrations,
-- and creates compliance_fine_rates.

-- ── 1. users: add login-security columns ────────────────────────────────────
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;

-- ── 2. transactions: add vendor_name ────────────────────────────────────────
ALTER TABLE transactions
  ADD COLUMN IF NOT EXISTS vendor_name VARCHAR(100) NOT NULL DEFAULT '';

-- ── 3. vendor_integrations ───────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS vendor_integrations (
  id              TEXT PRIMARY KEY,
  organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  vendor_id       TEXT NOT NULL,
  vendor_name     TEXT NOT NULL,
  method          TEXT NOT NULL DEFAULT 'proxy',
  status          TEXT NOT NULL DEFAULT 'active',
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (organization_id, vendor_id)
);

-- ── 3. compliance_fine_rates ─────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS compliance_fine_rates (
  id               TEXT PRIMARY KEY,
  organization_id  INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  framework        TEXT NOT NULL,
  per_event_amount DOUBLE PRECISION NOT NULL,
  currency         TEXT NOT NULL,
  updated_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_by       INTEGER REFERENCES users(id),
  UNIQUE (organization_id, framework)
);

-- Seed default fine rates for any organization that has none yet.
INSERT INTO compliance_fine_rates (id, organization_id, framework, per_event_amount, currency)
SELECT
  'default_' || o.id || '_' || fw.framework,
  o.id,
  fw.framework,
  fw.amount,
  fw.currency
FROM organizations o
CROSS JOIN (
  VALUES
    ('GDPR',    20000.0, 'EUR'),
    ('HIPAA',   15000.0, 'USD'),
    ('PCI-DSS',  5000.0, 'USD')
) AS fw(framework, amount, currency)
WHERE NOT EXISTS (
  SELECT 1 FROM compliance_fine_rates cfr
  WHERE cfr.organization_id = o.id AND cfr.framework = fw.framework
)
ON CONFLICT DO NOTHING;
