---
title: Assets
description: The source of truth for everything connected to MapexOS — the asset and template model, the EVA field schema, the device PKI and presence control plane, and the read-model cache that every other service is rebuilt from.
---

# Assets

Assets is the **source of truth** for everything connected to MapexOS. It owns *what* an asset is, *how* its payloads become normalized events, *how* it authenticates, and *whether it is online right now*. It does not ingest, transform, or route events itself — it **configures the entities the rest of the pipeline acts on**, and it **authors the cache that every other service reads**. When the [MQTT broker](./mqtt-broker.md), [LNS](./lns.md), [Router](./router.md), and [JS execution](./js-execution.md) need to know something about an asset, they read it from their own tiered cache — and **the bottom of every one of those caches falls back to Assets**, the one place that can always reconstruct the truth.

It carries two responsibilities: **the model** (Assets + Asset Templates) and the **device control plane** (PKI + presence/health).

> **Port** `5002` · **Go** (DDD + Hexagonal) · **MongoDB** (write model) + **MinIO/S3** (read-model projections) + **Redis** (health hot-state) · **mapexVault** for the signing CA and key-encryption key · four modules: `assets`, `assettemplates`, `healthmonitor`, `mqttcerts`.

---

## Asset & Template — onboard by configuration, not code

The model has two halves:

- An **Asset Template** is a reusable blueprint: its **classification** (category / manufacturer / model / version), the `assetIdPath` that locates the device id in a payload, the **transform scripts**, and the **EVA field schema**. Two scripts are required — `scriptValidator` and `scriptConversion` — and two are optional — `scriptProcessor` (preprocess) and `scriptTest` (for the in-console test runner).
- An **Asset** is one device instance bound to a template, carrying its protocol identity, route groups, health config, and active certificate. `assetUUID` is its stable, cross-platform business id.

One template serves thousands of assets across hundreds of sites. A new vendor or model is a **template configuration**, never a new code branch — this is what turns device onboarding from an engineering task into a setup task.

---

## EVA dynamic fields — where the `fieldId` is born

A template declares its dynamic fields, and **this is the origin of the typed columnar storage** you saw in [Events](./events.md). Each field is assigned a numeric id with a hard contract:

- `fieldId` is a `uint16`, **immutable** once assigned (a per-template `nextFieldId` counter only ever increments — ids are never reused),
- a declared `type` (`number` · `string` · `bool` · `date` · `geo`),
- a soft-delete `status` (a removed field is deprecated, never physically deleted),
- up to **200 active fields** per template.

That immutability *is* the contract between Assets and Events: Assets assigns the `fieldId`, Events stores values under it in its typed EVA maps. Because ids never move, history stays coherent forever.

---

## The source of truth — and the cache every service is rebuilt from

This is the architectural center of gravity. On **every change that affects how an asset is read or authenticated**, Assets writes two denormalized projections to MinIO/S3:

| Projection | Path | For |
|---|---|---|
| **Full read-model** | `mapex-assets/{orgId}/{assetUUID}.json` | Router, JS-execution, Events — full asset context. |
| **Slim auth projection** | `mapex-asset-auth/{assetUUID}.json` | the MQTT broker plugin and LNS — just what's needed to authenticate. |

Then it fires a **FANOUT invalidation** on NATS so every replica of every consumer drops its stale copy. The read-model is always written **before** the invalidate, so a service that re-fetches on the invalidation sees fresh data, never a gap.

```
                 ┌──────────── Assets (source of truth) ────────────┐
   write change → │  MongoDB (write model)                          │
                 │     │ project                                     │
                 │     ▼                                             │
                 │  MinIO  mapex-assets/… · mapex-asset-auth/…       │
                 │     │ then                                        │
                 │     ▼                                             │
                 │  FANOUT invalidate ─────────────────────────────┐│
                 └──────────────────────────────────────────────────┘
                                                                    ▼
   consumer cache:   L0 RAM → L1 disk → L2 MinIO → L3 HTTP fallback → Assets
                     (the fallback always reconstructs from the source of truth)
```

Every consumer — broker, LNS, Router, JS-execution — runs its own `TieredCache`, and **the last tier of all of them falls back over HTTP to Assets** (`GET /internal/assets/:assetUUID`), which repopulates the cache inline. No matter how cold a cache gets, there is always one authoritative place to rebuild it from.

---

## Authentication happens at the edge — Assets never authenticates

Assets **authors** the credentials but **never validates a connection** itself. The slim auth projection is already at the edge, so the servers that terminate the protocol decide every connection **locally**:

- The **[MQTT broker](./mqtt-broker.md) plugin** decides every MQTT `CONNECT` from the projection's `mqtt` block alone — bcrypt-comparing the presented password against `passwordHash`, or matching the client certificate's serial against `currentCertSerial`.
- The **[LNS](./lns.md)** authenticates LoRaWAN joins from the projection's `lorawan` block — the device identity and its envelope-encrypted keys (gateways authenticate by certificate serial).

There is **no per-connection HTTP callout to Assets** — the only edge→Assets HTTP path is the L3 cache-miss fallback above. Authentication is fast, made where the connection lands, and keeps working even while Assets is briefly unavailable.

---

## Device PKI control plane

Assets is a local certificate authority for devices (`mqttcerts` module). It issues **X.509 client certificates** — **ECDSA P-256, 128-bit serial, client-auth extended key usage** — signed by an **intermediate CA fetched once from [mapexVault](./vault.md) and held in RAM** (`atomic.Pointer`, never written to disk; a `caReady` gate returns `502` until the CA is loaded, with backoff retry on failure).

- **One active certificate per asset** — re-issuing requires `force=true`, otherwise `409`.
- The **certificate and private key PEM are returned exactly once** at issue time and **never persisted** — the asset keeps only metadata (serial, fingerprint, subject CN, issued/expires).
- **Revocation is a hard delete** plus a tamper-evident audit row that auto-expires after **30 days** (a MongoDB TTL index).

The same machinery issues certificates for LoRaWAN gateways.

---

## Connectivity health & presence

`healthmonitor` is a real near-real-time engine tracking each asset's **online / offline / last-seen**. It learns liveness two ways:

- **Heartbeats** on `asset.heartbeat.>` — *implicit* (js-execution emits one per data event) or *explicit* (an HTTP `POST /api/v1/heartbeat`, or MQTT broker presence advisories for MQTT assets) — refreshing a last-seen timestamp in Redis.
- A **periodic scan** that marks an asset offline once it has been silent past its threshold for `requiredMisses` consecutive sweeps (`cutoff = now − threshold`).

It runs in three modes per asset — **disabled**, **monitor-only** (persist state), or **monitor + route** — and on each transition it publishes two things: an `asset_status_save` to [Events](./events.md) (the queryable connectivity history) and, when the asset configures them, a `route.execute` with `eventSource = healthStatus` to the [Router](./router.md) (which routes presence changes only to `trigger` / `workflow`). Transitions are **exactly-once across replicas** (an atomic Redis `SREM`), with an anti-race guard for multi-broker reconnects. Enabling health monitoring requires at least one offline/online route group — a fail-fast `422` otherwise.

---

## Multi-protocol identity & secrets

An asset speaks one of three protocols, and **no secret is ever stored in plaintext**:

| Protocol | Identity & secret handling |
|---|---|
| **HTTP** | No device credential — admitted at the [HTTP Gateway](./http-gateway.md) by its DataSource. |
| **MQTT** | `password` **xor** `cert` (mutually exclusive). Passwords are **bcrypt-hashed**; certs come from the PKI plane. |
| **LoRaWAN** | Device (OTAA/ABP) or gateway. Device root/session keys are **envelope-encrypted with a mapexVault KEK** — only the four envelope fields persist; the runtime consumer is the [LNS](./lns.md). |

Plaintext (a generated password, a key, a cert PEM) is shown to the operator **once** at create/issue time and is irretrievable afterward.

---

## Inputs & outputs

**HTTP (in):** `/api/v1/assets` and `/api/v1/asset_templates` CRUD (JWT, permission- and coverage-gated); `/api/v1/mqtt_certs` and `/api/v1/gateway_certs` (issue/revoke, gated on CA readiness); internal API-key routes for the L3 cache fallback (`GET /internal/assets/:assetUUID`, `…/asset_auth/:assetUUID`, `…/scripts/:assetUUID`).

**NATS (consumed):** `asset.heartbeat.>`, MQTT presence advisories, the periodic health scan, and classification-name updates.

**NATS (produced):** `fanout.asset.invalidate` and `fanout.template.invalidate` (cache coherence), `events.asset_status_save` (connectivity history → Events), and `route.execute` (`eventSource=healthStatus` → Router) on health transitions.

**Object storage:** the two MinIO projections above, plus template scripts (`{orgId|mapexos_public}/{templateId}.json`) for JS-execution.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Where templates' scripts run | [JS Execution](./js-execution.md) |
| Where dynamic fields are stored & queried | [Events](./events.md) |
| Where presence changes are routed | [Router](./router.md) |
| The CA and KEK behind the control plane | [Vault](./vault.md) |
