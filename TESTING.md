# Comprehensive Testing Suite for Sovereign Privacy Gateway

This document describes the comprehensive testing suite implemented for the Sovereign Privacy Gateway project.

## Overview

The testing suite includes:

- **Unit Tests**: Test individual functions and components
- **Integration Tests**: Test complete workflows and system interactions
- **Functional Tests**: Test end-to-end user scenarios
- **API Tests**: Test all REST endpoints with various scenarios
- **Benchmark Tests**: Performance and load testing
- **Security Tests**: Vulnerability and security validation

## Test Structure

### Unit Tests

Located in `*_test.go` files alongside the source code:

#### Authentication Package (`pkg/auth/`)
- `auth_test.go` - Core authentication functions
- `apikeys_test.go` - API key management
- `organization_test.go` - Organization and team management
- `handlers_test.go` - HTTP handlers and middleware

#### NER Package (`pkg/ner/`)
- `engine_test.go` - PII detection and anonymization

#### Interceptor Package (`pkg/interceptor/`)
- `proxy_test.go` - Basic proxy functionality
- `proxy_integration_test.go` - Full proxy integration tests

#### Router Package (`pkg/router/`)
- `providers_test.go` - Basic provider routing
- `providers_integration_test.go` - Full routing integration tests

#### Audit Package (`pkg/audit/`)
- `logger_test.go` - Audit logging and statistics

### Integration Tests

#### Gateway Integration (`cmd/gateway/`)
- `integration_test.go` - Complete application integration tests

## Test Features

### Comprehensive Coverage

- **Authentication**: Registration, login, JWT validation, session management
- **API Key Management**: Creation, validation, revocation, permissions
- **Organization Management**: CRUD operations, team management, security settings
- **PII Detection**: All entity types, confidence scoring, anonymization/de-anonymization
- **Proxy Functionality**: Request routing, error handling, timeouts
- **Provider Routing**: Load balancing, fallback, circuit breaker patterns
- **Audit Logging**: Transaction logging, statistics, compliance tracking

### Test Fixtures and Utilities

- **Test Data**: Pre-defined users, API keys, and test scenarios in `testdata/fixtures.go`
- **Database Setup**: In-memory SQLite for isolated testing
- **Mock Servers**: HTTP test servers for AI provider simulation
- **Test Utilities**: Helper functions for common test operations

### Test Execution

#### Using the Test Runner Script

```bash
# Run all tests with coverage
./test_runner.sh

# Run only unit tests
./test_runner.sh --unit-only

# Run only integration tests
./test_runner.sh --integration-only

# Run with benchmarks
./test_runner.sh --with-benchmarks

# Set custom coverage threshold
./test_runner.sh --coverage-threshold 85
```

#### Using Make Commands

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests only
make test-integration

# Generate coverage report
make test-coverage

# Run benchmarks
make benchmark

# Run linting
make lint

# Format code
make format
```

#### Using Go Test Directly

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./pkg/auth

# Run specific test
go test ./pkg/auth -run=TestAuthSuite

# Run with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

## Test Categories

### 1. Unit Tests
Test individual functions and methods in isolation:
- Input validation
- Business logic
- Error handling
- Edge cases
- Performance critical paths

### 2. Integration Tests
Test interactions between components:
- Database operations
- HTTP request/response flows
- External service integration
- Cross-package dependencies

### 3. Functional Tests
Test complete user workflows:
- User registration and authentication flow
- API key management lifecycle
- Organization setup and management
- End-to-end proxy requests with PII detection

### 4. API Tests
Test REST endpoints comprehensively:
- All HTTP methods (GET, POST, PUT, DELETE, PATCH)
- Authentication and authorization
- Request validation
- Error responses
- JSON serialization/deserialization

### 5. Benchmark Tests
Performance and scalability testing:
- Function execution time
- Memory allocation patterns
- Concurrent request handling
- Throughput measurements

## Test Data Management

### Fixtures
- Predefined test users with various roles and states
- Sample API keys with different permissions
- Mock PII data for detection testing
- Realistic request/response payloads

### Database Setup
- In-memory SQLite for fast, isolated tests
- Automated schema creation and teardown
- Test data population and cleanup
- Transaction rollback for test isolation

### Mock Services
- HTTP test servers simulating AI providers
- Configurable response scenarios
- Error simulation capabilities
- Latency and timeout testing

## Coverage Requirements

- **Minimum Coverage**: 75% overall
- **Critical Packages**: 85% coverage required
  - `pkg/auth` - Authentication and security
  - `pkg/ner` - PII detection core functionality
  - `pkg/audit` - Compliance and logging

## Continuous Integration

### GitHub Actions Workflow (`.github/workflows/test.yml`)

- **Unit Tests**: Run on every commit and PR
- **Integration Tests**: Full system testing
- **Security Tests**: Vulnerability scanning with gosec and govulncheck
- **Build Tests**: Multi-platform build verification
- **Docker Tests**: Container functionality validation
- **Code Quality**: Linting and formatting checks

### Test Reports

- **Coverage Report**: HTML coverage report generated
- **Benchmark Results**: Performance metrics tracking
- **Security Scan**: Vulnerability assessment results
- **Test Results**: Detailed pass/fail information

## Best Practices

### Writing Tests

1. **Use Table-Driven Tests**: For testing multiple scenarios
2. **Test Edge Cases**: Empty inputs, boundary conditions, error states
3. **Mock External Dependencies**: Use test servers and mocks
4. **Descriptive Test Names**: Clearly indicate what is being tested
5. **Setup/Teardown**: Proper test isolation and cleanup

### Test Organization

1. **Test Suites**: Group related tests using testify suites
2. **Helper Functions**: Extract common test setup into utilities
3. **Test Data**: Use fixtures and factories for test data
4. **Parallel Tests**: Enable parallel execution where safe

### Performance Testing

1. **Benchmark Critical Paths**: Focus on high-usage functions
2. **Memory Profiling**: Track allocation patterns
3. **Race Detection**: Run tests with `-race` flag
4. **Load Testing**: Test concurrent request handling

## Security Testing

### Automated Security Checks

- **gosec**: Static analysis for security vulnerabilities
- **govulncheck**: Known vulnerability detection
- **Dependency scanning**: Third-party package security

### Manual Security Testing

- **Authentication bypass attempts**
- **SQL injection testing** (though using SQLite with prepared statements)
- **Input validation testing**
- **Authorization boundary testing**

## Debugging Tests

### Common Issues

1. **Test Isolation**: Ensure tests don't interfere with each other
2. **Race Conditions**: Use proper synchronization in concurrent tests
3. **External Dependencies**: Mock or stub external services
4. **Environment Setup**: Ensure consistent test environment

### Debugging Tools

- **Verbose Output**: Use `-v` flag for detailed test output
- **Test Selection**: Run specific tests with `-run` flag
- **Profiling**: Use pprof for performance analysis
- **Coverage Analysis**: Identify untested code paths

## Future Enhancements

### Planned Improvements

1. **Load Testing**: Add comprehensive load testing scenarios
2. **Chaos Engineering**: Introduce failure injection testing
3. **Property-Based Testing**: Add generative testing for edge cases
4. **Contract Testing**: Add API contract validation
5. **Visual Testing**: Screenshot comparison for dashboard UI

### Test Metrics Tracking

1. **Test Execution Time**: Monitor test suite performance
2. **Coverage Trends**: Track coverage over time
3. **Flaky Test Detection**: Identify and fix unstable tests
4. **Test Maintenance**: Regular test suite cleanup and updates

## Contributing to Tests

### Guidelines for Contributors

1. **Test Coverage**: New features must include comprehensive tests
2. **Test Documentation**: Document complex test scenarios
3. **Performance Impact**: Consider test execution time
4. **Test Data**: Use appropriate fixtures and avoid hardcoded values
5. **Error Scenarios**: Include negative test cases

### Review Process

1. **Code Review**: All tests reviewed for quality and coverage
2. **CI Validation**: Tests must pass all CI checks
3. **Performance Review**: Benchmark changes for performance impact
4. **Security Review**: Security-related changes require additional review