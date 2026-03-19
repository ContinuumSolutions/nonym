package migrations

import "os"

// GetAllMigrations returns all available migrations
func GetAllMigrations() []*Migration {
	if isPostgreSQL() {
		return getPostgreSQLMigrations()
	}
	return getSQLiteMigrations()
}

// isPostgreSQL checks if we're using PostgreSQL
func isPostgreSQL() bool {
	return os.Getenv("DB_DRIVER") == "postgres" ||
		   (os.Getenv("DB_HOST") != "" && os.Getenv("DB_NAME") != "")
}

// getPostgreSQLMigrations returns PostgreSQL migrations
func getPostgreSQLMigrations() []*Migration {
	return []*Migration{
		{
			Version:     1,
			Name:        "initial_schema",
			Description: "Create initial auth schema for PostgreSQL with INTEGER IDs",
			UpSQL: `
				-- Organizations table
				CREATE TABLE organizations (
					id SERIAL PRIMARY KEY,
					name VARCHAR(255) NOT NULL UNIQUE,
					slug VARCHAR(100) NOT NULL UNIQUE,
					description TEXT,
					settings JSONB DEFAULT '{}',
					is_active BOOLEAN DEFAULT TRUE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);

				-- Users table
				CREATE TABLE users (
					id SERIAL PRIMARY KEY,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					email VARCHAR(255) NOT NULL UNIQUE,
					password_hash VARCHAR(255) NOT NULL,
					first_name VARCHAR(100),
					last_name VARCHAR(100),
					role VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'viewer', 'owner')),
					is_active BOOLEAN DEFAULT TRUE,
					email_verified BOOLEAN DEFAULT FALSE,
					last_login TIMESTAMP WITH TIME ZONE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);

				-- User sessions table
				CREATE TABLE user_sessions (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					session_token VARCHAR(255) NOT NULL UNIQUE,
					ip_address INET,
					user_agent TEXT,
					expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					last_accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					is_active BOOLEAN DEFAULT TRUE
				);

				-- Refresh tokens table
				CREATE TABLE refresh_tokens (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					token_hash VARCHAR(255) NOT NULL UNIQUE,
					expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					last_used_at TIMESTAMP WITH TIME ZONE,
					ip_address INET,
					user_agent TEXT,
					is_revoked BOOLEAN DEFAULT FALSE
				);

				-- Auth events table for audit logging
				CREATE TABLE auth_events (
					id SERIAL PRIMARY KEY,
					type VARCHAR(100) NOT NULL,
					user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
					organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
					ip_address INET,
					user_agent TEXT,
					success BOOLEAN NOT NULL,
					error_reason TEXT,
					metadata JSONB DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);

				-- Password resets table
				CREATE TABLE password_resets (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					token VARCHAR(255) NOT NULL UNIQUE,
					expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					used_at TIMESTAMP WITH TIME ZONE,
					ip_address INET
				);

				-- Create indexes
				CREATE INDEX idx_users_organization_id ON users(organization_id);
				CREATE INDEX idx_users_email ON users(email);
				CREATE INDEX idx_users_active ON users(is_active);
				CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
				CREATE INDEX idx_user_sessions_token ON user_sessions(session_token);
				CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);
				CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
				CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
				CREATE INDEX idx_auth_events_user_id ON auth_events(user_id);
				CREATE INDEX idx_auth_events_organization_id ON auth_events(organization_id);
				CREATE INDEX idx_auth_events_created_at ON auth_events(created_at);
				CREATE INDEX idx_auth_events_type ON auth_events(type);
				CREATE INDEX idx_password_resets_token ON password_resets(token);
				CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS password_resets CASCADE;
				DROP TABLE IF EXISTS auth_events CASCADE;
				DROP TABLE IF EXISTS refresh_tokens CASCADE;
				DROP TABLE IF EXISTS user_sessions CASCADE;
				DROP TABLE IF EXISTS users CASCADE;
				DROP TABLE IF EXISTS organizations CASCADE;
			`,
		},
		{
			Version:     2,
			Name:        "updated_at_triggers",
			Description: "Add triggers for automatic updated_at timestamp updates",
			UpSQL: `
				-- Function to update updated_at timestamp
				CREATE OR REPLACE FUNCTION update_updated_at_column()
				RETURNS TRIGGER AS $$
				BEGIN
					NEW.updated_at = NOW();
					RETURN NEW;
				END;
				$$ language 'plpgsql';

				-- Triggers for organizations
				CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations
					FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

				-- Triggers for users
				CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
					FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
			`,
			DownSQL: `
				DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
				DROP TRIGGER IF EXISTS update_users_updated_at ON users;
				DROP FUNCTION IF EXISTS update_updated_at_column();
			`,
		},
		{
			Version:     3,
			Name:        "default_data",
			Description: "Insert default organization and admin user",
			UpSQL: `
				-- Insert default organization
				INSERT INTO organizations (name, slug, description)
				VALUES (
					'Default Organization',
					'default',
					'Default organization for initial setup'
				) ON CONFLICT (name) DO NOTHING;

				-- Insert default admin user (password: admin123)
				INSERT INTO users (
					organization_id,
					email,
					password_hash,
					first_name,
					last_name,
					role,
					is_active,
					email_verified
				) VALUES (
					1,
					'admin@localhost',
					'$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj8xwLdqRg3m',
					'Admin',
					'User',
					'owner',
					TRUE,
					TRUE
				) ON CONFLICT (email) DO NOTHING;
			`,
			DownSQL: `
				DELETE FROM users WHERE email = 'admin@localhost';
				DELETE FROM organizations WHERE name = 'Default Organization';
			`,
		},
		{
			Version:     4,
			Name:        "add_owner_id_to_organizations",
			Description: "Add owner_id column to organizations table",
			UpSQL: `
				-- Add owner_id column to organizations
				ALTER TABLE organizations ADD COLUMN owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL;

				-- Create index for owner_id
				CREATE INDEX idx_organizations_owner_id ON organizations(owner_id);
			`,
			DownSQL: `
				-- Remove index
				DROP INDEX IF EXISTS idx_organizations_owner_id;

				-- Remove owner_id column
				ALTER TABLE organizations DROP COLUMN IF EXISTS owner_id;
			`,
		},
		{
			Version:     5,
			Name:        "create_api_keys_table",
			Description: "Create API keys table for authentication",
			UpSQL: `
				-- API keys table
				CREATE TABLE api_keys (
					id SERIAL PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					key_hash VARCHAR(255) NOT NULL,
					masked_key VARCHAR(50) NOT NULL,
					permissions VARCHAR(500) NOT NULL,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					expires_at TIMESTAMP WITH TIME ZONE,
					status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired', 'deleted')),
					last_used TIMESTAMP WITH TIME ZONE
				);

				-- Create indexes for API keys
				CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
				CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
				CREATE INDEX idx_api_keys_status ON api_keys(status);
				CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
				CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS api_keys CASCADE;
			`,
		},
		{
			Version:     6,
			Name:        "create_transactions_table",
			Description: "Create transactions table for audit logging",
			UpSQL: `
				-- Transactions table for audit logging
				CREATE TABLE transactions (
					id SERIAL PRIMARY KEY,
					request_id VARCHAR(255),
					method VARCHAR(10) NOT NULL DEFAULT 'POST',
					path TEXT NOT NULL DEFAULT '/v1/chat/completions',
					provider VARCHAR(100),
					status VARCHAR(50) NOT NULL,
					status_code INTEGER,
					processing_time_ms DOUBLE PRECISION,
					redaction_count INTEGER DEFAULT 0,
					entities_detected JSONB DEFAULT '[]',
					organization_id INTEGER,
					user_id INTEGER,
					ip_address INET,
					user_agent TEXT,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);

				-- Create indexes for transactions
				CREATE INDEX idx_transactions_created_at ON transactions(created_at);
				CREATE INDEX idx_transactions_organization_id ON transactions(organization_id);
				CREATE INDEX idx_transactions_user_id ON transactions(user_id);
				CREATE INDEX idx_transactions_provider ON transactions(provider);
				CREATE INDEX idx_transactions_status ON transactions(status);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS transactions CASCADE;
			`,
		},
	}
}

// getSQLiteMigrations returns SQLite migrations
func getSQLiteMigrations() []*Migration {
	return []*Migration{
		{
			Version:     1,
			Name:        "initial_schema",
			Description: "Create initial auth schema for SQLite",
			UpSQL: `
				-- Organizations table
				CREATE TABLE organizations (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL UNIQUE,
					slug TEXT NOT NULL UNIQUE,
					description TEXT,
					settings TEXT DEFAULT '{}',
					is_active BOOLEAN DEFAULT TRUE,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- Users table
				CREATE TABLE users (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					email TEXT NOT NULL UNIQUE,
					password_hash TEXT NOT NULL,
					first_name TEXT,
					last_name TEXT,
					role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'viewer', 'owner')),
					is_active BOOLEAN DEFAULT TRUE,
					email_verified BOOLEAN DEFAULT FALSE,
					last_login DATETIME,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- User sessions table
				CREATE TABLE user_sessions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					session_token TEXT NOT NULL UNIQUE,
					ip_address TEXT,
					user_agent TEXT,
					expires_at DATETIME NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					last_accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					is_active BOOLEAN DEFAULT TRUE
				);

				-- Refresh tokens table
				CREATE TABLE refresh_tokens (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					token_hash TEXT NOT NULL UNIQUE,
					expires_at DATETIME NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					last_used_at DATETIME,
					ip_address TEXT,
					user_agent TEXT,
					is_revoked BOOLEAN DEFAULT FALSE
				);

				-- Auth events table for audit logging
				CREATE TABLE auth_events (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					type TEXT NOT NULL,
					user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
					organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
					ip_address TEXT,
					user_agent TEXT,
					success BOOLEAN NOT NULL,
					error_reason TEXT,
					metadata TEXT DEFAULT '{}',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- Password resets table
				CREATE TABLE password_resets (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					token TEXT NOT NULL UNIQUE,
					expires_at DATETIME NOT NULL,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					used_at DATETIME,
					ip_address TEXT
				);

				-- Create indexes
				CREATE INDEX idx_users_organization_id ON users(organization_id);
				CREATE INDEX idx_users_email ON users(email);
				CREATE INDEX idx_users_active ON users(is_active);
				CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
				CREATE INDEX idx_user_sessions_token ON user_sessions(session_token);
				CREATE INDEX idx_user_sessions_expires ON user_sessions(expires_at);
				CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
				CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);
				CREATE INDEX idx_auth_events_user_id ON auth_events(user_id);
				CREATE INDEX idx_auth_events_organization_id ON auth_events(organization_id);
				CREATE INDEX idx_auth_events_created_at ON auth_events(created_at);
				CREATE INDEX idx_auth_events_type ON auth_events(type);
				CREATE INDEX idx_password_resets_token ON password_resets(token);
				CREATE INDEX idx_password_resets_user_id ON password_resets(user_id);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS password_resets;
				DROP TABLE IF EXISTS auth_events;
				DROP TABLE IF EXISTS refresh_tokens;
				DROP TABLE IF EXISTS user_sessions;
				DROP TABLE IF EXISTS users;
				DROP TABLE IF EXISTS organizations;
			`,
		},
		{
			Version:     2,
			Name:        "updated_at_triggers",
			Description: "Add triggers for automatic updated_at timestamp updates",
			UpSQL: `
				-- Triggers for organizations
				CREATE TRIGGER update_organizations_updated_at
				AFTER UPDATE ON organizations
				FOR EACH ROW WHEN NEW.updated_at <= OLD.updated_at
				BEGIN
					UPDATE organizations SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;

				-- Triggers for users
				CREATE TRIGGER update_users_updated_at
				AFTER UPDATE ON users
				FOR EACH ROW WHEN NEW.updated_at <= OLD.updated_at
				BEGIN
					UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END;
			`,
			DownSQL: `
				DROP TRIGGER IF EXISTS update_organizations_updated_at;
				DROP TRIGGER IF EXISTS update_users_updated_at;
			`,
		},
		{
			Version:     3,
			Name:        "default_data",
			Description: "Insert default organization and admin user",
			UpSQL: `
				-- Insert default organization
				INSERT OR IGNORE INTO organizations (name, slug, description)
				VALUES (
					'Default Organization',
					'default',
					'Default organization for initial setup'
				);

				-- Insert default admin user (password: admin123)
				INSERT OR IGNORE INTO users (
					organization_id,
					email,
					password_hash,
					first_name,
					last_name,
					role,
					is_active,
					email_verified
				) VALUES (
					1,
					'admin@localhost',
					'$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj8xwLdqRg3m',
					'Admin',
					'User',
					'owner',
					TRUE,
					TRUE
				);
			`,
			DownSQL: `
				DELETE FROM users WHERE email = 'admin@localhost';
				DELETE FROM organizations WHERE name = 'Default Organization';
			`,
		},
		{
			Version:     4,
			Name:        "add_owner_id_to_organizations",
			Description: "Add owner_id column to organizations table",
			UpSQL: `
				-- Add owner_id column to organizations
				ALTER TABLE organizations ADD COLUMN owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL;

				-- Create index for owner_id
				CREATE INDEX idx_organizations_owner_id ON organizations(owner_id);
			`,
			DownSQL: `
				-- Remove index
				DROP INDEX IF EXISTS idx_organizations_owner_id;

				-- Remove owner_id column (SQLite doesn't support DROP COLUMN directly)
				-- This would require table recreation in a real scenario
				-- For now, we'll just note this limitation
			`,
		},
		{
			Version:     5,
			Name:        "create_api_keys_table",
			Description: "Create API keys table for authentication",
			UpSQL: `
				-- API keys table
				CREATE TABLE api_keys (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL,
					key_hash TEXT NOT NULL,
					masked_key TEXT NOT NULL,
					permissions TEXT NOT NULL,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					expires_at DATETIME,
					status TEXT DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired', 'deleted')),
					last_used DATETIME
				);

				-- Create indexes for API keys
				CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
				CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
				CREATE INDEX idx_api_keys_status ON api_keys(status);
				CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
				CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS api_keys;
			`,
		},
		{
			Version:     6,
			Name:        "create_transactions_table",
			Description: "Create transactions table for audit logging",
			UpSQL: `
				-- Transactions table for audit logging
				CREATE TABLE transactions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					request_id TEXT,
					method TEXT NOT NULL DEFAULT 'POST',
					path TEXT NOT NULL DEFAULT '/v1/chat/completions',
					provider TEXT,
					status TEXT NOT NULL,
					status_code INTEGER,
					processing_time_ms REAL,
					redaction_count INTEGER DEFAULT 0,
					entities_detected TEXT DEFAULT '[]',
					organization_id INTEGER,
					user_id INTEGER,
					ip_address TEXT,
					user_agent TEXT,
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- Create indexes for transactions
				CREATE INDEX idx_transactions_created_at ON transactions(created_at);
				CREATE INDEX idx_transactions_organization_id ON transactions(organization_id);
				CREATE INDEX idx_transactions_user_id ON transactions(user_id);
				CREATE INDEX idx_transactions_provider ON transactions(provider);
				CREATE INDEX idx_transactions_status ON transactions(status);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS transactions;
			`,
		},
	}
}