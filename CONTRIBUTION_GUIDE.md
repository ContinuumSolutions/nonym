# Contribution Guide

## Running Tests

All tests use the Go standard library test runner. No external services are required - every package uses in-memory SQLite and stub dependencies.

### Run the full suite

```bash
go test ./...
```

### Run a single package

```bash
go test ./internal/brain/...
go test ./internal/ledger/...
```

### Run with verbose output (see each test name)

```bash
go test -v ./...
```

### Run a single named test

```bash
go test -v -run TestPipelineRun_AcceptPath ./internal/brain/
go test -v -run TestHandlerScore_200WithFields ./internal/ledger/
```

### Run without cache (force a fresh run)

```bash
go test -count=1 ./...
```

### Run with the race detector

```bash
go test -race ./...
```

---

## Test Package Layout

Each `internal/<package>/` directory contains two test files:

| File | Purpose |
|---|---|
| `store_test.go` | Unit tests for the SQLite store layer. Opens an in-memory database, runs `Migrate()`, then exercises each store method directly. |
| `handler_test.go` | HTTP-level tests for the Fiber handlers. Uses `app.Test(httptest.NewRequest(...))` — no real TCP connections. |

Additional test files exist where needed:

| File | Purpose |
|---|---|
| `brain/kernel_test.go` | Pure logic tests for `EgoKernel.Triage`, `Decide`, entropy |
| `brain/service_test.go` | Tests for `MatrixFromPreferences`, `ApplyBiometricsGate`, `IsH2HI` |
| `brain/pipeline_test.go` | Tests for the full signal → LLM → Triage → Event pipeline using a stub `Analyser` |
| `integrations/crypto_test.go` | Tests for AES-256-GCM encrypt/decrypt and key masking |
| `ai/client_test.go` | Tests for LLM output parsing (no network calls) |
| `harvest/scanner_test.go` | Tests for sender normalisation and debt scoring logic |

---

## Writing New Tests

### Package convention

Tests live in the **same package** as the code they test (not `package foo_test`). This gives access to unexported helpers like `newTestDB` and `newTestStore` defined in `store_test.go`.

```go
// correct
package ledger

// avoid — external test packages cannot share helpers across files
package ledger_test
```

### In-memory database

Every store test creates a fresh in-memory SQLite database and calls `Migrate()` before use:

```go
func newTestStore(t *testing.T) *Store {
    t.Helper()
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("open db: %v", err)
    }
    t.Cleanup(func() { db.Close() })
    s := NewStore(db)
    if err := s.Migrate(); err != nil {
        t.Fatalf("Migrate: %v", err)
    }
    return s
}
```

Always register the `modernc.org/sqlite` driver in the test file:

```go
import _ "modernc.org/sqlite"
```

### Handler tests

Handler tests wire a real `fiber.App` with a test store, then call `app.Test()`:

```go
func setupApp(t *testing.T) *fiber.App {
    t.Helper()
    store := newTestStore(t)
    app := fiber.New()
    NewHandler(store).RegisterRoutes(app)
    return app
}

func TestHandlerGet_200(t *testing.T) {
    app := setupApp(t)
    req := httptest.NewRequest(http.MethodGet, "/my/route", nil)
    resp, err := app.Test(req)
    if err != nil {
        t.Fatalf("app.Test: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Errorf("want 200, got %d", resp.StatusCode)
    }
}
```

### Stubbing external dependencies

For packages that call the LLM or datasync engine, use the `Analyser` interface defined in `brain/pipeline.go`:

```go
type stubAnalyser struct{}

func (s *stubAnalyser) AnalyseBatch(ctx context.Context, signals []datasync.RawSignal) ([]*ai.AnalysedSignal, []error) {
    // Return nil per signal — pipeline skips gracefully
    return make([]*ai.AnalysedSignal, len(signals)), make([]error, len(signals))
}
```

For datasync adapters, implement the `datasync.Adapter` interface:

```go
type stubAdapter struct {
    slug    string
    signals []datasync.RawSignal
    err     error
}

func (a *stubAdapter) Slug() string { return a.slug }
func (a *stubAdapter) Pull(_ context.Context, _ datasync.Credentials, _ time.Time) ([]datasync.RawSignal, error) {
    return a.signals, a.err
}
```

### Encryption key for integration tests

The `integrations.Store` requires a 32-byte AES key. Use an all-zero key in tests — it is valid and produces deterministic ciphertext for the test environment:

```go
key := make([]byte, 32) // all-zero, valid AES-256 key
store := integrations.NewStore(db, key)
```

### Naming conventions

- `TestFoo_ConditionUnderTest` — preferred format
- `TestHandlerGet_NotFound`, `TestHandlerCreate_400MissingFields`
- `TestStore_OperationName_ExpectedOutcome`

### What not to test

- HTTP adapter implementations in `internal/datasync/` (they require live OAuth tokens and third-party APIs)
- `cmd/ek1/main.go` wiring — covered by `go build ./...`
- Swagger docs generation — covered by running `swag init`
