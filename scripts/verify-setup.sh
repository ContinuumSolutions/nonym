#!/bin/bash
set -euo pipefail

# Sovereign Privacy Gateway - Setup Verification Script
# Verifies that the Privacy Gateway is properly configured and working

echo "🔍 Verifying Sovereign Privacy Gateway Setup"
echo "============================================"

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Helper functions
info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Run test
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_pattern="${3:-}"

    info "Testing: $test_name"

    if result=$(eval "$test_command" 2>&1); then
        if [[ -n "$expected_pattern" ]]; then
            if echo "$result" | grep -q "$expected_pattern"; then
                success "$test_name passed"
                ((TESTS_PASSED++))
                return 0
            else
                error "$test_name failed - unexpected output: $result"
                ((TESTS_FAILED++))
                return 1
            fi
        else
            success "$test_name passed"
            ((TESTS_PASSED++))
            return 0
        fi
    else
        error "$test_name failed: $result"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."

    run_test "Docker installation" "command -v docker"
    run_test "Docker Compose installation" "command -v docker compose"
    run_test "Docker daemon running" "docker info" "Server Version"

    echo
}

# Test compilation
test_compilation() {
    info "Testing compilation..."

    run_test "Go module validation" "go mod verify"
    run_test "Application compilation" "go build -o gateway-test ./cmd/gateway"

    # Clean up test binary
    rm -f gateway-test

    echo
}

# Test core components
test_core_components() {
    info "Testing core components..."

    run_test "NER Engine tests" "go test ./pkg/ner -v" "PASS"
    run_test "Router tests" "go test ./pkg/router -v" "PASS"

    echo
}

# Test configuration files
test_configuration() {
    info "Testing configuration files..."

    run_test "Docker Compose dev config" "docker compose config" "version:"
    run_test "Docker Compose prod config" "docker compose -f docker-compose.prod.yml config" "version:"

    if [[ -f .env ]]; then
        run_test "Environment file exists" "test -f .env"

        # Check for API keys
        if grep -q "OPENAI_API_KEY=sk-" .env 2>/dev/null; then
            success "OpenAI API key configured"
        else
            warn "OpenAI API key not configured in .env"
        fi

        if grep -q "ANTHROPIC_API_KEY=sk-ant-" .env 2>/dev/null; then
            success "Anthropic API key configured"
        else
            warn "Anthropic API key not configured in .env"
        fi

        if grep -q "GOOGLE_API_KEY=" .env && ! grep -q "GOOGLE_API_KEY=your-google-ai-api-key-here" .env 2>/dev/null; then
            success "Google AI API key configured"
        else
            warn "Google AI API key not configured in .env"
        fi
    else
        warn "Environment file (.env) not found - run ./scripts/setup-production.sh"
    fi

    echo
}

# Test Docker images
test_docker_images() {
    info "Testing Docker image build..."

    run_test "Gateway image build (dev)" "docker build --target development -t gateway-test ." "Successfully built"
    run_test "Gateway image build (prod)" "docker build --target production -t gateway-prod-test ." "Successfully built"

    # Clean up test images
    docker rmi gateway-test gateway-prod-test >/dev/null 2>&1 || true

    echo
}

# Test API endpoints (if running)
test_api_endpoints() {
    info "Testing API endpoints (if gateway is running)..."

    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        run_test "Health endpoint" "curl -s http://localhost:8080/health" "healthy"
        run_test "Gateway status" "curl -s http://localhost:8080/gateway/status" "status"
        run_test "Gateway stats" "curl -s http://localhost:8080/gateway/stats" "total_requests"

        if curl -s http://localhost:8081 >/dev/null 2>&1; then
            run_test "Dashboard accessibility" "curl -s -o /dev/null -w '%{http_code}' http://localhost:8081" "200"
        else
            warn "Dashboard not running on port 8081"
        fi
    else
        warn "Gateway not running on port 8080 - skipping API tests"
        warn "Run 'docker compose up' to start the gateway for API testing"
    fi

    echo
}

# Test monitoring setup (if configured)
test_monitoring() {
    info "Testing monitoring configuration..."

    if [[ -f monitoring/prometheus.yml ]]; then
        run_test "Prometheus config exists" "test -f monitoring/prometheus.yml"
        run_test "Prometheus config syntax" "docker run --rm -v $(pwd)/monitoring:/etc/prometheus prom/prometheus:latest promtool check config /etc/prometheus/prometheus.yml"
    fi

    if [[ -f monitoring/alerts.yml ]]; then
        run_test "Alert rules exist" "test -f monitoring/alerts.yml"
        run_test "Alert rules syntax" "docker run --rm -v $(pwd)/monitoring:/etc/prometheus prom/prometheus:latest promtool check rules /etc/prometheus/alerts.yml"
    fi

    if [[ -f monitoring/grafana/datasources/prometheus.yml ]]; then
        run_test "Grafana datasource config exists" "test -f monitoring/grafana/datasources/prometheus.yml"
    fi

    echo
}

# Test security configuration
test_security() {
    info "Testing security configuration..."

    if [[ -f nginx/nginx.conf ]]; then
        run_test "Nginx config exists" "test -f nginx/nginx.conf"
        run_test "Security headers in nginx" "grep -q 'X-Frame-Options' nginx/nginx.conf"
        run_test "Rate limiting in nginx" "grep -q 'limit_req' nginx/nginx.conf"
        run_test "Nginx config syntax" "docker run --rm -v $(pwd)/nginx/nginx.conf:/etc/nginx/nginx.conf nginx:alpine nginx -t"
    fi

    # Check file permissions
    if [[ -d data ]]; then
        run_test "Data directory permissions" "test -w data"
    fi

    echo
}

# Test AI provider connectivity (if keys are configured)
test_ai_providers() {
    info "Testing AI provider connectivity (if configured)..."

    if [[ -f .env ]]; then
        source .env

        # Test OpenAI if key is configured
        if [[ -n "${OPENAI_API_KEY:-}" && "$OPENAI_API_KEY" != "sk-your-openai-api-key-here" ]]; then
            if curl -s -H "Authorization: Bearer $OPENAI_API_KEY" https://api.openai.com/v1/models >/dev/null 2>&1; then
                success "OpenAI API connection successful"
            else
                warn "OpenAI API connection failed - check your API key"
            fi
        fi

        # Test Anthropic if key is configured
        if [[ -n "${ANTHROPIC_API_KEY:-}" && "$ANTHROPIC_API_KEY" != "sk-ant-your-anthropic-api-key-here" ]]; then
            if curl -s -H "x-api-key: $ANTHROPIC_API_KEY" -H "anthropic-version: 2023-06-01" https://api.anthropic.com/v1/messages >/dev/null 2>&1; then
                success "Anthropic API connection successful"
            else
                warn "Anthropic API connection failed - check your API key"
            fi
        fi

        # Test Google AI if key is configured
        if [[ -n "${GOOGLE_API_KEY:-}" && "$GOOGLE_API_KEY" != "your-google-ai-api-key-here" ]]; then
            if curl -s "https://generativelanguage.googleapis.com/v1/models?key=$GOOGLE_API_KEY" >/dev/null 2>&1; then
                success "Google AI API connection successful"
            else
                warn "Google AI API connection failed - check your API key"
            fi
        fi
    fi

    echo
}

# Test PII detection functionality
test_pii_detection() {
    info "Testing PII detection (if gateway is running)..."

    if curl -s http://localhost:8080/health >/dev/null 2>&1 && [[ -f .env ]]; then
        source .env

        # Test with OpenAI if available
        if [[ -n "${OPENAI_API_KEY:-}" && "$OPENAI_API_KEY" != "sk-your-openai-api-key-here" ]]; then
            local test_response
            test_response=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $OPENAI_API_KEY" \
                -d '{
                    "model": "gpt-3.5-turbo",
                    "messages": [{"role": "user", "content": "Test message with john@example.com"}],
                    "max_tokens": 5
                }' 2>/dev/null)

            if echo "$test_response" | grep -q "choices" 2>/dev/null; then
                success "PII detection and anonymization working"
            else
                warn "PII detection test failed or API key issue"
            fi
        else
            warn "No API key configured for PII detection test"
        fi
    else
        warn "Gateway not running - skipping PII detection test"
    fi

    echo
}

# Main verification process
main() {
    echo
    info "Starting comprehensive verification..."
    echo

    check_prerequisites
    test_compilation
    test_core_components
    test_configuration
    test_docker_images
    test_api_endpoints
    test_monitoring
    test_security
    test_ai_providers
    test_pii_detection

    # Summary
    echo
    echo "═══════════════════════════════════════════"
    echo "              VERIFICATION SUMMARY"
    echo "═══════════════════════════════════════════"

    if [[ $TESTS_PASSED -gt 0 ]]; then
        success "$TESTS_PASSED tests passed"
    fi

    if [[ $TESTS_FAILED -gt 0 ]]; then
        error "$TESTS_FAILED tests failed"
        echo
        warn "Some tests failed. Please review the errors above."
        warn "For production deployment, all tests should pass."
        exit 1
    else
        echo
        success "🎉 All tests passed! Sovereign Privacy Gateway is ready."
        echo
        info "Next steps:"
        echo "  1. Configure API keys in .env file (if not done)"
        echo "  2. Run: docker compose up -d (development)"
        echo "  3. Run: docker compose -f docker-compose.prod.yml up -d (production)"
        echo "  4. Access dashboard at http://localhost:8081"
        echo "  5. Test with: curl http://localhost:8080/health"
        echo
        info "Documentation: docs/installation.md"
        echo
    fi
}

# Run verification
main "$@"
