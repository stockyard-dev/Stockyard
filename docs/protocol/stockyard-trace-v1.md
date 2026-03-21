# Stockyard Trace Protocol v1

## Overview

The Stockyard Trace Protocol defines a standard JSON schema for recording LLM request traces. It enables interoperability between Stockyard instances and third-party observability tools.

## Trace Object

Every LLM request produces a trace with these fields:

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique trace identifier (UUID or prefixed ID) |
| `service` | string | Service name (default: "proxy") |
| `operation` | string | Operation type (e.g., "chat_completion") |
| `status` | string | "ok" or "error" |
| `created_at` | string | RFC 3339 timestamp |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `request_id` | string | Client-provided request correlation ID |
| `parent_id` | string | Parent trace ID for nested/chained calls |
| `provider` | string | LLM provider name (openai, anthropic, etc.) |
| `model` | string | Model identifier (gpt-4o, claude-sonnet-4-20250514, etc.) |
| `duration_ms` | integer | Request duration in milliseconds |
| `tokens_in` | integer | Input token count |
| `tokens_out` | integer | Output token count |
| `cost_usd` | number | Estimated cost in USD |
| `metadata_json` | object | Arbitrary key-value metadata |

### Extension Mechanism

The `metadata_json` field supports arbitrary extensions. Reserved prefixes:
- `_smart_route_*` — Smart routing metadata
- `_firewall_*` — Firewall scan results
- `_memory_*` — Memory injection metadata
- `_ab_*` — A/B testing metadata
- `_cache_*` — Cache hit/miss metadata

## Versioning

The protocol version is `v1`. Future versions will maintain backward compatibility for required fields. New optional fields may be added without a version bump.

## Example

```json
{
  "id": "tr_abc123def456",
  "request_id": "req-789",
  "service": "proxy",
  "operation": "chat_completion",
  "provider": "openai",
  "model": "gpt-4o",
  "status": "ok",
  "duration_ms": 432,
  "tokens_in": 150,
  "tokens_out": 523,
  "cost_usd": 0.0089,
  "metadata_json": {
    "_smart_route_rule": "short-to-mini",
    "_cache_hit": false
  },
  "created_at": "2026-03-21T14:23:01Z"
}
```
