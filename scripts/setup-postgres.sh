#!/bin/bash
set -e

echo "🔧 Setting up Sovereign Privacy Gateway with PostgreSQL..."

# Update .env file with PostgreSQL configuration
cat > .env << 'EOF'
# Sovereign Privacy Gateway Configuration

# ==================== APPLICATION SETTINGS ====================
GATEWAY_TARGET=development
LOG_LEVEL=debug
PORT=8080
STRICT_MODE=false

# ==================== AI PROVIDER API KEYS ====================
OPENAI_API_KEY=sk-your-openai-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key-here
GOOGLE_API_KEY=your-google-api-key-here

# ==================== DATABASE SETTINGS ====================
# PostgreSQL Configuration
DB_HOST=postgres
DB_PORT=5432
DB_NAME=gateway
DB_USER=gateway
DB_PASSWORD=gateway_password
DB_SSL_MODE=disable

# ==================== REDIS SETTINGS ====================
REDIS_HOST=redis
REDIS_PORT=6379

# ==================== AUTHENTICATION ====================
JWT_SECRET=dev-jwt-secret-change-in-production
SESSION_SECRET=dev-session-secret-change-in-production
EOF

echo "✅ Updated .env file with PostgreSQL configuration"

# Stop any existing containers
echo "🛑 Stopping existing containers..."
docker compose down --volumes

# Start PostgreSQL service first to ensure it's ready
echo "🐘 Starting PostgreSQL service..."
docker compose up -d postgres

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
timeout 60 bash -c 'until docker compose exec postgres pg_isready -U gateway -d gateway; do sleep 2; done'

# Verify the auth schema was applied
echo "🔍 Verifying auth schema was applied..."
docker compose exec postgres psql -U gateway -d gateway -c "\dt" | grep -E "(organizations|users|api_keys)" || {
    echo "❌ Auth schema not found. Checking PostgreSQL logs..."
    docker compose logs postgres
    exit 1
}

echo "✅ Auth schema successfully applied!"

# Show the created tables
echo "📊 Auth tables created:"
docker compose exec postgres psql -U gateway -d gateway -c "\dt"

# Show default data
echo "👤 Default admin user created:"
docker compose exec postgres psql -U gateway -d gateway -c "SELECT email, role, is_active FROM users WHERE role='admin';"

echo ""
echo "🎉 PostgreSQL setup complete!"
echo "📖 Default admin credentials:"
echo "   Email: admin@localhost"
echo "   Password: admin123"
echo "   ⚠️  CHANGE THESE IN PRODUCTION!"
echo ""
echo "🚀 To start the full stack:"
echo "   docker compose up -d"
echo ""
echo "🔗 Access points:"
echo "   Gateway: http://localhost:80/health"
echo "   Dashboard: http://localhost:8081"
echo ""
echo "🗄️  Database connection:"
echo "   Host: localhost:5432"
echo "   Database: gateway"
echo "   User: gateway"
echo "   Password: gateway_password"