-- Complete database reset script
-- This will delete ALL data and tables, then recreate with INTEGER IDs only

-- Drop all tables in correct order (respecting foreign key dependencies)
DROP TABLE IF EXISTS password_resets CASCADE;
DROP TABLE IF EXISTS auth_events CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS provider_configs CASCADE;
DROP TABLE IF EXISTS settings CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS schema_migrations CASCADE;

-- Drop any remaining sequences
DROP SEQUENCE IF EXISTS organizations_id_seq CASCADE;
DROP SEQUENCE IF EXISTS users_id_seq CASCADE;
DROP SEQUENCE IF EXISTS transactions_id_seq CASCADE;
DROP SEQUENCE IF EXISTS api_keys_id_seq CASCADE;
DROP SEQUENCE IF EXISTS user_sessions_id_seq CASCADE;

-- Verify all tables are gone
SELECT tablename FROM pg_tables WHERE schemaname = 'public';