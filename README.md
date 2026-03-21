# Nonym

**Stop leaking data to vendors.** Nonym is a high-performance Privacy Gateway that intercepts all outbound traffic to AI providers and third-party monitoring tools (Sentry, Datadog, PostHog), stripping PII before it ever leaves your infrastructure.

## Overview

Every time your app sends an error report, an analytics event, or an AI prompt, it ships user data — names, emails, IP addresses, credit card numbers — straight to external vendors. Nonym sits in front of those vendors and redacts that data in real time.

Nonym acts as a transparent reverse proxy that:
- **Detects PII** using advanced pattern matching and NER algorithms
- **Anonymizes data** with tokenization before sending to AI providers or monitoring vendors
- **De-anonymizes responses** to restore original context for users
- **Routes intelligently** based on content sensitivity and provider capabilities
- **Attributes every request** to its originating vendor (Sentry, Datadog, PostHog, etc.) for full audit trails
- **Generates compliance reports** (GDPR, HIPAA, PCI-DSS) showing exactly what data was protected

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client App    │───▶│  Privacy Gateway │───▶│  AI Provider    │
│                 │    │                  │    │ (OpenAI/etc.)   │
│ Original Data   │    │  PII Detection   │    │ Anonymized Data │
└─────────────────┘    │  & Tokenization  │    └─────────────────┘
                       └──────────────────┘
```

## Features

### Privacy Protection
- **Real-time PII Detection**: Email addresses, SSNs, credit cards, API keys, phone numbers
- **Smart Anonymization**: Context-preserving tokenization with secure de-anonymization
- **Strict Mode**: Block requests with high-sensitivity data entirely
- **Confidence Scoring**: Detection accuracy metrics

### High Performance
- **Zero-allocation networking** for minimal latency (<20ms processing time)
- **Concurrent processing** with configurable goroutine pools
- **Streaming support** for real-time AI responses
- **Connection pooling** for upstream providers

### Intelligent Routing
- **Content-based routing**: Financial data → local LLM, General queries → fastest provider
- **Provider health checking** with automatic failover
- **Load balancing** across multiple AI endpoints

## Supported AI Providers

- **OpenAI** (GPT-4, GPT-3.5-turbo, etc.)
- **Anthropic** (Claude 3.5, Claude 3, etc.)
- **Google AI** (Gemini Pro, Gemini Flash, etc.)
- **Local LLMs** (Ollama, etc.)

## Quick Start

### Prerequisites
- Docker & Docker Compose
- 2GB+ RAM

### 1. Clone and configure

```bash
git clone https://github.com/ContinuumSolutions/nonym
cd nonym
cp .env.example .env
# Edit .env with your settings
```

### 2. Start the stack

Using Make:
```bash
make up          # start services
make build       # start with a fresh image build
```

Or with Docker Compose directly:
```bash
docker compose up -d
docker compose up -d --build   # with fresh build
```

### 3. Generate a Gateway API Key

The gateway requires API key authentication for all proxy requests.

```bash
# Register and login
curl -X POST http://localhost:8000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"yourpassword","name":"Admin"}'

curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"yourpassword"}'
# → copy the token from the response

# Create an API key
curl -X POST http://localhost:8000/api/v1/api-keys \
  -H "Authorization: Bearer <jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-app"}'
# → copy the spg_... key
```

### 4. Use the Gateway

Point your AI client at `http://localhost:8000` instead of the provider's API. Pass:
- `X-API-Key` — your gateway API key (`spg_...`)
- `Authorization` — your provider API key (`Bearer sk-...`)

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "X-API-Key: spg_your_gateway_key" \
  -H "Authorization: Bearer sk-your-openai-key" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "My email is john@example.com"}]
  }'
```

The gateway will detect the email, replace it with a token (`{{EMAIL_A1B2C3}}`), forward the anonymized request to OpenAI, and restore the original value in the response.

```python
import openai

client = openai.OpenAI(
    base_url="http://localhost:8000/v1",
    api_key="sk-your-openai-key",
    default_headers={"X-API-Key": "spg_your_gateway_key"}
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "Analyze this customer: john@example.com"}]
)
```

## Make Commands

```bash
make setup          # Create .env and data directories
make up             # Start all services
make build          # Start with fresh image build
make down           # Stop all services
make restart        # Restart all services
make rebuild        # Force rebuild and restart
make clean          # Stop and remove all volumes
make logs           # Tail logs from all services
make status         # Show service status + health check
make db-shell       # PostgreSQL shell
make redis-shell    # Redis CLI
make gateway-shell  # Shell into gateway container
make build-go       # Build gateway binary locally
make test           # Run tests
```

## Configuration

### Environment Variables

```bash
# Gateway
PORT=8000
LOG_LEVEL=info
STRICT_MODE=false    # true = block high-sensitivity data

# Database
DB_NAME=gateway
DB_USER=gateway
DB_PASSWORD=gateway_password

# Auth
JWT_SECRET=change-in-production
SESSION_SECRET=change-in-production
```

### Content-Based Routing

- **High Sensitivity** (SSN, Credit Cards): Blocked in strict mode, heavily anonymized otherwise
- **Medium Sensitivity** (Email, Phone): Anonymized with secure tokenization
- **Low Sensitivity** (General text): Passed through with monitoring

## Architecture

### Services

| Service    | Description                          |
|------------|--------------------------------------|
| `postgres` | Database for audit logs and users    |
| `redis`    | Session storage and caching          |
| `gateway`  | Go privacy engine (port 8000)        |

## Security & Compliance

- No persistence of original PII (only anonymized tokens stored)
- Encryption at rest for audit logs
- Network isolation with Docker networks
- Non-root containers
- Audit trail of all anonymization events
- GDPR/CCPA compliance through data minimization

## Testing

```bash
# All tests
make test
# or:
go test ./...

# Specific packages
go test ./pkg/ner -v          # PII detection engine
go test ./pkg/router -v       # Provider routing
go test ./pkg/interceptor -v  # HTTP proxy
go test ./pkg/audit -v        # Audit logging

# With coverage
make test-coverage
```

## Project Structure

```
├── cmd/gateway/          # Main application entry point
├── pkg/
│   ├── interceptor/      # HTTP proxy and request handling
│   ├── ner/              # Named Entity Recognition engine
│   ├── router/           # Provider selection and routing
│   ├── auth/             # Authentication and API key management
│   └── audit/            # Logging and analytics API
├── database/             # Database initialization scripts
├── redis/                # Redis configuration
└── scripts/              # Setup and utility scripts
```

## License

This project is licensed under the **Nonym Commercial License**.

- Free for personal, academic, and research use
- Free for internal business use within a single organization
- Commercial redistribution requires licensing
- SaaS offerings require commercial license

For commercial licensing, contact: licensing@nonym.io

## Support

- **Issues**: [GitHub Issues](https://github.com/ContinuumSolutions/nonym/issues)
- **Discussions**: [GitHub Discussions](https://github.com/ContinuumSolutions/nonym/discussions)
- **Security**: security@nonym.io
