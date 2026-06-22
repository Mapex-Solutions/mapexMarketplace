---
title: JS Execution Engine
description: V8 isolates, worker threads, caching strategy, limits, and security model for script execution in MapexOS (v1.0.0).
---

# JS Execution Engine

The **JS Execution Service** is responsible for running user-defined scripts in MapexOS. It executes the **decode → validate → transform** pipeline defined in Asset Templates, converting raw payloads into standardized events.

> **Applies to v1.0.0** — V8 isolates, worker threads, bytecode caching, 4-level cache pipeline, execution limits, and tenant isolation.

---

## Why JavaScript?

MapexOS chose JavaScript for template scripts because:

- **Universal familiarity**: most integration teams know JavaScript
- **Rich ecosystem**: JSON manipulation, date handling, string processing
- **V8 performance**: Google's V8 engine is highly optimized
- **Isolation**: V8 isolates provide memory and execution isolation

---

## Execution pipeline

Each event ingested by MapexOS goes through the template pipeline:

```txt
Raw Payload → Decode → Validate → Transform → Standard Event
```

### Pipeline stages

| Stage | Purpose | Script function |
|-------|---------|-----------------|
| **Decode** | Parse raw bytes/JSON into a structured object | `decode(payload, metadata)` |
| **Validate** | Check required fields, types, ranges | `validate(decoded, metadata)` |
| **Transform** | Map to Standard Event schema + EVA fields | `transform(validated, metadata)` |

Each stage is a JavaScript function defined in the Asset Template. If any stage fails, the event is rejected with a structured error.

---

## V8 Isolates

MapexOS uses **V8 isolates** to execute scripts securely:

- **Memory isolation**: each isolate has its own heap (no cross-tenant memory access)
- **Execution isolation**: scripts cannot access the host process or other isolates
- **Resource limits**: CPU time and memory ceilings are enforced per execution

### How isolates work

```txt
┌─────────────────────────────────────────┐
│           JS Execution Service          │
├─────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  │
│  │ Isolate │  │ Isolate │  │ Isolate │  │
│  │ Tenant A│  │ Tenant B│  │ Tenant C│  │
│  └─────────┘  └─────────┘  └─────────┘  │
│                                         │
│  Pooled isolates with bytecode cache    │
└─────────────────────────────────────────┘
```

---

## Worker threads

To maximize throughput, the JS Execution Service uses **worker threads**:

- **Parallel execution**: multiple scripts run concurrently
- **Non-blocking**: main thread handles orchestration, workers execute scripts
- **Pool management**: workers are reused to avoid startup overhead

### Scaling

```txt
JS Execution Service × N replicas
  └── Worker Pool per replica
        └── Isolate per execution context
```

Scale the service horizontally to increase scripts/second capacity.

---

## 4-Level cache pipeline

For optimal performance, MapexOS caches compiled scripts at multiple levels:

```txt
L0 (RAM) → L1 (NVMe/SSD) → L2 (MinIO/S3) → Compile fallback
```

### Cache levels

| Level | Storage | Latency | Description |
|-------|---------|---------|-------------|
| **L0** | In-memory (RAM) | ~μs | Hot scripts and bytecode |
| **L1** | Local disk (NVMe/SSD) | ~ms | Warm scripts, persists across restarts |
| **L2** | Object storage (MinIO/S3) | ~10-100ms | Shared cache across replicas |
| **Fallback** | On-demand compile | ~10-50ms | First execution of new/updated script |

### Cache keys

Cache keys are scoped to prevent cross-tenant contamination:

```txt
{orgId}:{templateId}:{templateVersion}:{stage}
```

Example:
```txt
org-123:tmpl-456:v3:decode
org-123:tmpl-456:v3:validate
org-123:tmpl-456:v3:transform
```

### Bytecode caching

V8 can serialize compiled bytecode, avoiding recompilation:

1. **First execution**: script is parsed and compiled
2. **Bytecode cached**: compiled bytecode is stored at L0/L1/L2
3. **Subsequent executions**: bytecode is loaded directly (skip parse/compile)

This significantly reduces latency for frequently-used templates.

---

## Execution limits

MapexOS enforces strict limits to protect the platform:

### Per-execution limits

| Limit | Default | Purpose |
|-------|---------|---------|
| **CPU timeout** | 100ms | Maximum execution time per script |
| **Memory ceiling** | 64MB | Maximum heap size per isolate |
| **Stack depth** | 1000 | Maximum call stack depth |

### Per-tenant quotas (configurable)

| Quota | Description |
|-------|-------------|
| **Executions/second** | Rate limit per tenant |
| **Concurrent executions** | Max parallel scripts per tenant |
| **Script size** | Maximum bytes per script |

> Limits are configurable per deployment. Adjust based on your workload and infrastructure capacity.

---

## Script context (available APIs)

Scripts have access to a limited, sandboxed API:

### Available

| API | Description |
|-----|-------------|
| `JSON.parse()` / `JSON.stringify()` | JSON manipulation |
| `Date` | Date/time operations |
| `Math` | Mathematical functions |
| `String` / `Array` / `Object` | Standard data types |
| `console.log()` | Logging (captured for debugging) |
| `Buffer` | Binary data handling |
| `TextEncoder` / `TextDecoder` | Text encoding |

### Not available (by design)

| API | Reason |
|-----|--------|
| `fetch()` / `XMLHttpRequest` | No network access from scripts |
| `require()` / `import` | No module loading |
| `fs` / `process` | No file system or process access |
| `eval()` / `Function()` | Disabled for security |
| `setTimeout` / `setInterval` | No async timers |

> Scripts are synchronous and deterministic. Side effects (HTTP calls, notifications) are handled by Triggers, not scripts.

---

## Error handling

When a script fails, MapexOS captures:

- **Error type**: syntax, runtime, timeout, memory
- **Error message**: description of the failure
- **Stage**: which pipeline stage failed (decode/validate/transform)
- **Stack trace**: for debugging (when debug mode is enabled)

### Error categories

| Category | Description | Action |
|----------|-------------|--------|
| **Syntax error** | Invalid JavaScript | Fix script in template |
| **Runtime error** | Exception during execution | Check logic and input |
| **Timeout** | Exceeded CPU limit | Optimize script or increase limit |
| **Memory limit** | Exceeded heap size | Reduce memory usage |
| **Validation rejection** | Script returned invalid result | Check validation logic |

---

## Debugging

### Debug mode per asset

Enable debug mode on specific assets for detailed logging:

```txt
debugEnabled = true  → Full execution logs (input, output, duration)
debugEnabled = false → Error logs only (default)
```

### Execution logs

When debug is enabled, logs include:

- Input payload
- Decoded output
- Validation result
- Transformed event
- Execution duration per stage
- Any warnings or errors

> **Production note**: Keep debug disabled in production to reduce log volume and cost. Enable only during investigation.

---

## Performance guidelines

### Write efficient scripts

```javascript
// GOOD: Direct property access
function decode(payload) {
  return {
    temperature: payload.t,
    humidity: payload.h
  };
}

// AVOID: Unnecessary operations
function decode(payload) {
  const copy = JSON.parse(JSON.stringify(payload)); // Unnecessary copy
  return { temperature: copy.t, humidity: copy.h };
}
```

### Avoid common pitfalls

| Pitfall | Impact | Solution |
|---------|--------|----------|
| Large loops | Timeout | Limit iterations, use built-in methods |
| Deep recursion | Stack overflow | Use iterative approach |
| Large string concatenation | Memory pressure | Use arrays and join |
| Excessive logging | Performance + cost | Log only errors in production |

---

## Security model

### Tenant isolation

- Scripts from different tenants never share memory
- Cache keys include tenant ID
- Execution metrics are scoped to tenant

### No network access

Scripts cannot make HTTP calls or access external systems. All external actions are handled by:

1. **Triggers**: for HTTP webhooks, notifications, etc.
2. **Route Groups**: for event fan-out to external systems

### Secrets handling

Scripts can reference secrets stored securely:

```javascript
function transform(validated, metadata) {
  // Secrets are injected by the platform, not hardcoded
  const apiKey = metadata.secrets.API_KEY;
  // ...
}
```

> Secrets are never logged. The platform injects them at runtime.

---

## Monitoring

### Key metrics

| Metric | Description |
|--------|-------------|
| `js_executions_total` | Total script executions |
| `js_execution_duration_ms` | Execution time histogram |
| `js_execution_errors_total` | Errors by type |
| `js_cache_hits_total` | Cache hits by level |
| `js_cache_misses_total` | Cache misses |
| `js_isolate_pool_size` | Current isolate pool size |

### Alerting recommendations

- **High error rate**: > 1% of executions failing
- **High latency**: p99 > 50ms
- **Cache miss rate**: > 10% (indicates cold cache or churn)

---

## Next steps

- [Assets & Templates](/docs/1.0.0/en/core-platform/assets-and-templates)
- [Events & Pipeline](/docs/1.0.0/en/core-platform/events-and-pipeline)
- [Security](/docs/1.0.0/en/architecture/security)
