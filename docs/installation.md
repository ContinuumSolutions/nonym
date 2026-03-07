# Sovereign Privacy Gateway - Installation Guide

Complete installation and setup guide for the Sovereign Privacy Gateway.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Deployment Options](#deployment-options)
- [API Provider Setup](#api-provider-setup)
- [Production Deployment](#production-deployment)
- [Monitoring Setup](#monitoring-setup)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows with WSL2
- **Memory**: 2GB+ RAM (4GB+ recommended for production)
- **Disk**: 5GB+ free space
- **Network**: Internet access for AI provider APIs

### Software Requirements

- **Docker**: 20.10+ ([Install Docker](https://docs.docker.com/get-docker/))
- **Docker Compose**: 2.0+ ([Install Docker Compose](https://docs.docker.com/compose/install/))
- **Git**: For cloning the repository
- **curl**: For testing API endpoints

### API Keys Required

You'll need API keys from at least one AI provider:

- **OpenAI**: [Get API key](https://platform.openai.com/api-keys)
- **Anthropic**: [Get API key](https://console.anthropic.com/)
- **Google AI**: [Get API key](https://aistudio.google.com/app/apikey)

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/sovereignprivacy/gateway.git
cd gateway
```

### 2. Run Setup Script

```bash
# Run the automated setup
./scripts/setup-production.sh
```

This script will:
- Create necessary directories
- Generate configuration files
- Set up monitoring
- Generate secure passwords

### 3. Configure API Keys

```bash
# Edit the environment file
nano .env

# Add your API keys:
OPENAI_API_KEY=sk-your-openai-api-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key
GOOGLE_API_KEY=your-google-ai-api-key
```

### 4. Start the Gateway

```bash
# Development mode
docker compose up -d

# Production mode
docker compose -f docker-compose.prod.yml up -d

# With monitoring
docker compose -f docker-compose.prod.yml --profile monitoring up -d
```

### 5. Verify Installation

```bash
# Run verification script
./scripts/verify-setup.sh

# Test health endpoint
curl http://localhost:8080/health

# Access dashboard
open http://localhost:8081
```

## Configuration

### Environment Variables

The gateway is configured via environment variables in the `.env` file:

#### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Main gateway port |
| `DASHBOARD_PORT` | `8081` | Dashboard port |
| `LOG_LEVEL` | `info` | Logging level (debug/info/warn/error) |
| `STRICT_MODE` | `false` | Block high-sensitivity requests |
| `RETENTION_DAYS` | `30` | Data retention period |

#### Privacy Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `STRICT_MODE` | `false` | Enable strict blocking mode |
| `MAX_CONCURRENCY` | `100` | Max concurrent requests |
| `RETENTION_DAYS` | `30` | Audit log retention |

#### AI Provider Keys

```bash
# Required: At least one provider
OPENAI_API_KEY=sk-your-key-here
ANTHROPIC_API_KEY=sk-ant-your-key-here
GOOGLE_API_KEY=your-google-key-here
```

### Routing Configuration

The gateway automatically routes requests based on content sensitivity:

- **High Sensitivity** (SSN, Credit Cards): Can be blocked in strict mode
- **Medium Sensitivity** (Email, Phone): Anonymized and routed to preferred provider
- **Low Sensitivity** (General content): Routed to fastest available provider

#### Custom Routing Rules

You can customize routing by editing the gateway configuration:

```go
// Example routing priority:
// 1. Financial content → Block or use most secure provider
// 2. Personal data → Anthropic (privacy-focused)
// 3. General queries → OpenAI (fast and capable)
// 4. Embeddings → Google (specialized)
```

## Deployment Options

### Development Mode

```bash
# Hot-reload development environment
docker compose up -d

# View logs
docker compose logs -f gateway

# Access services
# Gateway: http://localhost:8080
# Dashboard: http://localhost:8081
```

### Production Mode

```bash
# Production deployment
docker compose -f docker-compose.prod.yml up -d

# With full monitoring stack
docker compose -f docker-compose.prod.yml --profile monitoring --profile logging up -d
```

### Production with Reverse Proxy

```bash
# Start with nginx proxy
docker compose -f docker-compose.prod.yml --profile proxy up -d

# Services will be available at:
# Gateway: http://localhost/v1/*
# Dashboard: http://localhost/
```

## API Provider Setup

### OpenAI Setup

1. Go to [OpenAI API Keys](https://platform.openai.com/api-keys)
2. Click "Create new secret key"
3. Copy the key and add to `.env`:

```bash
OPENAI_API_KEY=sk-proj-your-key-here
```

### Anthropic Setup

1. Go to [Anthropic Console](https://console.anthropic.com/)
2. Navigate to "API Keys"
3. Create a new key and add to `.env`:

```bash
ANTHROPIC_API_KEY=sk-ant-your-key-here
```

### Google AI Setup

1. Go to [Google AI Studio](https://aistudio.google.com/app/apikey)
2. Create an API key
3. Add to `.env`:

```bash
GOOGLE_API_KEY=your-google-ai-key-here
```

## Production Deployment

### SSL/TLS Setup

For production with HTTPS:

1. **Generate SSL certificates**:
```bash
# Using Let's Encrypt
sudo certbot certonly --standalone -d your-domain.com

# Copy certificates
sudo cp /etc/letsencrypt/live/your-domain.com/fullchain.pem nginx/ssl/
sudo cp /etc/letsencrypt/live/your-domain.com/privkey.pem nginx/ssl/
```

2. **Update nginx configuration**:
```bash
# Edit nginx/nginx.conf
# Add SSL configuration block
```

3. **Update environment**:
```bash
DOMAIN=your-domain.com
LETSENCRYPT_EMAIL=admin@your-domain.com
```

### Environment-Specific Configuration

#### Staging Environment

```bash
# staging.env
LOG_LEVEL=debug
STRICT_MODE=true
RETENTION_DAYS=7
MAX_CONCURRENCY=50
```

#### Production Environment

```bash
# production.env
LOG_LEVEL=info
STRICT_MODE=false
RETENTION_DAYS=90
MAX_CONCURRENCY=200
```

### Database Backup

Automatic backups are configured in production mode:

```bash
# Backup retention (days)
BACKUP_RETENTION_DAYS=30

# Manual backup
docker exec -it gateway-backup-1 sh -c "
  sqlite3 /data/gateway.db '.backup /backups/manual_backup.db'
"
```

## Monitoring Setup

### Prometheus Metrics

The gateway exposes metrics at `/metrics`:

```bash
# View metrics
curl http://localhost:8080/metrics
```

### Grafana Dashboard

Access Grafana at `http://localhost:3000`:

- **Username**: admin
- **Password**: (set in `.env` file)

Pre-configured dashboards include:
- Request volume and response times
- PII detection statistics
- Provider health and performance
- Error rates and alerts

### Custom Alerts

Edit `monitoring/alerts.yml` to customize alerts:

```yaml
groups:
  - name: privacy-gateway
    rules:
      - alert: HighPIIDetection
        expr: rate(pii_detections_total[5m]) > 100
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High PII detection rate"
```

## Troubleshooting

### Common Issues

#### Gateway Won't Start

```bash
# Check logs
docker compose logs gateway

# Common causes:
# 1. Invalid API key format
# 2. Port already in use
# 3. Database permission issues
```

#### API Requests Failing

```bash
# Test health endpoint
curl -v http://localhost:8080/health

# Check if API keys are configured
curl -H "Authorization: Bearer test" http://localhost:8080/v1/models
```

#### High Memory Usage

```bash
# Check resource usage
docker stats

# Reduce concurrency
MAX_CONCURRENCY=50

# Enable strict mode to reduce processing
STRICT_MODE=true
```

#### PII Not Being Detected

```bash
# Test PII detection directly
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"My email is test@example.com"}]}'

# Check logs for detection events
docker compose logs gateway | grep "redaction"
```

### Debug Mode

Enable debug logging:

```bash
# Set debug level
LOG_LEVEL=debug

# Restart gateway
docker compose restart gateway

# View detailed logs
docker compose logs -f gateway
```

### Performance Optimization

```bash
# Optimize for high throughput
MAX_CONCURRENCY=500
SQLITE_CACHE_SIZE=50000
SQLITE_MMAP_SIZE=1073741824

# Optimize for low latency
MAX_CONCURRENCY=50
STRICT_MODE=false
```

### Backup and Recovery

```bash
# Create manual backup
docker exec gateway-db sqlite3 /data/gateway.db '.backup /tmp/backup.db'

# Restore from backup
docker exec gateway-db sqlite3 /data/gateway.db '.restore /tmp/backup.db'
```

## Support

If you encounter issues not covered in this guide:

1. Check the [GitHub Issues](https://github.com/sovereignprivacy/gateway/issues)
2. Run the verification script: `./scripts/verify-setup.sh`
3. Enable debug logging for more detailed information
4. Consult the [API documentation](api-reference.md)

For commercial support and licensing: licensing@sovereignprivacy.com
