---
title: MQTT Broker
description: The MQTT ingress edge of MapexOS — Eclipse Mosquitto with a single Go plugin that authenticates every connection locally off the Assets projection and turns authorized traffic into structured NATS events.
---

# MQTT Broker

The MQTT Broker is how devices that speak **MQTT** get into MapexOS. It is **not** a MapexOS microservice — it is **Eclipse Mosquitto 2.0** carrying a single in-house **Go plugin** (cgo) that turns the MQTT edge into a source of structured **NATS** events. Mosquitto handles the MQTT protocol; the plugin handles MapexOS: it **authenticates every connection locally**, authorizes every topic, and forwards every authorized message onto NATS — where [JS Execution](./js-execution.md) and the [Assets](./assets.md) health monitor pick it up.

It is the broker side of the rule you saw in [Assets](./assets.md): *Assets authors the credentials; the edge decides the connection.* This is that edge.

> Packaged as the Docker image `mapexos/mapex-broker-mqtt` (operators pull, never build) · **Mosquitto 2.0.x** + a **cgo** plugin (libmosquitto + OpenSSL) · listeners **1883** (TCP) / **8883** (mTLS, auto-detected) · **Pebble** (L1) + **MinIO** (L2) + **NATS Core** (no JetStream produced here).

---

## One plugin, four hooks

The plugin registers exactly four Mosquitto callbacks — the whole MapexOS behavior hangs off these:

| Hook | What the plugin does |
|---|---|
| `BASIC_AUTH` | Authenticate the connection against the Assets auth projection. |
| `ACL_CHECK` | Authorize every PUBLISH / SUBSCRIBE by topic. |
| `MESSAGE` | Forward every authorized publish onto NATS. |
| `DISCONNECT` | Emit a presence advisory. |

Mosquitto 2.0.x exposes no `CONNECT` hook, so a device coming **online** is derived from the **auth-success** edge — the broker never needs a separate connect signal.

---

## Zero-HTTP auth on the warm path

Every connection is authenticated **locally** — there is no per-connection call back to Assets. The plugin resolves the device's credentials through a **three-tier, self-healing cache**:

```
 CONNECT → L1 Pebble (embedded KV on disk, survives restarts)
              ↓ miss
           L2 MinIO   (mapex-asset-auth/{assetUUID}.json — the projection Assets writes)
              ↓ miss
           L3 HTTP    (GET /internal/asset_auth/:assetUUID → Assets)
```

Every L2 or L3 hit **warms L1**, so the warm path is pure local disk. A full miss **denies** (default-deny); a total store outage **fails closed**. L1 carries a TTL safety net (default 30 min), and a **FANOUT** consumer drops stale L1 entries the instant Assets changes a device — so the broker authenticates fast and never on stale credentials.

The credentials themselves come straight from the [Assets](./assets.md) auth projection's `mqtt` block. The broker enforces the **two mutually-exclusive modes** Assets declares:

- **password** — a local **bcrypt** compare against `passwordHash` (run on the broker thread, no HTTP),
- **cert** — equality of the client certificate's serial against `currentCertSerial`.

A password-mode asset presenting a cert is denied, and vice-versa; an unknown auth type is denied.

---

## A device can only touch its own topics

Authorization is a **pure-Go, allocation-free** check on a strict contract. The MQTT **username is the bare `assetUUID`** (globally unique), and a device may use exactly two topic shapes:

| Topic | Direction |
|---|---|
| `events/{assetUUID}/{eventType}` | the device **publishes** telemetry |
| `commands/{assetUUID}/{commandType}` | the device **subscribes** to commands |

The decisive rule: the `assetUUID` token in the topic **must equal the username**. One device physically cannot publish to or read another device's topics — cross-asset access is denied at the broker, in sub-microsecond string comparison, before anything reaches NATS.

---

## Liveness over correctness — the async publisher

An authorized publish must never let a slow backend stall a device. So the plugin hands each message to a **bounded async publisher**: a non-blocking enqueue onto a fixed-size channel, drained by a worker pool that publishes to NATS. If the queue is full, the message is **dropped and counted** — never blocked, never retried inline. A slow or unreachable NATS degrades into counted drops, while **MQTT clients keep connecting and publishing** unaffected. Every payload is copied across the cgo boundary before it reaches a worker, so the broker's own buffers are never touched off-thread.

---

## What it emits — raw bytes, structured subjects

The broker **does not decode or transform** payloads — it forwards the raw MQTT bytes and lets [JS Execution](./js-execution.md) normalize them. It publishes two NATS streams (Core, fire-and-forget):

| Signal | Subject | Consumed by |
|---|---|---|
| **Ingress** | `mqtt.data.{orgId}.{assetUUID}` | [JS Execution](./js-execution.md) (subscribes `mqtt.data.>`) |
| **Presence** | `mqtt.presence.advisory` (connect / disconnect) | the [Assets](./assets.md) health monitor |

Before publishing, ingress applies safety invariants — it drops subject tokens that are illegal on NATS and payloads larger than ~900 KiB, each as a counted warning rather than a malformed event.

**Consumes:** `fanout.asset.invalidate` (from Assets) to evict stale L1 entries. The broker runs **no HTTP server of its own** — it is a plugin, not a service.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Who authors the credentials it reads | [Assets](./assets.md) |
| What normalizes the bytes it forwards | [JS Execution](./js-execution.md) |
| The LoRaWAN counterpart at the edge | [LNS](./lns.md) |
