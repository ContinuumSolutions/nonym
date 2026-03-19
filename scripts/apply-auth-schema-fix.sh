#!/bin/bash

# Apply Auth Schema Fix Script
# This script ensures the database has all required columns for the auth system

echo "🔧 Applying auth schema fixes..."

# Check if we're adding to existing database or creating new
echo "Checking current database state..."

# Add missing columns if they don't exist (safe for existing databases)
docker compose exec -T postgres psql -U gateway -d gateway << 'EOF'
-- Add owner_id column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='organizations' AND column_name='owner_id') THEN
        ALTER TABLE organizations ADD COLUMN owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
        CREATE INDEX IF NOT EXISTS idx_organizations_owner_id ON organizations(owner_id);
        RAISE NOTICE 'Added owner_id column to organizations table';
    ELSE
        RAISE NOTICE 'owner_id column already exists';
    END IF;
END $$;

-- Add is_active column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='organizations' AND column_name='is_active') THEN
        ALTER TABLE organizations ADD COLUMN is_active BOOLEAN DEFAULT TRUE;
        RAISE NOTICE 'Added is_active column to organizations table';
    ELSE
        RAISE NOTICE 'is_active column already exists';
    END IF;
END $$;

-- Update role constraint to include 'owner' if needed
DO $$
BEGIN
    -- Drop old constraint if it exists
    IF EXISTS (SELECT 1 FROM information_schema.constraint_column_usage
               WHERE constraint_name = 'users_role_check') THEN
        ALTER TABLE users DROP CONSTRAINT users_role_check;
    END IF;

    -- Add new constraint with owner role
    ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role::text = ANY (ARRAY['owner'::character varying, 'admin'::character varying, 'user'::character varying, 'viewer'::character varying]::text[]));

    RAISE NOTICE 'Updated role constraint to include owner role';
END $$;

-- Verify the fixes
\d organizations;
\d+ users;

SELECT 'Auth schema fixes applied successfully!' as status;
EOF

echo "✅ Auth schema fixes completed!"
echo ""
echo "🧪 Testing registration endpoint..."

# Test the registration endpoint
response=$(curl -s -X POST http://localhost/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123","name":"Test User","organization":"Test Org"}')

if echo "$response" | grep -q "User registered successfully"; then
    echo "✅ Registration test successful!"
else
    echo "❌ Registration test failed:"
    echo "$response"
fi