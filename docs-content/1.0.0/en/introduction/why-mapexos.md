---
title: Why MapexOS?
description: The pattern we kept seeing — and the decision to build one governed
  platform instead of re-assembling the same multi-tenant plumbing from separate
  tools, one more time.
---

# Why MapexOS?

> **IoT-first, but not limited to IoT.**
> MapexOS doesn't see devices or sensors — it sees **Assets**.
> Any source. Any protocol. One abstraction.

This page is the *why* — the thinking behind the platform. For the *what*, start with
[What is MapexOS?](./what-is-mapexos.md). If you're deciding whether MapexOS fits the kind
of operation you run, keep reading.

---

## The pattern we kept seeing

After years building IoT solutions, distributed backends, and operational platforms, one
pattern kept repeating: whenever the goal was a **complete** operation, the open-source
ecosystem had an excellent tool for every individual piece — and each one solved only a
**piece**.

One tool is strong at connectivity. Another at dashboards. Another at automation flows.
Another at identity. Another at storage. All genuinely good at their slice.

But a real enterprise operation needs them *together and governed* — integrated,
multi-organization, with permissions, templates, assets, persistent logs, payload
validation, business rules, observability, APIs, a broker, and the capacity to scale. No
single tool covers that, so the answer was always the same: **assemble the pieces.**

---

## Assembling works — up to a point

For a small operation, composing best-of-breed tools works. You connect an ingestion service
to a flow engine, put an identity server in front, keep rules in a database, add an
automation tool, and ship.

For a while it gets you somewhere. But as the operation grows — more tenants, more sites,
more devices, more integration partners — the **seams between the tools** become the work.

| A win at month 1… | …a maintenance tax by month 12. |
|---|---|
| "The flow engine wires the broker to everything." | Every integration now lives in a flow no one wants to touch. |
| "The identity server gives us auth for free." | Organizations, sites, and zones live in auth groups that map to nothing else. |
| "The automation tool handles our logic." | Business logic is now split across the automation tool, the rules database, and a script no one remembers running. |
| "We'll add multi-tenancy later." | "Later" means rewriting the identity model under load. |

None of those tools is the problem — each is excellent at what it does. The cost is in the
**space between them**, and it compounds:

- **Dependency on several runtimes** for the platform's core behavior.
- **Integrations that are hard to maintain**, because the contracts between tools were never
  explicit.
- **Business rules scattered** across three or four runtimes.
- **Organizational hierarchies that don't fit** the shape real customers have
  (vendor → customer → site → building → zone).
- **Permission models that don't compose** across the toolchain.
- **No reuse** between customers — every new customer is a new project.
- **An architecture that was assembled, never designed end to end.**

That space-between-the-tools is what MapexOS removes.

---

## The decision

There's a moment — after the third or fourth time you rebuild the same multi-tenant,
multi-site, template-driven, audit-logged platform on a different glue stack — when you stop
and ask:

> *"Why am I rebuilding the same plumbing every time?"*

That question is why MapexOS exists.

MapexOS was not started to be "another open-source IoT project." It was started to be a
**single, open, enterprise-grade platform** that ships the full cycle of a connected
operation — ingestion, validation, processing, business rules, asset organization,
observability, integration — out of the box, so the next real fleet doesn't start from zero.

---

## Four commitments we made early

Every design decision traces back to one of these.

### 1. Multi-tenancy is layer zero, not an afterthought

Real customers run multi-tenant operations: a vendor serves several customers, a customer has
several sites, a site has buildings, floors, zones. Permissions, retention, visibility, and
templates must follow that tree.

In MapexOS, every event, Asset, template, rule, workflow, and trigger carries its
organization identity (`orgId` + a hierarchical `pathKey`) from the moment it enters. There
is no single-tenant mode retrofitted later — the org tree is the first thing the platform
knows.

### 2. Workflows must survive process restarts

When a maintenance work order is half-created and a server reboots, it must finish — not
vanish. When an escalation timer is mid-flight during a deploy, it must keep counting.

MapexOS ships a **durable workflow engine built into the platform**. Each step of every
workflow is checkpointed; a crashed worker resumes mid-graph. Durability is part of the
engine, not a separate system bolted on.

### 3. The platform must extend without redeploying

Operators add integrations constantly — notifications, payment gateways, ITSM, databases.
A new integration cannot mean a frontend release.

MapexOS uses a **plugin model**: new workflow node types are described by manifests, and the
editor loads them at runtime — so a connector can appear in the editor without rebuilding the
platform.

### 4. Routing is configuration, not code

Where an event goes — storage, workflow, trigger, notification — is defined by policy, not by
a deploy. Match rules live in a database and are applied at runtime: change a threshold,
redirect a tenant's events to a new destination, add a sink — no schema migration, no rebuild.

---

## What changes for your operation

These commitments become outcomes the business sees, not just the engineering team.

| What used to be… | …becomes… |
|---|---|
| A new customer requires a new deployment. | A new customer is a new node in the organization tree. |
| A new device vendor takes a quarter to onboard. | A new device vendor is one new Asset Template. |
| Every alert is a duplicate, a false positive, or both. | Alerts respect cooldowns, persistence windows, and incident lifecycles by design. |
| Compliance evidence requires manual log assembly. | Compliance evidence is a query against the event store. |
| A new connector requires a frontend release. | A new connector is a plugin manifest. |
| Logic lives in runtimes nobody dares change. | Logic lives in workflows with versioned definitions, replayable execution, and one audit trail. |

The trade: more discipline at the platform layer, dramatically less coupling at the
application layer.

---

## Why this matters now: AI needs a foundation

Most organizations talk about "applying AI to our data." What they usually have is
**fragmented telemetry across systems that don't speak to each other.**

AI on fragmented data is, at best, noisy. Before any AI agent can be trusted to flag an
anomaly, recommend an intervention, or take an autonomous action, the events feeding it must
be ingested from every source, **normalized** into one contract, **routed** to the right
place, **governed** with explicit organization boundaries, **stored** with defensible
retention, and **searchable** across heterogeneous payloads.

That layer — clean, governed, queryable events — is the foundation AI needs.
**MapexOS is being built to be that foundation.**

---

## Where the roadmap points *(planned)*

Today MapexOS ingests over HTTP, MQTT, and LoRaWAN, runs durable workflows and the routing
engine, and ships the full multi-tenant governance stack. The direction from here:

| Direction *(planned)* | What it would give you |
|---|---|
| **More protocols** (e.g. CoAP) | Reach more low-power and constrained-network devices without new gateways. |
| **A queryable catalog over governed datasets** | Make events discoverable across tenants and time windows. |
| **Open table format for the event store** | Long-lived datasets that analytics and AI systems read directly, without a custom export. |
| **AI orchestration surfaces** | Workflow nodes that drive AI agents on top of clean, governed events. |

The thesis is intentional: **AI should operate on clean, governed, contextualized events —
not on raw, fragmented data.**

---

## A platform, not a product line

MapexOS is one open ecosystem, not a set of paid tiers.

- **One source repository** for the backend services and the frontend (`mapexOS`).
- **One deployment distribution** for self-hosting (`mapexOSDeploy`).
- **One license** — Business Source License 1.1 — covering self-hosted use, modification, and
  contribution.
- **No separate "enterprise edition" code path.** The same images run a Raspberry Pi at a
  remote site and a regional cluster behind a load balancer.

On the **Change Date**, the BSL converts to **Apache 2.0**. The only constraint today is on
commercially *hosting* MapexOS for third parties — not on running it for your own operation.

---

## When MapexOS is a great fit

MapexOS is the right platform if several of these describe you:

- You operate **multiple customers, sites, or tenants** and need real isolation between them.
- You **onboard new devices, vendors, or third-party systems frequently**, and that cost is
  too high today.
- You need **automation that survives outages** — workflows that resume after crashes, retries
  that respect downstream limits, an audit trail over every action.
- You need **one platform for both IoT telemetry and business-system integration**, not two
  stacks.
- You need to **self-host** on infrastructure you control — for compliance, cost, latency, or
  data sovereignty.
- You are building the **event foundation future AI capabilities will run on**, and you want it
  open and inspectable.

If only one or two apply, MapexOS is probably overkill — start with the minimal tool that
solves the immediate problem. If three or more apply, the cost of *not* having MapexOS is
likely already showing up in your operation.

---

## Where to go next

| If you want to… | Go to |
|---|---|
| Understand how it's engineered | [Architecture Overview](../architecture/overview.md) |
| Learn the vocabulary | [Core Concepts](../getting-started/concepts.md) |
| Run it on your machine | [Quickstart](../getting-started/quickstart.md) |
