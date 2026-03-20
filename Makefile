# Nonym Makefile

.PHONY: help setup up down restart logs status build build-admin test clean rebuild \
        db-shell redis-shell gateway-shell lint format

# Default target
help:
	@echo "Nonym - Build Commands"
	@echo ""
	@echo "Core Operations:"
	@echo "  setup       Initial setup (creates .env, directories)"
	@echo "  up          Start all services"
	@echo "  build       Start all services with a fresh build"
	@echo "  down        Stop all services"
	@echo "  restart     Restart all services"
	@echo "  logs        View logs from all services"
	@echo "  status      Check service health"
	@echo "  rebuild     Force rebuild images and restart"
	@echo "  clean       Stop services and remove volumes"
	@echo ""
	@echo "Development:"
	@echo "  build-go    Build the Go gateway binary locally"
	@echo "  test        Run tests"
	@echo "  lint        Run go vet"
	@echo "  format      Format Go code"
	@echo ""
	@echo "Utilities:"
	@echo "  db-shell        Connect to PostgreSQL"
	@echo "  redis-shell     Connect to Redis"
	@echo "  gateway-shell   Shell into the gateway container"
	@echo ""
	@echo "Examples:"
	@echo "  make setup && make up    # Initial setup and start"
	@echo "  make build               # Start with fresh build"
	@echo "  make logs                # Tail all service logs"
	@echo "  make down && make clean  # Stop and remove volumes"

# Environment setup
setup:
	@if [ ! -f .env ]; then \
		if [ -f .env.example ]; then \
			cp .env.example .env; \
		else \
			echo "# Nonym Configuration" > .env; \
			echo "PORT=8000" >> .env; \
			echo "LOG_LEVEL=info" >> .env; \
			echo "STRICT_MODE=false" >> .env; \
			echo "DB_NAME=gateway" >> .env; \
			echo "DB_USER=gateway" >> .env; \
			echo "DB_PASSWORD=gateway_password" >> .env; \
			echo "JWT_SECRET=change-in-production-$(shell openssl rand -hex 32)" >> .env; \
			echo "SESSION_SECRET=change-in-production-$(shell openssl rand -hex 32)" >> .env; \
		fi; \
		echo "Created .env - edit it to add your settings"; \
	fi
	@mkdir -p data/{postgres,redis}

# Core operations
up: setup
	docker compose up -d

build: setup
	docker compose up -d --build

down:
	docker compose down

restart:
	docker compose restart

rebuild:
	docker compose down
	docker compose build --no-cache
	docker compose up -d

clean:
	docker compose down -v

# Monitoring
logs:
	docker compose logs -f

status:
	docker compose ps
	@echo ""
	@curl -s http://localhost:8000/health | python3 -m json.tool 2>/dev/null || echo "Gateway health check failed"

# Local Go builds
build-go:
	@mkdir -p bin
	cd cmd/gateway && go build -o ../../bin/gateway .

build-admin:
	@mkdir -p bin
	cd cmd/admin && go build -o ../../bin/admin .

# Tests
test:
	go test ./...

test-coverage:
	@mkdir -p coverage
	go test -coverprofile=coverage/coverage.out ./...
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html

# Code quality
lint:
	go vet ./...

format:
	gofmt -s -w .

# Shell access
db-shell:
	docker compose exec postgres psql -U $${DB_USER:-gateway} -d $${DB_NAME:-gateway}

redis-shell:
	docker compose exec redis redis-cli

gateway-shell:
	docker compose exec gateway sh
