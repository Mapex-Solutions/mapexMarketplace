---
title: What is MapexOS?
description: The self-hosted operating layer for your connected operation — turn every
  signal from devices, gateways, APIs and business systems into one governed stream of
  events you can ingest, automate, audit and scale, on infrastructure you control.
---

# What is MapexOS?

> **The operating layer for your connected operation.**
> IoT-first, but not limited to IoT — MapexOS doesn't see devices, it sees **Assets**:
> any source, any protocol, one governed model.
>
> **Connect. Automate. Scale.**

---

## The short answer

**MapexOS is a self-hosted, multi-tenant platform that turns every signal your operation
produces — sensors, gateways, APIs, business systems — into one governed stream of events
you can ingest, normalize, route, automate, and audit, end to end.** One platform, running
on infrastructure you control, in place of a stack of tools stitched together.

It is built for organizations that have outgrown stand-alone tools and need a system they
can actually *operate*: with identity, permissions, multi-tenancy, durability, and an audit
trail as first-class parts of the platform — not afterthoughts bolted on around it.

---

## The problem it removes

A serious connected operation usually ends up as seven things wired together: one tool to
ingest, another for identity, a database for rules, a queue for events, something for
automation, a dashboard, and a pile of glue code holding it upright. Each is strong at its
slice — and every one is a system to run, secure, integrate, and keep in sync.

That works until it doesn't: integrations rot, business rules end up living in three places,
multi-organization structure is modeled per project, and no one can answer an audit request
without a spreadsheet exercise.

**MapexOS collapses that into one architecture** — the same scalability across every
capability, no duplicated data, and no integrations for your team to hand-build and maintain.

---

## How it works — one pipeline, end to end

Every signal, whatever its source, follows the same governed path. Each stage is a real,
running service.

| Stage | What it does for your operation |
|---|---|
| **Ingest** | Authenticated intake over **HTTP**, **MQTT**, and **LoRaWAN** (network-server overlay on The Things Stack). Every source is an authenticated, governed entry point. |
| **Transform** | Per-Asset **decode → validate → convert** logic runs in sandboxed JavaScript, turning any vendor's payload into one normalized, typed event — no custom code branch per device. |
| **Route** | Events are matched against your rules and delivered only to the systems that should see them — signal, not noise. |
| **Automate** | A **durable workflow engine** and outbound triggers turn events into action: tickets, notifications, escalations, device commands. |
| **Store** | Every event is retained in a typed, queryable store with **per-Asset retention** — a complete, searchable audit trail by default. |

---

## What you actually get

### Automate the operation — not just alerts

The workflow engine is a real, durable runtime, not a thin rule box. It models genuine
operational logic — **nested AND / OR / NAND / NOR conditions** with 18 operators,
**persistent and ephemeral variables**, loops, sub-workflows, timers, fan-out/merge, and
**installable plugin nodes**. Flows **survive process restarts** by checkpointing each step,
and stay readable at scale with **Goto links** instead of wires crossing the canvas. The same
engine that opens a maintenance ticket from a vibration trend can escalate a high-value order
with a failed payment — one rulebook across IoT and business systems.

### Know the state of everything

Every Asset reports **online / offline and last-seen natively**, computed from heartbeats and
broker presence. Operations sees what's actually live without building heartbeat plumbing.

### Operate many tenants from one platform

Every event, Asset, template, and rule belongs to an **organization** in an unbounded
hierarchy (vendor → customer → site → building → floor → zone). Permissions, retention, and
visibility follow that tree through role-based access control — **one deployment serves
dozens of customers**, with zero shared visibility between them.

### Audit-ready by default

Every event and every action is retained and queryable by Asset, by site, by time window.
Compliance evidence becomes a query, not a quarterly project.

### Secure and sovereign

Secrets are **envelope-encrypted** and never leave in an API response. Devices get **per-Asset
X.509 certificates** from an internal CA. Customer code runs in **isolated sandboxes** with
hard limits. And because MapexOS is **self-hosted**, your operational data never leaves
infrastructure you control.

### Onboard in days, not quarters

A reusable **Asset Template** converts whatever a source sends into a standard event, so a new
vendor or model is a *configuration*, not a new code branch — one template serving hundreds of
Assets across hundreds of sites.

---

## Proof: what changes in practice

A regional cold-chain operator runs 400 freezers across 60 sites. Before MapexOS, every
vendor's sensor had its own portal, alerts fired on every door opening, and auditors assembled
evidence by hand each quarter.

On MapexOS, one **Asset Template** normalizes every brand into the same temperature event; a
**workflow** correlates temperature with door state and only escalates after a real violation
persists; a **trigger** opens one ITSM ticket per incident; the **event store** keeps every
evaluation — searchable and defensible; and a second operator runs on the **same deployment**
with full isolation.

The result operations teams report: audit requests answered in an afternoon, sharply lower
alert noise, and vendor onboarding measured in days. **That repeatable outcome is what MapexOS
exists to deliver.**

---

## Where MapexOS fits

| If you have been using… | …MapexOS adds |
|---|---|
| **A traditional IoT platform** (ingestion + dashboards + alerts) | Durable workflows, multi-tenant governance, plugin extensibility, and business-event automation — not just sensor charts. |
| **A DIY pipeline** (broker + functions + a database + glue) | Everything you'd build anyway: identity, RBAC, audit, secrets, reusable templates, workflow durability, and an operator UI. |
| **A generic event broker** (NATS, Kafka, RabbitMQ) | The application layer above transport: ingestion, normalization, routing, durable automation, audit, identity. |
| **A no-code automation tool** (Zapier, Make, n8n) | Multi-tenant isolation, durable workflows that survive restarts, sandboxed code, self-hosting, and vault-backed credentials. |

MapexOS doesn't replace your broker, database, or dashboard. It's the governed layer that
connects them and gives operators one place to control what flows between them.

---

## Run it your way

MapexOS is delivered as a monorepo of **ten backend services** and a **Vue 3** console,
with satellite repositories for the MQTT broker, the LoRaWAN network server, and the shared
SDK. The whole stack boots with **one command** and ships **multi-arch** — from a Raspberry Pi
at a remote site to an `x86_64` cluster in your data center.

It is distributed under the **Business Source License (BSL 1.1)**: read the source, modify it,
and run it in production for your own organization; on the Change Date it converts to Apache 2.0.

---

## Where to go next

| If you are… | Start here |
|---|---|
| Evaluating MapexOS | [Why MapexOS?](./why-mapexos.md) — the pain it solves and the decisions behind it |
| Learning the vocabulary | [Core Concepts](../getting-started/concepts.md) |
| Mapping the architecture | [Architecture Overview](../architecture/overview.md) |
| Ready to run it | [Quickstart](../getting-started/quickstart.md) |
