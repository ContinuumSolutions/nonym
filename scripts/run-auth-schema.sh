#!/bin/bash
set -e

echo "🗄️  Running auth schema on existing PostgreSQL instance..."

# Check if PostgreSQL is running
if ! docker compose ps postgres | grep -q "Up"; then
    echo "❌ PostgreSQL service is not running. Start it first with:"
    echo "   docker compose up -d postgres"
    exit 1
fi

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
timeout 30 bash -c 'until docker compose exec postgres pg_isready -U gateway -d gateway; do sleep 2; done' || {
    echo "❌ PostgreSQL is not responding. Check the service status."
    exit 1
}

# Run the auth schema
echo "📝 Applying auth schema..."
docker compose exec -T postgres psql -U gateway -d gateway < database/init/01_auth_schema.sql

echo "✅ Auth schema applied successfully!"

# Verify tables were created
echo "📊 Verifying tables:"
docker compose exec postgres psql -U gateway -d gateway -c "\dt"

echo "👤 Default admin user:"
docker compose exec postgres psql -U gateway -d gateway -c "SELECT id, email, role, is_active FROM users WHERE role='owner';"

echo "🏢 Default organization:"
docker compose exec postgres psql -U gateway -d gateway -c "SELECT id, name, slug, owner_id, is_active FROM organizations WHERE slug='default';"

echo ""
echo "🎉 Done! Auth schema is ready."