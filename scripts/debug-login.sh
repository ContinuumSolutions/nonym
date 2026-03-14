#!/bin/bash
set -e

echo "🔍 Debugging Login Process..."

# Test if the user exists in the database
echo "1. Checking if user exists in PostgreSQL:"
USER_COUNT=$(docker compose exec postgres psql -U gateway -d gateway -t -c "SELECT COUNT(*) FROM users WHERE email='admin@localhost';")
echo "   Users found: $USER_COUNT"

if [ "${USER_COUNT// /}" != "1" ]; then
    echo "❌ User not found in database!"
    exit 1
fi

# Check user details
echo "2. User details:"
docker compose exec postgres psql -U gateway -d gateway -c "SELECT id, email, role, is_active, organization_id FROM users WHERE email='admin@localhost';"

# Check if auth system is using PostgreSQL
echo "3. Testing auth system database connection..."
docker compose exec gateway sh -c 'echo "SELECT 1 as test;" | env'

# Test a simple health check
echo "4. Testing gateway health:"
HEALTH=$(curl -s http://localhost:80/health)
echo "   Health: $HEALTH"

# Test with different credentials
echo "5. Testing login with detailed curl output:"
curl -i -X POST http://localhost:80/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@localhost","password":"admin123"}'

echo ""
echo "🔧 Login debug complete."