---
title: Architecture Overview
description: How MapexOS is engineered — design principles, the event pipeline, multi-tenancy, durability, and the 10 services that compose the platform.
---

# Architecture Overview

> **IoT-first, but not limited to IoT.**
> MapexOS doesn't see devices or sensors — it sees **Assets**.
> Any source. Any protocol. One abstraction.
>
> **Connect. Automate. Scale.** — the open platform for data integration and intelligent automation.

Read this page if you are evaluating MapexOS against a production workload. Everything else in the documentation links back to one of the ideas on this page.

MapexOS treats every incoming signal — a device, a gateway, an API, a third-party webhook, an internal app — as an **Asset event**. From that single abstraction it gives operators one pipeline to **ingest, validate, normalize, route, store, and automate** that signal. The platform is **self-hosted, multi-tenant, multi-arch**, and engineered for the operational discipline of any other production system in your stack.

```
   Sources                       MapexOS                         Destinations
   ───────                       ───────                         ────────────
   Devices ──┐                                              ┌── Webhooks / APIs
   Gateways ─┤   Ingest → Validate → Transform → Route →    ├── Slack / Teams / Email
   APIs ─────┼──        Store / Notify / Automate           ├── NATS / MQTT
   Apps ─────┤                                              └── Custom plugins
   3rd-party ┘
```

---

## TL;DR — the 60-second pitch

- **One abstraction across every source.** Devices, gateways, APIs, apps, third-party webhooks all become **Asset events** — a normalized payload with `orgId`, `eventId`, `data`, `metadata`.
- **Ten independently-scalable services** (8 Go, 2 Node) bound by a single event contract on **NATS JetStream**.
- **A durable, crash-recoverable workflow engine** — a DAG runtime with retries, timers, and sub-workflows; every step is checkpointed to NATS KV, so a run resumes after a restart instead of starting over.
- **A plugin model in the n8n style** — new workflow nodes ship as declarative manifests served from the plugin registry; the editor loads them at runtime, so new connectors land without a frontend rebuild.
- **Decode / validate / transform code is sandboxed** in **V8 isolates** (`isolated-vm` on a worker pool); a tiered read-model cache (**L0 RAM → L1 disk → L2 MinIO/S3 → Fallback (rebuild the cache)**) keeps hot config fast platform-wide.
- **Multi-tenant from layer zero.** `orgId + pathKey` propagates through every contract; templates and policies inherit down the org tree.
- **Secrets never sit in plaintext.** Envelope encryption in `mapexVault`; the data plane sees only references like `secrets.slack.token`.
- **One command to boot the platform.** `docker compose up -d` from `mapexOSDeploy`; multi-arch images for Linux servers, Apple Silicon, Windows + Docker Desktop, and Raspberry Pi 4/5.
- **Licensed under BSL 1.1.** Self-host, modify, and run for your own organization; commercial SaaS distribution is reserved. Converts to Apache 2.0 on the Change Date.

If any of these are dealbreakers — or non-negotiable — the rest of this page tells you where they live in the code.

---

## What MapexOS is — and what it is not

**MapexOS is:**

- **IoT-first, but not limited to IoT.** Devices and sensors are first-class citizens, but every other source — REST API, webhook, internal app, third-party SaaS — enters through the same **Asset event** abstraction.
- **A durable automation runtime.** A DAG workflow engine with deterministic execution, retries with backoff, timers, sub-workflows, and idempotent triggers — state is checkpointed to NATS KV per step and survives process restarts.
- **A multi-tenant control plane.** Organization hierarchy, RBAC, group membership, and template inheritance are first-class concerns — not bolt-ons.
- **A policy-driven router.** Where an event goes — storage, lake house, workflow, trigger — is *configuration*, not code. Match rules live in MongoDB and apply at runtime (no schema migration, no redeploy); rules are read through a cache with a short refresh window.
- **An extensible editor.** New workflow nodes ship as **n8n-style plugins** described by manifests the editor loads at runtime; new connectors land in the editor without a frontend release.
- **Self-hostable, multi-arch.** One `docker compose up -d` boots the stack. Images for `linux/amd64` and `linux/arm64` — runs on Linux servers, Apple Silicon, Windows + Docker Desktop, and Raspberry Pi 4/5.

**MapexOS is NOT:**

- ❌ **Not a time-series database.** Persistence is delegated to **ClickHouse**; MapexOS is the pipeline that feeds it and the query surface that interprets it.
- ❌ **Not just another "IoT ingestion-and-dashboard" product.** Most platforms in this space stop at ingestion and a chart. MapexOS treats ingestion as the easy part and invests its complexity budget in **routing, durable workflows, plugin extensibility, and multi-tenancy** — the parts that actually break at scale.
- ❌ **Not a managed SaaS.** The platform is self-hosted, container-first. The commercial hosted distribution is a separate Mapex offering reserved under **BSL 1.1**.

> **Why the distinction matters.** A demo with a temperature widget is not a fleet platform. The hard part of operating real workloads starts *after* the data lands — and that is exactly where MapexOS spends its engineering.

---

## The event pipeline

Every event that enters MapexOS traverses the same canonical pipeline. **Each arrow on this diagram is a NATS subject under a JetStream stream, persisted and replayable.**

```
   INGEST                    TRANSFORM                ROUTE                    ACT  (fan-out, 0..N)
   ──────                    ─────────                ─────                    ────────────────────

  ┌───────────────┐                                                      ┌───────────────────────────┐
  │ http_gateway  │──┐    ┌───────────────┐     ┌───────────────┐   ┌───▶│ events    :5004 → ClickHo │
  │ :5001         │  │    │ js-executor   │     │ router        │   │    └───────────────────────────┘
  └───────────────┘  │    │ V8   :8000    │     │ :5003         │   │    ┌───────────────────────────┐
                     ├───▶│ decode →      │────▶│ match-rule    │───┤───▶│ workflow  :5007 → DAG      │
  ┌───────────────┐  │    │ validate →    │     │ evaluation    │   │    └───────────────────────────┘
  │ Mosquitto +   │  │    │ transform     │     │ + fan-out     │   │    ┌───────────────────────────┐
  │ broker plugin │──┘    └───────────────┘     └───────────────┘   ├───▶│ triggers  :5006 → 8 execs │
  └───────────────┘             │                                   │    └───────────────────────────┘
   authenticate +         Standard Event +                          │    ┌───────────────────────────┐
   persist raw copy       EVA dynamic fields                        └───▶│ lake-house   sink         │
                                                                         └───────────────────────────┘
```

**Sources** — Devices (HTTP / MQTT) and 3rd-party webhooks/APIs all enter at *Ingest*. **Each arrow is a NATS JetStream subject** (persisted and replayable): ingest emits `events.raw` and `processor.js.execute`; the router fans out over `events.save`, `workflow.execution.router`, `trigger.router.execute`, `events.lake_house`, and `events.router` (history).

The pipeline is **six stages**, each owned by a different service and each independently scalable:

| Stage | Owner service | What happens | Output |
|---|---|---|---|
| **1. Ingest** | `http_gateway`, MQTT broker | Authenticate the producer, attach `orgId` + `dataSourceId`, persist a raw audit copy | `events.raw`, `processor.js.execute` |
| **2. Decode** | `js-executor` | Run the asset's `decode` script in a V8 isolate to turn vendor bytes into a JSON object | `decoded` payload |
| **3. Validate** | `js-executor` | Run the `validate` script — reject malformed input before it reaches the router | `validated` payload |
| **4. Transform** | `js-executor` | Run the `transform` script — emit the **Standard Event** + EVA dynamic fields | `transformed` Standard Event |
| **5. Route** | `router` | Evaluate match rules over the Standard Event; fan out to 0..N downstream subjects | Per-kind subjects |
| **6. Act** | `events`, `workflow`, `triggers` | Persist, automate, or notify — each consumer is independent | ClickHouse rows, workflow runs, trigger calls |

> **Why each step is its own service.** Decode-validate-transform is CPU-bound and benefits from V8 pooling. Routing is rule-evaluation-heavy and needs hot asset lookups. Storage is I/O-bound on ClickHouse. Workflows are stateful with long-lived timers. Splitting these means **scaling the hotspot, not the whole platform.**

---

## The MapexOS ecosystem

MapexOS is a **monorepo** (`mapexOS`) of the backend services and the Vue 3 frontend, surrounded by a few **satellite repositories**. Most users only need the deploy repo; the others are for contributors, broker operators, LoRaWAN, and Go integrators.

| Repository | Role |
|---|---|
| **[`mapexOS`](https://github.com/Mapex-Solutions/mapexOS)** | Monorepo: the backend services and the Vue 3 frontend. |
| **[`mapexOSDeploy`](https://github.com/Mapex-Solutions/mapexOSDeploy)** | Docker Compose distribution that pulls pre-built multi-arch images from Docker Hub. **Start here to run the platform.** |
| **[`mapexMQTTBroker`](https://github.com/Mapex-Solutions/mapexMQTTBroker)** | The production MQTT broker — Eclipse Mosquitto v2 plus an in-house Go plugin that handles auth, ACL, presence, and ingress in a single `.so`. |
| **[`mapexLNS`](https://github.com/Mapex-Solutions/mapexLNS)** | LoRaWAN Network Server — a thin overlay on The Things Stack (GS/NS/JS roles) that dispatches decrypted uplinks to NATS. |
| **[`mapexGoKit`](https://github.com/Mapex-Solutions/mapexGoKit)** | Shared Go SDK used by every Go service — HTTP middleware, NATS/Mongo/ClickHouse/MinIO/Redis clients, observability, validation, contracts. |
| **[`mapexDevicesSimulator`](https://github.com/Mapex-Solutions/mapexDevicesSimulator)** | Desktop simulator that drives real HTTP/MQTT/LoRaWAN traffic into the platform without hardware. |

> **The MQTT broker is its own repo for a reason.** Every CONNECT/PUBLISH decision is made **locally** in the broker plugin off a three-tier cache (**Pebble → MinIO/S3 → HTTP fallback** to the `assets` service). No round-trip on the hot path. That guarantee is what lets a single broker carry tens of thousands of concurrent device connections without the control plane becoming the bottleneck.

---

## Design principles — and the tradeoffs we accepted

The architecture exists to answer six questions. Each row shows the question, the principle we chose, what we **gave up** to get it, and where the choice surfaces in the code.

| Question the principle answers | Choice we made | What we gave up | Where it lives |
|---|---|---|---|
| *How do services stay decoupled as the platform grows?* | **NATS JetStream** as the system of record for events; HTTP only for control plane | Synchronous request/response across services | All cross-service event flow uses subjects in `packages/contracts/services/*/events` |
| *How do we ship safe customer code without sandbox escapes?* | **V8 isolates** with no Node bindings, no filesystem, no network | "Just write any Node module" — only ES6+ allowed | `js-executor`, `js-workflow-executor` |
| *How do scripts run fast at scale?* | **Bytecode compilation + a tiered cache** (L0 RAM → L1 disk → L2 MinIO/S3 → Fallback) | Memory budget — RAM tier is bounded per pod | Tiered cache layer used by `assets`, `workflow`, `js-executor` |
| *How do workflow runs survive a crash?* | **DAG state checkpointed to NATS KV**, archived to MongoDB on terminal | Storage cost — every step is persisted | `workflow/src/modules/runtime` + `archiver` |
| *How do we serve many tenants safely on one stack?* | **`orgId + pathKey` in every contract**, RBAC enforced centrally, templates propagate down the tree | A flat model — there is no "single-tenant mode" | `mapexIam` is on every code path |
| *How do operators add new integrations without redeploying?* | **Plugin manifests** the editor loads at runtime from the plugin registry | Operators trust the registry they configure | `workflow/src/modules/plugins` |

These trade-offs are not retrofits — they are the reason the codebase looks the way it does.

---

## The 10 services — at a glance

```
   ┌─────────────────────────────────────────────────────────────────────────┐
   │                            CONTROL PLANE                                │
   │   mapexIam      mapexVault      assets       (frontend: mapexOS SPA)    │
   │   (auth/orgs)   (secrets/PKI)   (templates)                             │
   └─────────────────────────────────────────────────────────────────────────┘
   ┌─────────────────────────────────────────────────────────────────────────┐
   │                              DATA PLANE                                 │
   │                                                                         │
   │   http_gateway  ──┐                                                     │
   │                   ├─→  js-executor  ──→  router  ──┬─→  events          │
   │   MQTT broker  ──┘                                  ├─→  workflow       │
   │                                                     └─→  triggers       │
   │                                                                         │
   │              js-workflow-executor  (V8 for workflow code nodes)         │
   └─────────────────────────────────────────────────────────────────────────┘
```

| Service | Port | Stack | Role | Storage it owns |
|---|---|---|---|---|
| **mapexIam** | 5000 | Go | Organizations, users, roles, groups, JWT auth, RBAC cache | `dev-mapexos` (Mongo) + Redis DB 5 |
| **http_gateway** | 5001 | Go | Webhook ingestion, 5 auth strategies, datasource registry | `dev-http_gateway` (Mongo) |
| **assets** | 5002 | Go | Assets, templates, EVA fields, MQTT auth/ACL backend | `dev-assets` (Mongo) |
| **router** | 5003 | Go | Match-rule evaluation, fan-out to downstream destinations | `dev-router` (Mongo) |
| **events** | 5004 | Go | ClickHouse storage, 7 NATS consumers, EVA queries, TTL | `mapexos` (ClickHouse) + `dev-events` (Mongo) |
| **triggers** | 5006 | Go | 8 outbound executors: HTTP, MQTT, RabbitMQ, NATS, WebSocket, Email, Teams, Slack | `dev-triggers` (Mongo) |
| **workflow** | 5007 | Go | Durable, crash-recoverable DAG runtime, 17 core node types + plugin nodes, NATS-KV checkpointing | `dev-workflow` (Mongo) + NATS KV |
| **mapexVault** | 5010 | Go | Envelope-encrypted credentials, PKI authority | `dev-vault` (Mongo) |
| **js-executor** | 8000 | Node + V8 | Decode → Validate → Transform isolates for IoT events | MinIO/S3 (L2 cache) |
| **js-workflow-executor** | 8001 | Node + V8 | V8 isolates for workflow `Code` nodes | MinIO/S3 (L2 cache) |

Each service follows the same internal layout — **DDD + hexagonal**, modules in `src/modules/`, ports/adapters separating domain from infrastructure. That uniformity is what lets a new contributor land in any service and find their bearings in minutes.

### Edge servers — the three ingress points

Beyond the ten core services, the platform has **three ingress edges** — and two of them live in **their own repositories** with their own technology, because the protocol edge has very different operational characteristics:

| Edge | Transport | Stack | Ingress stream |
|---|---|---|---|
| **[HTTP Gateway](./http-gateway.md)** | HTTP / HTTPS | Go (Fiber) — a core service | `processor.js.execute` |
| **[MQTT Broker](./mqtt-broker.md)** | MQTT `1883` / `8883` | Eclipse Mosquitto v2 + a Go plugin | `mqtt.data.>` |
| **[LNS](./lns.md)** | LoRaWAN (UDP `1700` / WS `1887`) | The Things Stack overlay (Go) | `lorawan.data.>` |

All three authenticate **at the edge** off the same Assets-authored projection, and converge on `js-executor` for normalization — each on its own protocol stream.

---

## Multi-tenancy by design

Every contract carries the tenant context. It is not an HTTP header that gets dropped — it is a typed field on every DTO that crosses a service boundary.

### The org tree

```
RootOrg
  ├── Tenant-A           (pathKey: "rootOrg.tenantA")
  │     ├── Region-EU    (pathKey: "rootOrg.tenantA.regionEu")
  │     └── Region-US    (pathKey: "rootOrg.tenantA.regionUs")
  └── Tenant-B           (pathKey: "rootOrg.tenantB")
```

| Concern | How it works |
|---|---|
| **Identity** | `orgId` (immutable UUID) + `pathKey` (hierarchical string). Every event, asset, template, rule, workflow, and trigger carries both. |
| **Authorization** | RBAC roles are *defined per org* — there are no global "admin" or "viewer" roles. `mapexIam` evaluates the access matrix on every call. |
| **Inheritance** | Templates and policies declared at a parent org propagate to descendants unless explicitly overridden. |
| **State isolation** | All state keys are scoped: `state:{orgId}:{workflowId}:{var}`. There is no shared keyspace across tenants. |
| **Storage isolation** | Logical, not physical — same Mongo database, same ClickHouse cluster. Hard isolation is achieved by deploying multiple stacks. |
| **Retention** | TTL is configurable **per org × per stream** (raw, processed, router history). One tenant's compliance window does not impose cost on another. |

> **Why logical and not physical?** Hard per-tenant databases were considered and rejected — the operational cost of replicating Mongo, ClickHouse, NATS, and Redis per tenant is what makes most "multi-tenant" platforms unaffordable for SMB tenants. Logical isolation, with disciplined `orgId` filtering enforced in the contracts package, gives operators the *option* to shard later without code changes.

---

## Durability and reliability

What does "durable by default" mean concretely?

| Layer | Guarantee | Mechanism |
|---|---|---|
| **Ingestion** | Once `http_gateway` returns `2xx`, the event is persisted to JetStream | `events.raw` is a JetStream stream with file-backed storage |
| **Decode / validate / transform** | At-least-once delivery to `js-executor` | NATS JS consumer with explicit `Ack` after successful transform |
| **Routing** | Match rules evaluated against the latest asset snapshot, never a stale one | `router` reads via TieredCache (L0/L1) with FANOUT invalidation on asset mutations |
| **Workflow execution** | A crashed worker resumes the DAG mid-flight after restart | Per-step checkpoint to `NATS KV` (`exec.{uuid}`); NATS Schedule timers survive restarts |
| **Triggers** | Failed outbound calls retry with exponential backoff and land in a DLQ | NATS JS retry policy + per-trigger DLQ subjects |
| **Storage** | No event lands in ClickHouse twice for the same `eventId` | `events` service deduplicates on `(orgId, eventId)` via batch insert with ReplacingMergeTree |
| **Replay** | Operators can re-process historical events without writing code | JetStream consumers support `DeliverByStartTime` / `DeliverByStartSeq` |

> **What this is not.** MapexOS does not promise *exactly-once* end-to-end. It promises **at-least-once with stable idempotency keys** (`eventId`, `workflowUUID`, `executionStepId`). Downstream consumers must respect those keys to deduplicate.

---

## Horizontal scale — what to scale, and by what signal

MapexOS scales by adding instances of the **specific service** that is the bottleneck. Each service has a clear scaling vector and a clear bottleneck.

| Service | Scale by | Signal to watch |
|---|---|---|
| `http_gateway` | Stateless pods behind LB | Requests/second, p99 latency |
| MQTT broker | Mosquitto cluster + NATS leaf nodes | Concurrent connections, broker CPU |
| `js-executor` | Worker pool + isolate pool | Scripts/second, isolate pool saturation |
| `router` | NATS pull consumer concurrency | Pending messages on `events.processed` |
| `workflow` | Worker pool + NATS partition count | Pending messages on `workflow.execution.router`, NATS KV size |
| `triggers` | Per-executor worker pool | Retry queue depth, downstream call latency |
| `events` | Parallel JS consumers + ClickHouse insert batchers | Insert throughput, batch latency |
| ClickHouse | Replicas (read) + shards (write) | Disk I/O, replication lag |
| NATS JetStream | Cluster size + stream replicas | Storage pressure, ack pending |
| MongoDB | Replica set; shard when working set exceeds RAM | Working set vs RAM ratio |

> **Why not auto-scale everything from one signal?** Each stage has a different bottleneck — `http_gateway` is request-bound, `js-executor` is CPU-bound, `events` is disk-bound. A single auto-scaling policy across the platform would mis-scale most stages most of the time. We expose per-stage signals and let operators decide.

---

## Data architecture — operational, analytical, hot, cold

```
  ┌─ OPERATIONAL ───────────────┐   ┌─ ANALYTICAL ────────────────┐
  │  MongoDB                    │   │  ClickHouse                 │
  │  configs · governance       │   │  events.raw · processed     │
  │  templates · rules          │   │  router history · trig logs │
  └─────────────────────────────┘   └─────────────────────────────┘

  ┌─ HOT STATE ─────────────────┐   ┌─ OBJECT STORAGE ────────────┐
  │  Redis                      │   │  MinIO / S3                 │
  │  auth cache · rule counters │   │  read-models · definitions  │
  ├─────────────────────────────┤   │  artifacts · L2 cache       │
  │  NATS KV                    │   └─────────────────────────────┘
  │  workflow exec state        │
  │  plugin manifest cache      │   ┌─ EVENT BACKBONE ────────────┐
  └─────────────────────────────┘   │  NATS JetStream             │
                                     │  every cross-service        │
                                     │  subject · streams · replay │
                                     └─────────────────────────────┘
```

| Storage | Role | What lives there |
|---|---|---|
| **MongoDB** | Operational source of truth | Orgs, users, roles, assets, templates, datasources, route groups, workflow definitions, trigger configs, credentials (encrypted) |
| **ClickHouse** | Analytical event store | Raw events, processed events, router execution history, workflow logs, trigger logs |
| **Redis** | Low-latency shared state | Authorization / permission cache (DB 5 — shared across services) |
| **NATS KV** | Hot per-run state | Live workflow execution state (`exec.{uuid}`), plugin manifest cache |
| **NATS JetStream** | Event backbone | Every cross-service event subject and stream |
| **MinIO / S3** | Object + L2 cache | Bytecode artifacts, workflow definitions, plugin assets |

> **The discipline.** Each service has **exactly one** authoritative Mongo database — `dev-assets`, `dev-router`, `dev-workflow`, etc. Cross-service reads go through NATS or the contracts API, never through a shared Mongo collection. This is what keeps the services independently deployable.

---

## Caching strategy — predictable hot-path performance

Tight latency budgets in the hot path (router asset lookups, workflow definition resolution, script loading) are served by a uniform **TieredCache** — always the same four levels: **L0 RAM → L1 disk → L2 MinIO/S3 → Fallback (rebuild the cache)**.

```
L0  RAM                 (per-pod, ~MB, sub-ms)
 │
L1  disk (NVMe / SSD)   (per-pod, ~GB, ~1 ms)
 │
L2  MinIO / S3          (cluster-wide, unbounded, ~10 ms)
 │
Fallback                (rebuild the cache from the source of truth)
```

> **L2 is any S3-compatible object storage.** The L2 tier speaks the **S3 API**, so it works with **MinIO, AWS S3, DigitalOcean Spaces**, or any other S3-API-compatible service — pick the one your deployment already runs. On an L2 miss, the **Fallback** re-fetches from the owning service (over HTTP) and **rebuilds the cache** so the next read is hot again.

Where it's used:

- **`js-executor`** compiles and caches decode/validate/transform script bytecode per worker for fast re-execution.
- **`router`** caches asset snapshots used for match-rule evaluation.
- **`workflow`** caches `WorkflowDefinition`, `WorkflowInstance`, and plugin manifests.
- **`assets`** serves MQTT CONNECT/PUBLISH ACL decisions to the broker plugin off the cache — no hot-path round-trip to Mongo.

Invalidation is **event-driven** over NATS — a definition update fans out to every pod that holds the cache entry. There is no TTL guesswork.

---

## Extensibility — three planes, three contracts

Operators extend MapexOS in three independent ways, each with its own contract and its own blast radius.

| Plane | What you extend | Where it runs | Contract |
|---|---|---|---|
| **Templates** | Decode / validate / transform per asset | `js-executor` V8 isolate | JavaScript ES6+, no Node bindings |
| **Workflow code nodes** | Custom logic inside a workflow run | `js-workflow-executor` V8 isolate | JavaScript ES6+, the same isolate contract |
| **Plugins** | New workflow node types in the visual editor (n8n-style) | Frontend (UI) + workflow (manifest registry) | Declarative JSON manifest served from the plugin registry |

Each plane is sandboxed at a different boundary:

- **Templates** see one event at a time and never reach the host filesystem or network.
- **Workflow code nodes** see workflow context and exit cleanly on memory/timeout limits.
- **Plugins** load their UI from the marketplace URL the operator configures — the operator decides who they trust.

This split is why **new connectors land in the editor without a frontend release**, and why **new integration logic ships as a template, not a code change**.

---

## Security baseline

| Concern | How MapexOS addresses it |
|---|---|
| **Authentication (north-south)** | `http_gateway` supports API key, JWT, IP allowlist, OAuth 2.0, and basic auth per datasource |
| **Authentication (east-west)** | Service-to-service calls carry a JWT signed by `mapexIam`; consumers validate via the IAM JWKS |
| **Authorization** | RBAC roles per org, evaluated centrally by `mapexIam`, cached in Redis DB 5 |
| **Tenant isolation** | `orgId + pathKey` enforced in every contract, validated at every boundary |
| **Secret storage** | `mapexVault` holds credentials under envelope encryption (AES-256-GCM); the data plane sees only references like `secrets.<id>.<field>` |
| **PKI** | `mapexVault` is the certificate authority — mTLS material is provisioned, not pasted into configs |
| **Audit** | Every router execution writes a history row; every trigger call writes a log row; both queryable via the events service |
| **Sandboxing** | All customer code runs in V8 isolates with explicit time and memory limits |
| **Wire encryption** | All NATS, Mongo, ClickHouse, Redis, MinIO endpoints are TLS-capable; HTTP gateway terminates TLS at the LB |

The threat model assumes the platform operator is trusted, customer-supplied scripts are untrusted, and tenants must not be able to observe each other's data. The boundaries enforce that model.

---

## Observability

MapexOS is built to be **diagnosable from the outside**.

| Signal | Where to find it |
|---|---|
| **Structured JSON logs** | stdout of every service; correlation by `eventId` / `workflowUUID` |
| **Prometheus metrics** | `/metrics` on every Go service; per-NATS-consumer pending counts; per-isolate pool saturation |
| **Health probes** | `/health` (liveness) and `/ready` (readiness) on every service |
| **Distributed traces** | OpenTelemetry-ready (W3C `traceparent` propagated through NATS headers) |
| **Persistent audit logs** | Router execution history and trigger logs are queryable in ClickHouse |
| **Per-asset debug** | Opt-in `debugEnabled` flag per asset writes detailed pipeline history to ClickHouse — off by default to control cost |

> **Why debug is opt-in.** Persisting every step of every event for every asset would dominate ClickHouse storage. Production assets emit errors only; debug rides on a flag flipped during incident investigation.

---

## Deployment topologies

MapexOS ships as Docker Compose for single-node and as container images for any orchestrator. The shape of the deployment is a function of throughput, not a different product SKU.

| Topology | Use case | What changes |
|---|---|---|
| **Single-node** | POC, edge gateway, demo | One Compose file, all services on one host |
| **HA control plane** | Production, regional | NATS cluster (3+ nodes), Mongo replica set, multiple `mapexIam` and frontend pods |
| **Sharded data plane** | High-throughput ingestion | Multiple `http_gateway` and `js-executor` replicas; NATS leaf nodes for MQTT edge |
| **Edge + central** | Field deployments | MQTT broker + NATS leaf at the edge; full platform in the data center |

There is no "Enterprise edition" code path — the same binaries run all topologies.

---

## How to read the rest of the documentation

Different readers care about different parts of this page. Here is where to go next based on what you came for.

| If you are evaluating MapexOS as a... | Read next |
|---|---|
| **Platform architect** | [Core Concepts](../getting-started/concepts.md) → [Events](./events.md) → [mapexIam](./mapexiam.md) |
| **SRE / operator** | [Installation](../getting-started/installation.md) → [Observability](#observability) → [Quickstart](../getting-started/quickstart.md) |
| **Security reviewer** | [Security architecture](./security.md) → [mapexIam](./mapexiam.md) |
| **Integration developer** | [Assets](./assets.md) → [JS Execution](./js-execution.md) → [HTTP Gateway](./http-gateway.md) |
| **Workflow author** | [Core Concepts](../getting-started/concepts.md) → [JS Execution](./js-execution.md) → [Triggers](./triggers.md) |

---

## Service deep-dives

Each capability has its own page, grouped the way the platform is built.

**Go services**
- [**mapexIam**](./mapexiam.md) — organizations, users, roles, JWT, RBAC + coverage caches
- [**Router**](./router.md) — match rules, fan-out, route groups
- [**Workflow Engine**](./workflow.md) — durable DAG runtime, conditions, suspend/resume
- [**Triggers**](./triggers.md) — outbound executors, retries, DLQ
- [**Events**](./events.md) — ClickHouse storage, EVA queries, retention
- [**Assets**](./assets.md) — assets, templates, EVA fields, device PKI, presence
- [**Vault**](./vault.md) — envelope-encrypted secrets, KEKs, the certificate authority

**JS services**
- [**JS Execution**](./js-execution.md) — V8 isolates that normalize device payloads
- [**JS Workflow Execution**](./js-workflow-execution.md) — V8 isolates for workflow code nodes

**Edge servers**
- [**HTTP Gateway**](./http-gateway.md) — webhook ingestion, per-source auth, datasource registry
- [**MQTT Broker**](./mqtt-broker.md) — Mosquitto + Go plugin: local auth, ACL, presence, ingress
- [**LNS**](./lns.md) — LoRaWAN network server, an overlay on The Things Stack

**Cross-cutting**
- [**Security**](./security.md) — identity, isolation, secrets, and the threat model
