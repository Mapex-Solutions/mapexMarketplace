---
title: Router Service
description: Event routing layer — asset resolution, RouteGroup matching, fan-out to downstream services, and TieredCache strategy in MapexOS (v1.0.0).
---

# Router Service

The **Router Service** is the event routing layer of MapexOS. It consumes asset events from NATS JetStream, resolves asset context via a multi-level TieredCache, evaluates RouteGroup match rules, and publishes routed events to downstream services.

> **Applies to v1.0.0** — Go, port 5003, NATS JetStream consumers, TieredCache (L0/L1/L2), RouteGroup match policies, 5-target fan-out, batch processing, and Prometheus observability.

---

## Role in the pipeline

The Router sits between event ingestion and downstream processing. After the HTTP Gateway ingests and the JS Execution Engine decodes/validates/transforms events, the Router decides **where** each event goes.

```txt
HTTP Gateway → JS Execution → Router → Workflow engine
                                     → Triggers
                                     → Events (persistence)
                                     → Lake House (analytics)
                                     → Notifications
```

The Router does not modify event payloads. It reads the asset context, evaluates routing rules, and fans out the event to the correct downstream subjects.

---

## Responsibilities

- Consume asset events from NATS JetStream
- Resolve full asset context via TieredCache
- Load and cache RouteGroup configurations
- Evaluate match rules against event + asset data
- Fan-out routed events to downstream NATS subjects
- Emit routing history for audit and UI visibility
- Invalidate caches on asset changes
- Expose REST API for RouteGroup CRUD operations

---

## Non-responsibilities

The Router does **not**:

- **Ingest events from external sources** — that is the [HTTP Gateway](/docs/1.0.0/en/architecture/http-gateway)
- **Execute scripts** — that is the [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution)
- **Evaluate workflows** — that is the Workflow engine
- **Dispatch outbound actions** — that is the [Triggers Service](/docs/1.0.0/en/architecture/triggers)

---

## Data flow

### Consume

The Router consumes from a single NATS JetStream subject:

| Subject | Stream | Consumer type |
|---------|--------|---------------|
| `route.execute` | `ROUTE-GROUPS` | WorkQueue (durable) |

### Processing steps

```txt
1. Receive batch of messages from NATS
2. Resolve asset context via TieredCache
3. Load RouteGroups for the asset's organization
4. Evaluate match rules per RouteGroup
5. Fan-out matching events to downstream subjects
6. Emit routing history to audit subject
7. ACK processed messages
```

### Publish

The Router fans out to five downstream subjects:

| Subject | Purpose |
|---------|---------|
| `events.save` | Persist event to operational storage |
| `events.lake_house` | Persist event to analytical storage (ClickHouse) |
| `events.notification` | Trigger notification delivery |
| `workflow.{id}.execute` | Evaluate workflows for the event |
| `trigger.{id}.execute` | Execute trigger actions for the event |

Additionally, every routed event emits a history record:

| Subject | Purpose |
|---------|---------|
| `events.router` | Audit trail for routing decisions (visible in UI) |

---

## Asset resolution — TieredCache

Before evaluating routing rules, the Router must resolve the full asset context. MapexOS uses a **TieredCache** to minimize latency:

```txt
L0 (In-memory) → L1 (In-memory, longer TTL) → L2 (MinIO/S3) → HTTP fallback
```

### Cache levels

| Level | Storage | TTL | Description |
|-------|---------|-----|-------------|
| **L0** | In-memory | 5 min | Hot assets, fastest access |
| **L1** | In-memory | 1 hour | Warm assets, reduces downstream calls |
| **L2** | MinIO/S3 | Persistent | Shared across replicas |
| **Fallback** | HTTP endpoint | — | Calls Assets service (`ASSETS_URL`) |

### Cache invalidation

The Router subscribes to a FANOUT consumer on `fanout.asset.invalidate`. When an asset is created, updated, or deleted, the Assets service publishes an invalidation event. All Router replicas evict the affected asset from L0 and L1.

---

## RouteGroups

A **RouteGroup** is a configuration object that defines which events should be routed and where they should go. RouteGroups are scoped to an organization.

### Loading and caching

RouteGroups are loaded from MongoDB and cached in Redis:

| Source | TTL | Description |
|--------|-----|-------------|
| **Redis** | 60 min | Primary cache for RouteGroup configs |
| **MongoDB** | — | Source of truth, fallback on cache miss |

### Match rules

Each RouteGroup contains match rules that are evaluated against the event and asset context. Rules use a **policy** to determine how multiple conditions combine:

| Policy | Behavior |
|--------|----------|
| `all` | Every condition must match (AND logic) |
| `any` | At least one condition must match (OR logic) |

### Match operators

| Operator | Description |
|----------|-------------|
| `eq` | Equal |
| `neq` | Not equal |
| `gt` | Greater than |
| `gte` | Greater than or equal |
| `lt` | Less than |
| `lte` | Less than or equal |
| `in` | Value is in list |
| `nin` | Value is not in list |

### Example RouteGroup

```json
{
  "name": "Temperature alerts",
  "policy": "all",
  "rules": [
    { "field": "event.temperature", "operator": "gt", "value": 80 },
    { "field": "asset.type", "operator": "eq", "value": "sensor" }
  ],
  "targets": ["workflow", "events.save", "events.notification"]
}
```

This RouteGroup matches events where the temperature exceeds 80 **and** the asset type is `sensor`, then fans out to the Workflow engine, event persistence, and notifications.

---

## Batch processing

The Router processes messages in batches for throughput:

| Parameter | Default | Environment variable |
|-----------|---------|---------------------|
| **Batch size** | 8000 | `NATS_BATCH_SIZE` |

Each batch is fetched from the WorkQueue consumer, processed in memory (resolve, match, fan-out), and acknowledged. Batch size is configurable per deployment based on available memory and latency targets.

---

## Infrastructure

| Component | Role | Details |
|-----------|------|---------|
| **NATS JetStream** | Event bus | Consume `route.execute`, publish to 5+ subjects |
| **MongoDB** | Config store | RouteGroup definitions (source of truth) |
| **Redis** | Cache | RouteGroup cache (60 min TTL), counter cache (6 h TTL), shared cache for coverage/permissions |
| **MinIO/S3** | Object store | TieredCache L2 for asset context |

---

## Retry and fault tolerance

### Message retry

| Parameter | Value |
|-----------|-------|
| **Max attempts** | 5 |
| **Backoff intervals** | 1 s, 5 s, 30 s, 2 min, 10 min |

Messages that exceed the retry limit are moved to the dead-letter queue.

### Invalid payloads

Messages with malformed or unparseable payloads are **rejected immediately** without retry. These are not transient failures — retrying would produce the same result.

### Failure modes

| Failure | Behavior |
|---------|----------|
| Redis unavailable | Fall back to MongoDB for RouteGroups |
| TieredCache miss (all levels) | Call Assets service via HTTP |
| NATS publish failure | Retry with backoff |
| MongoDB unavailable | Cached RouteGroups continue to serve; new configs unavailable |

---

## API

### External API (JWT auth)

RouteGroup management for tenants:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/route_groups` | List RouteGroups |
| `POST` | `/api/v1/route_groups` | Create RouteGroup |
| `PATCH` | `/api/v1/route_groups/:id` | Update RouteGroup |
| `DELETE` | `/api/v1/route_groups/:id` | Delete RouteGroup |

### Internal API (API key auth)

Service-to-service access:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/internal/v1/routegroups` | List RouteGroups (internal) |

---

## Observability

### Health check

```
GET /health
```

Checks connectivity to:

- MongoDB
- Redis (app cache)
- Redis (shared cache)
- NATS core
- MinIO (optional — degraded but functional without it)

### Metrics

```
GET /metrics
```

The Router exposes **40+ Prometheus metrics** covering:

| Category | Examples |
|----------|---------|
| **Event processing** | Events received, processed, failed, latency histograms |
| **Cache performance** | TieredCache hits/misses per level, RouteGroup cache hit ratio |
| **Match evaluations** | Rules evaluated, matched, skipped per RouteGroup |
| **NATS publish** | Publish count per downstream subject, publish latency |

### Dashboard

A pre-built Grafana dashboard is available:

```
router-overview.json
```

---

## Scaling guidelines

### Horizontal scaling

The Router is stateless (all state lives in Redis, MongoDB, and NATS). Scale replicas freely:

```txt
Router × N replicas
  └── Each replica: own TieredCache L0/L1
  └── Shared: Redis, MongoDB, MinIO, NATS
```

### When to scale

| Signal | Action |
|--------|--------|
| Consumer lag increasing | Add replicas |
| High L0/L1 cache miss rate | Increase memory per replica or adjust TTLs |
| Publish latency rising | Check NATS cluster health, add replicas |
| Batch processing time > SLA | Reduce `NATS_BATCH_SIZE` or add replicas |

### Tuning

- **Batch size**: larger batches improve throughput but increase per-batch latency. Start at 8000 and adjust based on p99 processing time.
- **Cache TTLs**: shorter TTLs reduce stale data risk but increase load on MongoDB and the Assets service. The defaults (L0: 5 min, L1: 1 h, RouteGroup: 60 min) are suitable for most deployments.
- **Redis connections**: ensure the Redis connection pool is sized for the number of Router replicas.

---

## Next steps

- [Architecture Overview](/docs/1.0.0/en/architecture/overview)
- Workflow engine
- [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution)
- [Events & Pipeline](/docs/1.0.0/en/core-platform/events-and-pipeline)
- [Assets & Templates](/docs/1.0.0/en/core-platform/assets-and-templates)
