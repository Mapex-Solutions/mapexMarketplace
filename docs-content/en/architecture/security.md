---
title: Security
description: Security model, tenancy isolation, RBAC, auth options, and hardening guidance for MapexOS (v1.0.0).
---

# Security

This document describes the security model of **MapexOS v1.0.0** and provides practical guidance for operating MapexOS in enterprise environments.

> Scope note: MapexOS is designed as a **self-hosted** platform. Your final security posture depends on your deployment environment (Kubernetes / VMs), network controls, secrets management, and operational discipline.

---

## Security goals

MapexOS is built to support:

- **Multi-tenant isolation** through organization scoping and permission boundaries.
- **Least-privilege access** with **RBAC** (users/groups/roles scoped to org nodes).
- **Secure ingestion** of events via configurable authentication at the gateway edge.
- **Controlled execution** of user-defined scripts (decode/validate/transform) with isolation and limits.
- **Auditability** of automation decisions (rule evaluation logs + trigger execution logs).

---

## Trust boundaries

MapexOS typically interacts with four trust zones:

1. **Human operators** (Console/UI/API clients)
2. **Machine clients** (integrations, gateways, partners)
3. **Untrusted event sources** (devices/sensors/field systems)
4. **Core infrastructure** (NATS, MongoDB, Redis, ClickHouse, object storage)

Key principle: **treat inbound events as untrusted input** until validated by templates and routing policies.

---

## Authentication

### Control-plane authentication (users)

MapexOS control-plane access (Console/UI and management APIs) should be protected by:

- Strong identity (SSO or centralized IdP where available)
- MFA for privileged users
- Short-lived tokens and rotation policies

> Implementation detail depends on your deployment and gateway strategy. If you expose management APIs publicly, terminate TLS and enforce authentication at the edge.

### Data-plane authentication (ingestion)

MapexOS supports multiple authentication modes for HTTP ingestion (as described in the architecture docs), including:

- **API Key**
- **JWT**
- **IP allowlist / whitelist**
- **OAuth2** (where applicable to your edge)

Recommended pattern:

- Use **API keys** for server-to-server ingestion (integrations, gateways).
- Use **JWT** for user-bound ingestion workflows and internal services.
- Use **IP allowlists** only as an additional layer (never as the single control).

---

## Authorization (RBAC)

MapexOS uses **Role-Based Access Control**:

- Permissions are granted via **Roles**.
- Roles are assigned to **Users** or **Groups** via **Memberships**.
- Memberships are scoped to an **Organization node** (vendor/customer/site/building/floor/zone).
- Permissions propagate according to organization scope rules.

Security guidance:

- Create separate roles for **operators** vs **administrators**.
- Prefer **group-based** assignments (avoid user-by-user drift).
- Keep a minimal set of "break-glass" admin accounts with stronger controls.

---

## Tenant isolation

MapexOS is multi-tenant by design. Enterprise deployments should ensure:

- **Organization scoping** is enforced consistently in all services (API filters, event routing, queries, logs).
- **No cross-tenant reads/writes** without an explicit admin permission.
- **Per-tenant rate limits** at the gateway layer (protect shared infrastructure).
- Isolation in backing stores:
  - MongoDB collections should enforce tenant scoping in indexes and queries.
  - ClickHouse queries should require tenant filters (or separate databases if you need hard boundaries).
  - Object storage paths should be tenant-scoped.

> Enterprise hard boundary option: if your threat model requires it, run **separate MapexOS deployments per tenant** (most strict isolation).

---

## Script execution security (templates & rules)

MapexOS supports user-defined scripts for:

- Decode / validate / transform (asset templates)
- Rule evaluation and condition logic (where enabled)

Enterprise requirements:

- **Isolation**: scripts must execute in isolated runtimes (e.g., V8 isolates) to prevent memory sharing across tenants.
- **Limits**: enforce CPU timeouts, memory ceilings, and execution quotas.
- **No implicit network access** from scripts unless explicitly enabled and sandboxed.
- **Determinism**: avoid dependency on mutable global state.
- **Cache safety**: cache keys must include tenant + template version to avoid cross-tenant contamination.

Logging:

- Store script execution outcomes (success/failure), durations, and error categories for auditing.
- Never log secrets or raw credentials inside script logs.

---

## Secrets management

MapexOS uses secrets for API keys, JWT signing/verification, database credentials, and service-to-service auth.

Minimum enterprise controls:

- Store secrets in a dedicated system (Kubernetes Secrets + sealed-secrets, Vault, AWS Secrets Manager, etc.).
- Rotate secrets regularly (at least quarterly; sooner for exposed API keys).
- Use **separate credentials per environment** (dev/staging/prod).
- Enforce "no secrets in Git" with scanners.

---

## Transport security

Recommendations (deployment dependent):

- Terminate **TLS** for all public endpoints (Console/UI, APIs, ingestion).
- Use TLS (or mTLS) for internal service-to-service traffic when required by compliance.
- Secure NATS and databases with network policies and private networking.

---

## Data protection

MapexOS stores operational and analytic data across multiple systems.

Minimum enterprise posture:

- Encrypt data **at rest** at the infrastructure level (disk encryption, managed storage encryption).
- Encrypt backups and restrict restore permissions.
- Define retention/TTL policies for event data and logs.
- Restrict access to object storage buckets and ClickHouse endpoints to internal networks.

---

## Auditing & observability

For enterprise operations you should be able to answer:

- Who changed an asset/template/rule?
- What rule fired? What trigger executed? With which inputs?
- Which tenant produced abnormal traffic?

Recommendations:

- Centralize logs (OpenTelemetry/ELK/Cloud logging).
- Emit structured audit events for all admin actions (create/update/delete).
- Track rule execution logs and correlate with ingestion event IDs.

---

## Recommended hardening checklist

**P0 (must-have)**
- TLS on public endpoints
- RBAC with least privilege + group-based assignments
- API key rotation policy
- Tenant-scoped queries and indexes
- Rate limiting at ingestion

**P1 (strongly recommended)**
- Centralized identity (SSO/MFA)
- Network policies for NATS/DB isolation
- Backups + restore drill
- Audit log retention and tamper-resistant storage

**P2 (advanced)**
- mTLS service mesh for east-west traffic
- Per-tenant deployments for strict isolation
- Compliance mapping (SOC2/ISO) with control evidence

---

## Security reporting

If you discover a vulnerability, establish a security reporting channel for responsible disclosure (email or ticket intake), and document your patch/release process.
