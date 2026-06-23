---
title: JS Workflow Execution
description: The compute substrate for workflow code nodes — runs each node's JavaScript in a hardened V8 isolate and hands the result back to the workflow engine to resume the DAG.
---

# JS Workflow Execution

JS Workflow Execution is the **compute substrate for workflow `code` nodes**. When a [workflow](./workflow.md) reaches a node that runs custom JavaScript, the engine doesn't execute it inline — it **suspends the node**, dispatches the script here, and parks. This service runs the code in a hardened V8 sandbox and **publishes the result back to the workflow's resume subject**, waking the exact node so the DAG continues. It is the same suspend → run elsewhere → resume contract you saw with [Triggers](./triggers.md), applied to in-graph computation.

It shares its **sandbox engine** with [JS Execution](./js-execution.md) — the same `isolated-vm` V8 isolates on a Piscina worker pool, the same hard limits (**32 MB heap**, **10-second timeout**, isolate recycled every 10,000 runs, no access to Node, the network, or the filesystem). What differs is the **contract**: this service runs *one arbitrary user script per node* with a workflow's state in scope, not the fixed decode→validation→transform pipeline. As on its sibling, the code runs on **full V8** — modern **ECMAScript (ES2015+)** is native (classes, arrow functions, destructuring, `Map`/`Set`, the standard built-ins), bounded only by the isolate's **32 MB** budget.

> **Port** `8001` · **Node.js + TypeScript** · **`isolated-vm`** V8 isolates on **Piscina** · driven entirely by NATS — no functional HTTP surface · stateless.

---

## The code-node contract

Each request injects four globals into the isolate, giving the script the running workflow's context:

| Global | What it holds |
|---|---|
| `event` | The event that drives the run. |
| `state` | The workflow's persistent state. |
| `inputs` | The node's resolved inputs. |
| `nodes` | The outputs of prior nodes. |

The script returns two things — **`output`** (consumed by downstream nodes) and **`statePatch`** (merged into the workflow's state). There are two ways to return them, and the engine accepts both:

```js
// explicit
result = { output: { celsius }, statePatch: { lastReading: celsius } };

// or convenience — a bare object return is wrapped as { output: <value>, statePatch: {} }
return { celsius };
```

The service then publishes a `WorkflowScriptCallback` — `{ status: 'success' | 'error', output?, statePatch?, error? }` — to the `callbackSubject` the runtime supplied, on the workflow **resume** stream. **Exactly one callback is published per request**, success or error (including a `SCRIPT_NOT_FOUND` when a node's source is missing), so a workflow node never hangs waiting on a result that isn't coming.

---

## Faster cold starts — V8 bytecode caching

Unlike the per-event asset pipeline, this service **persists compiled V8 bytecode across cold starts**. On a fresh compile the worker emits the script's `cachedData`, which is stored (fire-and-forget) in a tiered **bytecode cache** (RAM → disk → MinIO); the next time a worker compiles that node — even in a different pod after a restart — it passes the bytecode back to V8 for a **~1–5 ms** compile instead of **~10–50 ms**. Bytecode is **definition-scoped** and reused across every running instance of a workflow; on a V8 version mismatch it silently falls back to a fresh compile. Source and bytecode are fetched **in parallel** to keep the cold path short.

The node's JavaScript source itself comes from a tiered cache (`{orgId}/{workflowId}/scripts/{nodeId}`), kept fresh by a **FANOUT** broadcast the workflow engine emits when a definition changes — so editing a code node takes effect without a restart.

---

## Failure handling

The engine treats misbehaving code as routine, with the same discipline as the asset executor:

- A script that **exhausts the 32 MB heap** disposes its isolate; the worker reports the out-of-memory condition and the message is **NACK'd for backoff retry** with a fresh isolate.
- **Any other script error** publishes an *error* callback to the workflow and **ACKs** the message — the workflow node receives the failure on its `error` edge rather than the script being retried forever.

---

## Inputs & outputs

**Consumes (NATS JetStream):** `workflow.js.code` (stream `JSWORKFLOWEXECUTOR-CODE`, load-balanced queue consumer) — one message per code node firing; and `fanout.workflow.definition.invalidate` (FANOUT) to drop stale cached sources/bytecode.

**Produces (NATS):** a `WorkflowScriptCallback` to the runtime-supplied `callbackSubject` on the workflow **resume** stream. The service hard-codes no outbound subject — it echoes back to wherever the runtime told it to.

It exposes **no functional HTTP API** — only `/health` and `/metrics`. Every interaction is over NATS, driven by the workflow engine.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| The engine that dispatches these nodes | [Workflow Engine](./workflow.md) |
| The sibling sandbox for device payloads | [JS Execution](./js-execution.md) |
