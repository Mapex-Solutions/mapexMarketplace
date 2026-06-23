---
title: Core Concepts
description: The vocabulary of MapexOS — the building blocks every operator, integrator, and architect needs to know before going deeper.
---

# Core Concepts

This page defines the building blocks of MapexOS — the words used everywhere else in the documentation. Read it once before going deeper; come back when a term is unclear.

> **What's real today.** MapexOS ingests over **HTTP, MQTT, and LoRaWAN**, runs the transform → route → automate → store pipeline, and ships the full multi-tenant governance stack. Additional protocols (e.g. CoAP) and the catalog/AI layers are on the roadmap.

---

## The five layers of MapexOS

Everything in MapexOS belongs to one of five layers. The rest of the page expands each one.

| Layer | What it is | Key concepts |
|---|---|---|
| **Tenancy & identity** | Who can do what, scoped to which part of the organization | Organization · User · Group · Role |
| **Data sources** | What produces events | Asset · Asset Template · Dynamic Fields (EVA) |
| **Event pipeline** | The path every event takes | Standard Event · Router · Route Group |
| **Automation** | What happens after an event arrives | Workflow · Node · Trigger · Plugin |
| **Storage & secrets** | Where state and credentials live | Events Service · Persistent Log · Credential · Vault |

---

## Tenancy & identity

### Organization

An **organization** is a unit of ownership inside MapexOS. Every event, asset, template, workflow, and rule belongs to exactly one organization. Permissions, retention, and visibility follow that boundary.

Organizations form a **hierarchical tree**, and each node carries a denormalized `pathKey` — a prefix that encodes its place in the tree, so the platform can resolve "everything under this org" with a fast prefix query instead of a recursive walk. The level labels are conventions; the platform doesn't care what you call them:

```
Vendor → Customer → Site → Building → Floor → Zone
```

In every case the rules are the same:

- Each level inherits context from its parent.
- Templates, rules, and assets defined at a parent organization can be **shared down the tree** to descendants.
- Retention (TTL), permissions, and audit boundaries are enforced **at the organization that owns the data**.

### User, Group, Role

- A **user** is an individual identity that authenticates and operates the platform. A user can belong to several organizations, with different roles in each.
- A **group** is a named collection of users; permissions granted to a group apply to every member.
- A **role** is a reusable set of permissions, **defined inside an organization** — the *Operator* role at *Customer A* and the *Operator* role at *Customer B* are separate definitions. That's what keeps tenants from leaking permissions into one another.

The complete access grant is a triple — **assignee + role + organization** — and role inheritance down the tree is gated per organization by a `rolePolicy` (a parent's roles reach a descendant only when that descendant opts in):

| Assignee (user or group) | Role | Organization (scope) | Effective access |
|---|---|---|---|
| João | Operator | *Site: São Paulo Plant* | Operate assets at the São Paulo Plant |
| *Maintenance Group* | Technician | *Building: Main Warehouse* | Every member maintains assets in that building |

---

## Data sources

### Asset

An **Asset** is anything that produces events into MapexOS. The name is deliberately broader than "device":

- An IoT sensor, a gateway, or a machine reporting its state.
- A third-party platform pushing webhooks, or a custom application emitting business events.
- A logical area or zone that aggregates other assets.

Every asset belongs to one organization, uses one **Asset Template**, and carries its identity, optional location in the org tree, custom metadata, and connectivity credentials.

### Asset Template

An **Asset Template** defines how a *class* of asset is integrated — so onboarding a new vendor or model is a **configuration**, not a new code branch. One template can serve hundreds or thousands of assets across many organizations.

Each template carries three scripts that run **in sequence** for every incoming payload — executed in a sandboxed V8 isolate — plus an optional test script used while authoring:

| Script | Purpose |
|---|---|
| **Preprocessor** | Decode / normalize the transport format (parse binary, Base64, decompress) — optional |
| **Validation** | Reject malformed input (required fields, ranges, schema) |
| **Conversion** | Produce the Standard Event — map the vendor payload to the MapexOS contract |

```
Raw payload → Preprocessor → Validation → Conversion → Standard Event
```

Templates can be a **system** template (shipped with MapexOS), an **organization** template (private), or a **shared** template (created by an org and shared down the tree).

### Dynamic Fields (EVA)

A template can declare **Dynamic Fields** — typed values extracted from the payload. Each field has a logical **name**, a **type**, a **payload path**, and a stable, immutable numeric **`fieldId`** (a template can hold up to 200):

```json
{ "sensor": { "readings": { "temp": 23.5, "humidity": 65 } } }
```

| fieldId | Field name | Type | Payload path |
|---|---|---|---|
| 1 | `temperature` | number | `sensor.readings.temp` |
| 2 | `humidity` | number | `sensor.readings.humidity` |

The `fieldId` is the key idea: events are stored in **typed columns keyed by that id**, so a value keeps its meaning forever even if the label changes. This is what makes events from different vendors **queryable as one set** — you ask for `temperature` across the whole fleet, regardless of how each raw payload named it.

---

## Event pipeline

### Standard Event

Every payload that enters MapexOS — over HTTP, MQTT, or LoRaWAN — is converted by its template into a single **Standard Event**. This is the exact, schema-validated contract the rest of the platform operates on:

```ts
{
  eventType: string,        // classification, e.g. "telemetry.temperature"
  eventId: string,          // unique id for this event
  data: Record<string, any>,// the normalized payload
  metadata?: Record<string, any>, // optional context (org, asset, location, correlation ids)
  created: string           // ISO 8601 timestamp
}
```

Routing, automation, storage, and audit all operate on Standard Events — never on raw vendor payloads.

### Router & Route Group

The **Router** receives Standard Events and dispatches each to one or more destinations, **in parallel**, based on configured rules. A single event can be persisted, sent to a workflow, and forwarded to a trigger in the same step.

A **Route Group** is a named set of routing rules attached to an asset's configuration. Each rule is a **match condition** plus a **destination kind**:

- A **match condition** is a list of clauses — a dot-path field (e.g. `data.temperature`), one of eight operators (`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`), and a value — combined with a policy of `all` (AND) or `any` (OR).
- A **destination kind** routes the matched event to a destination such as the **events store**, a **workflow**, a **trigger**, or a **lake-house** sink. Every routing decision also produces a **routing-history** record.

```
Always                     → events store   (persist)
If data.temperature ≥ 80   → workflow        (handle escalation)
If data.alert in ["high"]  → trigger         (notify operations)
```

---

## Automation

This is where the platform turns telemetry into business action.

### Workflow

A **workflow** is a directed acyclic graph (DAG) of **nodes** that runs when fired by an event or an API call. Workflows are **durable**: every step is checkpointed to a NATS key-value store, so a workflow that is mid-flight when a worker crashes **resumes from the last node**, not from the beginning. Timers, retries, and waits survive restarts too. Stateful logic — conditions, counters, cooldowns, persistence windows — lives entirely inside the workflow (in the `condition` and `set_state` nodes); there is no separate rules service.

A workflow has three entities, in increasing concreteness: a **Definition** (the DAG blueprint), an **Instance** (a definition bound to specific inputs), and an **Execution** (one actual run, with its own state, status, and history) — `Definition 1→N Instance 1→N Execution`.

**Workflow state** comes in two flavors, and the distinction matters:

| State | Scope | Use |
|---|---|---|
| **State** (variables) | **Persistent** across the whole run | Counters, accumulated values, flags — mutated by `set_state` (set / increment / append / remove) |
| **NodeStates** | **Ephemeral**, per node | Loop index, retry attempt, wait info — cleared when the node completes |

### Node

A **node** is one unit of work in a workflow. MapexOS ships **17 core node types** and accepts new ones as plugins:

| Group | Nodes |
|---|---|
| **Entry & exit** | `trigger_event`, `start`, `end` |
| **Logic & data** | `condition`, `switch`, `set_state`, `log`, `code` (user JavaScript in a V8 isolate) |
| **Flow control** | `fanout`, `merge`, `sequence`, `loop`, `goto`, `subworkflow` |
| **Time & signals** | `delay`, `wait_signal`, `wait_for` |

Two of these are signature features:

- **Conditions** (`condition`, `switch`, `wait_for`) evaluate **nestable AND / OR / NAND / NOR groups** with 18 operators (comparison, string, datetime, range) — enough to express real operational logic, not just a single threshold.
- **Goto** lets you reference a step by name through paired **sender/receiver "portals"** instead of dragging a wire across the canvas, so a large DAG stays readable.

### Trigger

Two things in MapexOS are called "trigger" — they are not the same:

| Term | What it means |
|---|---|
| **Workflow trigger** (`trigger_event` node) | The entry node of a workflow — defines what fires it |
| **Trigger** (executor) | A standalone **outbound action**, fired from the router or from inside a workflow |

A trigger executor delivers an action over one of **eight built-in transports**:

| Category | Executors |
|---|---|
| **Technical** | HTTP, MQTT, RabbitMQ, NATS, WebSocket |
| **Communication** | Email, Microsoft Teams, Slack |

### Plugin

A **plugin** adds new workflow node types or trigger types **without rebuilding the frontend**. A plugin is a declarative **manifest** (credentials, dynamic option-fetching, node types and their fields, operation templates); the workflow editor loads it at runtime and renders its form automatically. The model is inspired by n8n's node ecosystem, adapted to MapexOS's multi-tenant, durable-workflow constraints — so a new connector (an ITSM, a payment gateway, a database) is a manifest, not a platform release.

---

## Storage & secrets

### Events Service & Persistent Log

The **Events Service** is the terminal sink: it consumes Standard Events from the NATS backbone and stores them in **ClickHouse**, in the typed EVA columns described above, with **retention (TTL) configurable per organization**. Because every evaluation and action is also recorded, the result is a **persistent log** — an immutable, queryable trail of *what happened*, searchable by asset, by site, by time window, by outcome. Compliance evidence becomes a query, not a manual assembly.

### Credential & Vault

A **credential** is a reference to a secret — an API token, a username/password, a private key, a certificate. The **Vault** stores credentials with **envelope encryption** (a per-record key wrapped by a master key, AES-256-GCM) and never serializes plaintext into an API response. It also issues platform key material and signs **per-asset X.509 certificates** for device connectivity. The data plane (workflows, triggers, templates) only ever sees references like `{{credential.<id>.<field>}}`, resolved at runtime.

---

## How the concepts connect

```text
Organization
  ├── Users / Groups / Roles                (who can do what, where — scoped by pathKey)
  └── Assets
        └── use → Asset Template
                  ├── scripts → Preprocessor → Validation → Conversion
                  └── defines → Dynamic Fields (EVA, typed, fieldId-keyed)

Event flow
  Raw payload
    → Asset Template pipeline
    → Standard Event { eventType, eventId, data, metadata, created }
    → Router  (fan-out by Route Group rules: dot-path + operators, all/any)
        → Events Service     (persist in ClickHouse + EVA query + per-org TTL)
        → Workflow           (durable DAG of nodes, checkpointed)
              ├─ condition / switch   (AND/OR/NAND/NOR groups, 18 operators)
              ├─ set_state            (persistent State) · NodeStates (ephemeral)
              ├─ code                 (user JS in a V8 isolate)
              ├─ goto                 (sender/receiver portals)
              └─ trigger_event / plugin nodes  (act)
        → Trigger            (direct outbound action — 8 executors)

Secrets
  Workflow / Trigger / Template
    → {{credential.<id>.<field>}}
        → Vault (envelope-encrypted; device X.509 PKI; never plaintext on the data plane)
```

---

## Summary

| Concept | Purpose |
|---|---|
| **Organization** | Hierarchical multi-tenant unit (with `pathKey`); the boundary for permissions, retention, and visibility |
| **User / Group / Role** | Identity and access control, scoped per organization |
| **Asset** | A data-producing entity (sensor, machine, application, area) |
| **Asset Template** | Reusable integration: Preprocessor → Validation → Conversion + Dynamic Fields |
| **Dynamic Fields (EVA)** | Typed, `fieldId`-keyed extractions — makes heterogeneous events queryable as one set |
| **Standard Event** | The normalized `{eventType, eventId, data, metadata, created}` contract every service operates on |
| **Router / Route Group** | Fan-out engine; rules match on dot-path fields (8 operators, all/any) to destination kinds |
| **Workflow** | Durable DAG of nodes that automates the response to an event |
| **State / NodeStates** | Persistent run variables vs ephemeral per-node data |
| **Node** | One unit of work — 17 core types (condition, switch, set_state, code, goto, fanout, loop, …) plus plugins |
| **Trigger** | A workflow's entry node, or a standalone outbound action (8 executors) |
| **Plugin** | Manifest-served extension that adds nodes/triggers without a frontend release |
| **Events Service / Persistent Log** | ClickHouse storage with per-org retention and an immutable audit trail |
| **Credential / Vault** | Secret references resolved by an envelope-encrypting vault that also runs the device PKI |

---

## Where to go next

| To dive into… | Go to |
|---|---|
| The event pipeline & store | [Architecture → Events](../architecture/events.md) |
| Identity & multi-tenancy | [Architecture → mapexIam](../architecture/mapexiam.md) |
| Assets, templates & device backend | [Architecture → Assets](../architecture/assets.md) |
| How the platform is engineered | [Architecture Overview](../architecture/overview.md) |
