# Sovereign Privacy Gateway - Installation Guide

Complete installation and setup guide for the Sovereign Privacy Gateway with integrated monitoring.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Deployment Options](#deployment-options)
- [API Provider Setup](#api-provider-setup)
- [Production Deployment](#production-deployment)
- [Monitoring Setup](#monitoring-setup)
- [Single Port Access](#single-port-access)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows with WSL2
- **Memory**: 2GB+ RAM (4GB+ recommended for production with monitoring)
- **Disk**: 5GB+ free space (10GB+ with monitoring and logs)
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
- Set up monitoring configurations
- Generate secure passwords
- Create nginx reverse proxy configuration

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
# Development mode (basic gateway only)
docker compose up -d

# Production mode (with reverse proxy)
docker compose -f docker-compose.prod.yml up -d

# Production with monitoring (recommended)
docker compose -f docker-compose.prod.yml --profile monitoring up -d

# Full stack (monitoring + logging + backup)
docker compose -f docker-compose.prod.yml --profile full up -d
```

### 5. Verify Installation

```bash
# Run comprehensive verification
./scripts/verify-setup.sh

# Quick health check
curl http://localhost/health

# Test PII detection
curl -X POST http://localhost/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-openai-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "My email is test@example.com"}],
    "max_tokens": 10
  }'
```

## Single Port Access

### **All Services on Port 80**

The Privacy Gateway uses nginx reverse proxy to serve all interfaces on a single port:

| Service | URL | Description |
|---------|-----|-------------|
| **Privacy Dashboard** | `http://localhost/` | Main monitoring interface |
| **Grafana** | `http://localhost/grafana/` | Advanced metrics & dashboards |
| **Prometheus** | `http://localhost/prometheus/` | Raw metrics & queries |
| **Alertmanager** | `http://localhost/alertmanager/` | Alert management |

### **Convenience Redirects**

- `http://localhost/monitoring` → `/grafana/`
- `http://localhost/metrics` → `/prometheus/`
- `http://localhost/alerts` → `/alertmanager/`

### **Benefits**

1. **Simplified Networking**: Only port 80/443 exposed
2. **Better Security**: All services behind reverse proxy
3. **Easy SSL**: Single certificate for all services
4. **Professional URLs**: Clean, memorable endpoints

## Configuration

### Environment Variables

The gateway is configured via environment variables in the `.env` file:

#### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Main gateway port (internal) |
| `DASHBOARD_PORT` | `8081` | Dashboard port (internal) |
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

#### Monitoring Settings

```bash
# Grafana admin credentials
GRAFANA_USER=admin
GRAFANA_PASSWORD=secure-password-here

# Data retention
BACKUP_RETENTION_DAYS=7
```

### Routing Configuration

The gateway automatically routes requests based on content sensitivity:

- **High Sensitivity** (Personal/Healthcare/Financial) → **Anthropic** (privacy-focused)
- **Standard Content** → **OpenAI** (fast and capable)
- **Embeddings** → **Google AI** (specialized)

## Deployment Options

### Development Mode

```bash
# Hot-reload development environment (no monitoring)
docker compose up -d

# Access services:
# Gateway API: http://localhost:8080/v1/*
# Dashboard: http://localhost:8081/
```

### Production Mode (Recommended)

```bash
# Production with reverse proxy
docker compose -f docker-compose.prod.yml up -d

# Production with full monitoring stack
docker compose -f docker-compose.prod.yml --profile monitoring up -d

# Production with logging
docker compose -f docker-compose.prod.yml --profile monitoring --profile logging up -d

# All services accessible at:
# http://localhost/              # Privacy Dashboard
# http://localhost/grafana/      # Grafana Monitoring
# http://localhost/prometheus/   # Prometheus Metrics
# http://localhost/alertmanager/ # Alert Management
```

### Profile Options

| Profile | Services | Use Case |
|---------|----------|----------|
| Default | Gateway + Nginx | Basic production |
| `monitoring` | + Grafana + Prometheus + Alertmanager | Full monitoring |
| `logging` | + Loki + Promtail | Centralized logging |
| `backup` | + Automated backups | Data protection |
| `full` | All services | Complete stack |

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
# Uncomment HTTPS server block in nginx/nginx.conf
# Update server_name to your domain
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
# Enable automatic backups
docker compose -f docker-compose.prod.yml --profile backup up -d

# Manual backup
docker exec -it gateway-backup-1 sh -c "
  sqlite3 /data/gateway.db '.backup /backups/manual_backup.db'
"

# List backups
docker exec gateway-backup-1 ls -la /backups/
```

## Monitoring Setup

### Accessing Monitoring Interfaces

With the `monitoring` profile enabled, access monitoring at:

- **Grafana Dashboard**: `http://localhost/grafana/`
  - Username: `admin`
  - Password: Set in `.env` file (`GRAFANA_PASSWORD`)

- **Prometheus Metrics**: `http://localhost/prometheus/`
  - Query interface and service discovery
  - Raw metrics access

- **Alertmanager**: `http://localhost/alertmanager/`
  - Alert management and silencing
  - Notification routing

### Pre-configured Dashboards

Grafana includes dashboards for:

1. **Privacy Gateway Overview**
   - Request volume and success rates
   - PII detection statistics
   - Provider performance metrics

2. **Security Metrics**
   - Blocked request rates
   - High-sensitivity detections
   - Alert history

3. **Infrastructure Monitoring**
   - Container resource usage
   - Database performance
   - Network metrics

### Custom Alerts

Default alerts monitor:

- **High Error Rates**: >10% errors over 5 minutes
- **High Memory Usage**: >80% container memory
- **Service Downtime**: Health check failures
- **High PII Detection**: >50 detections per second
- **Blocked Requests**: >10 blocks per second

Configure additional alerts in `./monitoring/alerts.yml`:

```yaml
groups:
  - name: custom-alerts
    rules:
      - alert: CustomAlert
        expr: your_metric > threshold
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Custom alert triggered"
```

### Metrics Exposed

The gateway exposes Prometheus metrics:

```bash
# View all metrics
curl http://localhost/prometheus/api/v1/label/__name__/values

# Key metrics include:
# - privacy_requests_total
# - privacy_detections_total
# - privacy_blocked_total
# - privacy_processing_duration
# - privacy_provider_requests
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
# 4. Insufficient memory
```

#### Monitoring Not Accessible

```bash
# Check nginx configuration
docker compose logs nginx

# Verify nginx is proxying correctly
curl -I http://localhost/grafana/

# Check if monitoring services are running
docker compose ps

# Test direct service access (if needed for debugging)
docker exec -it gateway-grafana-1 curl localhost:3000
```

#### API Requests Failing

```bash
# Test health endpoint
curl -v http://localhost/health

# Check if API keys are configured correctly
grep "API_KEY" .env

# Test direct gateway access (bypass nginx)
curl http://localhost:8080/health
```

#### High Memory Usage

```bash
# Check resource usage
docker stats

# Optimize configuration
# Reduce concurrency:
MAX_CONCURRENCY=50

# Enable strict mode to reduce processing:
STRICT_MODE=true

# Reduce monitoring retention:
# Edit monitoring/prometheus.yml:
# --storage.tsdb.retention.time=30d
```

#### PII Not Being Detected

```bash
# Test PII detection directly with verbose output
curl -v -X POST http://localhost/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"My email is test@example.com"}]}'

# Check logs for detection events
docker compose logs gateway | grep "redaction"

# Verify NER engine is working
docker exec gateway-1 ./gateway --test-ner
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
# High throughput configuration
MAX_CONCURRENCY=500
SQLITE_CACHE_SIZE=50000
SQLITE_MMAP_SIZE=1073741824

# Low latency configuration
MAX_CONCURRENCY=50
STRICT_MODE=false

# Monitoring optimization
# Reduce Prometheus scrape interval in monitoring/prometheus.yml:
global:
  scrape_interval: 30s
  evaluation_interval: 30s
```

### Backup and Recovery

```bash
# Create manual backup
docker exec gateway-1 sqlite3 /data/gateway.db '.backup /tmp/backup.db'
docker cp gateway-1:/tmp/backup.db ./backup-$(date +%Y%m%d).db

# Restore from backup
docker cp ./backup-20240101.db gateway-1:/tmp/restore.db
docker exec gateway-1 sqlite3 /data/gateway.db '.restore /tmp/restore.db'

# Backup Grafana dashboards
docker exec gateway-grafana-1 grafana-cli admin export-dashboard > dashboards-backup.json
```

### Network Troubleshooting

```bash
# Test internal network connectivity
docker exec gateway-1 curl http://prometheus:9090/-/healthy
docker exec gateway-1 curl http://grafana:3000/api/health

# Check nginx reverse proxy
docker exec nginx-1 nginx -t  # Test configuration
docker exec nginx-1 curl localhost/health  # Test internal routing

# Verify service discovery
curl http://localhost/prometheus/api/v1/targets
```

## Support

If you encounter issues not covered in this guide:

1. **Run verification script**: `./scripts/verify-setup.sh`
2. **Check the logs**: `docker compose logs <service_name>`
3. **Enable debug logging**: Set `LOG_LEVEL=debug` in `.env`
4. **Test individual components**: Use the troubleshooting commands above
5. **Consult monitoring**: Check `http://localhost/grafana/` for system metrics
6. **GitHub Issues**: [Report issues](https://github.com/sovereignprivacy/gateway/issues)

For commercial support and licensing: licensing@sovereignprivacy.com

---

**🚀 Complete Privacy Gateway with unified monitoring - all accessible at `http://localhost/`**
