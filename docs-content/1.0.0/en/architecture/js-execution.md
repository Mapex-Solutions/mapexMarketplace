---
title: JS Execution
description: The normalization engine of MapexOS — runs each asset's template scripts inside hardened V8 isolates to turn any vendor's raw payload into one standard, typed event, safely and at scale.
---

# JS Execution

JS Execution is the **normalization engine** of MapexOS. It is where a raw device payload — whatever shape a vendor invented — becomes the platform's one **Standard Event**. Every [Asset Template](./assets.md) carries the JavaScript that knows how to read its devices; this service runs that code, per event, inside a **hardened V8 sandbox**, and hands the normalized result to the [Router](./router.md). It is the *transform* stage: between ingest — **any external source that enters the platform** ([HTTP](./http-gateway.md), [MQTT](./mqtt-broker.md), [LoRaWAN](./lns.md), and whatever comes next) — upstream and routing downstream. The engine is **source-agnostic**: each event is tagged with its `sourceType` (`http` · `mqtt` · `lorawan`), but the **same** decode→validation→transform pipeline runs regardless — so onboarding a new ingest protocol needs no change here.

Its hardest problem is that the code it runs is **written by customers**, not by the platform — so the whole design is about running untrusted code **safely, deterministically, and fast**.

> **Port** `8000` · **Node.js + TypeScript** (DDD + Hexagonal) · **`isolated-vm` V8 isolates** on a **Piscina** worker pool · reads asset/template scripts from a tiered cache (MinIO source of truth) · no owned database.

---

## The pipeline — decode → validation → transform

Each event runs a fixed three-phase pipeline, built from the template's scripts:

| Phase | Template script | Role |
|---|---|---|
| **decode** | `scriptProcessor` *(optional)* | Parse the raw payload — unpack a hex frame, split a CSV, decode base64. |
| **validation** | `scriptValidator` *(optional)* | Check the decoded data against expectations before it goes further. |
| **transform** | `scriptConversion` *(required)* | Emit the **Standard Event** — the normalized, typed shape the rest of MapexOS speaks. |

Each phase's JSON output becomes the next phase's `payload` input; only the final **transform** output is the standardized result. Because the per-device logic lives in a template, the same engine normalizes a LoRaWAN soil sensor and an HTTP turnstile with **no per-device branch in the platform** — onboarding a new vendor is writing three small scripts, not changing this service.

---

## A real sandbox — customer code cannot escape

Every script runs inside an **`isolated-vm` V8 isolate** on a Piscina worker thread, with hard limits:

- **32 MB heap cap** per isolate,
- **10-second execution timeout** per script,
- **no access to Node, the network, or the filesystem** — the worker imports only the isolation primitive and `worker_threads`; nothing from the application crosses into the sandbox.

Within those bounds the code runs on **full V8** — modern **ECMAScript (ES2015+)** is available natively, with no transpilation: classes, arrow functions, template literals, destructuring, `Map`/`Set`, and the standard built-ins. The only ceiling is the isolate's **32 MB** memory budget — a script that allocates past it is stopped, not silently truncated.

Each event gets a **fresh V8 context** (its `payload` injected by copy and released afterward), so one event can never see another's state. The isolate itself is **recycled every 10,000 events** to bound memory over time. Compiled scripts are cached per worker and per template, so the hot path compiles each template's code once, not once per event.

---

## Resilient to bad code

Untrusted code misbehaves, and the engine treats that as normal:

- A script that **exhausts the heap** disposes its isolate; the worker reports the out-of-memory condition, **rebuilds a fresh isolate**, and the event is **NACK'd for retry** — a transient failure, not a lost event.
- A script with a **parse or schema error** is a *permanent* failure: the message is rejected (to the dead-letter queue) rather than retried, because retrying can't fix it.
- A failure records which phase it failed at (`decode` / `validation` / `transform`) and emits a debug log to the [Events](./events.md) service, so a broken script is diagnosable, not silent.

Validation scripts get an injected helper, **`$mv`** (MapexValidator), with typed checks (`string` / `number` / `boolean` / `date` / `array` / `object`) and a `validate(value, schema)` entry point — so authors validate declaratively instead of hand-rolling guards.

---

## Built for throughput

The engine pulls events in **batches** and is tuned from a single knob. `CPU_LIMIT` derives the worker count, the batch size, and the in-flight limits; the process refuses to start if `CPU_LIMIT` exceeds the CPUs actually available, so it can never oversubscribe its host. Within a batch it **deduplicates asset lookups**, runs every event across the worker pool in parallel, and **flushes all downstream publishes once** before acknowledging — turning a burst of events into a single write round rather than one per event.

---

## Fed from the Assets source of truth

JS Execution owns no database. It resolves each event's asset and its template scripts from a **tiered cache** (RAM → disk → MinIO), whose source of truth is the [Assets](./assets.md) read-model. Two **FANOUT** consumers (`asset.invalidate`, `template.invalidate`) drop stale entries the moment an asset or template changes upstream — so a script edit takes effect without a restart, and no worker ever runs against a stale template.

---

## Inputs & outputs

**Consumes (NATS JetStream, batched):**

| Subject | Source |
|---|---|
| `processor.js.execute` | HTTP-gateway datasource events. |
| `mqtt.data.>` | MQTT telemetry from the [MQTT broker](./mqtt-broker.md). |
| `lorawan.data.>` | LoRaWAN uplinks from the [LNS](./lns.md). |
| `fanout.asset.invalidate` · `fanout.template.invalidate` | Cache coherence. |

Each ingress edge publishes to its **own protocol stream**, and every message carries its `orgId` / `assetUUID` and a `sourceType` tag — so the engine resolves the asset and runs the identical pipeline no matter how the data arrived.

**Produces (NATS, fire-and-forget, flushed once per batch):**

| Subject | Purpose |
|---|---|
| `route.execute` | The standardized event (`eventSource: "assetEvent"`) → the [Router](./router.md). |
| `events.logs.jsexecutor` · `events.raw` | Execution logs / debug → the [Events](./events.md) service. |
| `asset.heartbeat.{orgId}` | An implicit liveness signal (when the asset uses implicit heartbeat mode) → [Assets](./assets.md). |

**HTTP** (port `8000`): script **test** and **sample-payload** endpoints used by the template editor and test runner, plus `/health` and `/metrics`.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Where the scripts and fields are defined | [Assets](./assets.md) |
| What admits the events it consumes | [HTTP Gateway](./http-gateway.md) |
| Where its output is routed | [Router](./router.md) |
