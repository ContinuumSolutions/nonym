# Database Schema for Dashboard APIs

## Overview
The dashboard APIs require several new tables to store widget configurations and pre-computed metrics for performance.

## New Tables

### 1. dashboard_widgets
Stores widget configuration and layout information.

```sql
CREATE TABLE dashboard_widgets (
    id TEXT PRIMARY KEY,                    -- e.g., 'stat-total-requests'
    type TEXT NOT NULL,                     -- e.g., 'stat_card', 'success_rate'
    title TEXT NOT NULL,                    -- Display title
    subtitle TEXT,                          -- Optional subtitle
    endpoint TEXT NOT NULL,                 -- API endpoint path
    refresh_interval_seconds INTEGER NOT NULL DEFAULT 60,

    -- Grid placement
    grid_row INTEGER NOT NULL DEFAULT 1,
    grid_col_span INTEGER NOT NULL DEFAULT 3,
    grid_order INTEGER NOT NULL DEFAULT 0,

    -- Display options
    color_scheme TEXT DEFAULT 'blue',       -- blue, green, red, purple, teal, orange
    icon TEXT,                              -- SVG path or named icon
    value_format TEXT DEFAULT 'number',     -- number, percent, duration_ms, etc.
    threshold_warning REAL,                 -- Warning threshold value
    threshold_critical REAL,               -- Critical threshold value

    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2. metrics_hourly
Pre-computed hourly metrics for performance.

```sql
CREATE TABLE metrics_hourly (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hour_bucket TEXT NOT NULL,              -- '2026-03-16T14:00:00Z'

    -- Request metrics
    total_requests INTEGER NOT NULL DEFAULT 0,
    successful_requests INTEGER NOT NULL DEFAULT 0,
    failed_requests INTEGER NOT NULL DEFAULT 0,
    blocked_requests INTEGER NOT NULL DEFAULT 0,

    -- PII metrics
    requests_with_pii INTEGER NOT NULL DEFAULT 0,
    pii_entities_detected INTEGER NOT NULL DEFAULT 0,
    pii_entities_redacted INTEGER NOT NULL DEFAULT 0,

    -- Performance metrics
    avg_latency_ms REAL NOT NULL DEFAULT 0,
    p95_latency_ms REAL NOT NULL DEFAULT 0,
    p99_latency_ms REAL NOT NULL DEFAULT 0,

    -- Provider breakdown
    openai_requests INTEGER NOT NULL DEFAULT 0,
    anthropic_requests INTEGER NOT NULL DEFAULT 0,
    google_requests INTEGER NOT NULL DEFAULT 0,
    local_requests INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(hour_bucket)
);
```

### 3. metrics_daily
Pre-computed daily metrics for longer-term trends.

```sql
CREATE TABLE metrics_daily (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date_bucket TEXT NOT NULL,              -- '2026-03-16'

    -- Aggregate daily metrics (same structure as hourly)
    total_requests INTEGER NOT NULL DEFAULT 0,
    successful_requests INTEGER NOT NULL DEFAULT 0,
    failed_requests INTEGER NOT NULL DEFAULT 0,
    blocked_requests INTEGER NOT NULL DEFAULT 0,

    requests_with_pii INTEGER NOT NULL DEFAULT 0,
    pii_entities_detected INTEGER NOT NULL DEFAULT 0,
    pii_entities_redacted INTEGER NOT NULL DEFAULT 0,

    avg_latency_ms REAL NOT NULL DEFAULT 0,
    p95_latency_ms REAL NOT NULL DEFAULT 0,
    p99_latency_ms REAL NOT NULL DEFAULT 0,

    openai_requests INTEGER NOT NULL DEFAULT 0,
    anthropic_requests INTEGER NOT NULL DEFAULT 0,
    google_requests INTEGER NOT NULL DEFAULT 0,
    local_requests INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(date_bucket)
);
```

### 4. system_status
Real-time system status for gateway health widget.

```sql
CREATE TABLE system_status (
    component_key TEXT PRIMARY KEY,         -- e.g., 'database', 'ner_engine', 'openai'
    component_name TEXT NOT NULL,           -- e.g., 'Database', 'NER Engine', 'OpenAI'
    status TEXT NOT NULL,                   -- 'healthy', 'degraded', 'offline', 'unknown'
    is_provider BOOLEAN NOT NULL DEFAULT false,
    last_check TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    error_message TEXT,                     -- Optional error details

    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 5. live_metrics
Real-time metrics that update frequently.

```sql
CREATE TABLE live_metrics (
    metric_key TEXT PRIMARY KEY,            -- e.g., 'requests_per_second', 'active_connections'
    metric_value REAL NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Indexes

```sql
-- Widget lookups
CREATE INDEX idx_dashboard_widgets_enabled ON dashboard_widgets(enabled);
CREATE INDEX idx_dashboard_widgets_type ON dashboard_widgets(type);

-- Time-series lookups
CREATE INDEX idx_metrics_hourly_bucket ON metrics_hourly(hour_bucket DESC);
CREATE INDEX idx_metrics_daily_bucket ON metrics_daily(date_bucket DESC);

-- Status lookups
CREATE INDEX idx_system_status_provider ON system_status(is_provider);
CREATE INDEX idx_system_status_updated ON system_status(updated_at DESC);

-- Live metrics
CREATE INDEX idx_live_metrics_updated ON live_metrics(updated_at DESC);
```

## Initial Data

### Default Widget Configuration

```sql
INSERT INTO dashboard_widgets (id, type, title, endpoint, refresh_interval_seconds, grid_row, grid_col_span, grid_order, color_scheme, value_format) VALUES
-- Row 1: Stat cards
('stat-total-requests', 'stat_card', 'Total Requests', '/api/v1/dashboard/widgets/stat-total-requests', 30, 1, 3, 0, 'blue', 'number'),
('stat-pii-protected', 'stat_card', 'PII Protected', '/api/v1/dashboard/widgets/stat-pii-protected', 30, 1, 3, 1, 'green', 'number'),
('stat-blocked-requests', 'stat_card', 'Blocked Requests', '/api/v1/dashboard/widgets/stat-blocked-requests', 30, 1, 3, 2, 'red', 'number'),
('stat-avg-latency', 'stat_card', 'Avg Latency', '/api/v1/dashboard/widgets/stat-avg-latency', 30, 1, 3, 3, 'purple', 'duration_ms'),

-- Row 2: Medium widgets
('success-rate', 'success_rate', 'Success Rate', '/api/v1/dashboard/widgets/success-rate', 60, 2, 4, 0, 'green', 'percent'),
('protection-summary', 'protection_summary', 'Today''s Protection', '/api/v1/dashboard/widgets/protection-summary', 60, 2, 4, 1, 'blue', 'number'),
('gateway-status', 'gateway_status', 'Gateway Status', '/api/v1/dashboard/widgets/gateway-status', 15, 2, 4, 2, 'blue', 'number'),

-- Row 3: Large widgets
('top-providers', 'top_providers', 'Top Providers', '/api/v1/dashboard/widgets/top-providers', 60, 3, 4, 0, 'blue', 'number'),
('recent-activity', 'activity_chart', 'Recent Activity', '/api/v1/dashboard/widgets/recent-activity', 300, 3, 4, 1, 'blue', 'number'),
('live-stats', 'live_stats', 'Live Stats', '/api/v1/dashboard/widgets/live-stats', 5, 3, 4, 2, 'blue', 'number');
```

### Default System Components

```sql
INSERT INTO system_status (component_key, component_name, status, is_provider) VALUES
-- Internal components
('database', 'Database', 'healthy', false),
('ner_engine', 'NER Engine', 'healthy', false),
('router', 'Router', 'healthy', false),
('audit', 'Audit System', 'healthy', false),

-- Provider components
('openai', 'OpenAI', 'healthy', true),
('anthropic', 'Anthropic', 'healthy', true),
('google', 'Google', 'healthy', true),
('local', 'Local LLM', 'offline', true);
```

## Migration Strategy

1. **Create new tables** - Add tables incrementally to avoid downtime
2. **Populate initial data** - Insert default widget configurations
3. **Backfill metrics** - Generate historical hourly/daily metrics from existing audit logs
4. **Add triggers** - Create update triggers for timestamp fields
5. **Data retention** - Implement cleanup for old metrics (keep 30 days hourly, 365 days daily)

## Data Aggregation Jobs

### Hourly Aggregation
Run every hour to compute metrics_hourly from raw audit logs:
```sql
INSERT OR REPLACE INTO metrics_hourly (hour_bucket, total_requests, successful_requests, ...)
SELECT
    strftime('%Y-%m-%dT%H:00:00Z', created_at) as hour_bucket,
    COUNT(*) as total_requests,
    SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as successful_requests,
    -- ... other aggregations
FROM audit_logs
WHERE created_at >= datetime('now', '-2 hours')
GROUP BY hour_bucket;
```

### Daily Aggregation
Run daily to compute metrics_daily from metrics_hourly:
```sql
INSERT OR REPLACE INTO metrics_daily (date_bucket, total_requests, ...)
SELECT
    date(hour_bucket) as date_bucket,
    SUM(total_requests) as total_requests,
    -- ... other aggregations
FROM metrics_hourly
WHERE date(hour_bucket) = date('now', '-1 day')
GROUP BY date_bucket;
```

## Performance Considerations

- **Partitioning**: Consider table partitioning for metrics tables when data grows large
- **Archival**: Archive old metrics to separate tables/files
- **Caching**: Cache frequently accessed widget data in memory with TTL
- **Concurrent reads**: SQLite WAL mode for better read concurrency
- **Connection pooling**: Implement proper connection pooling in Go application