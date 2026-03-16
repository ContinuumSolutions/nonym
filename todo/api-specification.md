# Dashboard API Specification

## Base URL
All dashboard APIs are served under `/api/v1/dashboard/`

## Authentication
Dashboard APIs use the same API key authentication as existing proxy routes:
- Header: `X-API-Key: <api_key>`
- Same validation logic as pkg/interceptor auth

## Core Endpoints

### GET /api/v1/dashboard/layout
Returns the complete widget configuration for the dashboard.

**Response:** `DashboardLayoutResponse`
```json
{
  "widgets": [
    {
      "id": "stat-total-requests",
      "type": "stat_card",
      "title": "Total Requests",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/stat-total-requests",
      "refresh_interval_seconds": 30,
      "grid": {
        "row": 1,
        "col_span": 3,
        "order": 0
      },
      "display": {
        "color_scheme": "blue",
        "icon": "M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z",
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    }
  ]
}
```

### GET /api/v1/dashboard/widgets/:widget_id
Returns data for a specific widget. Response shape varies by widget type.

**Path Parameters:**
- `widget_id`: Widget identifier from layout response

**Response varies by widget type:**

#### stat_card widgets
```json
{
  "value": 89234,
  "unit": null,
  "delta": 1243,
  "delta_period": "vs last 24h"
}
```

#### success_rate widget
```json
{
  "success_rate": 97.9,
  "redacted_requests": 45535,
  "total_requests": 89234
}
```

#### protection_summary widget
```json
{
  "rows": [
    {
      "key": "protected",
      "label": "Protected",
      "value": 47382,
      "color": "green"
    },
    {
      "key": "blocked",
      "label": "Blocked",
      "value": 1847,
      "color": "red"
    },
    {
      "key": "detection_rate",
      "label": "Detection Rate",
      "value": "53.1%",
      "color": "accent"
    }
  ]
}
```

#### gateway_status widget
```json
{
  "status": "healthy",
  "uptime": 864000,
  "components": [
    {
      "key": "database",
      "name": "Database",
      "status": "healthy",
      "provider": false
    },
    {
      "key": "ner_engine",
      "name": "NER Engine",
      "status": "healthy",
      "provider": false
    },
    {
      "key": "openai",
      "name": "OpenAI",
      "status": "healthy",
      "provider": true
    }
  ],
  "memory_usage": {
    "heap": 128,
    "stack": 32,
    "total": 256
  }
}
```

#### top_providers widget
```json
{
  "providers": [
    {
      "provider": "openai",
      "label": "OpenAI",
      "requests": 52140,
      "percent": 58.4,
      "color": "#10A37F"
    },
    {
      "provider": "anthropic",
      "label": "Anthropic",
      "requests": 24831,
      "percent": 27.8,
      "color": "#C7A97B"
    }
  ]
}
```

#### activity_chart widget
```json
{
  "period_label": "Last 24 hours",
  "points": [
    {
      "timestamp": "2026-03-15T14:00:00Z",
      "count": 87
    },
    {
      "timestamp": "2026-03-15T15:00:00Z",
      "count": 134
    }
  ]
}
```

#### live_stats widget
```json
{
  "rows": [
    {
      "key": "requests_per_second",
      "label": "Req / sec",
      "value": 4.7,
      "format": "rate_per_sec"
    },
    {
      "key": "active_connections",
      "label": "Active Connections",
      "value": 23,
      "format": "number"
    },
    {
      "key": "error_rate",
      "label": "Error Rate",
      "value": 0.021,
      "format": "percent",
      "color_rule": {
        "threshold_warning": 0.03,
        "threshold_critical": 0.05,
        "direction": "higher_is_worse"
      }
    }
  ]
}
```

## Error Responses

### 401 Unauthorized
```json
{
  "error": "Invalid or missing API key"
}
```

### 404 Not Found
```json
{
  "error": "Widget not found",
  "widget_id": "invalid-widget-id"
}
```

### 500 Internal Server Error
```json
{
  "error": "Internal server error",
  "details": "Database connection failed"
}
```

## Implementation Notes

### Response Headers
```
Content-Type: application/json
Cache-Control: no-cache, no-store, must-revalidate
X-Content-Type-Options: nosniff
```

### Rate Limiting
- Per API key: 1000 requests/hour for layout endpoint
- Per API key: 10000 requests/hour for widget data endpoints
- Use existing rate limiting infrastructure

### Data Freshness
- Widget data should reflect real-time or near-real-time metrics
- Maximum staleness: 1 minute for cached aggregations
- Live stats should be computed on-demand

### Time Zones
- All timestamps in UTC (ISO 8601 format)
- Frontend handles local time zone display

### Widget Configuration Management
- Widget configurations should be stored in database
- Support for enabling/disabling widgets
- Admin endpoint (future): PUT /api/v1/dashboard/layout for configuration