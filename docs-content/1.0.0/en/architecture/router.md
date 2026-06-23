---
title: Router
description: The dispatcher of MapexOS — evaluates conditional match rules against every event and fans it out to persistence, automation, and notification, with routing decided from trusted asset state, not the payload.
---

# Router

The Router is the **dispatcher of MapexOS**. It sits at the *route* stage — between transform upstream and automate/persist downstream — and answers one question for every event: **where does this go?** A single event arriving here can fan out to persistence, a data lake, a notification, an outbound [trigger](./triggers.md), and a [workflow](./workflow.md) — all at once, or to none of them, decided rule by rule.

It is what makes the platform **dynamic**: routing behavior is **data, not code**. You change where events go by editing RouteGroups through the API — no redeploy, no restart. The same event stream feeds entirely different downstream topologies for two tenants, because each one's rules say so.

> **Port** `5003` · **Go** (DDD + Hexagonal) · **NATS JetStream** WorkQueue consumer · **MongoDB** (rules, source of truth) + **Redis** (cache-aside) + **TieredCache** (RAM → Disk → MinIO) · **Prometheus** instrumentation.

---

## The model: RouteGroup → Routers → Match rules

| Concept | What it is |
|---|---|
| **RouteGroup** | A named, versioned bundle of Routers, attached to assets. An asset can carry several. |
| **Router** | One destination — a `kind` plus an optional `match` condition. If the condition passes, the event goes to that kind. |
| **Match rule** | One condition: `{ field, operator, value }`. Routers combine rules with an `all` (AND) or `any` (OR) policy. A Router with no match config **always fires**. |

This is the whole dynamism in three layers: assets point at RouteGroups, RouteGroups hold Routers, Routers carry conditions. Edit any layer — as data — and the routing changes on the next event.

---

## Match rules — the conditions, exactly as they evaluate

A rule is `{ field, operator, value }`. The **field** is a **dot-path** into the event (`data.temperature`, `metadata.site`), resolved by walking nested maps — fast and predictable. The **operator** is one of eight:

| Operator | Meaning | How it compares |
|---|---|---|
| `eq` | equals | deep equality |
| `neq` | not equals | deep equality, negated |
| `gt` | greater than | numeric |
| `gte` | greater than or equal | numeric |
| `lt` | less than | numeric |
| `lte` | less than or equal | numeric |
| `in` | is one of | membership in a list |
| `nin` | is not one of | membership in a list, negated |

Rules combine under a policy:

| Policy | Behavior |
|---|---|
| `all` | every rule must pass (AND) |
| `any` | at least one rule must pass (OR) |

A field the event doesn't contain makes that rule fail with `field not found` — predictable behavior under partial data, never a crash. A Router with no rules at all is **always allowed**.

```json
{
  "name": "Cold-chain breach",
  "routers": [
    {
      "kind": "workflow",
      "match": {
        "policy": "all",
        "rules": [
          { "field": "data.temperature", "operator": "gt",  "value": 8 },
          { "field": "data.doorState",   "operator": "eq",  "value": "open" }
        ]
      }
    }
  ]
}
```

This Router starts a workflow only when temperature exceeds 8 **and** the door is open — every other event flows past it untouched.

---

## The five destinations

A matched Router publishes the enriched event to its kind's subject. Those five subjects are how the Router wires the ingest pipeline into every downstream capability:

| Kind | Subject | Goes to |
|---|---|---|
| `save_event` | `events.save` | the [Events](./events.md) service → ClickHouse (persistence) |
| `lake_house` | `events.lake_house` | the analytical lake-house sink |
| `notification` | `events.notification` | the notification path |
| `trigger` | `trigger.router.execute` | the [Triggers](./triggers.md) service (standalone outbound action) |
| `workflow` | `workflow.execution.router` | the [Workflow](./workflow.md) engine (start a durable run) |

One event can match several Routers and fan out to several kinds at once. Each dispatch is de-duplicated on JetStream by a `{eventTrackerId}-{routerIndex}` message id, so a redelivery never double-routes.

---

## Routing is decided from trusted state, not the payload

The Router never reads *which* RouteGroups apply from the inbound message. It resolves the event's asset from cache and takes the RouteGroup ids **from the asset itself**. A sender cannot forge its own routing by stuffing ids into the payload — routing is governed by the asset's configuration, which only the platform controls.

```
event { assetUUID }  →  resolve asset (cache)  →  asset.RouteGroupIds  →  evaluate
                              (source of truth — never the payload)
```

---

## Health-aware routing

The Router distinguishes two kinds of input through an `eventSource` discriminator:

- **`assetEvent`** — a normal data event → uses the asset's `RouteGroupIds`.
- **`healthStatus`** — an online/offline transition → uses the asset's `HealthMonitor.OfflineRouteGroupIds` or `OnlineRouteGroupIds`, chosen by the transition.

A device going offline can therefore drive a *different* set of routes than its telemetry does. Health transitions are deliberately allowed to reach only the **`trigger`** and **`workflow`** kinds — a presence change escalates or automates; it does not flood the lake-house or the notification feed.

---

## Every decision is explainable

For each RouteGroup it evaluates, the Router emits a **routing-history event** to `events.router` recording every Router's outcome and the **condition-by-condition reasoning** behind it:

```
"data.temperature" greater than 8 → allowed
"data.doorState" equals "open"    → denied (field not found)
policy: all → 1/2 rules passed    → denied
```

You can always answer *why did — or didn't — this event fire that workflow?* without re-running anything. Routing stops being a black box.

---

## How a batch routes

The Router pulls events in batches (default `8000`, `NATS_BATCH_SIZE`) from a JetStream **WorkQueue** and processes each batch in three phases:

```
 Phase 1 — route in parallel       a worker pool (NumCPU × 2) resolves each asset,
                                    evaluates its RouteGroups, and buffers the
                                    fan-out publishes
 Phase 2 — flush once              a single FlushConnection pushes every buffered
                                    publish to the wire in one round
 Phase 3 — settle sequentially     Ack / Nack / Reject each message by its result
```

Outcomes map onto JetStream semantics: a malformed message or one missing `orgId` / `assetUUID` / `event` is **Rejected** straight to the dead-letter queue (retrying can't fix it); a transient failure such as a cache miss is **Nacked** and redelivered. WorkQueue delivery guarantees each event is routed by exactly one replica.

---

## Asset context — a tiered cache kept coherent

Routing every event needs the full asset, fast. The Router resolves it through a **TieredCache** that falls through progressively cheaper-to-colder tiers:

```
L0  RAM   (256 MB, 5 min)  →  L1  Disk (/tmp/mapexos/cache)  →  L2  MinIO/S3  →  HTTP fallback → Assets
```

To keep every replica's cache honest, the Router runs **FANOUT** consumers: when the Assets service changes an asset or a template, it broadcasts an invalidation that each replica applies to its local L0/L1. No replica routes on a stale asset.

---

## Governance & multi-tenancy

RouteGroups are managed through a governed REST surface. Every route is **permission-gated** and **coverage-aware** — listings are scoped to the slice of the organization tree the caller may see, by `pathKey`. Like triggers, a RouteGroup can be a **system** template (global), a **vendor/customer template** inherited by descendants (`isTemplate`), or **org-local** — so a routing policy defined once high in the hierarchy applies to every site beneath it. Configs are read through Redis cache-aside (60-minute TTL) and warmed on write, so the routing hot path rarely touches MongoDB.

---

## Inputs & outputs

**Consumes (NATS JetStream):**

| Subject | Stream | Purpose |
|---|---|---|
| `route.execute` | `ROUTE-GROUPS` (WorkQueue + DLQ) | One asset event or health transition to route. |
| `mapexos.fanout.asset.invalidate` | FANOUT | Evict an asset from local L0/L1. |
| `mapexos.fanout.template.invalidate` | FANOUT | Evict a template from local L0/L1. |

**Produces (NATS):** `events.save`, `events.lake_house`, `events.notification`, `trigger.router.execute`, `workflow.execution.router`, and `events.router` (routing history). Dispatches are de-duplicated by message id.

**HTTP** (port `5003`):

- `GET/POST /api/v1/route_groups`, `GET /api/v1/route_groups/counter`, `GET/PATCH/DELETE /api/v1/route_groups/:id` — JWT, permission-gated, coverage-scoped.
- `GET /internal/route_groups?ids=…&projection=…` — API-key, service-to-service bulk lookup.

---

## Observability

Every stage is measured in Prometheus: events received / routed / failed with latency histograms, TieredCache hit/miss **per tier**, RouteGroup cache hit ratio, match evaluations and outcomes, and publish counts/latency **per downstream subject**. Paired with the per-decision routing history, you can see both the aggregate flow and the reason behind any single routing decision.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Where matched outbound actions run | [Triggers](./triggers.md) |
| Where matched workflows execute | [Workflow Engine](./workflow.md) |
| Where routed events are stored | [Events](./events.md) |
| Full platform context | [Architecture Overview](./overview.md) |
