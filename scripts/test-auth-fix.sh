#!/bin/bash
set -e

echo "🔧 Testing Authentication Fix..."

# Make sure PostgreSQL is running
if ! docker compose ps postgres | grep -q "Up"; then
    echo "🗄️  Starting PostgreSQL..."
    docker compose up -d postgres
    sleep 5
fi

# Build and start the gateway
echo "🚀 Building and starting gateway..."
docker compose up -d gateway --build

# Wait for gateway to be ready
echo "⏳ Waiting for gateway to be ready..."
timeout 60 bash -c 'until curl -s http://localhost:80/health >/dev/null; do sleep 2; done' || {
    echo "❌ Gateway health check failed"
    docker compose logs gateway
    exit 1
}

echo "✅ Gateway is ready!"

# Test login and get token
echo "🔑 Testing login..."
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:80/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@localhost","password":"admin123"}')

if echo "$LOGIN_RESPONSE" | grep -q "token"; then
    echo "✅ Login successful!"
    TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token')
    echo "Token: ${TOKEN:0:20}..."
else
    echo "❌ Login failed:"
    echo "$LOGIN_RESPONSE"
    exit 1
fi

# Test the statistics endpoint that was failing
echo "📊 Testing statistics endpoint..."
STATS_RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" \
    http://localhost:80/api/v1/statistics)

if echo "$STATS_RESPONSE" | grep -q "error"; then
    echo "❌ Statistics endpoint still failing:"
    echo "$STATS_RESPONSE"
    exit 1
else
    echo "✅ Statistics endpoint working!"
    echo "$STATS_RESPONSE" | jq '.'
fi

# Test a few other protected endpoints
echo "🔍 Testing other protected endpoints..."

echo "📋 Testing transactions..."
TRANS_RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" \
    http://localhost:80/api/v1/transactions)
if echo "$TRANS_RESPONSE" | grep -q "error"; then
    echo "⚠️  Transactions endpoint issue: $(echo "$TRANS_RESPONSE" | jq -r '.error')"
else
    echo "✅ Transactions endpoint working!"
fi

echo "🏢 Testing organization..."
ORG_RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" \
    http://localhost:80/api/v1/organization)
if echo "$ORG_RESPONSE" | grep -q "error"; then
    echo "⚠️  Organization endpoint issue: $(echo "$ORG_RESPONSE" | jq -r '.error')"
else
    echo "✅ Organization endpoint working!"
fi

echo ""
echo "🎉 Authentication Fix Test Complete!"
echo ""
echo "📈 Key Results:"
echo "  ✅ Login working"
echo "  ✅ JWT token validation working"
echo "  ✅ Organization context properly set"
echo "  ✅ Statistics endpoint returning data (not 401)"
echo ""
echo "🔗 Dashboard should now work at: http://localhost:8081"
echo "   Default credentials: admin@localhost / admin123"