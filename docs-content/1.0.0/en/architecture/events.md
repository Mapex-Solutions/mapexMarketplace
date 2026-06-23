---
title: Events
description: The system of record for MapexOS — a ClickHouse sink that turns every signal into a typed, queryable, per-tenant-retained event, traceable end to end by a single tracking id.
---

# Events

Events is the **system of record** of MapexOS. It is the end of the line: every other stage — ingest, transform, route, automate — hands its output here, and Events persists it into **ClickHouse** as a typed, queryable row. It makes no decisions and publishes nothing back to the bus; it is pure **storage and read**. When you ask *what happened, to which asset, when, and why* — this is the service that answers.

Its defining trick is that it stores **schema-less data in a typed, columnar shape**: arbitrary per-vendor fields become queryable typed columns without a migration, every row carries its own retention, and a single tracking id ties one event's entire journey together.

> **Port** `5004` · **Go** (DDD + Hexagonal) · **NATS JetStream** batch consumers → **ClickHouse** (bulk insert, TTL retention) · **MongoDB** (retention catalog) + **Redis** + **S3/MinIO** (tiered template cache) · three modules: `events`, `asset_status`, `retention`.

---

## EVA — typed storage for schema-less data

Every asset speaks a different payload: one sends `temperature`, another `co2`, a third `doorState`. Storing that as JSON blobs would make it unqueryable; storing a column per field would mean a migration per device. MapexOS does neither. It uses **EVA** (Entity-Value-Attribute): each template field is assigned a numeric `fieldId`, and the value lands in one of **four typed ClickHouse MAP columns**, keyed by that id:

| Column | Type | Holds |
|---|---|---|
| `eva_number` | `map<uint16, float64>` | numeric fields — temperature, pressure, count |
| `eva_string` | `map<uint16, string>` | text fields — status, location, device type |
| `eva_bool` | `map<uint16, uint8>` | booleans — alarm, online, active |
| `eva_date` | `map<uint16, datetime>` | timestamps — last update, alarm time |

The type comes from the field's declared type in the asset's template (with inference as a fallback; a geo field stores its latitude as a number *and* a `"lat,lng"` string). The result: a new vendor or model is a **template configuration**, never a schema change — yet every dynamic field is stored **typed and columnar**, so you can filter and aggregate on it directly. The `POST /store/query` endpoint takes a list of `EvaFilters` — conditions on those dynamic fields — and pushes them straight into ClickHouse.

To turn the numeric ids back into human-readable names on read, Events resolves the asset's template through a **tiered cache** (L0 RAM → L1 disk → L2 S3/MinIO → HTTP fallback to the Assets service), kept coherent by a **FANOUT** `template.invalidate` consumer so a template edit upstream never serves stale names.

---

## Per-row retention — each row knows how long to live

Retention is not one global setting. **Every row is stamped at write time with its own `retention_days`**, resolved per-organization and per-table from the `retention` module (a MongoDB catalog, Redis cache-aside), and ClickHouse's native **TTL** expires it automatically. Different classes of data live for different spans by default — raw payloads and debug logs briefly, execution histories longer, processed events longest — and any organization can set its own policy:

| Data class | Default lifetime |
|---|---|
| Raw ingest · JS-executor debug | 1 day |
| Router / trigger / workflow execution history | 7 days |
| Processed events · dead-letter | 30 days |

A new organization is seeded with default policies automatically (the module consumes the `organization.created` lifecycle event), and editing a policy issues the corresponding ClickHouse TTL change.

---

## What it stores — one consumer per stream

Events runs a batch consumer per stream, each writing its own ClickHouse table:

| Stream | What it captures |
|---|---|
| `events.save` | **Processed events** — the primary table, with full EVA mapping. |
| `events.raw` | Raw ingest payloads, as received. |
| `events.logs.jsexecutor` | JS-executor decode/validate/convert debug output. |
| `events.router` | Router decisions — the per-event routing history. |
| `events.trigger` | Trigger execution outcomes. |
| `events.workflow` (`EVENTS-WORKFLOW-LOGS`) | Terminal workflow executions. |
| `dlq` | Dead-letter entries from any service. |

Together these are a complete, queryable audit trail: the processed event itself, *and* the record of how it was routed, what it triggered, and which workflow it ran.

---

## Traceable end to end by one id

A single `eventTrackerId` (a UUID) is minted at ingest and propagated through every service — route, trigger, workflow — and written onto every row here. So reconstructing one event's entire story is a single key lookup, not a join across systems: you can follow it from the raw payload, through the routing decision, to the trigger that fired and the workflow that ran. Names (`asset_name`, `template_name`, …) are **denormalized at write time**, so reads never pay for a join.

---

## How a batch is persisted

All consumers share one generic three-phase pipeline, tuned for ClickHouse's love of large writes:

```
 Phase 1 — parse in parallel     a bounded worker pool (NumCPU × 2) parses,
                                  validates, and EVA-maps each message
 Phase 2 — one bulk insert       every valid row goes to ClickHouse in a single
                                  bulk INSERT — minimal write amplification
 Phase 3 — settle per message    Ack on success · Nack (retry) on insert failure
                                  · Reject (→ DLQ) on a parse/validation failure
```

The dead-letter consumer uses a deliberate variant that **never Nacks** — a message it can't parse is acknowledged and skipped rather than retried, so the DLQ can never feed itself in a loop.

---

## Beyond events — connectivity history

A second module, `asset_status`, persists every **online/offline transition** an asset makes into its own table and serves a **connectivity timeline**: `GET /api/v1/events/connectivity_history` and `…/assets/:assetUUID/connectivity_history`, filterable by time window and transition type. Operations can see exactly when a device dropped and came back — a first-class history, not something reconstructed from logs.

---

## Read & manage — the HTTP surface

All read routes are **cursor-paginated, organization-scoped, and permission-gated**, under `/api/v1/events`:

- Per-stream lists: `GET /raw`, `/jsexec`, `/router`, `/trigger`, `/workflow`, `/dlq` (+ `/dlq/counts` grouped by service).
- `POST /store/query` — processed events with optional `EvaFilters` (query by dynamic field).
- `GET /store/:eventTrackerId` — one processed event with its EVA ids resolved to template-defined names.
- `GET /workflow/execution/:executionId` — a single workflow record.
- Connectivity timelines (above).

Retention policies are managed under `/api/v1/retention`: `GET /`, `GET /:id`, `PUT /` (upsert by `{orgId, type}`), `DELETE /:id`.

---

## Inputs & outputs

**Consumes (NATS JetStream, batch pull):** `events.save`, `events.raw`, `events.logs.jsexecutor`, `events.router`, `events.trigger`, `events.workflow`, `dlq`, `events.asset_status_save`, `organization.created`, and the FANOUT `template.invalidate`.

**Produces:** nothing on the bus. Events is a terminal sink — its only outbound effect is a ClickHouse TTL change when a retention policy is edited.

---

## Observability

Per-consumer Prometheus metrics track messages consumed / parsed / inserted / failed, **ClickHouse bulk-insert latency per table**, EVA template-cache hit/miss, and batch sizes. Health surfaces the dependency graph (MongoDB, Redis, NATS required; ClickHouse and S3 degrade gracefully — the service keeps accepting messages from NATS even when writes are briefly unavailable).

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Who decides what gets persisted | [Router](./router.md) |
| Where dynamic fields are defined | [Architecture Overview](./overview.md) |
| What produces execution histories | [Triggers](./triggers.md) · [Workflow Engine](./workflow.md) |
