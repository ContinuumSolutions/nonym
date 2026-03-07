# Sovereign Privacy Gateway Makefile

.PHONY: help build admin-cli dev prod monitoring logging backup clean test

# Default target
help:
	@echo "Sovereign Privacy Gateway - Build Commands"
	@echo ""
	@echo "Development:"
	@echo "  dev         Start development environment (gateway + nginx + postgres)"
	@echo "  monitoring  Start with monitoring stack (Grafana, Prometheus, Alertmanager)"
	@echo "  logging     Start with logging stack (Loki, Promtail)"
	@echo "  backup      Start with backup service"
	@echo "  full        Start all services"
	@echo "  clean       Clean up containers and volumes"
	@echo ""
	@echo "Administration:"
	@echo "  admin-cli   Build admin CLI tool"
	@echo "  user-create Create a new user (requires admin-cli)"
	@echo "  user-list   List users in organization (requires admin-cli)"
	@echo "  org-list    List all organizations (requires admin-cli)"
	@echo ""
	@echo "Build:"
	@echo "  build       Build gateway application"
	@echo "  test        Run tests"
	@echo ""
	@echo "Examples:"
	@echo "  make dev                    # Start basic development environment"
	@echo "  make monitoring             # Start with monitoring enabled"
	@echo "  make admin-cli && ./admin   # Build and use admin CLI"

# Development environments
dev:
	@echo "🚀 Starting Sovereign Privacy Gateway (Development)"
	@mkdir -p data/{gateway,logs,postgres,redis}
	docker compose up -d postgres redis nginx gateway

monitoring:
	@echo "📊 Starting with monitoring stack"
	@mkdir -p data/{gateway,logs,postgres,redis,prometheus,grafana,alertmanager}
	docker compose --profile monitoring up -d

logging:
	@echo "📝 Starting with logging stack"
	@mkdir -p data/{gateway,logs,postgres,redis,loki}
	docker compose --profile logging up -d

backup:
	@echo "💾 Starting with backup service"
	@mkdir -p data/{gateway,logs,postgres,redis,backups}
	docker compose --profile backup up -d

full:
	@echo "🔥 Starting ALL services"
	@mkdir -p data/{gateway,logs,postgres,redis,prometheus,grafana,alertmanager,loki,backups}
	docker compose --profile full up -d

# Build commands
build:
	@echo "🔨 Building gateway application"
	cd cmd/gateway && go build -o ../../bin/gateway .

admin-cli:
	@echo "🔧 Building admin CLI"
	@mkdir -p bin
	cd cmd/admin && go build -o ../../bin/admin .
	@echo "✅ Admin CLI built: ./bin/admin"
	@echo ""
	@echo "Usage examples:"
	@echo "  ./bin/admin user create              # Create a new user"
	@echo "  ./bin/admin user list default       # List users in default org"
	@echo "  ./bin/admin user reset-password user@example.com"
	@echo "  ./bin/admin org create              # Create a new organization"
	@echo "  ./bin/admin org list                # List all organizations"

# Admin CLI shortcuts
user-create: admin-cli
	@echo "👤 Creating new user..."
	./bin/admin user create

user-list: admin-cli
	@echo "📋 Listing users..."
	./bin/admin user list

org-list: admin-cli
	@echo "🏢 Listing organizations..."
	./bin/admin org list

# Testing
test:
	@echo "🧪 Running tests"
	go test ./...

# Cleanup
clean:
	@echo "🧹 Cleaning up containers and volumes"
	docker compose down -v
	docker system prune -f

stop:
	@echo "⏹️ Stopping all services"
	docker compose down

restart:
	@echo "🔄 Restarting services"
	docker compose restart

logs:
	@echo "📋 Showing gateway logs"
	docker compose logs -f gateway

status:
	@echo "📊 Service status"
	docker compose ps

# Database operations
db-shell:
	@echo "🐘 Opening database shell"
	docker compose exec postgres psql -U gateway -d gateway

db-backup:
	@echo "💾 Creating database backup"
	@mkdir -p backups
	docker compose exec postgres pg_dump -U gateway gateway > backups/gateway_$(shell date +%Y%m%d_%H%M%S).sql

# Environment setup
setup:
	@echo "⚙️ Setting up development environment"
	@if [ ! -f .env ]; then \
		echo "Creating .env file from template..."; \
		cp .env.example .env; \
		echo "✅ Please edit .env file with your API keys"; \
	fi
	@mkdir -p data/{gateway,logs,postgres,redis,prometheus,grafana,alertmanager,loki,backups}
	@echo "📁 Created data directories"

# Health checks
health:
	@echo "🩺 Checking service health"
	@echo "Gateway: " && curl -s http://localhost/health | jq '.' || echo "❌ Gateway not responding"
	@echo "Nginx: " && curl -s -o /dev/null -w "%{http_code}" http://localhost && echo " ✅ Nginx OK" || echo " ❌ Nginx not responding"

# SSL setup (optional)
ssl-setup:
	@echo "🔒 Setting up SSL certificates"
	@mkdir -p nginx/ssl
	@openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
		-keyout nginx/ssl/key.pem \
		-out nginx/ssl/cert.pem \
		-subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
	@echo "✅ Self-signed SSL certificates created"
