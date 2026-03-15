# Quick Start Guide - Sovereign Privacy Gateway

Get up and running with the Privacy Gateway in under 5 minutes - **everything accessible on port 80!**

## Prerequisites

- Docker & Docker Compose installed
- At least one AI provider API key

## 1. Get API Keys

Choose at least one provider and get an API key:

### OpenAI (Recommended for general use)
1. Go to [OpenAI API Keys](https://platform.openai.com/api-keys)
2. Click "Create new secret key"
3. Copy the key (starts with `sk-`)

### Anthropic (Recommended for privacy-sensitive data)
1. Go to [Anthropic Console](https://console.anthropic.com/)
2. Navigate to "API Keys"
3. Create a new key (starts with `sk-ant-`)

### Google AI (Good for embeddings)
1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Create an API key

## 2. Quick Setup

```bash
# Clone repository
git clone https://github.com/sovereignprivacy/gateway
cd gateway

# Run setup script
./scripts/setup-production.sh

# Add your API keys to .env
nano .env
```

Add at least one API key:
```bash
OPENAI_API_KEY=sk-your-openai-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key-here
GOOGLE_API_KEY=your-google-key-here
```

## 3. Start Gateway

```bash
# Development mode (recommended for testing)
docker compose up -d

# Production mode with monitoring
docker compose -f docker-compose.prod.yml --profile monitoring up -d
```

## 4. Verify Setup

```bash
# Health check
curl http://localhost/health

# Test PII detection
curl -X POST http://localhost/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-openai-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "My email is john@example.com"
      }
    ],
    "max_tokens": 10
  }'
```

## 5. Access All Interfaces (Single Port!)

Everything is accessible on **port 80** through nginx:

### Main Interfaces
- **🏠 Privacy Dashboard**: `http://localhost/`
  Real-time monitoring, transaction history, system status

- **📊 Grafana Monitoring**: `http://localhost/grafana/`
  Advanced metrics, custom dashboards, analytics

- **🔍 Prometheus Metrics**: `http://localhost/prometheus/`
  Raw metrics, custom queries, service discovery

- **🚨 Alertmanager**: `http://localhost/alertmanager/`
  Alert management, notification rules, silence alerts

### Convenience URLs
- `http://localhost/monitoring` → Redirects to Grafana
- `http://localhost/metrics` → Redirects to Prometheus
- `http://localhost/alerts` → Redirects to Alertmanager

### Default Login (Grafana)
- **Username**: `admin`
- **Password**: Check your `.env` file (`GRAFANA_PASSWORD`)

## What's Protected

The gateway automatically detects and anonymizes:
- ✅ Email addresses
- ✅ Social Security Numbers (SSN)
- ✅ Credit card numbers
- ✅ API keys and tokens
- ✅ Phone numbers
- ✅ IP addresses

## How It Works

```
Your App → Privacy Gateway (Port 80) → AI Provider
          ↓                           (anonymized data)
    All Monitoring Interfaces
    (same port, different paths)
```

1. **Detect**: Gateway scans requests for PII
2. **Anonymize**: Replaces PII with tokens like `{{EMAIL_ABC123}}`
3. **Route**: Sends anonymized request to AI provider
4. **Restore**: Converts tokens back to original data in response
5. **Monitor**: Real-time dashboards track everything

## Architecture Benefits

### Single Port Access
- **Simplified networking**: Only port 80 exposed
- **Better security**: All services behind nginx reverse proxy
- **Easy SSL**: Configure HTTPS once for all services
- **Rate limiting**: Unified rate limiting across all interfaces

### Production Ready
- **Security headers**: CSP, HSTS, XSS protection
- **Resource limits**: Container memory and CPU limits
- **Health checks**: Automatic service health monitoring
- **Backup automation**: Automated database backups

## Next Steps

### For Development
```bash
# Hot-reload development
docker compose up -d

# View logs
docker compose logs -f gateway
```

### For Production
```bash
# Full production stack
docker compose -f docker-compose.prod.yml --profile monitoring --profile logging up -d

# Check everything is working
./scripts/verify-setup.sh
```

### For Monitoring
```bash
# Create custom Grafana dashboard
open http://localhost/grafana/

# Query custom metrics
open http://localhost/prometheus/

# Configure alerts
open http://localhost/alertmanager/
```

## Troubleshooting

### Gateway won't start
```bash
# Check logs
docker compose logs gateway

# Common issues:
# - Invalid API key format
# - Port 80 already in use
# - Insufficient memory
```

### Monitoring not accessible
```bash
# Check nginx logs
docker compose logs nginx

# Verify services are running
docker compose ps

# Test direct service access
docker exec -it gateway-grafana-1 curl localhost:3000
```

### Need help?
- **Documentation**: [docs/installation.md](docs/installation.md) for detailed setup
- **Monitoring Guide**: [docs/monitoring.md](docs/monitoring.md) for advanced monitoring
- **Verification**: Run `./scripts/verify-setup.sh` for diagnostic info
- **Issues**: Open an issue on GitHub

---

**🎉 You're now protecting your AI interactions with enterprise-grade privacy - all accessible on one port!**

**Quick Links:**
- Dashboard: http://localhost/
- Monitoring: http://localhost/grafana/
- Metrics: http://localhost/prometheus/
- Alerts: http://localhost/alertmanager/
