---
title: Triggers Service
description: Executor registry, placeholder resolution, dispatch pipeline, retry strategy, and observability for the Triggers Service in MapexOS (v1.0.0).
---

# Triggers Service

The **Triggers Service** is the outbound execution layer of MapexOS. It consumes trigger execution events from NATS (produced by the Workflow engine and Router), resolves dynamic placeholders, and dispatches actions through an executor registry.

> **Applies to v1.0.0** --- Go, port 5006, NATS JetStream consumption, executor registry with 8 adapters, cache-aside trigger config, retry with exponential backoff.

---

## Role in the pipeline

The Triggers Service sits at the **end of the automation pipeline**. After the Workflow engine evaluates workflows and determines that an action should fire, it publishes a trigger execution event. The Triggers Service picks up that event, resolves the trigger configuration, fills in dynamic placeholders, and dispatches the action through the appropriate executor.

```txt
Workflow engine / Router
       |
       v
  NATS JetStream (trigger.*.execute)
       |
       v
  Triggers Service
       |
       +---> Fetch trigger config (Redis cache / MongoDB fallback)
       +---> Resolve placeholders ({{path.to.field}})
       +---> Execute via selected adapter
       +---> Publish result to events.trigger
```

---

## Responsibilities

- Consume trigger execution events from NATS JetStream
- Fetch and cache trigger configurations (cache-aside pattern)
- Resolve dynamic placeholders in trigger payloads
- Dispatch actions through the executor registry
- Publish execution results (success or failure) to the Events Service
- Expose CRUD API for trigger definitions
- Retry failed executions with exponential backoff

## Non-responsibilities

| Concern | Owned by |
|---------|----------|
| Evaluate workflows and decide when to fire | Workflow engine |
| Route events to consumers and fan-out | [Router](/docs/1.0.0/en/architecture/router) |
| Persist execution logs and analytics | [Events Service](/docs/1.0.0/en/architecture/events) |

The Triggers Service **only executes** dispatched triggers. It does not decide whether a trigger should fire.

---

## Data flow

### NATS consumption

| Property | Value |
|----------|-------|
| Subject | `trigger.*.execute` |
| Stream | `TRIGGERS` |
| Delivery | WorkQueue (durable) |
| Batch size | 500 messages |
| Worker pool | 50 workers |

The batch size of 500 is intentionally smaller than other services because trigger execution is I/O-bound (outbound HTTP calls, SMTP delivery, etc.), and each message may involve significant network latency.

### Execution flow

1. **Fetch config** --- Load trigger definition using cache-aside (Redis first, MongoDB fallback).
2. **Resolve placeholders** --- Walk the trigger payload and replace <code v-pre>{{path.to.field}}</code> tokens with values from the execution event.
3. **Select executor** --- Route to the correct adapter based on `triggerType`.
4. **Execute** --- Dispatch the action through the executor.
5. **Publish result** --- Emit the outcome (success, failure, retry) to `events.trigger` for persistence by the Events Service.

---

## Executor registry

The executor registry provides 8 adapters organized in two categories.

### Technical executors

| Executor | Description |
|----------|-------------|
| **HTTP** | Outbound HTTP/HTTPS requests (webhooks, REST APIs) |
| **MQTT** | Publish messages to MQTT brokers |
| **RabbitMQ** | Publish messages to RabbitMQ exchanges |
| **NATS** | Publish messages to NATS subjects |
| **WebSocket** | Send messages over WebSocket connections |

### Communication executors

| Executor | Description |
|----------|-------------|
| **Email** | Send emails via SMTP |
| **Teams** | Post messages to Microsoft Teams channels |
| **Slack** | Post messages to Slack channels |

Each executor implements a common interface. Adding a new executor type requires implementing the interface and registering it in the registry.

---

## Placeholder resolution

Trigger payloads support dynamic values using the <code v-pre>{{path.to.field}}</code> syntax. The resolution engine:

- Traverses nested objects and arrays recursively
- Replaces every <code v-pre>{{...}}</code> token with the corresponding value from the trigger execution event payload
- Supports deep paths (e.g., <code v-pre>{{event.data.sensor.temperature}}</code>)
- Leaves unresolved tokens as-is (logged as warnings)

### Example

Given the execution event payload:

```json
{
  "event": {
    "assetName": "Pump-A3",
    "data": { "temperature": 87.5 }
  }
}
```

And a trigger HTTP body template:

```json
{
  "text": "Alert: {{event.assetName}} reached {{event.data.temperature}} C"
}
```

The resolved body becomes:

```json
{
  "text": "Alert: Pump-A3 reached 87.5 C"
}
```

---

## Domain model

The trigger definition stored in MongoDB contains the following key fields:

| Field | Description |
|-------|-------------|
| `triggerType` | Execution adapter (`http`, `mqtt`, `rabbitmq`, `nats`, `websocket`, `email`, `teams`, `slack`) |
| `category` | `technical` or `communication` |
| `config` | Union type --- one configuration block per `triggerType` (e.g., URL + headers for HTTP, topic + QoS for MQTT) |
| `isSystem` | System-level trigger (not editable by tenants) |
| `isTemplate` | Trigger template (reusable across organizations) |
| `orgId` | Organization scope |
| `pathKey` | Hierarchical tenant path for multi-tenant filtering |

---

## Infrastructure

| Component | Role |
|-----------|------|
| **MongoDB** | Trigger definitions (CRUD, queries, counter) |
| **Redis (app cache)** | Trigger config cache (cache-aside, 60-minute TTL) |
| **Redis (shared cache)** | Shared cache layer |
| **NATS JetStream** | Inbound execution events (`trigger.*.execute`), outbound results (`events.trigger`) |

---

## Caching

The Triggers Service uses a **cache-aside** pattern for trigger configurations:

1. On execution, check Redis for the trigger config by ID.
2. On cache hit, use the cached config directly.
3. On cache miss, load from MongoDB, store in Redis with a 60-minute TTL, then proceed.
4. On trigger update/delete via API, invalidate the cache entry.

This avoids a MongoDB read on every execution while keeping configs reasonably fresh.

---

## Retry and fault tolerance

Failed executions are retried with exponential backoff:

| Attempt | Delay |
|---------|-------|
| 1 | 1 second |
| 2 | 5 seconds |
| 3 | 30 seconds |
| 4 | 2 minutes |
| 5 | 10 minutes |

After 5 failed attempts, the message is moved to the dead-letter queue (DLQ) and the failure is recorded in the execution result published to `events.trigger`.

Retry applies to transient failures (network timeouts, 5xx responses). Permanent failures (4xx, invalid config) are not retried.

---

## API

The Triggers Service exposes a REST API for managing trigger definitions.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/triggers` | List triggers (paginated, filtered by org/category/type) |
| `GET` | `/api/v1/triggers/counter` | Count triggers matching filters |
| `POST` | `/api/v1/triggers` | Create a trigger definition |
| `PATCH` | `/api/v1/triggers/:id` | Update a trigger definition |
| `DELETE` | `/api/v1/triggers/:id` | Delete a trigger definition |

All endpoints are scoped to the authenticated organization context.

---

## Observability

### Health check

**Endpoint**: `GET /health`

Checks connectivity to:

| Dependency | Required |
|------------|----------|
| MongoDB | Yes |
| Redis (app cache) | Yes |
| Redis (shared cache) | Yes |
| NATS (core) | Yes |

### Key metrics

**Endpoint**: `GET /metrics`

| Metric area | What it measures |
|-------------|-----------------|
| Trigger processing | Messages consumed, processed, succeeded, failed |
| Executor performance | Latency and error rate per executor type |
| Placeholder resolution | Resolution count, unresolved tokens |
| Message outcomes | ACK, NACK, DLQ counts |

### Dashboard

Pre-built Grafana dashboard: `triggers-overview.json`

---

## Scaling guidelines

The Triggers Service is **I/O-bound**. Each execution involves outbound network calls (HTTP requests, SMTP delivery, broker publishing), so CPU is rarely the bottleneck.

- **Horizontal scaling**: Add replicas. NATS WorkQueue delivery ensures each message is processed by exactly one instance.
- **Worker pool tuning**: The default pool of 50 workers is sized for mixed workloads. Increase if most triggers target fast endpoints; decrease if most are slow (email, external APIs with rate limits).
- **Batch size**: The 500-message batch balances throughput with memory. Increase cautiously for high-volume deployments.
- **Redis**: Monitor cache hit rate. A consistently low hit rate indicates trigger churn or insufficient TTL.

```txt
Triggers Service x N replicas
  └── 50 workers per replica
        └── Executor adapter per trigger type
```

---

## Next steps

- [Events Service](/docs/1.0.0/en/architecture/events) --- where trigger execution results are persisted
- Workflow engine --- the upstream service that decides when triggers fire
- [Architecture Overview](/docs/1.0.0/en/architecture/overview) --- full platform context
