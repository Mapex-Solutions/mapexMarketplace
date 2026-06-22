---
title: HTTP Gateway
description: HTTP event ingestion, per-source authentication, security event reporting, and Data Source management in MapexOS (v1.0.0).
---

# HTTP Gateway

The **HTTP Gateway** is the HTTP entrypoint for external webhook event ingestion into MapexOS. It authenticates each inbound request according to its Data Source configuration, publishes accepted payloads to the processing pipeline, and reports authentication failures as security events.

> **Applies to v1.0.0** -- Go service, port 5001. MongoDB for Data Source configuration, Redis for cache-aside lookups, NATS JetStream for event publishing.

---

## Role in the pipeline

The HTTP Gateway is the first service an external event touches when entering MapexOS via HTTP:

```txt
External source (webhook / integration)
        |
   HTTP Gateway  ──  authenticate, publish
        |
   NATS JetStream
        |
   JS Execution Service (processor.js.execute)
        |
   Router → Rules → Triggers → Events
```

The Gateway's job ends once the payload is published to NATS. It does not parse, transform, or route events -- those responsibilities belong to downstream services.

---

## Responsibilities

- **Event ingestion**: accept HTTP POST requests from external systems (webhooks, integrations, partner APIs).
- **Per-source authentication**: validate each request against the authentication configuration of its Data Source.
- **Security event reporting**: on authentication failure, fire a security event to `events.raw` (fire-and-forget) for audit purposes, then return HTTP 401.
- **Event publishing**: on successful authentication, publish the raw payload to `processor.js.execute` via NATS JetStream for downstream processing.
- **Data Source management**: CRUD operations for Data Source configurations (authentication type, endpoint settings, metadata).
- **Data Source caching**: cache-aside pattern for Data Source lookups to reduce MongoDB load under high ingestion rates.

---

## Non-responsibilities

The HTTP Gateway does **not** handle:

| Concern | Owner |
|---------|-------|
| Script execution (decode/validate/transform) | [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution) |
| Event routing and fan-out | Router Service |
| Asset and template management | [Assets Service](/docs/1.0.0/en/architecture/assets) |
| Rule evaluation and automation | Workflow engine |
| Action dispatch | Triggers Service |
| Event persistence | Events Service |

---

## Data flow

### Successful ingestion

```txt
Client  ──POST /api/v1/events?ds=<dataSourceId>──>  Gateway
                                                       |
                                                  load Data Source config (cache or MongoDB)
                                                  authenticate request per DS auth type
                                                       |
                                                  ── auth OK ──
                                                       |
                                                  publish to processor.js.execute (NATS)
                                                       |
Client  <──── 202 Accepted ───────────────────────  Gateway
```

### Authentication failure

```txt
Client  ──POST /api/v1/events?ds=<dataSourceId>──>  Gateway
                                                       |
                                                  load Data Source config
                                                  authenticate request
                                                       |
                                                  ── auth FAILED ──
                                                       |
                                                  publish security event to events.raw (fire-and-forget)
                                                       |
Client  <──── 401 Unauthorized ───────────────────  Gateway
```

The security event published on failure contains the source IP, Data Source ID, auth type attempted, and failure reason. This enables security monitoring and anomaly detection downstream.

---

## Authentication types

Each Data Source is configured with one authentication method. The Gateway evaluates the configured method on every inbound request to that Data Source.

| Auth type | Mechanism | Details |
|-----------|-----------|---------|
| **JWT (HMAC)** | Shared-secret JWT validation | Validates signature using a pre-shared HMAC secret configured on the Data Source |
| **JWT (JWKS)** | Public-key JWT validation | Fetches and caches the JWKS key set from a configured URL, validates JWT signature |
| **OAuth2 (JWKS introspection)** | Token introspection via JWKS | Validates OAuth2 bearer tokens against a JWKS endpoint |
| **API Key** | Static key match | Checks for a matching key in a configured location: HTTP header, query parameter, or request body field |
| **IP Whitelist** | Source IP check | Validates the request's source IP against a configured allowlist |
| **None** | No authentication | Accepts all requests. Use only for trusted internal sources or development |

### Recommendations

- Use **API Key** for server-to-server integrations where simplicity is preferred.
- Use **JWT** or **OAuth2** for integrations that already issue tokens.
- Use **IP Whitelist** as an additional layer, not as the sole authentication control.
- Avoid **None** in production deployments.

---

## API surface

### Data Source management (platform auth required)

These endpoints require standard MapexOS platform authentication (JWT from MapexOS Core).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/data_sources` | List all Data Sources (paginated) |
| GET | `/api/v1/data_sources/:id` | Get a single Data Source |
| POST | `/api/v1/data_sources` | Create a new Data Source |
| PATCH | `/api/v1/data_sources/:id` | Update a Data Source |
| DELETE | `/api/v1/data_sources/:id` | Delete a Data Source |

### Event ingestion (per-Data Source auth)

This endpoint does **not** require platform authentication. Each request is authenticated according to the Data Source's own configuration.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/events?ds=<dataSourceId>` | Ingest a raw event for the specified Data Source |

The `ds` query parameter identifies which Data Source configuration to use for authentication and metadata attachment.

---

## Infrastructure

| Component | Role | Details |
|-----------|------|---------|
| **MongoDB** | Configuration store | Data Source documents (auth type, credentials, metadata) |
| **Redis** | Cache | Cache-aside for Data Source lookups; 6h TTL counter cache for rate tracking |
| **NATS JetStream** | Event bus | Publish to `processor.js.execute` (success) and `events.raw` (security events) |

---

## Caching

The Gateway uses a **cache-aside** pattern for Data Source lookups:

1. On inbound request, check Redis for the Data Source configuration.
2. On cache hit, use the cached config for authentication.
3. On cache miss, load from MongoDB, authenticate, and populate the cache.

This eliminates a MongoDB round-trip for the vast majority of requests under sustained traffic. The cache is invalidated when a Data Source is updated or deleted via the management API.

Additionally, a **counter cache** with a 6-hour TTL tracks per-Data Source request counts in Redis for rate monitoring without per-request database writes.

---

## Retry and fault tolerance

- **NATS JetStream publish**: if the NATS publish fails, the Gateway returns an error to the caller so the external system can retry. The Gateway does not buffer or retry internally -- this keeps the service stateless.
- **Fire-and-forget security events**: security event publishing to `events.raw` is best-effort. A failure to publish the security event does not affect the 401 response to the client.
- **Redis unavailability**: if Redis is down, the Gateway falls back to direct MongoDB lookups for every request. Throughput decreases but the service remains functional.
- **MongoDB unavailability**: if MongoDB is unreachable and the cache is also empty, the Gateway returns 503 (Service Unavailable) because it cannot load Data Source configurations.

---

## Observability

> MapexOS ships with **Prometheus** (15-day retention, 15s scrape interval) and **Grafana** pre-provisioned in the default Docker Compose deployment. Each service exposes a `/metrics` endpoint with a dedicated Grafana dashboard. No additional configuration is required to start monitoring.

### Health check

| Endpoint | Checks |
|----------|--------|
| `GET /health` | MongoDB connectivity, Redis app pool, Redis shared pool, NATS core connection |

Use this endpoint for container orchestration liveness and readiness probes.

### Key metrics

The Gateway exposes metrics at `/metrics`, covering:

| Category | Examples |
|----------|----------|
| **Authentication** | Attempts by auth type (success/failure), failure reasons |
| **Event processing** | Events received, events published, publish latency |
| **Payload size** | Payload size distribution (histogram) |
| **Data Source CRUD** | Operation latency per CRUD endpoint |
| **Cache** | Redis hit/miss ratio for Data Source lookups |

### Grafana dashboard

The pre-provisioned dashboard is `httpgw-overview.json`. It provides panels for ingestion throughput, authentication success/failure rates by type, payload size distribution, and Data Source cache performance.

---

## Scaling guidelines

The HTTP Gateway is stateless and scales horizontally behind a load balancer:

- **Ingestion throughput**: add Gateway replicas. Each replica independently authenticates and publishes to NATS.
- **Data Source count**: the cache-aside pattern absorbs lookup load. For deployments with thousands of Data Sources and high cache churn, increase Redis memory allocation.
- **NATS backpressure**: if NATS publish latency increases, scale the NATS cluster or the downstream consumers (JS Execution Service) to reduce queue depth.
- **Payload size**: large payloads increase memory pressure per request. Monitor payload size histograms and set appropriate request body limits at the reverse proxy or Gateway configuration level.

For most deployments, 2-3 Gateway replicas behind a load balancer provide sufficient throughput and redundancy.

---

## Next steps

- [MapexOS Core](/docs/1.0.0/en/architecture/mapexos-core)
- [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution)
- [Assets Service](/docs/1.0.0/en/architecture/assets)
- [Events & Pipeline](/docs/1.0.0/en/core-platform/events-and-pipeline)
