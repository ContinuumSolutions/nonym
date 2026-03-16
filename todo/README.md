# Dashboard API Implementation TODO

This directory contains the design and implementation plan for the new dashboard APIs that will serve the dynamic frontend widgets.

## Overview

The frontend has been redesigned to be fully dynamic, driven by backend configuration. Instead of hardcoded widgets, the backend now controls:
- Which widgets are displayed
- Widget ordering and layout
- Refresh intervals
- Visual styling (colors, icons, formatting)

## Implementation Files

1. **`api-specification.md`** - Complete OpenAPI/REST specification for all endpoints
2. **`database-schema.md`** - Required database schema changes and new tables
3. **`implementation-plan.md`** - Step-by-step implementation roadmap
4. **`widget-data-sources.md`** - How to compute each widget's data from existing systems
5. **`example-responses.md`** - Full example JSON responses for testing

## Key Architecture Decisions

### Dynamic Widget System
- `/api/v1/dashboard/layout` returns widget configurations
- `/api/v1/dashboard/widgets/:widget_id` serves individual widget data
- Backend completely controls what widgets appear and how they're styled

### Data Aggregation Strategy
- Leverage existing audit logs and metrics
- Pre-compute expensive aggregations (hourly/daily rollups)
- Cache frequently accessed data with appropriate TTLs

### Performance Considerations
- Different refresh intervals per widget (5s to 5min)
- Efficient database queries with proper indexing
- Optional data caching layer for high-frequency widgets

## Next Steps

1. Review and approve the API specification
2. Implement database schema changes
3. Create widget data computation logic
4. Implement REST endpoints in pkg/audit/dashboard.go
5. Add widget configuration management
6. Performance testing and optimization