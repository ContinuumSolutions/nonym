#!/bin/bash

# PostgreSQL Migration Script for Nonym
# This script connects to the PostgreSQL database and ensures all schema is up-to-date

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Database connection settings
DB_HOST="postgres"
DB_NAME="${DB_NAME:-gateway}"
DB_USER="${DB_USER:-gateway}"
DB_PASSWORD="${DB_PASSWORD:-gateway_password}"

echo -e "${BLUE}🗄️  Nonym - PostgreSQL Migration${NC}"
echo "=================================================="

# Function to run psql commands
run_psql() {
    docker compose exec -T postgres psql -h localhost -U "$DB_USER" -d "$DB_NAME" "$@"
}

# Function to check if postgres is ready
check_postgres() {
    echo -e "${YELLOW}📡 Checking PostgreSQL connection...${NC}"

    for i in {1..30}; do
        if docker compose exec postgres pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
            echo -e "${GREEN}✅ PostgreSQL is ready${NC}"
            return 0
        fi
        echo "⏳ Waiting for PostgreSQL... (attempt $i/30)"
        sleep 2
    done

    echo -e "${RED}❌ PostgreSQL not available after 60 seconds${NC}"
    exit 1
}

# Function to check current schema
check_schema() {
    echo -e "${YELLOW}🔍 Checking current database schema...${NC}"

    echo "Current tables:"
    run_psql -c "\dt" || echo "No tables found"

    echo -e "\nChecking specific tables and columns:"

    # Check organizations table
    echo -e "${BLUE}Organizations table:${NC}"
    run_psql -c "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = 'organizations' ORDER BY ordinal_position;" || echo "Table not found"

    # Check users table
    echo -e "${BLUE}Users table:${NC}"
    run_psql -c "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = 'users' ORDER BY ordinal_position;" || echo "Table not found"

    # Check transactions table
    echo -e "${BLUE}Transactions table:${NC}"
    run_psql -c "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = 'transactions' ORDER BY ordinal_position;" || echo "Table not found"

    # Check api_keys table
    echo -e "${BLUE}API Keys table:${NC}"
    run_psql -c "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = 'api_keys' ORDER BY ordinal_position;" || echo "Table not found"
}

# Function to run full migration
run_migration() {
    echo -e "${YELLOW}🚀 Running PostgreSQL migrations...${NC}"

    cat << 'EOF' | run_psql
-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create organizations table if not exists
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create users table if not exists
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    active BOOLEAN DEFAULT true,
    last_login TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create transactions table if not exists
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    request_id VARCHAR(255),
    method VARCHAR(10) NOT NULL,
    path TEXT NOT NULL,
    provider VARCHAR(100),
    status VARCHAR(50) NOT NULL,
    status_code INTEGER,
    redaction_count INTEGER DEFAULT 0,
    redaction_details JSONB DEFAULT '[]',
    processing_time REAL DEFAULT 0,
    client_ip VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create audit_logs table if not exists
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    details JSONB DEFAULT '{}',
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create api_keys table if not exists
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    encrypted_key TEXT NOT NULL,
    masked_key VARCHAR(50) NOT NULL,
    permissions JSONB NOT NULL DEFAULT '[]',
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) DEFAULT 'active',
    last_used TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_organizations_slug ON organizations(slug);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);
CREATE INDEX IF NOT EXISTS idx_transactions_organization_id ON transactions(organization_id);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_transactions_provider ON transactions(provider);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_organization_id ON audit_logs(organization_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default organization if it doesn't exist
INSERT INTO organizations (id, name, slug, description)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'Default Organization',
    'default',
    'Default organization for the Nonym'
) ON CONFLICT (id) DO NOTHING;

-- Insert default admin user if it doesn't exist
INSERT INTO users (id, email, password, name, role, organization_id)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@gateway.local',
    '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewztHOt0S40OLR2a', -- password: admin123
    'System Administrator',
    'admin',
    '00000000-0000-0000-0000-000000000001'
) ON CONFLICT (id) DO NOTHING;

EOF

    echo -e "${GREEN}✅ PostgreSQL migration completed successfully${NC}"
}

# Function to verify migration
verify_migration() {
    echo -e "${YELLOW}✅ Verifying migration results...${NC}"

    echo "Final table count:"
    run_psql -c "SELECT schemaname, tablename FROM pg_tables WHERE schemaname = 'public';"

    echo -e "\nRecord counts:"
    run_psql -c "
    SELECT 'organizations' as table_name, count(*) as records FROM organizations
    UNION ALL
    SELECT 'users' as table_name, count(*) as records FROM users
    UNION ALL
    SELECT 'transactions' as table_name, count(*) as records FROM transactions
    UNION ALL
    SELECT 'audit_logs' as table_name, count(*) as records FROM audit_logs
    UNION ALL
    SELECT 'api_keys' as table_name, count(*) as records FROM api_keys
    ORDER BY table_name;
    "

    echo -e "${GREEN}✅ Migration verification completed${NC}"
}

# Main execution
main() {
    case "${1:-migrate}" in
        "check")
            check_postgres
            check_schema
            ;;
        "migrate"|"")
            check_postgres
            echo -e "\n${YELLOW}📋 Current schema status:${NC}"
            check_schema
            echo -e "\n${YELLOW}🔄 Running migration...${NC}"
            run_migration
            echo -e "\n${YELLOW}🔍 Verifying results...${NC}"
            verify_migration
            ;;
        "reset")
            echo -e "${RED}⚠️  WARNING: This will delete ALL data in the database!${NC}"
            read -p "Are you sure you want to reset the database? (type 'yes' to confirm): " confirm
            if [ "$confirm" = "yes" ]; then
                check_postgres
                echo -e "${YELLOW}🗑️  Dropping all tables...${NC}"
                run_psql -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
                run_migration
                verify_migration
                echo -e "${GREEN}✅ Database reset completed${NC}"
            else
                echo "Reset cancelled."
            fi
            ;;
        "connect")
            check_postgres
            echo -e "${BLUE}🔗 Connecting to PostgreSQL (type \\q to exit)${NC}"
            docker compose exec postgres psql -h localhost -U "$DB_USER" -d "$DB_NAME"
            ;;
        *)
            echo "Usage: $0 {check|migrate|reset|connect}"
            echo ""
            echo "Commands:"
            echo "  check   - Check current database schema"
            echo "  migrate - Run full migration (default)"
            echo "  reset   - Reset database (DESTRUCTIVE)"
            echo "  connect - Connect to PostgreSQL shell"
            exit 1
            ;;
    esac
}

main "$@"