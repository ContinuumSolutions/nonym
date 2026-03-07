# Sovereign Privacy Gateway Makefile

.PHONY: help setup up down restart logs status build test clean rebuild

# Default target
help:
	@echo "Sovereign Privacy Gateway - Build Commands"
	@echo ""
	@echo "Core Operations:"
	@echo "  setup       Initial setup (creates .env, directories)"
	@echo "  up          Start all services (Gateway + Vue Dashboard + Database + Redis + Nginx)"
	@echo "  down        Stop all services"
	@echo "  restart     Restart all services"
	@echo "  logs        View logs from all services"
	@echo "  status      Check service health"
	@echo ""
	@echo "Development:"
	@echo "  build       Build the Go gateway application"
	@echo "  test        Run all tests"
	@echo "  clean       Clean up containers and volumes"
	@echo "  rebuild     Rebuild and restart services"
	@echo ""
	@echo "Utilities:"
	@echo "  db-shell        Connect to PostgreSQL database"
	@echo "  redis-shell     Connect to Redis instance"
	@echo "  gateway-shell   Connect to gateway container"
	@echo "  dashboard-shell Connect to dashboard container"
	@echo ""
	@echo "Examples:"
	@echo "  make setup && make up    # Initial setup and start"
	@echo "  make logs                # View all service logs"
	@echo "  make down && make clean  # Stop and cleanup"

# Environment setup
setup:
	@echo "⚙️ Setting up development environment"
	@if [ ! -f .env ]; then \
		echo "Creating .env file from template..."; \
		if [ -f .env.example ]; then \
			cp .env.example .env; \
		else \
			echo "# Sovereign Privacy Gateway Configuration" > .env; \
			echo "# AI Provider API Keys" >> .env; \
			echo "OPENAI_API_KEY=sk-your-openai-api-key" >> .env; \
			echo "ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key" >> .env; \
			echo "GOOGLE_API_KEY=your-google-ai-api-key" >> .env; \
			echo "# Gateway Configuration" >> .env; \
			echo "PORT=8080" >> .env; \
			echo "LOG_LEVEL=info" >> .env; \
			echo "STRICT_MODE=false" >> .env; \
			echo "# Database Configuration" >> .env; \
			echo "DB_NAME=gateway" >> .env; \
			echo "DB_USER=gateway" >> .env; \
			echo "DB_PASSWORD=gateway_password" >> .env; \
			echo "# Authentication" >> .env; \
			echo "JWT_SECRET=change-in-production-$(shell openssl rand -hex 32)" >> .env; \
			echo "SESSION_SECRET=change-in-production-$(shell openssl rand -hex 32)" >> .env; \
		fi; \
		echo "✅ Created .env file - please edit with your API keys"; \
	else \
		echo "✅ .env file already exists"; \
	fi
	@echo "📁 Creating data directories..."
	@mkdir -p data/{postgres,redis}
	@echo "✅ Setup complete!"

# Core operations
up:
	@echo "🚀 Starting Sovereign Privacy Gateway with Vue.js Dashboard"
	@make setup
	docker compose up -d
	@echo ""
	@echo "✅ Services started!"
	@echo "🌐 Dashboard: http://localhost"
	@echo "🔧 Default login: admin@localhost / admin123"

down:
	@echo "⏹️ Stopping all services"
	docker compose down

restart:
	@echo "🔄 Restarting all services"
	docker compose restart

# Monitoring and logs
logs:
	@echo "📋 Showing logs from all services"
	docker compose logs -f

status:
	@echo "📊 Service Status"
	@echo "=================="
	docker compose ps
	@echo ""
	@echo "🩺 Health Check"
	@echo "==============="
	@curl -s http://localhost/health | jq '.' 2>/dev/null || echo "❌ Gateway health check failed"
	@echo ""

# Development
build:
	@echo "🔨 Building gateway application"
	@mkdir -p bin
	cd cmd/gateway && go build -o ../../bin/gateway .
	@echo "✅ Gateway built: ./bin/gateway"

test:
	@echo "🧪 Running tests"
	go test ./... -v

clean:
	@echo "🧹 Cleaning up containers and volumes"
	docker compose down -v
	docker system prune -f

rebuild:
	@echo "🔄 Rebuilding and restarting services"
	docker compose down
	docker compose build --no-cache
	docker compose up -d

# Shell access
db-shell:
	@echo "🐘 Opening PostgreSQL database shell"
	docker compose exec postgres psql -U ${DB_USER:-gateway} -d ${DB_NAME:-gateway}

redis-shell:
	@echo "🔴 Opening Redis shell"
	docker compose exec redis redis-cli

gateway-shell:
	@echo "🚪 Opening gateway container shell"
	docker compose exec gateway sh

dashboard-shell:
	@echo "🖥️ Opening dashboard container shell"
	docker compose exec dashboard sh

# Health and debugging
health:
	@echo "🩺 Comprehensive Health Check"
	@echo "============================="
	@echo "Testing gateway health endpoint..."
	@curl -s http://localhost/health | jq '.' 2>/dev/null && echo "✅ Gateway: OK" || echo "❌ Gateway: FAILED"
	@echo ""
	@echo "Testing dashboard availability..."
	@curl -s -o /dev/null -w "%{http_code}" http://localhost >/dev/null && echo "✅ Dashboard: OK" || echo "❌ Dashboard: FAILED"
	@echo ""
	@echo "Testing API endpoint..."
	@curl -s -o /dev/null -w "%{http_code}" http://localhost/api/health >/dev/null && echo "✅ API: OK" || echo "❌ API: FAILED"

# Database operations
db-backup:
	@echo "💾 Creating database backup"
	@mkdir -p backups
	docker compose exec postgres pg_dump -U ${DB_USER:-gateway} ${DB_NAME:-gateway} > backups/gateway_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "✅ Backup created in backups/ directory"

db-restore:
	@echo "📥 Restoring database from backup"
	@echo "Available backups:"
	@ls -la backups/*.sql 2>/dev/null || echo "No backups found in backups/ directory"
	@echo "Usage: docker compose exec postgres psql -U gateway -d gateway < backups/your-backup.sql"

# Development utilities
dev-setup:
	@echo "🛠️ Setting up development environment"
	@make setup
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Development environment ready"

dashboard-build:
	@echo "🏗️ Building Vue.js dashboard locally"
	cd dashboard && npm install && npm run build
	@echo "✅ Dashboard built"

dashboard-dev:
	@echo "🖥️ Starting dashboard in development mode"
	cd dashboard && npm run dev

# Quick commands
quick-start: setup up

quick-stop: down clean

reset: down clean up

# SSL setup (optional)
ssl-setup:
	@echo "🔒 Setting up self-signed SSL certificates"
	@mkdir -p nginx/ssl
	openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout nginx/ssl/key.pem \
		-out nginx/ssl/cert.pem \
		-subj "/C=US/ST=State/L=City/O=SovereignPrivacy/CN=localhost"
	@echo "✅ Self-signed SSL certificates created in nginx/ssl/"
	@echo "⚠️  Remember to update nginx configuration to use HTTPS"
