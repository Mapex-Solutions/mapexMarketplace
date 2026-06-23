---
title: HTTP Gateway
description: The front door of MapexOS — an authenticated HTTP ingestion edge where every external source enters with its own auth policy, gets a correlation id, and is admitted to the pipeline with the leanest possible payload.
---

# HTTP Gateway

The HTTP Gateway is the **front door** of MapexOS. It is where the outside world gets *in*: webhooks, device telemetry, and API calls from gateways and business systems arrive here over HTTP. Its job is narrow and load-bearing — **authenticate each request against the policy of its source, mint a correlation id, and admit a clean, minimal payload to the pipeline**. It does not decode, transform, or route; it authenticates and forwards, then hands off to the [JS execution](./js-execution.md) → [Router](./router.md) stages over NATS.

Everything the gateway does is anchored on one concept: the **DataSource**.

> **Port** `5001` · **Go** (DDD + Hexagonal) · **MongoDB** (DataSource registry) + **Redis** (cache-aside) · publishes to **NATS JetStream** · no inbound NATS — all input is HTTP.

---

## The DataSource — a governed entry point

Every external integration is modeled as a **DataSource**: a configuration record that says *who* may send data, *how* they authenticate, and *which asset* the data belongs to. One device fleet posting over an API key, one partner system using OAuth2, one on-prem gateway restricted to an IP range — each is its own DataSource, governed independently.

| Field | Role |
|---|---|
| `auth` | The authentication strategy for this source (see below). |
| `assetBind` | How an incoming payload maps to an Asset. |
| `enabled` | A kill switch — a disabled source is refused before anything else runs. |
| `orgId` / `pathKey` | The tenant this source belongs to — the trust anchor. |

DataSources are managed through a governed REST surface (`/api/v1/data_sources`, permission-gated and org-scoped, with pagination, filtering, and projection), and read on the hot ingest path through a **Redis cache-aside** layer (24-hour TTL, hit/miss metered) so admitting a request rarely touches MongoDB.

---

## Per-source authentication at the edge

The gateway doesn't impose one global auth scheme. **Each DataSource carries its own**, and the ingest middleware switches on it per request:

| Strategy | How it authenticates |
|---|---|
| `apiKey` | A key in a configured header or query field. |
| `jwt` | A signed JWT validated against a shared secret (configurable header). |
| `oauth2` | A bearer token validated against a JWKS endpoint (OIDC-style). |
| `ip_whitelist` | The caller's IP must fall within allowed CIDR ranges. |
| `none` | Open ingestion, for trusted-network sources. |

Before any of this runs, a **disabled DataSource is rejected outright** — the kill switch costs nothing. The same applies to an unknown auth type: refused, never admitted by default.

```
POST /api/v1/events?ds={id}
   │
   ├─ resolve DataSource (Redis → MongoDB)
   ├─ disabled?            → 403   (+ audit)
   ├─ authenticate by type → 401 on failure (+ audit)
   └─ admitted             → publish to the pipeline → 201
```

---

## Rejected traffic is recorded, not just dropped

Security at an ingestion edge is only as good as its visibility. **Every rejection** — a disabled source, a bad key, an IP outside the range — fires a `RawEventDTO{ success: false }` to `events.raw` in a detached goroutine, without slowing the `401`/`403` it returns to the caller. That record lands in ClickHouse via the [Events](./events.md) service, so failed and forged attempts become a **queryable audit trail**, not a silent drop.

---

## The correlation id is born here

On every accepted request the gateway mints a fresh **`eventTrackerId`** (a UUID) and attaches it to the payload. That single id then threads through js-execution → router → triggers → workflow → storage — it is the thread that makes an event **traceable end to end**. When the Events service lets you reconstruct one event's whole journey by a single id, *this* is where that id starts.

---

## A deliberately minimal handoff

What the gateway forwards to the pipeline is intentionally lean. The published `processor.js.execute` payload carries only:

- `sourceType: "http"` — the origin,
- the DataSource's `orgId` and `assetBind` — enough to resolve the Asset,
- the raw `event` body,
- the `eventTrackerId`.

It deliberately omits the asset's name, description, and `pathKey` — **js-execution reads those from the Asset cache, the source of truth.** Nothing is duplicated onto the wire that the platform already knows; the edge stays thin and the Asset stays authoritative.

---

## Heartbeats — presence without spoofing

Beyond telemetry, the gateway accepts an explicit **heartbeat** (`POST /api/v1/heartbeat?ds={id}`, body `{ assetUUID }`) that drives an asset's online state. It publishes to `asset.heartbeat.{orgId}` for the Assets health monitor. Crucially, the **`orgId` and `pathKey` come from the resolved (post-authentication) DataSource, never from the request body** — so a forged body cannot make one tenant report presence for another.

---

## Inputs & outputs

**HTTP (in):**

| Route | Result |
|---|---|
| `POST /api/v1/events?ds={id}` | Admit telemetry → `201 { success: true }`. |
| `POST /api/v1/heartbeat?ds={id}` | Drive presence → `200 { success: true }`. |
| `/api/v1/data_sources` (CRUD) | Manage sources — auth + permission-gated. |

**NATS (out):**

| Subject | Purpose |
|---|---|
| `processor.js.execute` | Admitted event → the JS-execution pipeline (JetStream, acknowledged). |
| `events.raw` | Auth-failure audit record (fire-and-forget). |
| `asset.heartbeat.{orgId}` | Presence signal → the Assets health monitor (core publish). |

The gateway consumes **nothing** from NATS — its entire input surface is HTTP.

---

## Observability

Prometheus metrics cover authentication outcomes **per strategy** (including the `disabled` gate), auth duration, DataSource cache hit/miss, and publish results — plus `/health` and `/swagger`. You can see, per source and per auth type, exactly what is being admitted and what is being turned away.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| What happens to an admitted event | [JS Execution](./js-execution.md) |
| Where the asset model lives | [Architecture Overview](./overview.md) |
| Where rejected-traffic records are stored | [Events](./events.md) |
