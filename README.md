# Sovereign Privacy Gateway

A high-performance Privacy Air-Gap middleware written in Go that intercepts data between internal systems and external AI models. The gateway uses Named Entity Recognition (NER) to detect and anonymize Personally Identifiable Information (PII) in real-time, ensuring sensitive data never leaves your infrastructure.

## Overview

The Sovereign Privacy Gateway acts as a transparent reverse proxy that:
- **Detects PII** using advanced pattern matching and NER algorithms
- **Anonymizes data** with tokenization before sending to AI providers
- **De-anonymizes responses** to restore original context for users
- **Routes intelligently** based on content sensitivity and provider capabilities
- **Provides real-time monitoring** via a modern Vue.js dashboard

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client App    │───▶│  Privacy Gateway │───▶│  AI Provider    │
│                 │    │                  │    │ (OpenAI/etc.)   │
│ Original Data   │    │  PII Detection   │    │ Anonymized Data │
└─────────────────┘    │  & Tokenization  │    └─────────────────┘
                       │                  │
                       │  ┌─────────────┐ │
                       │  │ Vue.js      │ │
                       │  │ Dashboard   │ │
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

### 📊 **Modern Dashboard**
- **Vue.js interface** with responsive design and Tailwind CSS
- **Real-time monitoring** of privacy protection events
- **Authentication system** with login/signup functionality
- **Statistics and analytics** for privacy compliance

## Supported AI Providers

- **OpenAI** (GPT-4, GPT-3.5-turbo, etc.)
- **Anthropic** (Claude 3.5, Claude 3, etc.)
- **Google AI** (Gemini Pro, Gemini Flash, etc.)

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Make (for convenience commands)
- API keys from at least one AI provider
- 2GB+ RAM

### 1. Clone and Setup

```bash
git clone https://github.com/sovereignprivacy/gateway
cd gateway

# Set up environment
make setup
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

### 3. Start the Stack

```bash
# Start all services (Gateway + Vue Dashboard + Database + Redis + Nginx)
make up

# Or use docker compose directly:
docker compose up -d
```

### 4. Access the Vue.js Dashboard

Visit **http://localhost** to access the modern Vue.js dashboard with Tailwind CSS styling.

**Default Login:**
- Email: `admin@localhost`
- Password: `admin123`

### 5. Generate Gateway API Key

**⚠️ IMPORTANT: API Key Authentication Required**

As of the latest version, the Sovereign Privacy Gateway requires API key authentication for all proxy requests to enhance security.

1. **Visit the Dashboard**: Go to http://localhost and log in
2. **Navigate to Integrations**: Click on "Integrations" in the sidebar
3. **Generate API Key**:
   - Go to the "API Keys" tab
   - Click "Generate Key"
   - Give it a name (e.g., "Development Key")
   - Copy the generated API key (starts with `spg_`)

### 6. Test the Gateway

```bash
# Health check (no auth required)
curl http://localhost/health

# Test PII detection with your SPG API key
curl -X POST http://localhost/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: spg_your_gateway_api_key_here" \
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

**Note**: You need TWO API keys:
- **SPG API Key** (`X-API-Key` header) - Authenticates you to the gateway
- **Provider API Key** (`Authorization` header) - Your OpenAI/Anthropic/etc. key

The gateway will:
1. Detect the email and SSN
2. Replace them with tokens like `{{EMAIL_A1B2C3}}` and `{{SSN_D4E5F6}}`
3. Send the anonymized request to OpenAI
4. Restore the original data in the response

## Available Make Commands

### Core Operations
```bash
make setup          # Initial setup (creates .env, directories)
make up              # Start all services
make down            # Stop all services
make restart         # Restart all services
make logs            # View logs from all services
make status          # Check service health
```

### Development
```bash
make build           # Build the Go gateway application
make test            # Run all tests
make clean           # Clean up containers and volumes
make rebuild         # Rebuild and restart services
```

### Utilities
```bash
make db-shell        # Connect to PostgreSQL database
make redis-shell     # Connect to Redis instance
make gateway-shell   # Connect to gateway container
make dashboard-shell # Connect to dashboard container
```

## API Usage

### Standard AI Provider Integration

Instead of calling AI providers directly, point your applications to the gateway:

```python
import openai

# Before: Direct to OpenAI
# openai.api_base = "https://api.openai.com/v1"

# After: Through Privacy Gateway (with required SPG API key)
openai.api_base = "http://localhost/v1"

# IMPORTANT: Add SPG API key to all requests
openai.default_headers = {
    "X-API-Key": "spg_your_gateway_api_key_here"
}

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
LOG_LEVEL=info

# Privacy Settings
STRICT_MODE=false    # Set to true to block high-sensitivity data

# AI Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...

# Database Configuration
DB_NAME=gateway
DB_USER=gateway
DB_PASSWORD=gateway_password

# Authentication
JWT_SECRET=change-in-production
SESSION_SECRET=change-in-production
```

### Content-Based Routing

The gateway automatically applies different privacy policies:

- **High Sensitivity** (SSN, Credit Cards): Blocked in strict mode, heavily anonymized otherwise
- **Medium Sensitivity** (Email, Phone Numbers): Anonymized with secure tokenization
- **Low Sensitivity** (General text): Passed through with monitoring

## System Architecture

### Services

The complete stack consists of 5 Docker services:

1. **postgres** - Database for audit logs and user data
2. **redis** - Session storage and caching
3. **gateway** - Go application (privacy engine)
4. **dashboard** - Vue.js frontend application
5. **nginx** - Reverse proxy and load balancer

### Networks

- **frontend** - Dashboard and Nginx communication
- **backend** - Gateway, database, and Redis communication

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

## Dashboard Features

### Modern Vue.js Interface
- **Responsive design** with Tailwind CSS
- **Lucide icons** for consistent iconography
- **Stripe-inspired** color scheme and styling
- **Dark mode support** (coming soon)

### Real-time Monitoring
- Live transaction monitoring
- PII detection statistics
- Provider health and performance
- Privacy compliance metrics

### Authentication System
- Secure login/signup flow
- JWT-based session management
- Protected routes and navigation guards
- User management dashboard

## Testing

### Unit Tests
```bash
# Run all tests
make test

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

## Development

### Project Structure
```
├── cmd/gateway/          # Main application entry point
├── pkg/
│   ├── interceptor/      # HTTP proxy and request handling
│   ├── ner/             # Named Entity Recognition engine
│   ├── router/          # Provider selection and routing
│   └── audit/           # Logging and dashboard API
├── dashboard/           # Vue.js dashboard application
├── nginx/               # Nginx configuration
├── database/            # Database initialization scripts
├── redis/               # Redis configuration
└── scripts/             # Setup and utility scripts
```

### Vue.js Dashboard
```
dashboard/
├── src/
│   ├── components/      # Reusable Vue components
│   ├── views/          # Page components (Login, Dashboard)
│   ├── stores/         # Pinia state management
│   ├── services/       # API service layer
│   └── router/         # Vue Router configuration
├── public/             # Static assets
├── Dockerfile          # Multi-stage Docker build
└── package.json        # Dependencies and scripts
```

### Building from Source
```bash
# Build the gateway binary
go build -o gateway ./cmd/gateway

# Build Vue.js dashboard
cd dashboard
npm install
npm run build

# Build with Docker
docker compose build
```

## License

This project is licensed under the **Sovereign Privacy Gateway Commercial License**.

- ✅ **Free for personal, academic, and research use**
- ✅ **Free for internal business use within a single organization**
- ❌ **Commercial redistribution requires licensing**
- ❌ **SaaS offerings require commercial license**

For commercial licensing, contact: licensing@sovereignprivacy.com

## Migration to v2.0

**⚠️ Breaking Change: API Key Authentication Required**

If you're upgrading from v1.x, API key authentication is now required for all proxy requests. See our [Migration Guide](docs/MIGRATION.md) for step-by-step instructions.

**Quick Migration:**
1. Generate SPG API key in dashboard
2. Add `X-API-Key` header to all requests
3. Keep your existing AI provider keys

[View detailed migration guide →](docs/MIGRATION.md)
[View client integration examples →](examples/client-integration.md)

## Support & Community

- **Documentation**: https://docs.sovereignprivacy.com
- **Migration Guide**: [docs/MIGRATION.md](docs/MIGRATION.md)
- **Client Examples**: [examples/client-integration.md](examples/client-integration.md)
- **Issues**: [GitHub Issues](https://github.com/sovereignprivacy/gateway/issues) for bug reports and feature requests
- **Discussions**: [GitHub Discussions](https://github.com/sovereignprivacy/gateway/discussions) for questions and community
- **Security**: security@sovereignprivacy.com for security issues

---

**Protecting your data sovereignty, one request at a time.**
