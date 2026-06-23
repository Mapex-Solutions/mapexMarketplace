---
title: Security
description: How MapexOS secures a connected operation end to end — identity, multi-tenancy, secrets, device trust, edge authentication, sandboxed code, and a complete audit trail — and how to harden a deployment.
---

# Security

Security in MapexOS is not a feature bolted on the side — it is **woven through every layer**, because the platform's job is to be *operated*: with identity, isolation, secrets, and audit as first-class parts of the architecture. This page is the **map**: it shows how those layers fit together and points to the service that enforces each one. Nothing here is a separate "security product" — it is how the system is built.

The shape is **defense in depth**. A request is identified, scoped to a tenant, authorized, served by code it cannot escape, and recorded — and a secret or a device key is encrypted at rest, handed out only to a named service, and never returned in an API response.

| Layer | What it protects | Enforced by |
|---|---|---|
| **Identity & access** | who you are, what you may do, where | [mapexIam](./mapexiam.md) |
| **Multi-tenant isolation** | one organization can't see another | [mapexIam](./mapexiam.md) · every service |
| **Secrets & keys** | credentials, encryption keys, the CA | [Vault](./vault.md) |
| **Device trust** | only real devices connect, as themselves | [Assets](./assets.md) · [Edge Servers](#authentication-at-the-edge) |
| **Edge authentication** | every connection proven before admission | [HTTP Gateway](./http-gateway.md) · [MQTT Broker](./mqtt-broker.md) · [LNS](./lns.md) |
| **Code isolation** | customer code can't reach the host | [JS Execution](./js-execution.md) · [JS Workflow Execution](./js-workflow-execution.md) |
| **Audit & traceability** | a complete, queryable record of what happened | [Events](./events.md) |

---

## Identity & access

Every request is authenticated and authorized against [mapexIam](./mapexiam.md), the identity control plane. Users log in with **email + bcrypt password** and receive a short-lived **access JWT** plus a **refresh JWT**; refresh tokens are stored per session and **rotated with replay protection**. Authorization is **role-based**: a user's resolved permissions for an organization are computed once and cached, and **every other service reads that cache** through shared middleware — which is why each service describes its routes as *permission-gated*. Roles cascade down the organization tree only where a descendant opts in (`merge` vs `strict`), so inherited access is deliberate, never accidental.

---

## Multi-tenant isolation

A single MapexOS deployment serves many organizations with **zero shared visibility**. Every asset, event, template, role, and credential is scoped to an **organization** in an unbounded hierarchy (`vendor → customer → site → building → floor → zone`), carried as a denormalized **`pathKey`**. Listing, querying, and routing are filtered by the caller's **coverage** — the slice of the tree they are allowed to reach — so one tenant cannot enumerate, read, or route another's data. Isolation is a property of the data model and the cache, not a filter someone has to remember to apply.

---

## Secrets & keys

No secret is ever stored in plaintext or returned in an API response. [Vault](./vault.md) is the **root of trust**:

- Third-party **credentials** are sealed with **two-layer envelope encryption** (a master key wraps a per-record data key; AES-256-GCM). The encrypted fields physically cannot serialize into a response; plaintext is handed out only over a hidden, API-key-gated internal endpoint, for the life of one request.
- Services that need to encrypt their own secrets get a **context-scoped key (KEK)** and run the encryption **locally** — the master key never leaves Vault. This is how [Assets](./assets.md) seals LoRaWAN device keys.
- Vault is also the **certificate authority** (see below); its CA private keys are envelope-encrypted and decrypted per request, never cached.

---

## Device trust & PKI

Devices prove who they are with credentials [Assets](./assets.md) issues and [Vault](./vault.md) roots. An MQTT device authenticates by **bcrypt password** or by a **per-device X.509 client certificate** (ECDSA P-256), signed by an **intermediate CA fetched from Vault** and held only in RAM. A certificate's private key is returned **once** at issue time and **never persisted** — only its metadata is kept — and revocation is immediate. LoRaWAN gateways authenticate by **mTLS** with the serial pinned from the asset; LoRaWAN device keys are **envelope-encrypted** with a Vault KEK. One device's credentials grant access to exactly that device — nothing else.

---

## Authentication at the edge

Every external connection is proven **before** anything it sends is admitted, and the decision is made **at the edge it lands on** — off the same Assets-authored projection, with no central auth bottleneck:

- The **[HTTP Gateway](./http-gateway.md)** authenticates each source by its own policy (API key, JWT, OAuth2/JWKS bearer, IP allow-list, or none); a disabled source is refused before any auth runs, and **every rejection is audited**.
- The **[MQTT Broker](./mqtt-broker.md)** decides every `CONNECT` locally (bcrypt or cert serial), **fails closed** on a missing entry or a store outage, and enforces a strict ACL: a device may only touch **its own** topics — cross-asset access is denied in the broker.
- The **[LNS](./lns.md)** only admits **registered** gateways and draws every device's identity and keys from the Assets model.

Tenancy can't be forged at the edge: a request's `orgId` is taken from the **authenticated source**, never from the payload body.

---

## Sandboxed code execution

MapexOS runs **customer-authored JavaScript** — template transform scripts and workflow code nodes — and treats all of it as untrusted. Every script runs inside an **`isolated-vm` V8 isolate** ([JS Execution](./js-execution.md) · [JS Workflow Execution](./js-workflow-execution.md)) with a hard **memory cap**, a **wall-clock timeout**, and **no access to Node, the network, or the filesystem**. A script can compute and transform, and nothing more — it cannot reach the host, another tenant, or the outside world.

---

## Audit & traceability

Everything that happens is recorded and queryable. [Events](./events.md) persists every event and every execution to ClickHouse with **per-organization retention**, so compliance evidence is a query, not a project. A single **`eventTrackerId`** follows one event end to end — ingest → route → trigger → workflow → storage — so a full story is one lookup. Routing decisions are **explainable** (each records why it did or didn't match), and **rejected** connections are audited too, not silently dropped.

---

## Transport security

External transports use **TLS**: the HTTP edge over HTTPS, MQTT over TLS/mTLS (port `8883`), LoRaWAN Basics Station over mTLS. Certificate verification is **on** — the platform ships no connector or client that disables it — and outbound connectors carry their own per-target TLS and timeouts. Internal service-to-service calls run on a trusted network and are gated by API keys.

---

## Secure by default

The defaults exist to fit a laptop, not to ship to production unguarded — so the platform **refuses to run unguarded**. Every Mapex service has a **boot-time security guard**: when `GO_ENV` / `NODE_ENV` is not a development value **and** any sensitive variable (`AUTH_SECRET`, `INTERNAL_API_KEY`, `NATS_PASSWORD`, the credential master key, …) is missing or still equal to its dev default, the service **fails fast with a loud fatal error** instead of booting with a leaked or weak secret. In local mode it prints a per-service `[SECURITY WARNING]` so the dev posture is never silent.

---

## Hardening checklist

MapexOS is **self-hosted**, so the final posture is yours. Before production:

1. **Generate every secret.** Replace all dev defaults — `AUTH_SECRET`, `INTERNAL_API_KEY`, the Vault master key, datastore passwords (e.g. `openssl rand -hex 32`). The boot guard enforces this; don't fight it.
2. **Set `GO_ENV`/`NODE_ENV=prod`** and point services at a real, git-ignored env file (never commit it).
3. **Terminate TLS everywhere** external — HTTPS at the gateway, mTLS on the broker and LNS — with certificates you control.
4. **Run on Kubernetes** with real resource requests/limits and network policies that close internal ports to the outside.
5. **Lock down the datastores** — MongoDB, ClickHouse, Redis, MinIO, NATS — to the cluster network with authentication enabled.
6. **Rotate** JWT secrets, API keys, and the Vault master key on a schedule, and set per-organization **retention** to your compliance needs.
7. **Scope operators** with least-privilege roles in the organization tree rather than broad recursive grants.

---

## Reporting a vulnerability

If you find a security issue, please disclose it responsibly to the maintainers rather than opening a public issue, and allow time for a fix before any public discussion.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Identity, roles, and tenancy | [mapexIam](./mapexiam.md) |
| Secrets, keys, and the CA | [Vault](./vault.md) |
| Device certificates & presence | [Assets](./assets.md) |
| The full platform map | [Architecture Overview](./overview.md) |
