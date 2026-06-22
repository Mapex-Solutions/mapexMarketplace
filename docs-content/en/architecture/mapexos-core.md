---
title: MapexOS Core
description: IAM, multi-tenant governance, RBAC, organization hierarchy, and cache invalidation in MapexOS Core (v1.0.0).
---

# MapexOS Core

**MapexOS Core** is the identity, access, and tenant management layer of the platform. It owns authentication (JWT/OAuth2), custom RBAC, organization hierarchy, and the cache invalidation pipeline that keeps downstream authorization state consistent.

> **Applies to v1.0.0** -- Go service, port 5000. MongoDB for operational data, Redis for auth and coverage caches, NATS JetStream for cache invalidation.

---

## Role in the pipeline

MapexOS Core sits **outside** the event-processing pipeline. It provides the identity and governance context that every other service relies on:

```txt
Console / API clients
        |
   MapexOS Core  ──  auth, RBAC, org context
        |
   HTTP Gateway / MQTT / other services (consume auth context)
```

Every authenticated request in the platform ultimately resolves permissions and organization coverage through Core, either directly or via cached authorization tokens.

---

## Responsibilities

- **Authentication**: issue, validate, and refresh JWT tokens (login, logout, refresh flows via OAuth2/JWT).
- **Custom RBAC**: manage roles with arbitrary permission sets -- no fixed role catalog. Organizations create roles that match their governance model.
- **Groups**: aggregate users into flexible groups for bulk permission assignment.
- **Memberships**: connect users and groups to organization nodes with role bindings.
- **Organization hierarchy**: model multi-level structures (Vendor, Customer, Site, Building, Floor, Zone, etc.).
- **Coverage resolution**: compute the effective permission set for a user across the organization tree.
- **Cache invalidation**: propagate permission and access changes to downstream caches via NATS.
- **Onboarding orchestration**: guided user provisioning endpoint for initial setup flows.
- **Lists**: manage classification lists (manufacturers, models, categories) used by other services.

---

## Non-responsibilities

MapexOS Core does **not** handle:

| Concern | Owner |
|---------|-------|
| Event ingestion (HTTP webhooks) | [HTTP Gateway](/docs/1.0.0/en/architecture/http-gateway) |
| Script execution (decode/validate/transform) | [JS Execution Engine](/docs/1.0.0/en/architecture/js-execution) |
| Event routing and fan-out | Router Service |
| Rule evaluation and stateful automation | Workflow engine |
| Action dispatch (HTTP, Slack, Teams, Email) | Triggers Service |
| Event persistence (ClickHouse) | Events Service |

---

## Module initialization

Core initializes 10 modules in a strict, dependency-ordered sequence:

```txt
1. lists
2. organizations
3. authorization_cache
4. roles
5. groups
6. memberships
7. users
8. auth
9. cache_invalidation
10. onboarding_orchestrator
```

Each module registers its routes, repositories, and consumers before the next module starts. This order guarantees that dependencies (e.g., organizations exist before roles reference them) are satisfied at boot time.

---

## Data flow

### Authentication flow

```txt
Client  ──POST /api/v1/auth/login──>  Core
                                        |
                                   validate credentials
                                   build JWT (access + refresh)
                                        |
Client  <──── JWT tokens ──────────  Core
```

### Authorization resolution (internal)

Other services call Core's internal endpoints to build authorization context:

```txt
Service  ──POST /internal/auth/build-authorization──>  Core
                                                         |
                                                    resolve roles + permissions
                                                    for user in org context
                                                         |
Service  <──── authorization payload ────────────────  Core
```

```txt
Service  ──POST /internal/auth/build-coverage──>  Core
                                                     |
                                                resolve org tree coverage
                                                for user
                                                     |
Service  <──── coverage payload ─────────────  Core
```

Internal endpoints are authenticated with API keys, not user JWTs.

---

## Organization hierarchy

MapexOS supports a multi-level organization tree. There is no fixed depth -- deployments define the hierarchy that matches their domain:

```txt
Vendor (root)
  └── Customer A
        ├── Site 1
        │     ├── Building X
        │     └── Building Y
        └── Site 2
  └── Customer B
        └── Site 3
```

### Coverage and permission resolution

When a user has a membership at a given organization node, their permissions **propagate** down the subtree according to the coverage model:

- A membership at "Customer A" grants access to Site 1, Site 2, and all descendants.
- A more specific membership at "Site 1" can **narrow or extend** the permission set for that subtree.
- Coverage resolution walks the org tree from root to leaf, merging permission sets at each level.

---

## Custom RBAC model

MapexOS does not ship with predefined roles (no "Admin", "Viewer", "Editor" out of the box). Instead:

1. **Organizations create roles** with a set of permissions drawn from the platform's permission catalog.
2. **Roles are assigned** to users or groups via memberships scoped to an organization node.
3. **Permission resolution** computes the effective set by merging all applicable role bindings across the org tree.

This model supports enterprise scenarios where different business units require different governance structures without platform-level changes.

---

## Cache invalidation

Authorization and coverage data is cached in Redis for performance. When the underlying data changes, Core publishes invalidation events via NATS JetStream so downstream caches stay consistent.

### Invalidation triggers

| NATS Subject | Trigger |
|--------------|---------|
| `role.permissions.changed` | Role permissions updated |
| `organization.access_policy.changed` | Org access policy modified |
| `membership.changed` | Membership created, updated, or removed |
| `group.changed` | Group membership modified |

Consumers listening on these subjects flush or rebuild the affected cache entries. This ensures that permission changes take effect across the platform within seconds, not on the next cache expiry.

---

## API surface

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/login` | Authenticate and receive JWT tokens |
| POST | `/api/v1/auth/logout` | Invalidate session |
| POST | `/api/v1/auth/refresh` | Refresh access token |
| GET | `/api/v1/auth/coverage` | Get current user's organization coverage |
| GET | `/api/v1/auth/permissions` | Get current user's effective permissions |

### Domain resources (CRUD)

| Resource | Base path |
|----------|-----------|
| Users | `/api/v1/users` |
| Organizations | `/api/v1/organizations` |
| Roles | `/api/v1/roles` |
| Groups | `/api/v1/groups` |
| Memberships | `/api/v1/memberships` |
| Lists | `/api/v1/lists` |

### Onboarding

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/onboarding/users` | Guided user provisioning |

### Internal endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/internal/auth/build-authorization` | API Key | Build authorization payload for a user |
| POST | `/internal/auth/build-coverage` | API Key | Build coverage payload for a user |

---

## Infrastructure

| Component | Role | Details |
|-----------|------|---------|
| **MongoDB** | Operational store | Users, organizations, roles, groups, memberships, lists |
| **Redis** | Cache | Auth token cache, coverage resolution cache |
| **NATS JetStream** | Event bus | Cache invalidation events (publish and consume) |

---

## Caching

Core uses Redis for two distinct cache domains:

- **Auth cache**: session tokens and validated JWT state. Keyed by session/user ID.
- **Coverage cache**: pre-computed permission sets and org tree traversals. Keyed by user ID and org node.

Cache entries are invalidated reactively via NATS events (see [Cache invalidation](#cache-invalidation) above), not on a fixed TTL. This provides both low latency and strong consistency.

---

## Retry and fault tolerance

- **NATS JetStream durability**: cache invalidation messages are persisted in JetStream. If Core restarts, unacknowledged messages are redelivered.
- **MongoDB retries**: transient write failures are retried with backoff at the repository layer.
- **Redis fallback**: if Redis is unavailable, authorization resolution falls back to MongoDB (higher latency, but the service remains functional).

---

## Observability

> MapexOS ships with **Prometheus** (15-day retention, 15s scrape interval) and **Grafana** pre-provisioned in the default Docker Compose deployment. Each service exposes a `/metrics` endpoint with a dedicated Grafana dashboard. No additional configuration is required to start monitoring.

### Health check

| Endpoint | Checks |
|----------|--------|
| `GET /health` | MongoDB connectivity, Redis app pool, Redis shared pool, NATS core connection |

A failed health check on any dependency returns an unhealthy status. Use this endpoint for container orchestration liveness/readiness probes.

### Key metrics

Core exposes approximately **280 metric series** at `/metrics`, covering:

| Category | Examples |
|----------|----------|
| **Authentication** | Login attempts (success/failure), token refresh rate, session operations |
| **CRUD per module** | Create/read/update/delete counters for users, orgs, roles, groups, memberships, lists |
| **Cache performance** | Redis hit/miss ratio, cache rebuild duration, invalidation event throughput |
| **HTTP latency** | Request duration histograms per endpoint |

### Grafana dashboard

The pre-provisioned dashboard is `mapexos-overview.json`. It provides panels for auth activity, CRUD throughput per module, cache hit rates, and infrastructure health.

---

## Scaling guidelines

MapexOS Core scales differently from pipeline services because its workload is governance-oriented, not event-throughput-oriented:

- **Read-heavy workloads** (permission checks, coverage resolution): scale Redis and add Core replicas behind a load balancer.
- **Write-heavy workloads** (bulk user provisioning, org restructuring): ensure MongoDB write capacity and monitor cache invalidation throughput.
- **Cache invalidation lag**: if permission changes take too long to propagate, check NATS consumer lag and Redis write latency.

For most deployments, a single Core instance with Redis is sufficient. Scale horizontally when concurrent authenticated users exceed single-instance capacity.

---

## Next steps

- [HTTP Gateway](/docs/1.0.0/en/architecture/http-gateway)
- [Assets Service](/docs/1.0.0/en/architecture/assets)
- [Security](/docs/1.0.0/en/architecture/security)
- [Events & Pipeline](/docs/1.0.0/en/core-platform/events-and-pipeline)
