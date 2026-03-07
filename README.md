# Sovereign Privacy Gateway

A high-performance Privacy Air-Gap middleware written in Go that intercepts data between internal systems and external AI models. The gateway uses Named Entity Recognition (NER) to detect and anonymize Personally Identifiable Information (PII) in real-time, ensuring sensitive data never leaves your infrastructure.

## Overview

The Sovereign Privacy Gateway acts as a transparent reverse proxy that:
- **Detects PII** using advanced pattern matching and NER algorithms
- **Anonymizes data** with tokenization before sending to AI providers
- **De-anonymizes responses** to restore original context for users
- **Routes intelligently** based on content sensitivity and provider capabilities
- **Provides real-time monitoring** via a comprehensive dashboard

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client App    │───▶│  Privacy Gateway │───▶│  AI Provider    │
│                 │    │                  │    │ (OpenAI/etc.)   │
│ Original Data   │    │  PII Detection   │    │ Anonymized Data │
└─────────────────┘    │  & Tokenization  │    └─────────────────┘
                       │                  │
                       │  ┌─────────────┐ │
                       │  │  Dashboard  │ │
                       │  │ (Real-time  │ │
                       │  │ Monitoring) │ │
                       └──┴─────────────┴─┘
```

## Features

### 🔒 **Privacy Protection**
- **Real-time PII Detection**: Email addresses, SSNs, credit cards, API keys, phone numbers
- **Smart Anonymization**: Context-preserving tokenization with secure de-anonymization
- **Strict Mode**: Block requests with high-sensitivity data entirely
- **Confidence Scoring**: AI-powered detection accuracy metrics

### 🚀 **High Performance**
- **Zero-allocation networking** for minimal latency (<20ms processing time)
- **Concurrent processing** with configurable goroutine pools
- **Streaming support** for real-time AI responses
- **Connection pooling** for upstream providers

### 🎯 **Intelligent Routing**
- **Content-based routing**: Financial data can be blocked, General queries → Fastest provider
- **Provider health checking** with automatic failover
- **Load balancing** across multiple AI endpoints
- **Custom routing rules** based on sensitivity levels

### 📊 **Monitoring & Audit**
- **Real-time dashboard** with transaction monitoring
- **Comprehensive logging** of all anonymization events
- **Statistics and analytics** for privacy compliance
- **Prometheus metrics** with Grafana dashboards

## Supported AI Providers

- **OpenAI** (GPT-4, GPT-3.5-turbo, etc.)
- **Anthropic** (Claude 3.5, Claude 3, etc.)
- **Google AI** (Gemini Pro, Gemini Flash, etc.)

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Make (optional, for convenience commands)
- Go 1.24+ (optional, for building admin CLI)
- API keys from at least one AI provider
- 2GB+ RAM

### 1. Clone and Setup

```bash
git clone https://github.com/sovereignprivacy/gateway
cd gateway

# Set up environment (creates data directories and .env file)
make setup

# OR manually:
cp .env.example .env
mkdir -p data/{gateway,logs,postgres,redis}
```

### 2. Configure API Keys

```bash
# Edit the .env file
nano .env

# Add your API keys:
OPENAI_API_KEY=sk-your-openai-api-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key
GOOGLE_API_KEY=your-google-ai-api-key
```

### 3. Choose Your Deployment

The new unified `docker-compose.yml` supports different profiles:

```bash
# Basic development (Gateway + Nginx + Database)
make dev

# With monitoring (adds Grafana, Prometheus, Alertmanager)
make monitoring

# With logging (adds Loki, Promtail)
make logging

# Full production stack (everything)
make full

# OR use docker compose directly:
docker compose --profile monitoring up -d
```

### 4. Access the Modern Dashboard

Visit **http://localhost** (note: port 80, not 8081!) to access the modern Tailwind CSS dashboard.

**Default Login:**
- Email: `admin@localhost`
- Password: `admin123`

**Quick Access URLs:**
- **Dashboard**: http://localhost
- **Monitoring**: http://localhost/monitoring (Grafana)
- **Metrics**: http://localhost/metrics (Prometheus)
- **Alerts**: http://localhost/alerts (Alertmanager)

### 5. Test the Gateway

```bash
# Health check
curl http://localhost/health

# Test PII detection with OpenAI
curl -X POST http://localhost/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-openai-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "My email is john@example.com and my SSN is 123-45-6789"
      }
    ]
  }'
```

The gateway will:
1. Detect the email and SSN
2. Replace them with tokens like `{{EMAIL_A1B2C3}}` and `{{SSN_D4E5F6}}`
3. Send the anonymized request to OpenAI
4. Restore the original data in the response

### 6. User Management

Build the admin CLI for user management:

```bash
# Build admin CLI
make admin-cli

# Create a new user
./bin/admin user create

# List users
./bin/admin user list

# Reset password
./bin/admin user reset-password user@example.com
```

## API Usage

### Standard AI Provider Integration

Instead of calling AI providers directly, point your applications to the gateway:

```python
import openai

# Before: Direct to OpenAI
# openai.api_base = "https://api.openai.com/v1"

# After: Through Privacy Gateway
openai.api_base = "http://localhost:8080/v1"

# Use OpenAI SDK normally - PII protection is automatic
response = openai.ChatCompletion.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Analyze this customer data..."}]
)
```

### Provider Routing

The gateway automatically routes based on content sensitivity:

```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "General business question..."
    }
  ]
}
```

Routes to the fastest available provider (typically OpenAI).

```json
{
  "model": "claude-3-sonnet",
  "messages": [
    {
      "role": "user",
      "content": "Personal healthcare data..."
    }
  ]
}
```

Routes to Anthropic for privacy-sensitive content.

## Configuration

### Environment Variables

```bash
# Gateway Configuration
PORT=8080
DASHBOARD_PORT=8081
LOG_LEVEL=info

# Privacy Settings
STRICT_MODE=false    # Set to true to block high-sensitivity data
RETENTION_DAYS=30    # Audit log retention

# AI Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
```

### Content-Based Routing

The gateway automatically applies different privacy policies:

- **High Sensitivity** (SSN, Credit Cards): Blocked in strict mode, heavily anonymized otherwise
- **Medium Sensitivity** (Email, Phone Numbers): Anonymized with secure tokenization
- **Low Sensitivity** (General text): Passed through with monitoring

## Deployment

### Unified Docker Compose

The new unified `docker-compose.yml` replaces the old multi-file approach. Use profiles to control which services to start:

```bash
# Development Environment
make dev                    # Basic: Gateway + Nginx + PostgreSQL + Redis

# Production Profiles
make monitoring            # + Prometheus + Grafana + Alertmanager
make logging              # + Loki + Promtail
make backup              # + Database backup service
make full                # All services

# Manual profile usage
docker compose --profile monitoring up -d
docker compose --profile logging up -d
docker compose --profile full up -d
```

### Build Commands

```bash
# Build gateway application
make build

# Build admin CLI
make admin-cli

# Run tests
make test

# View logs
make logs

# Check service health
make health
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests (coming soon)
kubectl apply -f k8s/

# Or use Helm chart (coming soon)
helm install privacy-gateway ./charts/privacy-gateway
```

## Security & Compliance

### Data Protection
- **No data persistence** of original PII (only anonymized tokens)
- **Encryption at rest** for audit logs and configuration
- **TLS termination** at nginx layer
- **Network isolation** with Docker networks
- **Non-root containers** with read-only filesystems

### Compliance Features
- **Audit trail** of all anonymization events
- **Data retention policies** with automatic cleanup
- **Privacy impact assessments** via dashboard analytics
- **GDPR/CCPA compliance** through data minimization

### Security Headers
- Content Security Policy (CSP)
- HTTP Strict Transport Security (HSTS)
- X-Frame-Options, X-Content-Type-Options
- Rate limiting and DDoS protection

## Monitoring & Observability

### Built-in Dashboard
- Real-time transaction monitoring
- PII detection statistics
- Provider health and performance
- Privacy compliance metrics

### Metrics & Alerting
- **Prometheus metrics** export
- **Grafana dashboards** for visualization
- **Custom alert rules** for privacy violations
- **Log aggregation** with Loki

### Health Checks
- `/health` - Overall system health
- `/gateway/status` - Detailed component status
- `/gateway/stats` - Performance statistics

## Testing

### Unit Tests
```bash
# Run all tests
go test ./...

# Test specific components
go test ./pkg/ner -v          # PII detection engine
go test ./pkg/router -v       # Provider routing
go test ./pkg/interceptor -v  # HTTP proxy
go test ./pkg/audit -v        # Audit logging

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests
```bash
# End-to-end testing
go test ./cmd/gateway -v

# Performance benchmarks
go test -bench=. ./pkg/...
```

### Verification Script
```bash
# Comprehensive system verification
./scripts/verify-setup.sh
```

## Development

### Project Structure
```
├── cmd/gateway/          # Main application entry point
├── pkg/
│   ├── interceptor/      # HTTP proxy and request handling
│   ├── ner/             # Named Entity Recognition engine
│   ├── router/          # Provider selection and routing
│   └── audit/           # Logging and dashboard API
├── dashboard/           # Web dashboard interface
├── nginx/               # Nginx configuration
├── monitoring/          # Prometheus, Grafana configs
├── scripts/             # Setup and utility scripts
└── docs/               # Documentation
```

### Building from Source
```bash
# Build the gateway binary
go build -o gateway ./cmd/gateway

# Build with optimizations
CGO_ENABLED=1 go build -ldflags="-w -s" -o gateway ./cmd/gateway

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o gateway-linux ./cmd/gateway
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make changes and add tests
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## Documentation

- **[Installation Guide](docs/installation.md)** - Complete setup instructions
- **[API Reference](docs/api-reference.md)** - API documentation (coming soon)
- **[Configuration Guide](docs/configuration.md)** - Advanced configuration (coming soon)
- **[Security Guide](docs/security.md)** - Security best practices (coming soon)

## License

This project is licensed under the **Sovereign Privacy Gateway Commercial License**.

- ✅ **Free for personal, academic, and research use**
- ✅ **Free for internal business use within a single organization**
- ❌ **Commercial redistribution requires licensing**
- ❌ **SaaS offerings require commercial license**

For commercial licensing, contact: licensing@sovereignprivacy.com

## Support & Community

- **Documentation**: https://docs.sovereignprivacy.com
- **Issues**: [GitHub Issues](https://github.com/sovereignprivacy/gateway/issues) for bug reports and feature requests
- **Discussions**: [GitHub Discussions](https://github.com/sovereignprivacy/gateway/discussions) for questions and community
- **Security**: security@sovereignprivacy.com for security issues

## Roadmap

### v1.1 - Enhanced NER
- [ ] Advanced ML models for entity detection
- [ ] Custom entity type definitions
- [ ] Multi-language PII detection

### v1.2 - Enterprise Features
- [ ] SSO/SAML integration
- [ ] Advanced access controls
- [ ] Multi-tenant support

### v1.3 - AI Enhancements
- [ ] LLM-powered content classification
- [ ] Intent-based routing
- [ ] Automated policy learning

---

**Protecting your data sovereignty, one request at a time.**
