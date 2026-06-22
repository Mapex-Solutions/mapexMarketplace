---
title: Installation
description: Boot the full MapexOS stack on one machine with a single command — prerequisites, access, what's running, and the path to production.
---

# Installation

The whole platform — every service plus its infrastructure — boots from one Docker Compose distribution: [`mapexOSDeploy`](https://github.com/Mapex-Solutions/mapexOSDeploy). Pre-built multi-arch images, one command.

## Prerequisites

- **Docker** and **Docker Compose v2**.
- A host with roughly **4 cores / 8 GB RAM**. The stack is tuned to fit a laptop (see [Resource footprint](#resource-footprint)).
- Linux, macOS (Apple Silicon), or Windows + Docker Desktop. Images are published for **`linux/amd64`** and **`linux/arm64`**, so the same stack runs on a Raspberry Pi 4/5.

## Boot the stack

```bash
git clone https://github.com/Mapex-Solutions/mapexOSDeploy.git
cd mapexOSDeploy
docker compose up -d
```

Wait **~2 minutes** for first-boot initialization — the Mongo replica-set, NATS streams, MinIO buckets, and ClickHouse tables are provisioned automatically.

## Access

| Surface | URL | Credentials |
|---|---|---|
| **Frontend** | <http://localhost> | `admin@mapex.local` / `mapex@123` *(change on first login)* |
| **Grafana** | <http://localhost:3001> | `admin` / `admin` — 8 dashboards pre-loaded |
| **MinIO console** | <http://localhost:9001> | `mapex_admin` / `mapex_admin_secret_change_me` |

To serve the UI to other machines on your network, set the public host:

```bash
MAPEXOS_PUBLIC_HOST=192.168.0.50 docker compose up -d
```

> A red `[SECURITY WARNING]` line per service in the logs is **expected** in local mode — it's telling you the stack is running with dev defaults. See [Going to production](#going-to-production) to harden it.

## What's running

**Application services** (source in [`mapexOS`](https://github.com/Mapex-Solutions/mapexOS)):

| Service | Port | Purpose |
|---|---|---|
| Frontend | 80 | Vue 3 + Quasar console |
| mapex-iam | 5000 | Users, organizations, roles, auth |
| http-gateway | 5001 | Webhook ingestion, datasource registry |
| assets | 5002 | Assets, templates, EVA fields, device PKI + presence |
| router | 5003 | Event routing, match rules, fan-out |
| events | 5004 | ClickHouse storage, retention |
| triggers | 5006 | Outbound executors (HTTP, MQTT, NATS, …) |
| workflow | 5007 | Durable DAG workflow engine + plugins |
| mapex-vault | 5010 | Credential vault, PKI authority |
| js-executor | 8000 | V8 isolates for asset scripts |
| js-wf-executor | 8001 | V8 isolates for workflow code nodes |

**Infrastructure:**

| Service | Port | Image |
|---|---|---|
| MongoDB | 27017 | `mongo:7.0.34` (replica-set `rs0`) |
| Redis | 6379 | `redis:7.4-alpine` |
| ClickHouse | 8123 / 9440 | `clickhouse/clickhouse-server` |
| MinIO | 9000 / 9001 | `minio/minio` |
| NATS | 4222 / 8222 | `nats:2-alpine` (JetStream) |
| MQTT Broker | 1883 | `mapexos/mapex-broker-mqtt` |
| Prometheus | 9090 | `prom/prometheus` |
| Grafana | 3001 | `grafana/grafana` |

## Resource footprint

Every long-running service ships a `deploy.resources.limits` block **sized to fit a single ~4-core / 8 GB laptop** — the limits sum to roughly **10.5 vCPU / ~5 GB RAM**. These are **ceilings, not reservations**: Docker never reserves the cores or RAM, so the totals can exceed the host and the stack still runs fine — the scheduler time-shares real cores and idle containers cost nothing. MongoDB and ClickHouse are additionally tuned internally (`--wiredTigerCacheSizeGB`, `max_server_memory_usage`) so their caches respect the container cap. Init one-shots are left uncapped on purpose.

## Stop & reset

```bash
docker compose down       # stop containers, keep data
docker compose down -v    # stop + drop volumes (full reset)
```

## Going to production

The defaults exist so the stack fits a laptop — **not** so it performs under load. For production:

1. Build a real env file: `cp infra/envs/production.example.env infra/envs/production.env`, then fill in every value (the template includes generators, e.g. `openssl rand -hex 32` for `AUTH_SECRET`).
2. Point the compose files at `production.env` and set `GO_ENV=prod`.
3. Run on a **Kubernetes cluster**, where you size `resources.requests`/`limits` (and HPA/VPA) to real demand — give MongoDB/ClickHouse/Prometheus real headroom.

Every Mapex service has a **security guard** that refuses to boot when `GO_ENV`/`NODE_ENV` is not a dev value **and** any sensitive variable (`AUTH_SECRET`, `INTERNAL_API_KEY`, `NATS_PASSWORD`, …) is missing or still equals its dev default — a loud fatal error instead of a silent boot with a leaked secret.

> `infra/envs/production.env` is git-ignored. Never commit it.

## Next

→ [Quickstart](./quickstart.md) — wire your first sensor end to end.
