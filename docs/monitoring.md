# Monitoring Guide - Sovereign Privacy Gateway

Complete guide to monitoring your Privacy Gateway deployment with Grafana, Prometheus, and Alertmanager.

## Overview

The Privacy Gateway includes a comprehensive monitoring stack accessible through a single port via nginx reverse proxy:

- **Gateway Dashboard**: `http://localhost/` - Main Privacy Gateway interface
- **Monitoring (Grafana)**: `http://localhost/grafana/` - Metrics, charts, and dashboards
- **Metrics (Prometheus)**: `http://localhost/prometheus/` - Raw metrics and query interface
- **Alerts (Alertmanager)**: `http://localhost/alertmanager/` - Alert management interface

## Quick Start

### Enable Monitoring

```bash
# Start with monitoring stack
docker compose -f docker-compose.prod.yml --profile monitoring up -d

# Access interfaces (all on port 80)
open http://localhost/              # Privacy Gateway Dashboard
open http://localhost/grafana/      # Grafana Monitoring
open http://localhost/prometheus/   # Prometheus Metrics
open http://localhost/alertmanager/ # Alert Manager
```

### Default Credentials

- **Grafana**:
  - Username: `admin`
  - Password: Set in `.env` file (`GRAFANA_PASSWORD`)

## Monitoring Interfaces

### 1. Privacy Gateway Dashboard (`/`)

The main dashboard provides real-time monitoring of:
- Request volume and success rates
- PII detection statistics
- Provider performance
- Recent transactions
- System health status

**Key Features:**
- Real-time WebSocket updates
- Transaction history with filtering
- Privacy compliance metrics
- Provider routing statistics

### 2. Grafana Monitoring (`/grafana/`)

Advanced monitoring with customizable dashboards:

**Pre-configured Dashboards:**
- **Privacy Gateway Overview**: System health and performance
- **PII Detection Analytics**: Detection rates and types
- **Provider Performance**: Response times and success rates
- **Security Metrics**: Blocked requests and threats
- **Infrastructure**: Resource usage and capacity

**Key Metrics:**
- `privacy_requests_total`: Total requests processed
- `privacy_detections_total`: PII instances detected
- `privacy_blocked_total`: Requests blocked in strict mode
- `privacy_processing_duration`: Request processing time
- `privacy_provider_requests`: Per-provider request counts

### 3. Prometheus Metrics (`/prometheus/`)

Raw metrics and query interface:

**Useful Queries:**
```promql
# Request rate over time
rate(privacy_requests_total[5m])

# PII detection rate
rate(privacy_detections_total[5m])

# Error rate by provider
rate(privacy_requests_total{status="error"}[5m]) by (provider)

# 95th percentile response time
histogram_quantile(0.95, privacy_processing_duration)
```

### 4. Alertmanager (`/alertmanager/`)

Alert management and notification routing:

**Pre-configured Alerts:**
- High error rates (>10% over 5 minutes)
- High memory usage (>80% of container limit)
- Service downtime
- High PII detection rate (>50 instances/second)
- Blocked request spikes (>10 blocks/second)

## Configuration

### Custom Dashboards

Add custom Grafana dashboards:

```bash
# Place dashboard JSON files here
./monitoring/grafana/dashboards/

# Grafana will auto-import on startup
```

### Custom Alerts

Edit `./monitoring/alerts.yml`:

```yaml
groups:
  - name: custom-alerts
    rules:
      - alert: CustomMetricHigh
        expr: your_custom_metric > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Custom metric is too high"
```

### Alert Notifications

Configure notifications in `./monitoring/alertmanager.yml`:

```yaml
receivers:
  - name: 'email-alerts'
    email_configs:
      - to: 'admin@yourcompany.com'
        subject: 'Privacy Gateway Alert'
        body: 'Alert: {{ .GroupLabels.alertname }}'

  - name: 'slack-alerts'
    slack_configs:
      - api_url: 'YOUR_SLACK_WEBHOOK_URL'
        channel: '#alerts'
        title: 'Privacy Gateway Alert'
        text: '{{ .CommonAnnotations.summary }}'
```

## Access Control

### Grafana Security

Configure authentication in environment:

```bash
# Basic authentication (default)
GF_SECURITY_ADMIN_USER=admin
GF_SECURITY_ADMIN_PASSWORD=your-secure-password

# LDAP/SSO integration (advanced)
# GF_AUTH_LDAP_ENABLED=true
# GF_AUTH_LDAP_CONFIG_FILE=/etc/grafana/ldap.toml
```

### Network Security

All monitoring services are:
- Internal to Docker network (no external ports)
- Proxied through nginx with rate limiting
- Protected by same security headers as main application

## Data Retention

### Prometheus Data

Configure retention in `docker-compose.prod.yml`:

```yaml
prometheus:
  command:
    - '--storage.tsdb.retention.time=90d'  # Keep 90 days
    - '--storage.tsdb.retention.size=50GB' # Max 50GB storage
```

### Grafana Data

Grafana data is persisted in Docker volume:
```bash
# Backup Grafana data
docker run --rm -v grafana-data:/data -v $(pwd):/backup ubuntu tar czf /backup/grafana-backup.tar.gz -C /data .

# Restore Grafana data
docker run --rm -v grafana-data:/data -v $(pwd):/backup ubuntu tar xzf /backup/grafana-backup.tar.gz -C /data
```

## Troubleshooting

### Common Issues

#### Grafana not accessible at `/grafana/`

```bash
# Check Grafana configuration
docker logs privacy-gateway-grafana-1

# Verify nginx proxy configuration
docker logs privacy-gateway-nginx-1

# Test direct access (temporarily)
docker exec -it privacy-gateway-grafana-1 curl localhost:3000
```

#### Prometheus metrics not showing

```bash
# Check Prometheus configuration
docker logs privacy-gateway-prometheus-1

# Test metrics endpoint
curl http://localhost/prometheus/api/v1/query?query=up

# Verify service discovery
curl http://localhost/prometheus/api/v1/targets
```

#### Alerts not firing

```bash
# Check alert rules
curl http://localhost/prometheus/api/v1/rules

# Check Alertmanager status
curl http://localhost/alertmanager/api/v1/status

# View active alerts
curl http://localhost/alertmanager/api/v1/alerts
```

### Performance Tuning

#### Reduce Prometheus Storage

```bash
# Shorter retention for high-volume metrics
# Add to prometheus.yml:
global:
  evaluation_interval: 30s  # Increase interval
  scrape_interval: 30s      # Increase scrape interval
```

#### Optimize Grafana

```bash
# Environment variables for better performance
GF_RENDERING_SERVER_URL=http://renderer:8081
GF_RENDERING_CALLBACK_URL=http://grafana:3000
GF_LOG_FILTERS=rendering:debug
```

## Custom Metrics

### Adding Application Metrics

Expose custom metrics from your Privacy Gateway:

```go
import "github.com/prometheus/client_golang/prometheus"

// Custom counter
var customRequestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "privacy_custom_requests_total",
        Help: "Total custom requests",
    },
    []string{"type"},
)

// Register metric
prometheus.MustRegister(customRequestsTotal)

// Use in application
customRequestsTotal.WithLabelValues("type").Inc()
```

### Query Custom Metrics

```promql
# Rate of custom requests
rate(privacy_custom_requests_total[5m])

# Custom requests by type
sum(privacy_custom_requests_total) by (type)
```

## Integration

### External Monitoring Systems

#### Datadog Integration

```bash
# Add Datadog agent as sidecar
datadog-agent:
  image: datadog/agent:latest
  environment:
    - DD_API_KEY=your-datadog-key
    - DD_PROMETHEUS_SCRAPE_ENABLED=true
    - DD_PROMETHEUS_SCRAPE_URL=http://prometheus:9090
```

#### New Relic Integration

```bash
# Environment variables for New Relic
NEW_RELIC_LICENSE_KEY=your-license-key
NEW_RELIC_APP_NAME="Privacy Gateway"
```

## Best Practices

1. **Monitor Key Metrics**: Focus on request rates, error rates, and PII detection
2. **Set Meaningful Alerts**: Avoid alert fatigue with appropriate thresholds
3. **Regular Backups**: Back up Grafana dashboards and Prometheus data
4. **Security First**: Use strong passwords and limit access to monitoring interfaces
5. **Resource Monitoring**: Watch container resource usage and scale accordingly

---

**All monitoring services accessible at: `http://localhost/` with subpaths!** 📊
