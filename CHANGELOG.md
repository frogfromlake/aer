# Changelog

All notable breaking changes and significant releases are documented here.

## [2.0.0] — Phase 47: BFF API Consistency & Input Validation

### Breaking Changes

- **`GET /api/v1/metrics`** — `startDate` and `endDate` are now **required** query parameters.
  Previously, omitting both parameters silently defaulted to the last 24 hours. Clients that
  relied on the implicit fallback will now receive `400 Bad Request`.

- **`GET /api/v1/metrics/available`** — `startDate` and `endDate` are now **required** query
  parameters. Previously, this endpoint returned all distinct metric names from all time.
  It now returns only metric names that have data within the specified time range.

### Improvements

- **`GET /api/v1/entities`** and **`GET /api/v1/languages`** — the `limit` parameter is now
  validated in the handler layer. Values outside `[1, 1000]` return `400 Bad Request` instead
  of being silently clamped to `100`. The storage layer no longer contains business-logic
  validation for this parameter.

- **All data endpoints** now have consistent, uniform date parameter semantics: both `startDate`
  and `endDate` are always required. There are no implicit defaults or silent fallbacks.

### Migration

```
# Before (Phase 47)
GET /api/v1/metrics                           # returned last 24h — now returns 400
GET /api/v1/metrics?startDate=2025-01-01Z     # returned last 24h as end — now returns 400
GET /api/v1/metrics/available                 # returned all-time metrics — now returns 400

# After (Phase 47)
GET /api/v1/metrics?startDate=2025-01-01T00:00:00Z&endDate=2025-01-02T00:00:00Z
GET /api/v1/metrics/available?startDate=2025-01-01T00:00:00Z&endDate=2025-01-02T00:00:00Z
```
