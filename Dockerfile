# Sovereign Privacy Gateway - Production Docker Image
# Multi-stage build for optimal security and size

# ────────────────────────────────────────────────────────────────────────────
# Stage 1: Build Environment
# ────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application with optimization
ARG CGO_ENABLED=1
ARG GOOS=linux
ARG GOARCH=amd64
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gateway \
    ./cmd/gateway

# Verify the binary
RUN chmod +x gateway

# ────────────────────────────────────────────────────────────────────────────
# Stage 2: Production Runtime
# ────────────────────────────────────────────────────────────────────────────
FROM alpine:latest AS production

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    curl \
    wget \
    tzdata \
    sqlite \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 -S spguser && \
    adduser -u 1001 -S spguser -G spguser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=spguser:spguser /build/gateway /app/gateway

# Copy dashboard files
COPY --chown=spguser:spguser dashboard/ /app/dashboard/

# Create directories with proper permissions
RUN mkdir -p /data /app/logs /tmp && \
    chown -R spguser:spguser /app /data && \
    chmod 755 /app/gateway && \
    chmod -R 755 /app/dashboard && \
    chmod 755 /data

# Switch to non-root user
USER spguser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT:-8080}/health || exit 1

# Expose ports
EXPOSE 8080 8081

# Environment defaults
ENV PORT=8080 \
    DASHBOARD_PORT=8081 \
    DATABASE_PATH=/data/gateway.db \
    LOG_LEVEL=info \
    STRICT_MODE=false

# Start the application
CMD ["/app/gateway"]

# ────────────────────────────────────────────────────────────────────────────
# Stage 3: Development Environment
# ────────────────────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS development

# Install development tools
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev \
    sqlite-dev \
    curl \
    make \
    bash

WORKDIR /app

# Copy source
COPY . .

# Download dependencies
RUN go mod download

# Create development user
RUN addgroup -g 1001 -S devuser && \
    adduser -u 1001 -S devuser -G devuser && \
    chown -R devuser:devuser /app

# Create data directories
RUN mkdir -p /data /tmp && \
    chown -R devuser:devuser /data

USER devuser

# Development environment variables
ENV PORT=8080 \
    DASHBOARD_PORT=8081 \
    LOG_LEVEL=debug \
    DATABASE_PATH=/data/gateway.db

# Default command for development
CMD ["go", "run", "./cmd/gateway"]
