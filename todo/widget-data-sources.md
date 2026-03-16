# Widget Data Sources & Computation

This document explains how to compute data for each dashboard widget type from existing systems and audit logs.

## Data Source Mapping

### Existing Data Sources
1. **Audit Logs** (`audit_logs` table) - Request/response transactions
2. **Application Metrics** - Runtime statistics from Go application
3. **NER Engine** - PII detection results
4. **Router Status** - Provider health and routing decisions
5. **System Resources** - Memory, CPU, connection counts

## Widget Implementations

### 1. stat_card Widgets

#### Total Requests
**Data Source**: Audit logs
```go
func (w *WidgetService) GetTotalRequestsData() (*StatCardData, error) {
    // Current period (last 24h)
    current, err := w.db.QueryRow(`
        SELECT COUNT(*)
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
    `).Scan(&currentCount)

    // Previous period (24-48h ago)
    previous, err := w.db.QueryRow(`
        SELECT COUNT(*)
        FROM audit_logs
        WHERE created_at BETWEEN datetime('now', '-48 hours') AND datetime('now', '-24 hours')
    `).Scan(&previousCount)

    return &StatCardData{
        Value: currentCount,
        Delta: currentCount - previousCount,
        DeltaPeriod: "vs last 24h",
    }, nil
}
```

#### PII Protected
**Data Source**: Audit logs + NER results
```go
func (w *WidgetService) GetPIIProtectedData() (*StatCardData, error) {
    // Count requests where PII was detected and redacted
    current, err := w.db.QueryRow(`
        SELECT COUNT(*)
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
        AND pii_detected = true
        AND redacted = true
    `).Scan(&currentCount)

    // Calculate delta vs previous period
    // ... similar to total requests

    return &StatCardData{
        Value: currentCount,
        Delta: currentCount - previousCount,
        DeltaPeriod: "vs last 24h",
    }, nil
}
```

#### Blocked Requests
**Data Source**: Audit logs (blocked/strict mode)
```go
func (w *WidgetService) GetBlockedRequestsData() (*StatCardData, error) {
    // Count requests blocked due to high sensitivity
    current, err := w.db.QueryRow(`
        SELECT COUNT(*)
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
        AND status = 'blocked'
    `).Scan(&currentCount)

    return &StatCardData{
        Value: currentCount,
        Delta: currentCount - previousCount,
        DeltaPeriod: "vs last 24h",
    }, nil
}
```

#### Average Latency
**Data Source**: Audit logs (processing time)
```go
func (w *WidgetService) GetAvgLatencyData() (*StatCardData, error) {
    // Calculate average latency for last 24h
    var currentAvg float64
    err := w.db.QueryRow(`
        SELECT AVG(processing_time_ms)
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
        AND processing_time_ms IS NOT NULL
    `).Scan(&currentAvg)

    // Previous period average
    var previousAvg float64
    err = w.db.QueryRow(`
        SELECT AVG(processing_time_ms)
        FROM audit_logs
        WHERE created_at BETWEEN datetime('now', '-48 hours') AND datetime('now', '-24 hours')
        AND processing_time_ms IS NOT NULL
    `).Scan(&previousAvg)

    return &StatCardData{
        Value: currentAvg,
        Unit: "ms",
        Delta: currentAvg - previousAvg,
        DeltaPeriod: "vs last 24h",
    }, nil
}
```

### 2. success_rate Widget

**Data Source**: Audit logs (success vs error status)
```go
func (w *WidgetService) GetSuccessRateData() (*SuccessRateData, error) {
    // Count successful vs total requests
    var total, successful, withRedaction int

    err := w.db.QueryRow(`
        SELECT
            COUNT(*) as total,
            SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successful,
            SUM(CASE WHEN pii_detected = true THEN 1 ELSE 0 END) as with_redaction
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
    `).Scan(&total, &successful, &withRedaction)

    successRate := float64(successful) / float64(total) * 100

    return &SuccessRateData{
        SuccessRate:      successRate,
        RedactedRequests: withRedaction,
        TotalRequests:    total,
    }, nil
}
```

### 3. protection_summary Widget

**Data Source**: Audit logs + NER statistics
```go
func (w *WidgetService) GetProtectionSummaryData() (*ProtectionSummaryData, error) {
    var protected, blocked, total int

    err := w.db.QueryRow(`
        SELECT
            SUM(CASE WHEN pii_detected = true AND redacted = true THEN 1 ELSE 0 END) as protected,
            SUM(CASE WHEN status = 'blocked' THEN 1 ELSE 0 END) as blocked,
            COUNT(*) as total
        FROM audit_logs
        WHERE date(created_at) = date('now')  -- Today only
    `).Scan(&protected, &blocked, &total)

    detectionRate := float64(protected+blocked) / float64(total) * 100

    return &ProtectionSummaryData{
        Rows: []ProtectionRow{
            {
                Key:   "protected",
                Label: "Protected",
                Value: protected,
                Color: "green",
            },
            {
                Key:   "blocked",
                Label: "Blocked",
                Value: blocked,
                Color: "red",
            },
            {
                Key:   "detection_rate",
                Label: "Detection Rate",
                Value: fmt.Sprintf("%.1f%%", detectionRate),
                Color: "accent",
            },
        },
    }, nil
}
```

### 4. gateway_status Widget

**Data Source**: System status table + runtime metrics
```go
func (w *WidgetService) GetGatewayStatusData() (*GatewayStatusData, error) {
    // Get component statuses
    rows, err := w.db.Query(`
        SELECT component_key, component_name, status, is_provider
        FROM system_status
        ORDER BY is_provider, component_name
    `)
    defer rows.Close()

    components := []ComponentStatus{}
    overallHealthy := true

    for rows.Next() {
        var comp ComponentStatus
        err := rows.Scan(&comp.Key, &comp.Name, &comp.Status, &comp.Provider)
        if err != nil {
            return nil, err
        }

        if comp.Status != "healthy" {
            overallHealthy = false
        }

        components = append(components, comp)
    }

    // Get memory usage from Go runtime
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    status := "healthy"
    if !overallHealthy {
        status = "degraded"
    }

    return &GatewayStatusData{
        Status:     status,
        Uptime:     int(time.Since(startTime).Seconds()),
        Components: components,
        MemoryUsage: MemoryUsage{
            Heap:  int(m.HeapInuse / 1024 / 1024),     // Convert to MB
            Stack: int(m.StackInuse / 1024 / 1024),    // Convert to MB
            Total: int(m.Sys / 1024 / 1024),           // Convert to MB
        },
    }, nil
}
```

### 5. top_providers Widget

**Data Source**: Audit logs (provider routing)
```go
func (w *WidgetService) GetTopProvidersData() (*TopProvidersData, error) {
    rows, err := w.db.Query(`
        SELECT
            provider,
            COUNT(*) as requests,
            COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percent
        FROM audit_logs
        WHERE created_at >= datetime('now', '-24 hours')
        AND provider IS NOT NULL
        GROUP BY provider
        ORDER BY requests DESC
        LIMIT 10
    `)
    defer rows.Close()

    providers := []ProviderData{}
    providerColors := map[string]string{
        "openai":    "#10A37F",
        "anthropic": "#C7A97B",
        "google":    "#4285F4",
        "local":     "#BF5AF2",
    }

    for rows.Next() {
        var p ProviderData
        err := rows.Scan(&p.Provider, &p.Requests, &p.Percent)
        if err != nil {
            return nil, err
        }

        p.Label = strings.Title(p.Provider)
        p.Color = providerColors[p.Provider]
        providers = append(providers, p)
    }

    return &TopProvidersData{
        Providers: providers,
    }, nil
}
```

### 6. activity_chart Widget

**Data Source**: Pre-computed hourly metrics
```go
func (w *WidgetService) GetActivityChartData() (*ActivityChartData, error) {
    rows, err := w.db.Query(`
        SELECT hour_bucket, total_requests
        FROM metrics_hourly
        WHERE hour_bucket >= datetime('now', '-24 hours')
        ORDER BY hour_bucket ASC
    `)
    defer rows.Close()

    points := []ActivityPoint{}

    for rows.Next() {
        var p ActivityPoint
        var hourBucket string
        err := rows.Scan(&hourBucket, &p.Count)
        if err != nil {
            return nil, err
        }

        // Convert to ISO 8601 timestamp
        t, _ := time.Parse("2006-01-02T15:04:05Z", hourBucket)
        p.Timestamp = t.Format(time.RFC3339)
        points = append(points, p)
    }

    return &ActivityChartData{
        PeriodLabel: "Last 24 hours",
        Points:      points,
    }, nil
}
```

### 7. live_stats Widget

**Data Source**: Real-time application metrics
```go
func (w *WidgetService) GetLiveStatsData() (*LiveStatsData, error) {
    // Get current metrics from live_metrics table and runtime
    var activeConnections, requestsPerSecond float64

    // Active connections from application state
    activeConnections = float64(getCurrentActiveConnections())

    // Requests per second (last minute average)
    err := w.db.QueryRow(`
        SELECT COUNT(*) / 60.0
        FROM audit_logs
        WHERE created_at >= datetime('now', '-1 minute')
    `).Scan(&requestsPerSecond)

    // Cache hit rate from application cache
    cacheHitRate := getCacheHitRate()

    // Error rate (last 5 minutes)
    var errorRate float64
    err = w.db.QueryRow(`
        SELECT
            SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) * 1.0 / COUNT(*)
        FROM audit_logs
        WHERE created_at >= datetime('now', '-5 minutes')
    `).Scan(&errorRate)

    uptime := int(time.Since(startTime).Seconds())

    return &LiveStatsData{
        Rows: []LiveStatRow{
            {
                Key:    "requests_per_second",
                Label:  "Req / sec",
                Value:  requestsPerSecond,
                Format: "rate_per_sec",
            },
            {
                Key:    "active_connections",
                Label:  "Active Connections",
                Value:  activeConnections,
                Format: "number",
            },
            {
                Key:    "cache_hit_rate",
                Label:  "Cache Hit Rate",
                Value:  cacheHitRate,
                Format: "percent",
            },
            {
                Key:    "error_rate",
                Label:  "Error Rate",
                Value:  errorRate,
                Format: "percent",
                ColorRule: &ColorRule{
                    ThresholdWarning:  0.03,
                    ThresholdCritical: 0.05,
                    Direction:         "higher_is_worse",
                },
            },
            {
                Key:    "uptime",
                Label:  "Uptime",
                Value:  float64(uptime),
                Format: "duration_human",
            },
        },
    }, nil
}
```

## Data Update Strategies

### Real-time Data (< 30 seconds)
- **live_stats**: Computed on-demand from application state
- **gateway_status**: Updated by background monitoring jobs every 15 seconds

### Near Real-time Data (30-60 seconds)
- **stat_card** widgets: Cached for 30 seconds, computed from audit logs
- **success_rate**: Cached for 60 seconds
- **protection_summary**: Cached for 60 seconds

### Periodic Data (5+ minutes)
- **top_providers**: Updated every 5 minutes from audit logs
- **activity_chart**: Pre-computed hourly, updated every hour

## Performance Optimizations

### Database Indexes
```sql
-- Essential indexes for widget queries
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_status_created_at ON audit_logs(status, created_at);
CREATE INDEX idx_audit_logs_provider_created_at ON audit_logs(provider, created_at);
CREATE INDEX idx_audit_logs_pii_created_at ON audit_logs(pii_detected, redacted, created_at);
```

### Caching Strategy
- **Memory cache**: 5-minute TTL for expensive computations
- **Database cache**: Pre-computed hourly/daily aggregations
- **Connection pooling**: Dedicated read replicas for dashboard queries

### Query Optimization
- Use prepared statements for repeated queries
- Limit time ranges to reduce scan costs
- Aggregate at database level instead of application level
- Use EXPLAIN QUERY PLAN to optimize slow queries

## Error Handling

### Data Availability
- Graceful degradation when audit logs are incomplete
- Default values when metrics computation fails
- Fallback to cached data during database issues

### System Health
- Circuit breaker pattern for failing provider health checks
- Timeout handling for slow database queries
- Monitoring and alerting for widget data freshness