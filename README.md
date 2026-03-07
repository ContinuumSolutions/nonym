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
- **Content-based routing**: Financial data → Local LLM, General queries → Cloud AI
- **Provider health checking** with automatic failover
- **Load balancing** across multiple AI endpoints
- **Custom routing rules** based on sensitivity levels

### 📊 **Monitoring & Audit**
- **Real-time dashboard** with transaction monitoring
- **Comprehensive logging** of all anonymization events
- **Statistics and analytics** for privacy compliance
- **WebSocket updates** for live monitoring

## Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- 8GB+ RAM (for local LLM support)

### 1. Clone and Build

```bash
git clone https://github.com/sovereignprivacy/gateway
cd gateway
go mod tidy
```

### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env with your API keys and configuration
```

### 3. Start with Docker Compose

```bash
# Start all services (Gateway + Dashboard + Ollama)
docker compose up -d

# Or just the gateway for development
docker compose up gateway
```

### 4. Test the Gateway

```bash
# Health check
curl http://localhost:8080/health

# Test PII detection with OpenAI
curl -X POST http://localhost:8080/v1/chat/completions \
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

### 5. Access the Dashboard

Visit http://localhost:8081 to access the real-time monitoring dashboard.

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
  "provider": "local",  // Force routing to local LLM
  "messages": [
    {
      "role": "user",
      "content": "Sensitive financial data..."
    }
  ]
}
```

## Configuration

### Environment Variables

```bash
# Gateway Configuration
PORT=8080
DASHBOARD_PORT=8081
DATABASE_PATH=/data/gateway.db
LOG_LEVEL=info

# Provider URLs
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GOOGLE_API_KEY=...
LOCAL_LLM_URL=http://localhost:11434

# Privacy Settings
STRICT_MODE=false
RETENTION_DAYS=30
```

### Routing Rules

Configure custom routing in `config/routing.yaml`:

```yaml
rules:
  - pattern: "/v1/chat/completions"
    provider: "local"
    conditions: ["contains:financial", "contains:medical"]
    security_level: "high"

  - pattern: "/v1/embeddings"
    provider: "google"
    security_level: "standard"
```

## Deployment

### Production Docker Compose

```bash
# Use production configuration
docker compose -f docker-compose.yml up -d

# With monitoring stack
docker compose --profile monitoring up -d

# With log aggregation
docker compose --profile logging up -d
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/

# Or use Helm
helm install privacy-gateway ./charts/privacy-gateway
```

### Environment-Specific Configurations

- **Development**: Single instance with debug logging
- **Staging**: Multi-instance with metrics collection
- **Production**: High availability with SSL termination

## Security & Compliance

### Data Protection
- **No data persistence** of original PII (only anonymized tokens)
- **Encryption at rest** for audit logs and configuration
- **TLS termination** at nginx layer
- **Network isolation** with Docker networks

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
- Prometheus metrics export
- Grafana dashboards
- Custom alert rules for privacy violations
- Log aggregation with Loki

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
go test ./pkg/ner -v
go test ./pkg/interceptor -v
```

### Integration Tests
```bash
# End-to-end testing
go test ./cmd/gateway -v

# Load testing
go test -bench=. ./pkg/...
```

### Performance Benchmarks
```bash
# Benchmark PII detection
go test -bench=BenchmarkNER ./pkg/ner

# Benchmark proxy performance
go test -bench=BenchmarkProxy ./pkg/interceptor
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
├── api/                 # Protocol buffer definitions
├── nginx/               # Nginx configuration
└── docs/               # Documentation
```

### Building from Source
```bash
# Build the gateway binary
go build -o gateway ./cmd/gateway

# Build with optimizations
CGO_ENABLED=1 go build -ldflags="-w -s" -o gateway ./cmd/gateway

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build -o gateway-linux ./cmd/gateway
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make changes and add tests
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

### Code Style
- Follow Go naming conventions
- Use `gofmt` and `golint`
- Write tests for new features
- Update documentation

## License

This project is licensed under the **Sovereign Privacy Gateway Commercial License**.

- ✅ **Free for personal, academic, and research use**
- ✅ **Free for internal business use within a single organization**
- ❌ **Commercial redistribution requires licensing**
- ❌ **SaaS offerings require commercial license**

For commercial licensing, contact: licensing@sovereignprivacy.com

## Support & Community

- **Documentation**: https://docs.sovereignprivacy.com
- **Issues**: GitHub Issues for bug reports and feature requests
- **Discussions**: GitHub Discussions for questions and community
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