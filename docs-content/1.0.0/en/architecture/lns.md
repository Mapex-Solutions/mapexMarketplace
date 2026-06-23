---
title: LNS
description: The LoRaWAN edge of MapexOS — a network server built as a thin overlay on The Things Stack that draws its devices from the Assets model and turns decrypted uplinks into NATS events.
---

# LNS

The LNS (LoRaWAN Network Server) is how **LoRaWAN** devices reach MapexOS. Rather than reimplement a LoRaWAN stack, it **embeds [The Things Stack](https://www.thethingsindustries.com/stack/)** — the production-grade open LoRaWAN engine — as a dependency and runs only the protocol roles it needs, slotting MapexOS in at a few precise seams. The Things Stack handles the radio protocol; MapexOS owns **identity, keys, and data flow**: the LNS feeds TTS its devices from the [Assets](./assets.md) model, and takes the decrypted uplinks back out onto **NATS** for [JS Execution](./js-execution.md) to normalize — exactly like the HTTP and MQTT ingress edges.

> Embeds **The Things Stack v3** as a pinned Go dependency, run from its own composition root (TTS is never forked) · gateway transports **UDP** Semtech packet-forwarder (`:1700`) + **Basics Station** WebSocket (`:1887`) · **Redis** (hot session) + **MinIO**/**NVMe** (cold-config cache) · no HTTP server, no MongoDB.

---

## Overlay, not fork

The LNS starts The Things Stack from **its own composition root** and lights up exactly three roles on one shared component:

- **Gateway Server (GS)** — terminates gateway traffic,
- **Network Server (NS)** — MAC layer, frame counters, scheduling,
- **Join Server (JS)** — OTAA join + session-key derivation.

It deliberately does **not** start TTS's Application Server, Identity Server, or Console — because MapexOS already owns those concerns ([JS Execution](./js-execution.md) is the codec, [Assets](./assets.md) is the device registry, and the MapexOS frontend is the console). TTS is a pinned dependency, never edited; unused roles are simply never constructed. The result is a real, battle-tested LoRaWAN engine with MapexOS substituted in only where it matters.

---

## Devices come from Assets, not a TTS database

This is the core trick. The LNS **replaces TTS's device database** with a **hydrate-on-miss** adapter:

```
 TTS asks for a device
     │  HOT  → Redis session store (nsredis / jsredis)
     │  miss
     └─ COLD → the Assets cold config via the tiered cache
               (L1 NVMe → L2 MinIO mapex-asset-auth/{assetUUID}.json → Assets HTTP)
               → mapped to a TTS device → cached → returned
```

The device's identity, region, class, and **root keys** come from the same [Assets](./assets.md) model the rest of the platform uses (`orgId` = TTS application, `assetUUID` = TTS device). Root keys are **decrypted with a key-encryption key fetched from [Vault](./vault.md)** at boot and held only in RAM — the LNS never persists plaintext keys. Both **OTAA** (1.0.3 and 1.1) and **ABP** activation are supported, and a `fanout.asset.invalidate` event evicts both cache tiers the moment a device changes upstream.

---

## Cold vs hot state

The split is deliberate and load-bearing:

| State | What it is | Where it lives |
|---|---|---|
| **Cold** | identity, root keys, region, class, ABP session — **read-only**, rebuildable from the asset | the Assets model (cached) |
| **Hot** | frame counters, derived session keys, MAC state — **disposable** | Redis only |

So the LNS owns **no database of record**. Lose the hot Redis state and a device simply **rejoins** (OTAA) or re-hydrates (ABP) — there is nothing irreplaceable to back up.

---

## Decrypted uplinks become NATS events

When TTS validates an uplink's MIC and AES-decrypts it, the LNS dispatches the **plaintext payload** onto NATS — TTS believes it is feeding an Application Server; it is feeding the MapexOS pipeline. It publishes per-device subjects (NATS Core, fire-and-forget):

| Signal | Subject | Goes to |
|---|---|---|
| **Uplink** | `lorawan.data.{orgId}.{assetUUID}` | [JS Execution](./js-execution.md) — normalized like any other ingress |
| **Join** | `lorawan.join.{orgId}.{assetUUID}` | an OTAA join / presence signal |
| **Downlink status** | `lorawan.downlink.status.{orgId}.{assetUUID}` | the command lifecycle (`queued` → `sent` → `ack`/`nack`/`failed`) |

The uplink envelope carries the `orgId`, `assetUUID`, `fPort`, `fCnt`, and the base64 plaintext payload — so JS Execution resolves the asset and runs the **same decode → validation → transform** pipeline it runs for HTTP and MQTT. The LNS does **not** decode application payloads; that is JS Execution's job.

---

## Gateways and downlinks

Gateways are **first-class, registered entities**: only known gateways connect (`RequireRegisteredGateways`), each drawn from the Assets cold config with its own **frequency plan** (multi-region, plans embedded in the binary). Gateway authentication is **EUI** (UDP) or **mTLS** (Basics Station) — the certificate serial pinned from the asset, the CA chain provisioned from [Vault](./vault.md) PKI.

For the reverse path, the LNS **consumes** downlink commands from NATS (JetStream stream `LORAWAN_DOWNLINK`) and pushes them onto the Network Server's downlink queue, emitting the status events above as the command moves through its lifecycle.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Where device identity & keys are defined | [Assets](./assets.md) |
| The KEK and gateway CA behind it | [Vault](./vault.md) |
| What normalizes the uplinks it emits | [JS Execution](./js-execution.md) |
| The MQTT counterpart at the edge | [MQTT Broker](./mqtt-broker.md) |
