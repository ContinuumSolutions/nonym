# Dashboard API Implementation Plan

## Phase 1: Database Setup (1-2 days)

### 1.1 Database Migration
- [ ] Create migration script for new tables
- [ ] Add database schema in `pkg/audit/schema.sql`
- [ ] Update database initialization in `pkg/audit/database.go`
- [ ] Test migration on development database

### 1.2 Database Models
- [ ] Create Go structs for new tables in `pkg/audit/models.go`:
  ```go
  type DashboardWidget struct {
      ID                     string    `json:"id" db:"id"`
      Type                   string    `json:"type" db:"type"`
      Title                  string    `json:"title" db:"title"`
      Subtitle               *string   `json:"subtitle" db:"subtitle"`
      Endpoint               string    `json:"endpoint" db:"endpoint"`
      RefreshIntervalSeconds int       `json:"refresh_interval_seconds" db:"refresh_interval_seconds"`
      GridRow                int       `json:"grid_row" db:"grid_row"`
      GridColSpan            int       `json:"grid_col_span" db:"grid_col_span"`
      GridOrder              int       `json:"grid_order" db:"grid_order"`
      ColorScheme            string    `json:"color_scheme" db:"color_scheme"`
      Icon                   *string   `json:"icon" db:"icon"`
      ValueFormat            string    `json:"value_format" db:"value_format"`
      ThresholdWarning       *float64  `json:"threshold_warning" db:"threshold_warning"`
      ThresholdCritical      *float64  `json:"threshold_critical" db:"threshold_critical"`
      Enabled                bool      `json:"enabled" db:"enabled"`
      CreatedAt              time.Time `json:"created_at" db:"created_at"`
      UpdatedAt              time.Time `json:"updated_at" db:"updated_at"`
  }

  type MetricsHourly struct {
      ID                   int       `json:"id" db:"id"`
      HourBucket           string    `json:"hour_bucket" db:"hour_bucket"`
      TotalRequests        int       `json:"total_requests" db:"total_requests"`
      SuccessfulRequests   int       `json:"successful_requests" db:"successful_requests"`
      FailedRequests       int       `json:"failed_requests" db:"failed_requests"`
      BlockedRequests      int       `json:"blocked_requests" db:"blocked_requests"`
      RequestsWithPII      int       `json:"requests_with_pii" db:"requests_with_pii"`
      PIIEntitiesDetected  int       `json:"pii_entities_detected" db:"pii_entities_detected"`
      PIIEntitiesRedacted  int       `json:"pii_entities_redacted" db:"pii_entities_redacted"`
      AvgLatencyMs         float64   `json:"avg_latency_ms" db:"avg_latency_ms"`
      P95LatencyMs         float64   `json:"p95_latency_ms" db:"p95_latency_ms"`
      P99LatencyMs         float64   `json:"p99_latency_ms" db:"p99_latency_ms"`
      OpenAIRequests       int       `json:"openai_requests" db:"openai_requests"`
      AnthropicRequests    int       `json:"anthropic_requests" db:"anthropic_requests"`
      GoogleRequests       int       `json:"google_requests" db:"google_requests"`
      LocalRequests        int       `json:"local_requests" db:"local_requests"`
      CreatedAt            time.Time `json:"created_at" db:"created_at"`
  }
  ```

### 1.3 Database Access Layer
- [ ] Create CRUD operations in `pkg/audit/repository.go`
- [ ] Widget configuration queries
- [ ] Metrics aggregation queries
- [ ] System status queries

## Phase 2: Widget Data Computation (2-3 days)

### 2.1 Widget Service Layer
- [ ] Create `pkg/audit/widgets.go` with widget data computation logic
- [ ] Implement data fetchers for each widget type:
  ```go
  type WidgetService struct {
      db *sql.DB
  }

  func (w *WidgetService) GetStatCardData(widgetID string) (*StatCardData, error)
  func (w *WidgetService) GetSuccessRateData() (*SuccessRateData, error)
  func (w *WidgetService) GetProtectionSummaryData() (*ProtectionSummaryData, error)
  func (w *WidgetService) GetGatewayStatusData() (*GatewayStatusData, error)
  func (w *WidgetService) GetTopProvidersData() (*TopProvidersData, error)
  func (w *WidgetService) GetActivityChartData() (*ActivityChartData, error)
  func (w *WidgetService) GetLiveStatsData() (*LiveStatsData, error)
  ```

### 2.2 Data Source Integration
- [ ] Hook into existing audit log system
- [ ] Real-time metrics from application state
- [ ] System health checks integration
- [ ] Memory usage from Go runtime
- [ ] Provider health status checks

### 2.3 Caching Layer
- [ ] Implement in-memory cache for widget data
- [ ] TTL-based cache invalidation
- [ ] Cache warming for frequently accessed widgets

## Phase 3: REST API Implementation (1-2 days)

### 3.1 API Handlers
- [ ] Create `pkg/audit/dashboard_handlers.go`
- [ ] Implement handlers:
  ```go
  func GetDashboardLayout(c *fiber.Ctx) error
  func GetWidgetData(c *fiber.Ctx) error
  ```

### 3.2 Route Registration
- [ ] Add routes in `cmd/gateway/main.go`:
  ```go
  dashboard := app.Group("/api/v1/dashboard")
  dashboard.Get("/layout", audit.GetDashboardLayout)
  dashboard.Get("/widgets/:widget_id", audit.GetWidgetData)
  ```

### 3.3 API Authentication
- [ ] Add API key validation middleware
- [ ] Reuse existing auth from proxy routes
- [ ] Rate limiting implementation

## Phase 4: Background Jobs (1 day)

### 4.1 Metrics Aggregation
- [ ] Create `pkg/audit/aggregator.go`
- [ ] Hourly metrics computation job
- [ ] Daily metrics computation job
- [ ] Historical data backfill script

### 4.2 System Monitoring
- [ ] Create `pkg/audit/monitor.go`
- [ ] Component health check jobs
- [ ] Provider status monitoring
- [ ] Live metrics collection

### 4.3 Job Scheduling
- [ ] Integrate with existing cron/scheduler
- [ ] Background goroutines with proper cancellation
- [ ] Error handling and logging

## Phase 5: Testing & Optimization (1-2 days)

### 5.1 Unit Tests
- [ ] Widget service tests
- [ ] Database repository tests
- [ ] API handler tests
- [ ] Data computation validation

### 5.2 Integration Tests
- [ ] End-to-end API tests
- [ ] Database migration tests
- [ ] Performance tests for large datasets

### 5.3 Load Testing
- [ ] High-frequency widget polling
- [ ] Concurrent dashboard users
- [ ] Database performance under load

## Phase 6: Documentation & Deployment (1 day)

### 6.1 API Documentation
- [ ] OpenAPI/Swagger specification
- [ ] API usage examples
- [ ] Frontend integration guide

### 6.2 Deployment
- [ ] Update Docker configuration
- [ ] Environment variable configuration
- [ ] Production database migration
- [ ] Monitoring and alerting setup

## Key Files to Create/Modify

### New Files
```
pkg/audit/
├── dashboard_handlers.go    # REST API handlers
├── widgets.go              # Widget data computation
├── repository.go           # Database access layer
├── aggregator.go          # Background metric aggregation
├── monitor.go             # System health monitoring
├── models.go              # Database model structs
└── schema.sql             # Database schema

todo/
├── api-specification.md
├── database-schema.md
├── implementation-plan.md
└── widget-data-sources.md
```

### Modified Files
```
cmd/gateway/main.go         # Add dashboard routes
pkg/audit/database.go       # Schema initialization
```

## Dependencies & Libraries

### Required Go Packages
- `github.com/gofiber/fiber/v2` (existing)
- `database/sql` (existing)
- `github.com/mattn/go-sqlite3` (existing)
- Standard library: `time`, `encoding/json`, `sync`

### No Additional Dependencies
- Leverage existing Fiber framework
- Use existing SQLite database
- Build on current audit system

## Configuration

### Environment Variables
```bash
# Dashboard settings
DASHBOARD_CACHE_TTL=300                    # Widget data cache TTL (seconds)
DASHBOARD_METRICS_RETENTION_DAYS=30       # How long to keep hourly metrics
DASHBOARD_AGGREGATION_ENABLED=true        # Enable background aggregation jobs

# Rate limiting
DASHBOARD_RATE_LIMIT=1000                  # Requests per hour per API key
```

## Success Criteria

- [ ] All widgets render with real data
- [ ] Dashboard updates every 5-30 seconds based on widget type
- [ ] API response times < 100ms for cached data
- [ ] API response times < 500ms for real-time data
- [ ] Memory usage increase < 50MB
- [ ] Database queries optimized with proper indexes
- [ ] 100% test coverage for widget data computation
- [ ] Load test: 10 concurrent users, 5-second polling for 10 minutes

## Risk Mitigation

### Performance Risks
- **Risk**: Slow database queries with large audit logs
- **Mitigation**: Pre-computed aggregations, proper indexing, query optimization

### Memory Risks
- **Risk**: High memory usage from caching
- **Mitigation**: TTL-based cache expiration, memory monitoring, cache size limits

### Data Accuracy Risks
- **Risk**: Stale or incorrect metrics
- **Mitigation**: Real-time validation, data freshness checks, fallback to live computation