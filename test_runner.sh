#!/bin/bash

# Comprehensive Test Runner for Sovereign Privacy Gateway
# This script runs all unit, integration, and functional tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT="10m"
COVERAGE_THRESHOLD=80
PARALLEL_TESTS=4

# Print header
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Sovereign Privacy Gateway Tests     ${NC}"
echo -e "${BLUE}========================================${NC}"
echo

# Function to print status
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run tests with coverage
run_tests() {
    local test_type=$1
    local test_path=$2
    local extra_args=$3

    print_status "Running $test_type tests..."

    if [ -n "$extra_args" ]; then
        go test -timeout=$TEST_TIMEOUT -v -race -coverprofile="coverage-$test_type.out" $test_path $extra_args
    else
        go test -timeout=$TEST_TIMEOUT -v -race -coverprofile="coverage-$test_type.out" $test_path
    fi

    if [ $? -eq 0 ]; then
        print_success "$test_type tests passed!"
    else
        print_error "$test_type tests failed!"
        return 1
    fi
}

# Function to check coverage
check_coverage() {
    local coverage_file=$1
    local test_type=$2

    if [ -f "$coverage_file" ]; then
        local coverage=$(go tool cover -func="$coverage_file" | grep "total:" | awk '{print $3}' | sed 's/%//')

        if [ -n "$coverage" ]; then
            print_status "$test_type coverage: ${coverage}%"

            if (( $(echo "$coverage >= $COVERAGE_THRESHOLD" | bc -l) )); then
                print_success "$test_type coverage meets threshold (${COVERAGE_THRESHOLD}%)"
            else
                print_warning "$test_type coverage below threshold (${COVERAGE_THRESHOLD}%)"
                return 1
            fi
        else
            print_warning "Could not parse coverage for $test_type"
        fi
    else
        print_warning "Coverage file not found for $test_type"
    fi
}

# Function to run benchmarks
run_benchmarks() {
    print_status "Running benchmark tests..."
    go test -bench=. -benchmem -run=^$ ./... > benchmark_results.txt 2>&1

    if [ $? -eq 0 ]; then
        print_success "Benchmarks completed!"
        echo "Benchmark results saved to benchmark_results.txt"
    else
        print_warning "Some benchmarks failed or encountered issues"
    fi
}

# Function to generate test report
generate_report() {
    print_status "Generating test report..."

    # Combine coverage files
    echo "mode: set" > coverage-combined.out
    tail -n +2 coverage-*.out >> coverage-combined.out 2>/dev/null || true

    # Generate HTML coverage report
    go tool cover -html=coverage-combined.out -o coverage-report.html

    # Calculate total coverage
    if [ -f "coverage-combined.out" ]; then
        local total_coverage=$(go tool cover -func=coverage-combined.out | grep "total:" | awk '{print $3}' | sed 's/%//')

        echo -e "\n${BLUE}========================================${NC}"
        echo -e "${BLUE}           TEST SUMMARY                 ${NC}"
        echo -e "${BLUE}========================================${NC}"
        echo -e "Total Coverage: ${total_coverage}%"
        echo -e "Coverage Report: coverage-report.html"
        echo -e "Benchmark Results: benchmark_results.txt"
        echo -e "${BLUE}========================================${NC}\n"

        if (( $(echo "$total_coverage >= $COVERAGE_THRESHOLD" | bc -l) )); then
            print_success "All tests completed successfully with adequate coverage!"
            return 0
        else
            print_warning "Tests completed but coverage is below threshold"
            return 1
        fi
    else
        print_warning "Could not generate combined coverage report"
        return 1
    fi
}

# Main test execution
main() {
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi

    # Check if bc is available for coverage calculation
    if ! command -v bc &> /dev/null; then
        print_warning "bc not available - coverage threshold checking disabled"
        COVERAGE_THRESHOLD=0
    fi

    # Clean up old coverage files
    rm -f coverage-*.out coverage-report.html benchmark_results.txt

    # Parse command line arguments
    UNIT_TESTS=true
    INTEGRATION_TESTS=true
    BENCHMARKS=false
    VERBOSE=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            --unit-only)
                INTEGRATION_TESTS=false
                shift
                ;;
            --integration-only)
                UNIT_TESTS=false
                shift
                ;;
            --with-benchmarks)
                BENCHMARKS=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --coverage-threshold)
                COVERAGE_THRESHOLD=$2
                shift 2
                ;;
            --timeout)
                TEST_TIMEOUT=$2
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --unit-only           Run only unit tests"
                echo "  --integration-only    Run only integration tests"
                echo "  --with-benchmarks     Include benchmark tests"
                echo "  --verbose             Enable verbose output"
                echo "  --coverage-threshold N Set coverage threshold (default: 80)"
                echo "  --timeout DURATION    Set test timeout (default: 10m)"
                echo "  -h, --help           Show this help message"
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done

    # Set up test environment variables
    export CGO_ENABLED=1
    export JWT_SECRET="test-jwt-secret-key-for-testing-only"
    export LOG_LEVEL="error"  # Reduce log noise during tests

    print_status "Starting test suite execution..."
    print_status "Test timeout: $TEST_TIMEOUT"
    print_status "Coverage threshold: $COVERAGE_THRESHOLD%"
    echo

    # Run tests
    test_failed=false

    if [ "$UNIT_TESTS" = true ]; then
        # Unit tests for each package
        run_tests "auth" "./pkg/auth" || test_failed=true
        check_coverage "coverage-auth.out" "auth" || true

        run_tests "apikeys" "./pkg/auth" "-run=.*APIKey.*" || test_failed=true

        run_tests "organization" "./pkg/auth" "-run=.*Organization.*" || test_failed=true

        run_tests "ner" "./pkg/ner" || test_failed=true
        check_coverage "coverage-ner.out" "ner" || true

        run_tests "interceptor" "./pkg/interceptor" || test_failed=true
        check_coverage "coverage-interceptor.out" "interceptor" || true

        run_tests "router" "./pkg/router" || test_failed=true
        check_coverage "coverage-router.out" "router" || true

        run_tests "audit" "./pkg/audit" || test_failed=true
        check_coverage "coverage-audit.out" "audit" || true
    fi

    if [ "$INTEGRATION_TESTS" = true ]; then
        # Integration tests
        run_tests "interceptor-integration" "./pkg/interceptor" "-run=.*Integration.*" || test_failed=true

        run_tests "router-integration" "./pkg/router" "-run=.*Integration.*" || test_failed=true

        run_tests "gateway-integration" "./cmd/gateway" "-run=.*Integration.*" || test_failed=true
        check_coverage "coverage-gateway-integration.out" "gateway-integration" || true
    fi

    # Run benchmarks if requested
    if [ "$BENCHMARKS" = true ]; then
        run_benchmarks || true  # Don't fail on benchmark issues
    fi

    # Generate final report
    generate_report
    report_status=$?

    # Clean up temporary files
    rm -f coverage-*.out

    # Final status
    if [ "$test_failed" = true ]; then
        print_error "Some tests failed!"
        exit 1
    elif [ $report_status -ne 0 ]; then
        print_warning "Tests passed but coverage/reporting issues"
        exit 1
    else
        print_success "All tests completed successfully!"
        exit 0
    fi
}

# Run main function
main "$@"