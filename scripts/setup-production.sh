#!/bin/bash
set -euo pipefail

# Nonym - Production Setup Script
# This script sets up a production-ready deployment

echo "🚀 Setting up Nonym for Production"
echo "================================================="

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; exit 1; }
success() { echo -e "${GREEN}✅ $1${NC}"; }

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."

    command -v docker >/dev/null 2>&1 || error "Docker is not installed"
    command -v docker compose >/dev/null 2>&1 || error "Docker Compose is not installed"

    # Check Docker is running
    docker info >/dev/null 2>&1 || error "Docker is not running"

    success "Prerequisites check passed"
}

# Create directory structure
create_directories() {
    info "Creating directory structure..."

    local data_dir="${DATA_DIR:-./data}"

    mkdir -p "$data_dir"/{gateway,prometheus,grafana,alertmanager,loki,backups,logs}
    mkdir -p ./nginx/ssl
    mkdir -p ./monitoring/{prometheus,grafana,loki,alertmanager}

    success "Directory structure created"
}

# Set up environment file
setup_environment() {
    info "Setting up environment configuration..."

    if [[ ! -f .env ]]; then
        cp .env.production .env
        warn "Created .env file from template"
        warn "Please edit .env with your actual API keys before continuing"

        # Generate secure passwords
        local grafana_password
        grafana_password=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)

        local session_secret
        session_secret=$(openssl rand -base64 64 | tr -d "=+/" | cut -c1-64)

        # Update .env with generated values
        sed -i.bak "s/secure-admin-password-change-me/$grafana_password/g" .env
        sed -i.bak "s/change-this-to-a-random-64-character-string-for-production/$session_secret/g" .env

        info "Generated secure passwords in .env file"
        info "Grafana admin password: $grafana_password"
    else
        warn ".env file already exists - skipping environment setup"
    fi
}

# Set up monitoring configuration
setup_monitoring() {
    info "Setting up monitoring configuration..."

    # Prometheus configuration
    cat > ./monitoring/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alerts.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'sovereign-privacy-gateway'
    static_configs:
      - targets: ['gateway:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s

  - job_name: 'nginx'
    static_configs:
      - targets: ['nginx:80']
    metrics_path: '/metrics'
EOF

    # Alert rules
    cat > ./monitoring/alerts.yml << 'EOF'
groups:
  - name: sovereign-privacy-gateway
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"

      - alert: HighMemoryUsage
        expr: container_memory_usage_bytes{name="gateway"} / container_spec_memory_limit_bytes{name="gateway"} > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is above 80%"

      - alert: ServiceDown
        expr: up{job="sovereign-privacy-gateway"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Service is down"
          description: "Nonym is not responding"

      - alert: HighPIIDetection
        expr: rate(pii_detections_total[5m]) > 50
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High PII detection rate"
          description: "Detected {{ $value }} PII instances per second"

      - alert: BlockedRequests
        expr: rate(blocked_requests_total[5m]) > 10
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "High blocked request rate"
          description: "{{ $value }} requests blocked per second"
EOF

    # Grafana datasources
    mkdir -p ./monitoring/grafana/{dashboards,datasources}

    cat > ./monitoring/grafana/datasources/prometheus.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    jsonData:
      timeInterval: "15s"
EOF

    success "Monitoring configuration created"
}

# Set up nginx configuration
setup_nginx() {
    info "Setting up Nginx configuration..."

    cat > ./nginx/nginx.conf << 'EOF'
worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
    use epoll;
    multi_accept on;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Logging
    log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                   '$status $body_bytes_sent "$http_referer" '
                   '"$http_user_agent" "$http_x_forwarded_for"';
    access_log /var/log/nginx/access.log main;

    # Performance
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    gzip on;
    gzip_vary on;
    gzip_min_length 1000;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    # Security headers
    add_header X-Frame-Options DENY always;
    add_header X-Content-Type-Options nosniff always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=gateway:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=dashboard:10m rate=5r/s;

    upstream gateway {
        server gateway:8080;
        keepalive 32;
    }

    upstream dashboard {
        server gateway:8081;
        keepalive 32;
    }

    server {
        listen 80;
        server_name _;

        # Health check (bypass rate limiting)
        location = /health {
            proxy_pass http://gateway;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        # Privacy Gateway API
        location ~ ^/(v1|api)/ {
            limit_req zone=gateway burst=20 nodelay;

            proxy_pass http://gateway;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_buffering off;
            proxy_request_buffering off;
            proxy_read_timeout 300;
            proxy_connect_timeout 10;
        }

        # Gateway status endpoints
        location ~ ^/gateway/(status|stats)$ {
            limit_req zone=dashboard burst=10 nodelay;

            proxy_pass http://gateway;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }

        # Dashboard
        location / {
            limit_req zone=dashboard burst=15 nodelay;

            proxy_pass http://dashboard;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;

            # WebSocket support
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
        }
    }
}
EOF

    success "Nginx configuration created"
}

# Set file permissions
set_permissions() {
    info "Setting file permissions..."

    local data_dir="${DATA_DIR:-./data}"

    # Set directory permissions
    find "$data_dir" -type d -exec chmod 755 {} \; 2>/dev/null || true

    # Set file permissions
    find "./monitoring" -type f -exec chmod 644 {} \; 2>/dev/null || true
    find "./nginx" -type f -exec chmod 644 {} \; 2>/dev/null || true

    # Make scripts executable
    chmod +x scripts/*.sh 2>/dev/null || true

    success "File permissions set"
}

# Validate configuration
validate_config() {
    info "Validating Docker Compose configuration..."

    docker compose -f docker-compose.yml config >/dev/null || error "Development Docker Compose configuration is invalid"
    docker compose -f docker-compose.prod.yml config >/dev/null || error "Production Docker Compose configuration is invalid"

    success "Configuration validation passed"
}

# Pull required images
pull_images() {
    info "Pulling Docker images..."

    docker compose -f docker-compose.prod.yml pull

    success "Docker images pulled"
}

# Create API key reminder
create_api_key_reminder() {
    info "Creating API key setup reminder..."

    cat > ./API_KEYS_SETUP.md << 'EOF'
# API Keys Setup Required

Before starting the Privacy Gateway, you need to configure at least one AI provider API key.

## Required Steps:

1. Edit the `.env` file and add your API keys:

```bash
# OpenAI (recommended for general use)
OPENAI_API_KEY=sk-your-openai-api-key-here

# Anthropic (recommended for privacy-sensitive content)
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key-here

# Google AI (recommended for embeddings)
GOOGLE_API_KEY=your-google-ai-api-key-here
```

2. Get API keys from:
   - OpenAI: https://platform.openai.com/api-keys
   - Anthropic: https://console.anthropic.com/
   - Google AI: https://aistudio.google.com/app/apikey

3. After adding keys, start the gateway:
```bash
# Development
docker compose up -d

# Production
docker compose -f docker-compose.prod.yml up -d
```

4. Test the setup:
```bash
curl http://localhost:8080/health
```

## Need Help?

- See docs/installation.md for detailed setup instructions
- Run ./scripts/verify-setup.sh to validate your configuration
EOF

    success "API key setup reminder created"
}

# Main setup process
main() {
    echo
    info "Starting production setup process..."
    echo

    check_prerequisites
    create_directories
    setup_environment
    setup_monitoring
    setup_nginx
    set_permissions
    validate_config
    pull_images
    create_api_key_reminder

    echo
    success "🎉 Production setup completed successfully!"
    echo
    info "IMPORTANT: Configure your API keys before starting!"
    echo
    warn "Next steps:"
    echo "  1. Edit .env file with your AI provider API keys"
    echo "  2. See API_KEYS_SETUP.md for detailed instructions"
    echo "  3. Review monitoring/prometheus.yml and monitoring/alerts.yml"
    echo "  4. Configure SSL certificates in nginx/ssl/ (if using HTTPS)"
    echo "  5. Run: docker compose -f docker-compose.prod.yml up -d"
    echo "  6. Access dashboard at http://localhost:8081"
    echo "  7. Access monitoring at http://localhost:3000 (Grafana)"
    echo
    info "Documentation: docs/installation.md"
    echo
}

# Run main function
main "$@"
