---
title: Triggers
description: The service that talks to the outside world — one egress point, eight protocol connectors, fired standalone from the Router or integrated with workflows through a fully async resume.
---

# Triggers

Triggers is **how MapexOS reaches the outside world**. Everything upstream — ingest, transform, route, automate — happens inside the platform; Triggers is the **single egress point** where an event becomes a real action against an external system: an HTTP webhook, an MQTT publish, an email, a Slack message. It is the only service in MapexOS that performs outbound I/O.

It reaches the outside in **two ways**, and it runs in **two modes** — on its own, driven by the Router, or integrated with the Workflow engine through a fully asynchronous resume. The same connector catalog backs both.

> **Port** `5006` · **Go** (DDD + Hexagonal) · **8 protocol connectors** behind one port · **NATS JetStream** consumers + **MongoDB** (source of truth) + **Redis** (cache-aside) · **Prometheus** instrumentation.

---

## Two ways to reach the outside world

| Way | What it is |
|---|---|
| **Stored triggers** | A persisted, reusable definition — the configured catalog of side-effects. Created through the REST API, stored in MongoDB, cached in Redis, and **fired by id**. This is what the Router uses: "a rule matched → fire trigger `X`." |
| **Generic connector calls** | A fully-resolved action that arrives over NATS and is dispatched straight to a connector — **no database lookup**. This is the path that powers **plugins**: the action is assembled elsewhere (the Workflow engine) and Triggers simply performs it. |

Both paths converge on the same **connector catalog**. The difference is only *where the action comes from* — a stored definition fetched by id, or a ready-made action handed over on the wire.

---

## Two modes of operation

### Standalone — driven by the Router

The Router evaluates match rules against every event and fans out to the triggers that should fire. It publishes to `trigger.router.execute`; Triggers fetches each stored trigger by id, resolves its placeholders, and dispatches it. No workflow, no orchestration — a direct **event → action** reflex.

```
Event → Router (match rules, fan-out) → trigger.router.execute → Triggers → external system
```

### Integrated — driven by the Workflow engine

A workflow node that needs an outbound action **does not perform it inline**. The engine suspends the node, hands the resolved action to Triggers on `trigger.workflow.execute`, and **parks** — passing a `callbackSubject` that points back at its own resume stream. Triggers executes, then publishes the result to that subject, which wakes the exact node and continues the flow.

```
Workflow node suspends (waitType: callback)
   │  resolved action + callbackSubject  →  trigger.workflow.execute
   ▼
Triggers   (dispatch through the connector → outbound I/O)
   │  result  →  callbackSubject  (workflow.resume.*)
   ▼
Workflow RESUME  →  the parked node wakes and the flow continues
```

This is what makes workflow execution **100% asynchronous**: the workflow worker never blocks waiting on an external call. It parks, Triggers does the talking, and the resume picks up where it left off. (See the other half of this contract in [Workflow Engine → suspension & resume](./workflow.md).)

The workflow path carries two modes on one subject, routed by a `mode` field: `trigger` (fire a stored trigger by id) and `plugin` (run a fully-resolved action with no fetch).

---

## The connector catalog — 8 adapters, one port

Every connector implements one `TriggerExecutor` port and is registered by a type string at startup. Dispatch is a runtime lookup by `triggerType` — **a new connector is a new executor, not a change to the service**.

| Category | Connectors |
|---|---|
| **Technical** | `http` · `mqtt` · `rabbitmq` · `nats` · `websocket` |
| **Communication** | `email` · `teams` · `slack` |

If an event names a type with no registered executor, the message is rejected to the dead-letter queue rather than silently dropped.

---

## Templates that flow down the org tree

A trigger is not always a leaf-level object. Its scope is one of three levels, so an integration can be **defined once and inherited** rather than copied across every site:

| Level | Meaning |
|---|---|
| **System** (`isSystem`) | A global MapexOS template — no organization, available everywhere. |
| **Template** (`isTemplate`) | A vendor / customer template — **inherited by every descendant** in the organization hierarchy. |
| **Local** | An organization-specific trigger, scoped to its `orgId` and `pathKey`. |

Define a Slack escalation or an ITSM webhook once at the vendor level, and every customer, site, and floor beneath it inherits the same action — without duplicating it sixty times.

---

## Placeholder resolution — triggers are templates

A trigger config carries `{{path.to.field}}` placeholders that are resolved against the inbound event payload by dot-path navigation, just before execution. One stored trigger serves thousands of events, each filled with its own data.

Given an event payload:

```json
{ "assetName": "Pump-A3", "data": { "temperature": 87.5 } }
```

and a trigger body template:

```json
{ "text": "Alert: {{assetName}} reached {{data.temperature}} C" }
```

the dispatched body becomes:

```json
{ "text": "Alert: Pump-A3 reached 87.5 C" }
```

---

## How a batch executes — three phases

Triggers pulls work in batches and processes each batch in **three deliberate phases**, so a burst of actions costs one network round, not one per message:

```
 Phase 1 — execute in parallel     every message runs on its own goroutine,
                                    bounded by a worker semaphore (default 50)
 Phase 2 — flush once              a single FlushConnection pushes all the
                                    fire-and-forget publishes to the wire
                                    in one TCP round
 Phase 3 — settle sequentially     Ack / Nack / Reject each message by its result
```

The semaphore caps concurrency so a slow endpoint can't exhaust the process; the single flush turns N publishes into one round trip; the settle phase is where each message's fate is decided.

---

## Failure handling — mapped to JetStream semantics

Outcomes map cleanly onto JetStream's delivery model, so retries are meaningful and dead ends are not retried forever:

| Outcome | What happens |
|---|---|
| **Permanent error** (malformed message, unknown connector, bad config / unresolved placeholder) | **Reject** → straight to the dead-letter queue. No pointless retries. |
| **Transient error** (the outbound call failed, the config couldn't be fetched) | **Nack** → JetStream redelivers; once delivery attempts are exhausted, the message lands in the dead-letter queue. |
| **Disabled trigger** | **Ack** with no action — a no-op, not an error. |

Every outcome is recorded (see [Observability](#observability)), so a DLQ message is always explained by a persisted audit row.

---

## Secure by default

The outbound edge is where a platform is easiest to weaken — so the connectors don't. TLS is **verified, not skipped** (`InsecureSkipVerify` is off), each connector enforces its own connect and publish timeouts (e.g. MQTT connect 10s / publish 5s), and credentials are supplied per connector (HTTP `Authorization` / Bearer, broker TLS). MapexOS ships no connector that disables certificate verification.

---

## Governance & multi-tenancy

The CRUD surface is governed end to end. Every route is **permission-gated** (`TriggerList`, `TriggerRead`, `TriggerCreate`, `TriggerUpdate`, `TriggerDelete`) and **coverage-aware**: a hierarchical middleware scopes every listing to the slice of the organization tree the caller is allowed to see, by `pathKey`. Trigger configs are read through a **cache-aside** layer (Redis, 60-minute TTL) and invalidated on update or delete, so execution rarely touches MongoDB while staying fresh.

---

## Inputs & outputs

**Consumes (NATS JetStream, stream `TRIGGERS`):**

| Subject | Source | Purpose |
|---|---|---|
| `trigger.router.execute` | Router | Fire a stored trigger by id (standalone mode). |
| `trigger.workflow.execute` | Workflow | Workflow-driven execution, routed by `mode` (`trigger` / `plugin`). |

**Produces:**

- **Outbound side-effects** to external systems through the selected connector — the actual job of the service.
- **Audit event** → `events.trigger` (per execution: trigger, type, success, duration, error), de-duplicated on JetStream by a `{eventTrackerId}-triggerlog` message id, consumed by the [Events](./events.md) service → ClickHouse.
- **Workflow resume** → the dynamic `callbackSubject` on the workflow resume stream, waking the parked node.

**HTTP** (port `5006`, under `/api/v1/triggers`): `GET /`, `GET /counter`, `POST /`, `GET /:id`, `PATCH /:id`, `DELETE /:id`.

---

## Observability

Every execution is measured. Prometheus counters and histograms track batch size, **executor duration per connector type**, cache hit/miss, placeholder resolutions, publish outcomes, and DLQ counts — and the **workflow-driven path carries its own dedicated metric set** (including resume publishes), so standalone and integrated traffic are never conflated. Combined with the idempotent audit row per execution, you can answer *what fired, where the time went, and why* — per protocol and per origin.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Who decides a trigger should fire | [Router](./router.md) |
| The async contract on the workflow side | [Workflow Engine](./workflow.md) |
| Where audit rows are stored and queried | [Events](./events.md) |
| Full platform context | [Architecture Overview](./overview.md) |
