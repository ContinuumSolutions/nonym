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
			Description: "Create initial auth schema for PostgreSQL with UUID support",
			UpSQL: `
				-- Enable UUID extension
				CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

				-- Organizations table
				CREATE TABLE organizations (
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
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
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
					organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
					user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
					user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
					type VARCHAR(100) NOT NULL,
					user_id UUID REFERENCES users(id) ON DELETE SET NULL,
					organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
					ip_address INET,
					user_agent TEXT,
					success BOOLEAN NOT NULL,
					error_reason TEXT,
					metadata JSONB DEFAULT '{}',
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);

				-- Password resets table
				CREATE TABLE password_resets (
					id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
					user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
				INSERT INTO organizations (id, name, slug, description)
				VALUES (
					'00000000-0000-0000-0000-000000000001',
					'Default Organization',
					'default',
					'Default organization for initial setup'
				) ON CONFLICT (id) DO NOTHING;

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
					'00000000-0000-0000-0000-000000000001',
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
				DELETE FROM organizations WHERE id = '00000000-0000-0000-0000-000000000001';
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
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
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
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
					organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
					user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
					user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
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
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
					type TEXT NOT NULL,
					user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
					organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL,
					ip_address TEXT,
					user_agent TEXT,
					success BOOLEAN NOT NULL,
					error_reason TEXT,
					metadata TEXT DEFAULT '{}',
					created_at DATETIME DEFAULT CURRENT_TIMESTAMP
				);

				-- Password resets table
				CREATE TABLE password_resets (
					id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6)))),
					user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
				INSERT OR IGNORE INTO organizations (id, name, slug, description)
				VALUES (
					'00000000-0000-0000-0000-000000000001',
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
					'00000000-0000-0000-0000-000000000001',
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
				DELETE FROM organizations WHERE id = '00000000-0000-0000-0000-000000000001';
			`,
		},
	}
}