# EK-1 Ego-Kernel — developer Makefile
.DEFAULT_GOAL := help
BINARY        := ek1
BUILD_FLAGS   := -ldflags="-s -w"
GO            := go

.PHONY: help build run dev docker-build docker-up docker-down docker-logs \
        docker-update release clean test lint tidy

# ── Help ──────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  EK-1 Ego-Kernel — Makefile"
	@echo ""
	@echo "  Development:"
	@echo "    make build        Build the Go binary locally"
	@echo "    make run          Build and run locally (needs Ollama on host)"
	@echo "    make tidy         Tidy Go modules"
	@echo "    make test         Run tests"
	@echo "    make lint         Run go vet"
	@echo ""
	@echo "  Docker (development — builds from source):"
	@echo "    make docker-build Build the ek1 Docker image"
	@echo "    make docker-up    Start the full stack (builds locally)"
	@echo "    make docker-down  Stop and remove containers"
	@echo "    make docker-logs  Tail all service logs"
	@echo ""
	@echo "  Distribution:"
	@echo "    make release-up   Start using pre-built release images"
	@echo "    make release-down Stop the release stack"
	@echo "    make clean        Remove build artefacts"
	@echo ""

# ── Local build ───────────────────────────────────────────────────────────────
build:
	@echo "Building $(BINARY)..."
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) -o $(BINARY) ./cmd/ek1
	@echo "Done → ./$(BINARY)"

run: build
	@echo "Running EK-1 on :3000 (make sure Ollama is running on the host)..."
	./$(BINARY)

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

# ── Docker (from source) ──────────────────────────────────────────────────────
docker-build:
	docker compose build ek1

docker-up:
	docker compose up -d
	@echo "Stack started → http://localhost"

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# ── Release (pre-built images) ────────────────────────────────────────────────
release-up:
	docker compose -f docker-compose.release.yml up -d
	@echo "Release stack started → http://localhost"

release-down:
	docker compose -f docker-compose.release.yml down

# ── Clean ─────────────────────────────────────────────────────────────────────
clean:
	rm -f $(BINARY)
	$(GO) clean -cache
