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

build-admin:
	@echo "🔨 Building admin application"
	@mkdir -p bin
	cd cmd/admin && go build -o ../../bin/admin .
	@echo "✅ Admin built: ./bin/admin"

# Test targets
test:
	@echo "🧪 Running comprehensive test suite"
	@./test_runner.sh --coverage-threshold 75

test-unit:
	@echo "🧪 Running unit tests"
	@./test_runner.sh --unit-only --coverage-threshold 75

test-integration:
	@echo "🔗 Running integration tests"
	@./test_runner.sh --integration-only

test-coverage:
	@echo "📊 Generating test coverage report"
	@mkdir -p coverage
	go test -timeout=10m -v -race -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "✅ Coverage report generated: coverage/coverage.html"

benchmark:
	@echo "⚡ Running benchmark tests"
	@./test_runner.sh --with-benchmarks

test-deps:
	@echo "📦 Installing test dependencies"
	go mod download
	@if ! command -v bc >/dev/null 2>&1; then \
		echo "Installing bc for coverage calculations..."; \
		if command -v apt-get >/dev/null 2>&1; then \
			sudo apt-get update && sudo apt-get install -y bc; \
		elif command -v yum >/dev/null 2>&1; then \
			sudo yum install -y bc; \
		elif command -v brew >/dev/null 2>&1; then \
			brew install bc; \
		else \
			echo "Please install 'bc' manually"; \
		fi \
	fi

# Code quality
lint:
	@echo "🔍 Running linters"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not found, running go vet only..."; \
		go vet ./...; \
	fi

format:
	@echo "🎨 Formatting Go code"
	gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

format-check:
	@echo "🎨 Checking code formatting"
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files need formatting:"; \
		gofmt -s -l .; \
		exit 1; \
	fi

security:
	@echo "🔒 Running security checks"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -fmt=json -out=gosec-report.json -stdout ./...; \
	fi
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	fi

# Tool installation
tools:
	@echo "🔧 Installing development tools"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install golang.org/x/tools/cmd/goimports@latest

clean:
	@echo "🧹 Cleaning up containers and volumes"
	docker compose down -v
	docker system prune -f

clean-test:
	@echo "🧹 Cleaning test artifacts"
	rm -rf coverage/
	rm -f coverage-*.out
	rm -f coverage-report.html
	rm -f benchmark_results.txt
	rm -f gosec-report.json
	go clean -cache
	go clean -testcache

clean-all: clean clean-test
	@echo "🧹 Deep cleaning all artifacts"
	rm -rf bin/

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
