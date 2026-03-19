# Example API Responses

Complete example JSON responses for all dashboard API endpoints, useful for testing and frontend development.

## Layout Endpoint

### GET /api/v1/dashboard/layout

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
    },
    {
      "id": "stat-pii-protected",
      "type": "stat_card",
      "title": "PII Protected",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/stat-pii-protected",
      "refresh_interval_seconds": 30,
      "grid": {
        "row": 1,
        "col_span": 3,
        "order": 1
      },
      "display": {
        "color_scheme": "green",
        "icon": "M12 1L3 5V11C3 16.55 6.84 21.74 12 23C17.16 21.74 21 16.55 21 11V5L12 1M12 7C13.4 7 14.8 8.6 14.8 10V11H15.5C16.4 11 17 11.4 17 12V16C17 16.6 16.6 17 16 17H8C7.4 17 7 16.6 7 16V12C7 11.4 7.4 11 8 11H8.5V10C8.5 8.6 9.9 7 12 7M12 8.2C10.2 8.2 9.8 9.2 9.8 10V11H14.2V10C14.2 9.2 13.8 8.2 12 8.2Z",
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    },
    {
      "id": "stat-blocked-requests",
      "type": "stat_card",
      "title": "Blocked Requests",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/stat-blocked-requests",
      "refresh_interval_seconds": 30,
      "grid": {
        "row": 1,
        "col_span": 3,
        "order": 2
      },
      "display": {
        "color_scheme": "red",
        "icon": "M12 2C13.1 2 14 2.9 14 4C14 5.1 13.1 6 12 6C10.9 6 10 5.1 10 4C10 2.9 10.9 2 12 2ZM21 9V7L15 1H5C3.89 1 3 1.89 3 3V21C3 22.11 3.89 23 5 23H11V21H5V3H14V8H19V9H21ZM12.5 10V12H10.5V10H12.5ZM12.5 13V15H10.5V13H12.5ZM12.5 16V18H10.5V16H12.5ZM20.5 13.5L19 12L20.5 10.5L22 12L20.5 13.5ZM16 12L17.5 10.5L19 12L17.5 13.5L16 12Z",
        "value_format": "number",
        "threshold_warning": 100,
        "threshold_critical": 500
      }
    },
    {
      "id": "stat-avg-latency",
      "type": "stat_card",
      "title": "Avg Latency",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/stat-avg-latency",
      "refresh_interval_seconds": 30,
      "grid": {
        "row": 1,
        "col_span": 3,
        "order": 3
      },
      "display": {
        "color_scheme": "purple",
        "icon": "M12 20A8 8 0 0 0 20 12A8 8 0 0 0 12 4A8 8 0 0 0 4 12A8 8 0 0 0 12 20M12 2A10 10 0 0 1 22 12A10 10 0 0 1 12 22C6.47 22 2 17.5 2 12A10 10 0 0 1 12 2M12.5 7V12.25L17 14.92L16.25 16.15L11 13V7H12.5Z",
        "value_format": "duration_ms",
        "threshold_warning": 200,
        "threshold_critical": 500
      }
    },
    {
      "id": "success-rate",
      "type": "success_rate",
      "title": "Success Rate",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/success-rate",
      "refresh_interval_seconds": 60,
      "grid": {
        "row": 2,
        "col_span": 4,
        "order": 0
      },
      "display": {
        "color_scheme": "green",
        "icon": null,
        "value_format": "percent",
        "threshold_warning": 95,
        "threshold_critical": 90
      }
    },
    {
      "id": "protection-summary",
      "type": "protection_summary",
      "title": "Today's Protection",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/protection-summary",
      "refresh_interval_seconds": 60,
      "grid": {
        "row": 2,
        "col_span": 4,
        "order": 1
      },
      "display": {
        "color_scheme": "blue",
        "icon": null,
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    },
    {
      "id": "gateway-status",
      "type": "gateway_status",
      "title": "Gateway Status",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/gateway-status",
      "refresh_interval_seconds": 15,
      "grid": {
        "row": 2,
        "col_span": 4,
        "order": 2
      },
      "display": {
        "color_scheme": "blue",
        "icon": null,
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    },
    {
      "id": "top-providers",
      "type": "top_providers",
      "title": "Top Providers",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/top-providers",
      "refresh_interval_seconds": 60,
      "grid": {
        "row": 3,
        "col_span": 4,
        "order": 0
      },
      "display": {
        "color_scheme": "blue",
        "icon": null,
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    },
    {
      "id": "recent-activity",
      "type": "activity_chart",
      "title": "Recent Activity",
      "subtitle": "Last 24 hours",
      "endpoint": "/api/v1/dashboard/widgets/recent-activity",
      "refresh_interval_seconds": 300,
      "grid": {
        "row": 3,
        "col_span": 4,
        "order": 1
      },
      "display": {
        "color_scheme": "blue",
        "icon": null,
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    },
    {
      "id": "live-stats",
      "type": "live_stats",
      "title": "Live Stats",
      "subtitle": null,
      "endpoint": "/api/v1/dashboard/widgets/live-stats",
      "refresh_interval_seconds": 5,
      "grid": {
        "row": 3,
        "col_span": 4,
        "order": 2
      },
      "display": {
        "color_scheme": "blue",
        "icon": null,
        "value_format": "number",
        "threshold_warning": null,
        "threshold_critical": null
      }
    }
  ]
}
```

## Widget Data Endpoints

### StatCard Widgets

#### GET /api/v1/dashboard/widgets/stat-total-requests
```json
{
  "value": 89234,
  "unit": null,
  "delta": 1243,
  "delta_period": "vs last 24h"
}
```

#### GET /api/v1/dashboard/widgets/stat-pii-protected
```json
{
  "value": 47382,
  "unit": null,
  "delta": 892,
  "delta_period": "vs last 24h"
}
```

#### GET /api/v1/dashboard/widgets/stat-blocked-requests
```json
{
  "value": 1847,
  "unit": null,
  "delta": -23,
  "delta_period": "vs last 24h"
}
```

#### GET /api/v1/dashboard/widgets/stat-avg-latency
```json
{
  "value": 142.7,
  "unit": "ms",
  "delta": -8.3,
  "delta_period": "vs last 24h"
}
```

### Success Rate Widget

#### GET /api/v1/dashboard/widgets/success-rate
```json
{
  "success_rate": 97.9,
  "redacted_requests": 45535,
  "total_requests": 89234
}
```

### Protection Summary Widget

#### GET /api/v1/dashboard/widgets/protection-summary
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
    },
    {
      "key": "entities_redacted",
      "label": "Entities Redacted",
      "value": 127438,
      "color": "purple"
    },
    {
      "key": "strict_mode_triggers",
      "label": "Strict Mode Triggers",
      "value": 234,
      "color": "orange"
    }
  ]
}
```

### Gateway Status Widget

#### GET /api/v1/dashboard/widgets/gateway-status
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
      "key": "router",
      "name": "Router",
      "status": "healthy",
      "provider": false
    },
    {
      "key": "audit_system",
      "name": "Audit System",
      "status": "healthy",
      "provider": false
    },
    {
      "key": "openai",
      "name": "OpenAI",
      "status": "healthy",
      "provider": true
    },
    {
      "key": "anthropic",
      "name": "Anthropic",
      "status": "healthy",
      "provider": true
    },
    {
      "key": "google",
      "name": "Google",
      "status": "degraded",
      "provider": true
    },
    {
      "key": "local",
      "name": "Local LLM",
      "status": "offline",
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

### Top Providers Widget

#### GET /api/v1/dashboard/widgets/top-providers
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
    },
    {
      "provider": "google",
      "label": "Google",
      "requests": 8901,
      "percent": 9.9,
      "color": "#4285F4"
    },
    {
      "provider": "local",
      "label": "Local LLM",
      "requests": 3362,
      "percent": 3.7,
      "color": "#BF5AF2"
    },
    {
      "provider": "custom",
      "label": "Custom Endpoint",
      "requests": 320,
      "percent": 0.4,
      "color": "#FF6B35"
    }
  ]
}
```

### Activity Chart Widget

#### GET /api/v1/dashboard/widgets/recent-activity
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
    },
    {
      "timestamp": "2026-03-15T16:00:00Z",
      "count": 156
    },
    {
      "timestamp": "2026-03-15T17:00:00Z",
      "count": 189
    },
    {
      "timestamp": "2026-03-15T18:00:00Z",
      "count": 234
    },
    {
      "timestamp": "2026-03-15T19:00:00Z",
      "count": 267
    },
    {
      "timestamp": "2026-03-15T20:00:00Z",
      "count": 198
    },
    {
      "timestamp": "2026-03-15T21:00:00Z",
      "count": 176
    },
    {
      "timestamp": "2026-03-15T22:00:00Z",
      "count": 143
    },
    {
      "timestamp": "2026-03-15T23:00:00Z",
      "count": 98
    },
    {
      "timestamp": "2026-03-16T00:00:00Z",
      "count": 67
    },
    {
      "timestamp": "2026-03-16T01:00:00Z",
      "count": 45
    },
    {
      "timestamp": "2026-03-16T02:00:00Z",
      "count": 32
    },
    {
      "timestamp": "2026-03-16T03:00:00Z",
      "count": 28
    },
    {
      "timestamp": "2026-03-16T04:00:00Z",
      "count": 34
    },
    {
      "timestamp": "2026-03-16T05:00:00Z",
      "count": 41
    },
    {
      "timestamp": "2026-03-16T06:00:00Z",
      "count": 58
    },
    {
      "timestamp": "2026-03-16T07:00:00Z",
      "count": 78
    },
    {
      "timestamp": "2026-03-16T08:00:00Z",
      "count": 112
    },
    {
      "timestamp": "2026-03-16T09:00:00Z",
      "count": 145
    },
    {
      "timestamp": "2026-03-16T10:00:00Z",
      "count": 167
    },
    {
      "timestamp": "2026-03-16T11:00:00Z",
      "count": 189
    },
    {
      "timestamp": "2026-03-16T12:00:00Z",
      "count": 201
    },
    {
      "timestamp": "2026-03-16T13:00:00Z",
      "count": 178
    }
  ]
}
```

### Live Stats Widget

#### GET /api/v1/dashboard/widgets/live-stats
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
      "key": "queue_depth",
      "label": "Queue Depth",
      "value": 5,
      "format": "number"
    },
    {
      "key": "cache_hit_rate",
      "label": "Cache Hit Rate",
      "value": 0.734,
      "format": "percent"
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
    },
    {
      "key": "avg_response_time",
      "label": "Avg Response Time",
      "value": 142,
      "format": "duration_ms",
      "color_rule": {
        "threshold_warning": 200,
        "threshold_critical": 500,
        "direction": "higher_is_worse"
      }
    },
    {
      "key": "memory_usage_percent",
      "label": "Memory Usage",
      "value": 0.68,
      "format": "percent",
      "color_rule": {
        "threshold_warning": 0.8,
        "threshold_critical": 0.9,
        "direction": "higher_is_worse"
      }
    },
    {
      "key": "uptime",
      "label": "Uptime",
      "value": 864000,
      "format": "duration_human"
    }
  ]
}
```

## Error Response Examples

### Widget Not Found (404)
```json
{
  "error": "Widget not found",
  "widget_id": "invalid-widget-id"
}
```

### Unauthorized (401)
```json
{
  "error": "Invalid or missing API key"
}
```

### Internal Server Error (500)
```json
{
  "error": "Internal server error",
  "details": "Database connection failed"
}
```

### Degraded Service (206 Partial Content)
When some widget data is unavailable but others succeed:
```json
{
  "warning": "Some data sources unavailable",
  "partial_data": true,
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
      "value": null,
      "format": "number",
      "error": "Data source unavailable"
    }
  ]
}
```

## Testing Data Sets

### High Load Scenario
For stress testing the dashboard with high numbers:
```json
{
  "value": 1250000,
  "delta": 45000,
  "delta_period": "vs last 24h"
}
```

### Error Scenario
For testing error conditions and thresholds:
```json
{
  "success_rate": 85.2,
  "redacted_requests": 45535,
  "total_requests": 89234
}
```

### Zero Data Scenario
For testing edge cases with no data:
```json
{
  "value": 0,
  "delta": 0,
  "delta_period": "vs last 24h"
}
```

### Large Provider List
For testing UI with many providers:
```json
{
  "providers": [
    {"provider": "openai", "label": "OpenAI", "requests": 52140, "percent": 45.2, "color": "#10A37F"},
    {"provider": "anthropic", "label": "Anthropic", "requests": 24831, "percent": 21.5, "color": "#C7A97B"},
    {"provider": "google", "label": "Google", "requests": 15200, "percent": 13.2, "color": "#4285F4"},
    {"provider": "local", "label": "Local LLM", "requests": 8900, "percent": 7.7, "color": "#BF5AF2"},
    {"provider": "azure", "label": "Azure OpenAI", "requests": 6700, "percent": 5.8, "color": "#0078D4"},
    {"provider": "cohere", "label": "Cohere", "requests": 4200, "percent": 3.6, "color": "#39594C"},
    {"provider": "huggingface", "label": "Hugging Face", "requests": 2100, "percent": 1.8, "color": "#FFD21E"},
    {"provider": "custom", "label": "Custom Endpoint", "requests": 1400, "percent": 1.2, "color": "#FF6B35"}
  ]
}
```