---
title: Events Service
description: NATS consumers, ClickHouse bulk inserts, EVA field resolution, batch processing, retention management, and observability for the Events Service in MapexOS (v1.0.0).
---

# Events Service

The **Events Service** is the hot event analytics layer of MapexOS. It consumes events from multiple NATS streams, persists them to ClickHouse with low-latency bulk inserts, and serves query APIs for event investigation and analytics.

> **Applies to v1.0.0** --- Go, port 5004, 7 NATS consumers, 7 ClickHouse tables, three-phase batch processing, EVA field storage, per-org retention policies.

---

## Role in the pipeline

The Events Service is a **terminal consumer** in the MapexOS pipeline. It does not produce events that feed into other processing stages. Instead, it collects execution results, logs, and processed events from every upstream service and writes them into ClickHouse for querying, debugging, and analytics.

```txt
HTTP Gateway ─────────────────────────────────────┐
JS Executor ──────────────────────────────────────┤
Router ───────────────────────────────────────────┤
Workflow engine ──────────────────────────────────────┤  NATS JetStream
Triggers ─────────────────────────────────────────┤
DLQ ──────────────────────────────────────────────┤
Processed Events ─────────────────────────────────┘
                          |
                          v
                   Events Service
                          |
                          v
                     ClickHouse
                   (7 event tables)
```

---

## Responsibilities

- Consume events from 7 NATS subjects in parallel
- Parse, validate, and map messages to ClickHouse row schemas
- Bulk-insert batches into ClickHouse with low latency
- Store dynamic event fields using the EVA (Entity-Value-Attribute) model
- Resolve EVA field IDs using a tiered template cache
- Manage per-organization, per-event-type retention policies
- Serve query APIs for event investigation across all 7 tables
- Serve detail endpoints with full EVA field expansion

## Non-responsibilities

| Concern | Owned by |
|---------|----------|
| Ingest events from external sources | [HTTP Gateway](/docs/1.0.0/en/architecture/http-gateway) |
| Execute decode/validate/transform scripts | [JS Execution](/docs/1.0.0/en/architecture/js-execution) |
| Evaluate workflows | Workflow engine |
| Dispatch outbound actions | [Triggers Service](/docs/1.0.0/en/architecture/triggers) |

The Events Service **only persists and queries**. It does not transform, route, or act on events.

---

## NATS consumers and ClickHouse tables

The service runs 7 independent consumers, each writing to a dedicated ClickHouse table:

| NATS Subject | ClickHouse Table | Purpose |
|-------------|-----------------|---------|
| `events.raw` | `eventsRaw` | Raw event payloads for debugging |
| `events.logs.jsexecutor` | `eventsJsExecutor` | JS Executor debug and error logs |
| `dlq.mapexos` | `eventsDLQ` | Dead-letter queue (failed events from any stage) |
| `events.router` | `eventsRouter` | Router execution history |
| `events.businessrule` | `eventsBusinessRule` | Rule evaluation logs |
| `events.trigger` | `eventsTrigger` | Trigger execution results |
| `events.save` | `events` | Processed/stored events (primary table) |

Each consumer is independent. A failure in one consumer does not affect the others.

---

## Three-phase batch processing

Every consumer follows the same three-phase processing model:

### Phase 1 --- Parallel parse (bounded worker pool)

- Fetch up to **5000 messages** from NATS in a single batch.
- Parse, validate, and map each message to its ClickHouse row schema using a bounded worker pool.
- Invalid messages are flagged for rejection.

### Phase 2 --- Single bulk insert

- All successfully parsed rows are inserted into ClickHouse in a **single bulk INSERT** statement.
- This minimizes ClickHouse write amplification and maximizes throughput.

### Phase 3 --- Per-message acknowledgment

| Outcome | Action |
|---------|--------|
| Parse succeeded + insert succeeded | **ACK** the message |
| Insert failed (ClickHouse error) | **NACK** the message (will be redelivered) |
| Parse/validation failed | **REJECT** the message to DLQ (`dlq.mapexos`) |

This design ensures that transient ClickHouse failures trigger redelivery, while permanently invalid messages are routed to the DLQ for investigation.

---

## EVA (Entity-Value-Attribute) storage

MapexOS stores dynamic event fields using the **EVA model**. Instead of storing field names as strings on every row, each field is assigned a numeric ID at the template level.

### How it works

1. **Template definition**: Each Asset Template defines EVA fields with a numeric ID, name, and type.
2. **Event storage**: When an event is written to ClickHouse, dynamic fields are stored as `(fieldId, value)` pairs rather than `(fieldName, value)`.
3. **Query expansion**: When a client queries event details, the Events Service resolves numeric IDs back to field names using the template cache.

### Benefits

| Benefit | Description |
|---------|-------------|
| **Compact storage** | Numeric IDs are smaller than repeated field name strings |
| **Fast queries** | ClickHouse operates on fixed-width numeric columns |
| **Schema-less** | New fields can be added to templates without altering the ClickHouse schema |

### Template cache (tiered)

EVA field resolution requires template metadata. The Events Service uses a tiered cache to avoid repeated lookups:

| Tier | Storage | Latency | Description |
|------|---------|---------|-------------|
| In-memory | Application RAM | Microseconds | Hot templates currently in use |
| Redis | Shared cache | Low milliseconds | Warm templates across replicas |
| MinIO/S3 | Object storage | 10-100ms | Cold templates, full template archive |
| MongoDB | Source of truth | Milliseconds | Fallback if not found in any cache tier |

---

## Retention management

The Events Service manages data retention per organization and per event type.

### Retention policies

- Each organization can define retention durations for each of the 7 event tables.
- When a new organization is created, default retention policies are auto-generated.
- Retention is enforced at the ClickHouse level using TTL expressions.

### Retention API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/retention` | List retention policies for the organization |
| `PUT` | `/api/v1/retention` | Update retention policies |
| `DELETE` | `/api/v1/retention` | Reset to default retention |

---

## Infrastructure

| Component | Role |
|-----------|------|
| **ClickHouse** | 7 event tables (primary analytical store) |
| **MongoDB** | Retention policy definitions |
| **Redis (app cache)** | Template cache for EVA resolution |
| **Redis (shared cache)** | Shared cache layer |
| **MinIO/S3** | L2 template cache (cold storage) |
| **NATS JetStream** | Inbound event streams (7 subjects) |

---

## Caching

The Events Service caches two categories of data:

### Template cache (EVA resolution)

Used during event detail queries to expand numeric field IDs to human-readable names. Follows the tiered model described in the EVA section (in-memory, Redis, MinIO/S3, MongoDB fallback).

### Retention cache

Retention policies are cached in Redis to avoid MongoDB reads on every retention check. Cache is invalidated on policy updates via the API.

---

## Observability

### Health check

**Endpoint**: `GET /health`

| Dependency | Required |
|------------|----------|
| MongoDB | Yes |
| Redis (app cache) | Yes |
| Redis (shared cache) | Yes |
| NATS (core) | Yes |
| ClickHouse | Optional (degraded mode) |
| MinIO/S3 | Optional (degraded mode) |

ClickHouse and MinIO are marked optional because the service can still accept and queue messages via NATS even if writes are temporarily unavailable. However, query APIs will return errors in degraded mode.

### Key metrics

**Endpoint**: `GET /metrics`

| Metric area | What it measures |
|-------------|-----------------|
| Event processing | Messages consumed, parsed, inserted, failed --- per consumer |
| ClickHouse insert latency | Bulk insert duration histogram per table |
| EVA field resolution | Cache hits/misses, resolution latency |
| Batch sizes | Actual batch sizes vs configured maximum (5000) |

### Dashboard

Pre-built Grafana dashboard: `events-overview.json`

---

## Query APIs

The Events Service exposes read endpoints for each event table, plus a detail endpoint for processed events.

### List queries

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/events/raw` | Raw event payloads |
| `GET` | `/api/v1/events/jsexec` | JS Executor logs |
| `GET` | `/api/v1/events/router` | Router execution history |
| `GET` | `/api/v1/events/businessrule` | Rule evaluation logs |
| `GET` | `/api/v1/events/trigger` | Trigger execution results |
| `GET` | `/api/v1/events/store` | Processed/stored events |

All list endpoints support filtering and pagination.

### Event detail

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/events/store/:eventTrackerId` | Full event detail with EVA field expansion |

The detail endpoint resolves numeric EVA field IDs to their template-defined names and types, returning a fully expanded event object.

---

## Scaling guidelines

The Events Service is **write-throughput-bound**. ClickHouse bulk inserts are the primary bottleneck, not CPU or memory.

- **Horizontal scaling**: Add replicas. Each consumer uses NATS WorkQueue delivery, so messages are distributed across instances automatically.
- **Batch tuning**: The default batch of 5000 messages is optimized for ClickHouse write efficiency. Smaller batches increase write frequency (higher overhead); larger batches increase memory usage and latency.
- **ClickHouse**: For very high event volumes, consider ClickHouse sharding and replication. The Events Service writes to a single endpoint; ClickHouse handles distribution internally.
- **Consumer independence**: The 7 consumers are independent. If one table receives disproportionate traffic, you can tune its batch size or worker pool separately.
- **Template cache**: Monitor EVA cache hit rates. Low hit rates for the in-memory tier indicate template churn or insufficient memory allocation.

```txt
Events Service x N replicas
  └── 7 independent consumers per replica
        └── Batch processor (up to 5000 msgs)
              └── ClickHouse bulk INSERT
```

---

## Next steps

- [Triggers Service](/docs/1.0.0/en/architecture/triggers) --- upstream service whose execution results are persisted here
- Workflow engine --- upstream service whose evaluation logs are persisted here
- [JS Execution](/docs/1.0.0/en/architecture/js-execution) --- upstream service whose debug logs are persisted here
- [Architecture Overview](/docs/1.0.0/en/architecture/overview) --- full platform context
