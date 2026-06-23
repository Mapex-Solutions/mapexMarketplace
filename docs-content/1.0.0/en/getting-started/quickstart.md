---
title: Quickstart
description: Wire a temperature sensor end to end — from a raw reading to a stored event you can see in Grafana — in a few minutes.
---

# Quickstart

Take one sensor from a raw reading to a **stored, queryable event** — the whole pipeline, in a few minutes. This is the guided happy path; the field-by-field forms and ready-to-paste JSON live in the deploy repo's [`quickstart/`](https://github.com/Mapex-Solutions/mapexOSDeploy/tree/main/quickstart) folder.

**Before you start:** the stack is running and you're logged in to the frontend (`admin@mapex.local` / `mapex@123`). If not, see [Installation](./installation.md).

## What you'll build

You'll create four things in the console, then push data through them:

```
Asset Template → Route Group → Datasource → Asset → (send readings) → Grafana
```

Pick an ingestion path — the steps are the same; only the datasource differs:

| Path | Best for |
|---|---|
| **HTTP** | Devices that POST JSON to a webhook (REST, gateways, lambdas) |
| **MQTT** | Devices that publish to an MQTT broker (most off-the-shelf firmware) |

## 1 · Asset Template — *Temperature Sensor*

A template defines how a class of device is integrated. Create one with the three scripts that turn a raw payload into a Standard Event:

- **Preprocessor** — normalize the transport format (optional).
- **Validation** — reject malformed readings.
- **Conversion** — emit the Standard Event.

Declare a **dynamic field** `temperature` (type `number`) so readings are stored in a typed, queryable column. See [Core Concepts → Asset Template](./concepts.md#data-sources).

## 2 · Route Group — *Save Temperature Events*

A route group decides where matched events go. Create one that **persists every reading to the events store** (ClickHouse). One rule, one destination — that's enough for the first run.

## 3 · Datasource

**HTTP path:** create a datasource to get a **webhook URL + API key** the device will POST to.
**MQTT path:** the device publishes to the broker (`localhost:1883`) using the asset's credentials — no datasource needed.

## 4 · Asset — *weather-http-001*

Create the asset and bind it to the **template**, the **route group**, and the **protocol** (HTTP or MQTT). This is the thing that produces events.

## 5 · Send test readings

Use the ready-made Node.js script from the quickstart folder to push fake readings:

```bash
cd mapexOSDeploy/quickstart/device-http   # or device-mqtt
node send-events.js                        # (device-mqtt: publish-events.js)
```

## 6 · Watch it land

Open **Grafana** at <http://localhost:3001> (`admin` / `admin`) and filter the events dashboard by your **`assetUUID`**. Each reading you send appears within seconds — ingested, validated, converted, routed, and stored.

## Cleanup

Everything you created is regular platform data — delete it from the UI (Assets, Datasources, Route Groups, Templates). There's no teardown script.

## Where to go next

You've run one device through one route. From here:

| To learn… | Go to |
|---|---|
| The vocabulary behind every step | [Core Concepts](./concepts.md) |
| How the platform is engineered | [Architecture Overview](../architecture/overview.md) |
| Why it's built this way | [Why MapexOS?](../introduction/why-mapexos.md) |
