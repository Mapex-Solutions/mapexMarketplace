---
title: Workflow Engine
description: The automation brain of MapexOS — a durable, NATS-native DAG engine that decides and delegates, surviving restarts and resuming mid-graph.
---

# Workflow Engine

The Workflow Engine is the **automation brain** of MapexOS: a durable, DAG-based runtime that executes the workflows operators design in the visual editor. Its defining trait is how it handles side-effects — instead of performing a node's **outbound action** inline, it **resolves the action, hands it to a specialized service, then suspends and resumes on the callback**: outbound HTTP/MQTT actions go to the [Triggers](./triggers.md) service, `code` nodes to [JS Workflow Execution](./js-workflow-execution.md), credential decryption to the Vault service. The engine still makes the internal calls orchestration needs — it calls the Vault service to decrypt credentials and runs an HTTP proxy for dynamic plugin options — but it **never blocks a worker waiting on an outbound action**. That decomposition, plus a per-step checkpoint to NATS KV, is what keeps every run **deterministic and crash-recoverable**: a node that triggers an action doesn't hold the run open — it parks, and a callback wakes it.

> **Port** `5007` · **Go** (DDD + Hexagonal) · **NATS JetStream + KV** for live state (no Redis) · **MongoDB** + **MinIO** + **TieredCache** · **Vue 3 / Quasar** editor.

---

## The model: Definition → Instance → Execution

| Entity | What it is |
|---|---|
| **Definition** | The DAG blueprint — nodes, edges, and node scripts. |
| **Instance** | A definition bound to specific inputs (`externalInputs`) — the configuration for a tenant or asset. |
| **Execution** | One actual run, with its own mutable state, status, and history. |

Cardinality is `Definition 1→N Instance 1→N Execution`. Definitions and instances are cached (TieredCache → MongoDB). An execution's live working state runs in NATS KV; its lifecycle is mirrored to MongoDB for the UI, and when it finishes it is also published to the Events service for permanent storage (see [Storage](#storage)).

---

## How a run executes

The runtime walks the DAG **one node at a time**. After each advance it **checkpoints the entire execution to a NATS KV record** (`exec.{uuid}`) holding the full mutable state — `State`, `NodeOutputs`, `NodeStates`, `ActiveNodeIDs`, `ExecutionPath`. If a worker crashes, a restart reloads that record and **resumes from the last completed node**, not from the beginning.

```
 ┌─ execute() ──────────────────────────────────────────────┐
 │  pick next node → run executor → apply result            │
 │        │                                                  │
 │        ├── checkpoint(execution) → NATS KV  exec.{uuid}   │
 │        │                                                  │
 │        ├── more nodes & steps < 300 ? ──▶ loop inline     │
 │        ├── steps ≥ 300 ? ──────────────▶ re-enqueue RESUME│
 │        └── node suspended ? ───────────▶ park + dispatch  │
 └──────────────────────────────────────────────────────────┘
```

A single cycle runs up to `MaxInlineSteps = 300` nodes inline; beyond that the runtime **re-enqueues itself** to the resume stream rather than holding a worker.

---

## Suspension & resume — one uniform async contract

Nodes that can't finish synchronously **suspend**, and the contract is **data-driven**: an executor signals suspension simply by putting a `waitType` key in its `NodeState`. The walker checkpoints and dispatches by type:

| `waitType` | Wakes when… | Used by |
|---|---|---|
| **timer** | a NATS Schedule timer fires | `delay`, retry backoff, wait expiry |
| **signal** | an external HTTP signal arrives | `wait_signal` |
| **condition** | the condition is re-evaluated true | `wait_for` |
| **callback** | an external service returns a result | `code`, `subworkflow`, plugin nodes |

Callback and signal wake-ups arrive on the `WORKFLOW-RESUME` stream; timer wake-ups fire on the `WORKFLOW-SCHEDULE` stream — both resume the exact execution. This one contract covers timers, human/external signals, polled conditions, and cross-service callbacks — no special-casing per node.

---

## The node catalog — 17 core executors + plugins

Every `core/*` node maps to a built-in executor (`registry_builder.go`); any non-`core/` type routes to a single **generic plugin executor**.

| Group | Nodes |
|---|---|
| **Inline** | `start` · `end` · `condition` · `switch` · `set_state` · `log` · `goto` |
| **Async (suspend)** | `delay` · `wait_signal` · `code` · `subworkflow` · `trigger_event` |
| **Control flow** | `fanout` · `merge` · `sequence` · `loop` · `wait_for` |
| **Plugin** | any `<plugin>/<action>` type → the generic plugin executor |

*(The editor also offers `text_note` and `group_frame` — visual-only decorations with no executor.)*

---

## Conditions — AND / OR / NAND / NOR + 18 operators

Conditions are not a single threshold. The engine evaluates **nestable groups** with a recursive, short-circuiting `EvaluateGroup` — `AND`, `OR`, `NAND` (`!AND`), `NOR` (`!OR`) — over **18 operators**:

- **Comparison:** `equals`, `notEquals`, `greaterThan(Equals)`, `lessThan(Equals)`, `between`
- **String:** `contains`, `notContains`, `startsWith`, `endsWith`, `regex`
- **Date / time:** `beforeDate`, `afterDate`, `beforeTime`, `afterTime`, `betweenDate`, `betweenTime`

The same evaluator backs `condition`, `switch` (match mode `first` / `all`), and `wait_for` (a single-condition group). A field that can't be resolved degrades to **non-match** rather than throwing — predictable behavior under partial data.

---

## State — persistent vs ephemeral

Two scopes, and the distinction is load-bearing:

| Scope | Lifetime | Mutated by |
|---|---|---|
| **`State`** | Persists across the whole run | `set_state` patches: `set` · `increment` · `decrement` · `append` · `remove` (a nil value deletes the key) |
| **`NodeStates[nodeId]`** | Ephemeral, per node | loop index, merge branch count, retry attempt, wait info — cleared when the node completes |

Node inputs are read by a **value resolver with six sources**: `literal` (with `{{...}}` template interpolation), `event`, `state`, `variable` (an alias of `state`), `input` (the instance's `externalInputs`), and `node_output` (`NodeOutputs[nodeId]`).

---

## Parallelism & control flow

- **fanout → merge** — `fanout` (≤ 20 branches) spawns goroutines, each with a **deep-copied, isolated** copy of state/outputs, so branches can't corrupt each other. `merge` joins them with strategy `all` / `any` / `first`. If branches suspend, the whole fanout suspends and resumes per branch.
- **loop** — iterates a resolved array (≤ 10 000 iterations), injecting `loop_item` / `loop_index`. The walker keeps a **LIFO loop stack**, so nested loops and post-loop continuation work correctly.
- **switch** — multi-way branching on the shared condition evaluator.
- **subworkflow** — runs another workflow as a node (depth ≤ 10, with dedup), mapping `inputMappings` in and `outputMappings` back.

---

## Goto — virtual links that keep the DAG clean

`core/goto` comes in paired **sender / receiver** portals matched by `pairLabel`. At build time the GraphBuilder **injects an adjacency edge** from each sender to its receiver — so the editor's graph stays acyclic and readable while the run can still "jump" across it. An orphaned sender fails fast with `GOTO_NO_RECEIVER`. This is what lets a large workflow avoid wires crossing the whole canvas.

---

## Retry & error handling

A node can route failures down an explicit **`error` output edge**. With an `ErrorHandler` enabled, the node **retries with exponential backoff** — `initialInterval × multiplier^attempt`, capped at `MaxRetryDelaySeconds = 3600`, hard limit `MaxRetryAttempts = 10` — implemented as a `timer` suspension on NATS Schedule (so retries survive restarts too).

---

## Plugin nodes — pure orchestration

A plugin node (`<plugin>/<action>`, e.g. `telegram/message`) runs through the generic **PluginExecutor**, which:

1. loads the plugin **manifest**,
2. resolves the node's field values and `{{context.field}}` templates (over manifest / credentials / config / workflow / event),
3. **decrypts credentials** through the Vault service, then
4. **suspends with a ready action payload** that the Triggers service performs.

The engine never opens the socket itself — it assembles the action and hands it off. New connectors are manifests, not engine code.

---

## Inputs & outputs

**Consumes (NATS JetStream):**

| Stream / subject | Purpose |
|---|---|
| `WORKFLOW-EXECUTION` · `…workflow.execution.>` | Execution commands, routed **by a `mode` field** (`newInstance`, `signal`, `signalOrStart`, `subworkflow`) — never by subject. From the router and from subworkflows. |
| `WORKFLOW-RESUME` · `…workflow.resume.>` | Callback / timer / signal results that wake suspended runs. |
| `WORKFLOW-SCHEDULE` · `…workflow.schedule.fired` | NATS Schedule timer fires (delay / retry / wait expiry). |

**Produces (NATS):** `…trigger.workflow.execute` → **Triggers** (plugin dispatch + `trigger_event`); `…workflow.js.code` → **JS Workflow Execution** (for `code` nodes); per-instance subworkflow executions; `workflow.state.>` lifecycle events (`WORKFLOW-STATE`) → the in-service **archiver**; and on terminal, `…events.workflow` (`EVENTS-WORKFLOW-LOGS`) → the **Events** service for permanent storage.

**HTTP** (port `5007`, under `/api/v1`): `workflow_definitions`, `workflow_instances` (incl. `POST /:id/execute`), `workflow_executions`, `plugins` (incl. `/enabled`), `load_options` (dynamic plugin dropdowns).

---

## Storage

An execution moves through **three tiers** — and the runtime itself **only ever writes NATS KV and NATS streams; it never touches MongoDB**. The `archiver` (the sole MongoDB writer) consumes the `workflow.state.>` lifecycle events (`WORKFLOW-STATE`) and persists them.

| Tier | Where | Role |
|---|---|---|
| **Live working state** | **NATS KV** (`exec.{uuid}`, native CAS via revision — **no Redis**) | The running execution; deleted from KV once terminal. |
| **Hot history (UI/query)** | **MongoDB** (`dev-workflow`), via the archiver | A lightweight record on `created`, status updates on `waiting`/`resumed`, the full document on terminal — kept under a **TTL** so the console can list and query recent runs. |
| **Cold / permanent record** | **Events service → ClickHouse** (`EVENTS-WORKFLOW-LOGS`) | On terminal, the archiver also publishes the finished execution to the Events service for durable, analytical storage. |

Definitions, instances, and plugin manifests are served from a **TieredCache** (L0 RAM → L1 disk) backed by **MongoDB**; definition blobs live in **MinIO/S3**.

---

## Tested coverage

Every capability on this page is exercised by Go unit tests **and** a full end-to-end DAG suite (`runtime/e2e/dag/core/`): all 17 core executors plus the plugin executor; the AND/OR/NAND/NOR groups and 18 operators; per-step KV checkpoint and suspend/resume; NATS-Schedule timers and retry with backoff; fanout/merge, the loop stack, goto edge injection, and subworkflow depth + dedup; plugin manifest resolution; and the archiver and `load_options` paths.

## Where to go next

| To dive into… | Go to |
|---|---|
| The concepts behind nodes & state | [Core Concepts → Automation](../getting-started/concepts.md#automation) |
| Where outbound actions actually fire | [Triggers](./triggers.md) |
| Where `code` nodes run | [JS Workflow Execution](./js-workflow-execution.md) |
| How events reach a workflow | [Router](./router.md) |
