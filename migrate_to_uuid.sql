-- Migration script to convert database from INTEGER IDs to UUID IDs
-- Run this to fix the type mismatch between Go code (UUIDs) and database (integers)

BEGIN;

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Step 1: Drop foreign key constraints that reference the tables we'll modify
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_user_id_fkey;
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_organization_id_fkey;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_organization_id_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_user_id_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_organization_id_fkey;
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_owner_id_fkey;
ALTER TABLE user_sessions DROP CONSTRAINT IF EXISTS user_sessions_user_id_fkey;
ALTER TABLE user_sessions DROP CONSTRAINT IF EXISTS user_sessions_organization_id_fkey;
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_user_id_fkey;
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_organization_id_fkey;
ALTER TABLE auth_events DROP CONSTRAINT IF EXISTS auth_events_user_id_fkey;
ALTER TABLE auth_events DROP CONSTRAINT IF EXISTS auth_events_organization_id_fkey;
ALTER TABLE password_resets DROP CONSTRAINT IF EXISTS password_resets_user_id_fkey;

-- Step 2: Add UUID columns alongside existing integer columns
ALTER TABLE organizations ADD COLUMN id_uuid UUID DEFAULT uuid_generate_v4();
ALTER TABLE users ADD COLUMN id_uuid UUID DEFAULT uuid_generate_v4();
ALTER TABLE users ADD COLUMN organization_id_uuid UUID;

-- Step 3: Update organization UUIDs for existing users
UPDATE users SET organization_id_uuid = org.id_uuid
FROM organizations org WHERE users.organization_id = org.id;

-- Step 4: Update organizations.owner_id to use UUIDs
ALTER TABLE organizations ADD COLUMN owner_id_uuid UUID;
UPDATE organizations SET owner_id_uuid = u.id_uuid
FROM users u WHERE organizations.owner_id = u.id;

-- Step 5: Drop old integer columns and rename UUID columns
-- Drop dependent objects first
DROP INDEX IF EXISTS idx_users_organization_id;
DROP INDEX IF EXISTS idx_organizations_owner_id;

-- Organizations table
ALTER TABLE organizations DROP COLUMN id CASCADE;
ALTER TABLE organizations DROP COLUMN owner_id;
ALTER TABLE organizations RENAME COLUMN id_uuid TO id;
ALTER TABLE organizations RENAME COLUMN owner_id_uuid TO owner_id;
ALTER TABLE organizations ADD PRIMARY KEY (id);

-- Users table
ALTER TABLE users DROP COLUMN id CASCADE;
ALTER TABLE users DROP COLUMN organization_id;
ALTER TABLE users RENAME COLUMN id_uuid TO id;
ALTER TABLE users RENAME COLUMN organization_id_uuid TO organization_id;
ALTER TABLE users ADD PRIMARY KEY (id);

-- Step 6: Update other tables to use UUID references
-- Transactions table
ALTER TABLE transactions ADD COLUMN user_id_uuid UUID;
ALTER TABLE transactions ADD COLUMN organization_id_uuid UUID;
ALTER TABLE transactions DROP COLUMN user_id;
ALTER TABLE transactions DROP COLUMN organization_id;
ALTER TABLE transactions RENAME COLUMN user_id_uuid TO user_id;
ALTER TABLE transactions RENAME COLUMN organization_id_uuid TO organization_id;

-- User sessions table (if exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'user_sessions') THEN
        ALTER TABLE user_sessions DROP COLUMN IF EXISTS id CASCADE;
        ALTER TABLE user_sessions DROP COLUMN IF EXISTS user_id;
        ALTER TABLE user_sessions DROP COLUMN IF EXISTS organization_id;
        ALTER TABLE user_sessions ADD COLUMN id UUID PRIMARY KEY DEFAULT uuid_generate_v4();
        ALTER TABLE user_sessions ADD COLUMN user_id UUID;
        ALTER TABLE user_sessions ADD COLUMN organization_id UUID;
    END IF;
END $$;

-- Step 7: Recreate indexes
CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_organizations_owner_id ON organizations(owner_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_organization_id ON transactions(organization_id);

-- Step 8: Recreate foreign key constraints
ALTER TABLE users ADD CONSTRAINT users_organization_id_fkey
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE organizations ADD CONSTRAINT organizations_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE transactions ADD CONSTRAINT transactions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE transactions ADD CONSTRAINT transactions_organization_id_fkey
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

-- API keys should already be UUID, but ensure constraints are correct
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'api_keys') THEN
        ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_user_id_fkey;
        ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_organization_id_fkey;
        ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
        ALTER TABLE api_keys ADD CONSTRAINT api_keys_organization_id_fkey
            FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
    END IF;
END $$;

COMMIT;

-- Verify the changes
\d users
\d organizations
\d transactions
\d api_keys