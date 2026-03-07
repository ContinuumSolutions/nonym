# Multi-stage build for Sovereign Privacy Gateway
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the gateway binary
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s -extldflags '-static'" \
    -a -installsuffix cgo \
    -o gateway ./cmd/gateway

# Production stage
FROM alpine:latest AS production

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    curl \
    sqlite \
    tzdata \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 spg && \
    adduser -D -s /bin/sh -u 1001 -G spg spg

# Create necessary directories
RUN mkdir -p /app /data /app/logs && \
    chown -R spg:spg /app /data

# Copy binary and assets
COPY --from=builder /app/gateway /app/
COPY --from=builder /app/dashboard /app/dashboard

# Set ownership
RUN chown -R spg:spg /app

# Switch to non-root user
USER spg

# Set working directory
WORKDIR /app

# Expose ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the gateway
CMD ["./gateway"]

# Development stage
FROM production AS development

# Switch back to root for development tools
USER root

# Install development tools
RUN apk add --no-cache \
    git \
    make \
    bash \
    vim

# Install Go for development
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

# Switch back to spg user
USER spg

# Default command for development
CMD ["./gateway"]
