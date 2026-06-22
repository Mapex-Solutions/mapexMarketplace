---
title: Assets Service
description: Asset and template management, EVA fields, MQTT auth callout, tiered caching, and fanout invalidation in MapexOS (v1.0.0).
---

# Assets Service

The **Assets Service** is the source of truth for Assets and Asset Templates in MapexOS. It manages CRUD operations, EVA (Entity-Value-Attribute) field configuration, template scripting, MQTT device authentication, and a multi-level read-model cache that distributes asset and template data across the platform.

> **Applies to v1.0.0** -- Go service, port 5002. MongoDB for authoritative data, Redis for application cache, MinIO/S3 for L2 read models, NATS Core + Leaf for messaging and auth callout.

---

## Role in the pipeline

The Assets Service provides the configuration and metadata that the event-processing pipeline depends on. Templates define how raw payloads are decoded, validated, and transformed; assets represent the physical or logical entities that produce events.

```txt
Assets Service
   |
   ├── Template scripts (decode/validate/transform) ──> JS Execution Service
   ├── Asset metadata ──> Router, Workflow engine, Events
   ├── MQTT Auth Callout ──> NATS (device authentication)
   └── L2 Read Models ──> MinIO/S3 ──> all consumers
```

The Assets Service does not sit in the hot path of event processing. Instead, it provides the reference data that pipeline services cache and consume.

---

## Responsibilities

- **Asset CRUD**: create, read, update, and delete assets with full metadata (name, identifiers, classification, organization scope).
- **Asset Template CRUD**: manage templates that define the decode/validate/transform scripts and EVA field schemas.
- **EVA field configuration**: define dynamic schema fields (Entity-Value-Attribute) on templates, enabling typed custom fields on events without schema migrations.
- **Template scripting**: store and version the JavaScript scripts (decode, validate, transform) that the JS Execution Service runs.
- **MQTT device authentication**: respond to NATS Auth Callout requests (`$SYS.REQ.USER.AUTH`) to authenticate MQTT devices. Validates credentials against asset records and signs JWTs with Ed25519.
- **L2 read-model publishing**: on asset or template create/update, write optimized read-model copies to MinIO/S3 for distributed caching by downstream services.
- **Fanout invalidation**: publish invalidation events to `fanout.asset.invalidate` and `fanout.template.invalidate` so consumers with ephemeral FANOUT subscriptions can flush stale cache entries.
- **List name synchronization**: subscribe to `mapexos.lists.name_updated` to keep manufacturer, model, and category display names in templates consistent with Core's list definitions.
- **Internal fallback endpoints**: serve asset and template data via `/internal/assets/{uuid}` and `/internal/templates/{id}` for downstream services that experience an L2 cache miss.

---

## Non-responsibilities

The Assets Service does **not** handle:

| Concern | Owner |
|---------|-------|
| Script execution (decode/validate/transform) | [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution) |
| Event routing and fan-out | Router Service |
| Rule evaluation and automation | Workflow engine |
| Action dispatch | Triggers Service |
| Event persistence | Events Service |

---

## Data flow

### Asset/template create or update

```txt
Client  ──POST/PATCH /api/v1/assets──>  Assets Service
                                           |
                                      write to MongoDB (authoritative)
                                      write read model to MinIO/S3 (L2)
                                      publish fanout.asset.invalidate (NATS)
                                           |
Client  <──── 200/201 ─────────────  Assets Service

Downstream services (JS Execution, Router, etc.)
        |
   receive invalidation event
   flush local cache (L0/L1)
   on next access: fetch from L2 (MinIO) or fallback to HTTP
```

### MQTT device authentication

```txt
MQTT Device  ──CONNECT──>  NATS (with credentials)
                              |
                         $SYS.REQ.USER.AUTH (Auth Callout)
                              |
                         Assets Service
                              |
                         validate credentials against asset record
                         sign JWT with Ed25519
                              |
NATS  <──── signed JWT ──  Assets Service
        |
MQTT Device  <──── CONNACK ──  NATS
```

### L2 cache miss (internal fallback)

```txt
JS Execution Service
        |
   L0 miss → L1 miss → L2 miss (MinIO)
        |
   GET /internal/templates/{id} ──>  Assets Service
                                       |
                                  load from MongoDB
                                  write to MinIO/S3 (repopulate L2)
                                       |
JS Execution Service  <──── template data ──  Assets Service
```

---

## EVA (Entity-Value-Attribute) fields

EVA fields allow templates to define a dynamic schema without requiring database migrations. Each template declares a set of typed fields that events flowing through that template will carry.

| Concept | Description |
|---------|-------------|
| **Entity** | The asset or template the field belongs to |
| **Value** | The runtime value extracted during the transform stage |
| **Attribute** | The field definition (name, type, unit, constraints) |

EVA fields are defined at the template level and applied during the transform script stage. They appear as typed columns in ClickHouse for analytical queries.

---

## TieredCache architecture

The Assets Service maintains a 4-level cache hierarchy for asset and template data. This cache is consumed by the Assets Service itself and by downstream services.

```txt
L0 (RAM) → L1 (Disk) → L2 (MinIO/S3) → Fallback (HTTP)
```

| Level | Storage | TTL | Capacity | Latency |
|-------|---------|-----|----------|---------|
| **L0** | In-memory (RAM) | 5 minutes | 256 MB | ~microseconds |
| **L1** | Local disk (NVMe/SSD) | 1 hour | 10 GB | ~milliseconds |
| **L2** | MinIO/S3 (object storage) | No TTL | Unbounded | ~10-100 ms |
| **Fallback** | HTTP to Assets Service | N/A | N/A | ~50-200 ms |

### Cache invalidation flow

When an asset or template is created or updated:

1. The Assets Service writes to MongoDB (source of truth).
2. An optimized read model is written to MinIO/S3 (L2).
3. An invalidation event is published to `fanout.asset.invalidate` or `fanout.template.invalidate`.
4. All consumers with ephemeral FANOUT subscriptions flush their L0 and L1 entries for the affected resource.
5. On next access, consumers repopulate L0/L1 from L2 (or fallback to HTTP if L2 is also stale).

---

## API surface

### Assets (platform auth required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/assets` | List assets (paginated, filtered) |
| GET | `/api/v1/assets/:id` | Get a single asset |
| POST | `/api/v1/assets` | Create an asset |
| PATCH | `/api/v1/assets/:id` | Update an asset |
| DELETE | `/api/v1/assets/:id` | Delete an asset |

### Asset Templates (platform auth required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/asset_templates` | List templates (paginated, filtered) |
| GET | `/api/v1/asset_templates/:id` | Get a single template |
| GET | `/api/v1/asset_templates/:id/available-fields` | Get available EVA fields for a template |
| POST | `/api/v1/asset_templates` | Create a template |
| PATCH | `/api/v1/asset_templates/:id` | Update a template |
| DELETE | `/api/v1/asset_templates/:id` | Delete a template |

### Internal endpoints (service-to-service)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/internal/assets/:uuid` | Fetch asset data (repopulates L2 on cache miss) |
| GET | `/internal/templates/:id` | Fetch template data (repopulates L2 on cache miss) |

---

## Infrastructure

| Component | Role | Details |
|-----------|------|---------|
| **MongoDB** | Authoritative store | Assets, templates, EVA field definitions, scripts |
| **Redis** | Application cache | App pool and shared pool for hot lookups |
| **MinIO/S3** | L2 read models | Optimized copies of assets and templates for distributed caching |
| **NATS Core** | Messaging | Fanout invalidation (`fanout.asset.invalidate`, `fanout.template.invalidate`), list name sync (`mapexos.lists.name_updated`) |
| **NATS Leaf** | Auth callout | `$SYS.REQ.USER.AUTH` for MQTT device authentication |

---

## Caching

See [TieredCache architecture](#tieredcache-architecture) above for the full 4-level cache design.

Key operational notes:

- **L0 and L1 are local** to each service replica. They are not shared across instances.
- **L2 is shared** via MinIO/S3. All replicas read from and write to the same bucket.
- **Fanout invalidation** ensures L0/L1 entries are flushed within seconds of a change. The invalidation uses NATS ephemeral consumers, so no durable subscription management is required.
- **Fallback HTTP** is the last resort. If L2 is unavailable, the Assets Service serves the data directly from MongoDB and repopulates L2.

---

## Retry and fault tolerance

- **MongoDB writes**: transient failures are retried with backoff at the repository layer.
- **MinIO/S3 writes**: L2 write failures are logged but do not block the API response. The read model will be repopulated on the next cache miss via the fallback HTTP endpoint.
- **NATS fanout**: invalidation publish failures are logged. Downstream caches will eventually expire via L0/L1 TTLs (5 minutes and 1 hour respectively), providing a bounded staleness window.
- **Auth Callout**: if the Assets Service cannot respond to `$SYS.REQ.USER.AUTH` within the NATS timeout, the MQTT connection is rejected. The device retries per its MQTT reconnect policy.

---

## Observability

> MapexOS ships with **Prometheus** (15-day retention, 15s scrape interval) and **Grafana** pre-provisioned in the default Docker Compose deployment. Each service exposes a `/metrics` endpoint with a dedicated Grafana dashboard. No additional configuration is required to start monitoring.

### Health check

| Endpoint | Critical checks | Non-critical checks |
|----------|-----------------|---------------------|
| `GET /health` | MongoDB, Redis app pool, Redis shared pool, NATS Core | NATS Leaf, MinIO |

Critical check failures mark the service as unhealthy. Non-critical check failures are reported but do not affect the overall health status. This distinction allows the service to remain operational when MinIO or NATS Leaf are temporarily unavailable.

### Key metrics

The Assets Service exposes metrics at `/metrics`, covering:

| Category | Examples |
|----------|----------|
| **Asset operations** | CRUD counters and latency per operation |
| **Template operations** | CRUD counters and latency per operation |
| **Auth Callout** | MQTT auth requests, success/failure rate, response latency |
| **Cache performance** | Hits/misses per cache level (L0, L1, L2, fallback) |
| **EVA field mapping** | Field resolution counters, mapping errors |
| **Fanout** | Invalidation events published, consumer acknowledgments |

### Grafana dashboard

The pre-provisioned dashboard is `assets-overview.json`. It provides panels for asset and template CRUD throughput, MQTT auth callout activity, cache hit ratios across all four levels, and infrastructure health.

---

## Scaling guidelines

The Assets Service workload is a mix of CRUD operations, cache management, and auth callout responses:

- **High CRUD throughput**: add Assets Service replicas behind a load balancer. MongoDB write scaling may require replica set tuning for write-heavy migrations or bulk imports.
- **High MQTT connection rate**: the auth callout handler is CPU-bound (Ed25519 signing). Add replicas to increase auth callout capacity.
- **Cache pressure**: if L0/L1 miss rates are high, check whether the 256 MB L0 limit and 10 GB L1 limit are appropriate for your asset count. Increase limits or add replicas.
- **L2 (MinIO/S3) latency**: if L2 reads are slow, ensure MinIO is deployed with sufficient IOPS. For S3, check network latency to the bucket region.
- **Fanout lag**: if invalidation events are delayed, check NATS cluster health and consumer count. Ephemeral consumers are lightweight, but a large number of subscribers increases fan-out time.

For most deployments, a single Assets Service instance handles the CRUD and caching workload. Scale horizontally when MQTT device counts or concurrent API clients exceed single-instance capacity.

---

## Next steps

- [MapexOS Core](/docs/1.0.0/en/architecture/mapexos-core)
- [HTTP Gateway](/docs/1.0.0/en/architecture/http-gateway)
- [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution)
- [Assets & Templates](/docs/1.0.0/en/core-platform/assets-and-templates)
