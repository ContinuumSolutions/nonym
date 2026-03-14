#!/bin/bash
set -e

echo "🚀 Quick Start: Sovereign Privacy Gateway with PostgreSQL"

# Update .env with PostgreSQL settings if needed
if ! grep -q "DB_HOST=postgres" .env 2>/dev/null; then
    echo "📝 Updating .env with PostgreSQL configuration..."
    cp .env.example .env
    echo "✅ Updated .env file"
fi

echo "🗄️  Starting PostgreSQL database..."
docker compose up -d postgres

echo "⏳ Waiting for PostgreSQL to be ready..."
timeout 60 bash -c 'until docker compose exec postgres pg_isready -U gateway -d gateway; do sleep 2; done'

echo "✅ PostgreSQL is ready!"

# Verify auth schema
echo "🔍 Verifying auth schema..."
ADMIN_COUNT=$(docker compose exec postgres psql -U gateway -d gateway -t -c "SELECT COUNT(*) FROM users WHERE role='admin';")
if [ "${ADMIN_COUNT// /}" = "1" ]; then
    echo "✅ Auth schema applied successfully!"
else
    echo "❌ Auth schema verification failed"
    exit 1
fi

echo ""
echo "🎉 Database Setup Complete!"
echo ""
echo "📊 Database Summary:"
docker compose exec postgres psql -U gateway -d gateway -c "\dt"

echo ""
echo "👤 Admin User:"
docker compose exec postgres psql -U gateway -d gateway -c "SELECT o.name as organization, u.email, u.role, u.is_active FROM users u JOIN organizations o ON u.organization_id = o.id;"

echo ""
echo "🔐 Default Credentials:"
echo "   Email: admin@localhost"
echo "   Password: admin123"
echo "   ⚠️  CHANGE THESE IN PRODUCTION!"
echo ""
echo "🚀 Next Steps:"
echo "   1. Start the full stack: docker compose up -d"
echo "   2. Access dashboard: http://localhost:8081"
echo "   3. Test gateway: http://localhost:80/health"
echo ""
echo "🗄️  Database Connection:"
echo "   docker compose exec postgres psql -U gateway -d gateway"