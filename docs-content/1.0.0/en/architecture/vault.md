---
title: Vault
description: The root of trust for MapexOS — an envelope-encrypted secret store, a key-distribution service, and the certificate authority behind every device identity. Secrets are stored, never surrendered.
---

# Vault

mapexVault is the **root of trust** of MapexOS. It holds the things that must never leak: third-party **credentials**, the **keys** that let other services encrypt their own secrets, and the **certificate authority** behind every device identity. It sits off the telemetry path — it ingests no events and routes nothing — but the rest of the platform's security is built on it. Its guiding rule is simple: **a secret goes in, and the plaintext never comes back out in an API response** — only to a named internal service, over an authenticated internal channel, for exactly as long as the request lives.

It does three jobs, in three modules: **credentials**, **KEK**, and **PKI**.

> **Port** `5010` · **Go** (DDD + Hexagonal) · **MongoDB** (encrypted at rest) · **NATS JetStream** (scheduled token refresh) · **AES-256-GCM** envelope encryption · **crypto/x509** CA. Secret material is tagged `json:"-"` — it cannot serialize into a response.

---

## Credentials — two-layer envelope encryption

A credential is a third-party secret — an API token, an OAuth grant, a username/password (`manual` · `oauth2` · `userAndPass`). It is stored under **two layers of envelope encryption**:

```
 master key  ──wraps──▶  per-record DEK  ──encrypts──▶  the secret data
 (in Vault only)         (stored encrypted)              (AES-256-GCM)
```

A single **master key** (held only by Vault) wraps a **per-record data-encryption key**; that DEK encrypts the payload. Only the four envelope fields are persisted (`encryptedDEK`, `dekNonce`, `encryptedData`, `dataNonce`) — every one tagged `json:"-"`, so the secret **physically cannot** be marshaled into an API response. Plaintext is handed out on exactly one path: a hidden, API-key-gated `GET /internal/credentials/:id/decrypt`, used by the [Workflow](./workflow.md) engine to run a plugin's action — and even there it lives only for the request.

Because each record has its own DEK, compromising one record's key exposes one record — not the store.

---

## Token refresh — scheduled, never polled

OAuth and token credentials expire, so Vault keeps them alive **without a polling cron**. When a credential is stored, Vault schedules a single **NATS JetStream message** to fire at **`expiry − 15 minutes`**; when it fires, Vault runs the token exchange and **re-arms the next timer**. An hourly **reconciler** reseeds any timer lost to a NATS restart or drift, and already-expired credentials are rescheduled to refresh almost immediately on boot.

The clever part is the separation: a credential's **`providerConfig`** — *how* to refresh it (the HTTP method, URL, headers, and where in the response the new token lives) — is stored **unencrypted on purpose**. The secret values are injected only at execution time through `{{credential.*}}` placeholders. So the refresh worker knows *where and how* to refresh **without ever decrypting the secret** to find out.

---

## KEK — let services encrypt locally, without the master key

Not every service should call Vault to encrypt every secret, and none should ever hold the master key. So Vault hands out **key-encryption keys**: a context-scoped AES-256 key (for example `lorawan_device_keys`) served over `GET /internal/kek/:context`. A service fetches its KEK once and then runs **its own local envelope encryption** with it — the way [Assets](./assets.md) seals LoRaWAN device keys before storing them.

The master key never leaves Vault; the consuming service holds only a scoped key for its own context. KEKs are seeded at deploy time, not authored at runtime, and the endpoint returns `503` until a context is seeded — a clear "not ready yet" the caller can retry against.

---

## PKI — the on-demand certificate authority

Vault is the **certificate authority** beneath every device identity. It stores the **root and intermediate CA** with their private keys **envelope-encrypted at rest**, and exposes three internal operations:

| Endpoint | Returns | Consumed by |
|---|---|---|
| `GET /internal/pki/intermediate_ca_bundle` | the intermediate CA cert + key | [Assets](./assets.md) — held in RAM to sign per-device client certs |
| `GET /internal/pki/ca_chain` | the public root + intermediate chain | anyone verifying a MapexOS cert |
| `POST /internal/pki/sign_server` | a signed server certificate | the [MQTT broker](./mqtt-broker.md)'s TLS identity |

The defining property: **the CA private key is decrypted per request and discarded** — it is never cached in memory between calls, and the caller is expected to discard it the moment it is used. The whole device-PKI story you saw in [Assets](./assets.md) — per-device ECDSA certs signed by an intermediate CA held in RAM — **roots here**: Assets fetches the bundle once at boot, and Vault is where that bundle's trust actually comes from.

---

## Multi-tenant by default

Every external credential operation is scoped to the caller's `orgId` and hierarchical `pathKey` from the request context — one organization can never read or refresh another's secrets. Credential templates are excluded from owned reads and cannot be created or flipped through the external API.

---

## Inputs & outputs

**HTTP (external, JWT + permissions)** under `/api/v1/credentials`: `POST` · `GET` · `GET /:id` · `PATCH /:id` · `DELETE /:id`.

**HTTP (internal, API-key, hidden):** `/internal/credentials/:id/decrypt` (→ Workflow), `/internal/kek/:context` (→ Assets), `/internal/pki/intermediate_ca_bundle` (→ Assets), `/internal/pki/ca_chain`, `/internal/pki/sign_server` (→ MQTT broker).

**NATS (self-delivered):** per-credential refresh schedules on `MAPEXVAULT-SCHEDULE`, and the reconciler stream that keeps them seeded.

It publishes nothing onto the cross-service bus and consumes no telemetry — Vault is infrastructure the other services *call*, not a stage in the pipeline.

---

## Where to go next

| To dive into… | Go to |
|---|---|
| Who uses the CA and the LoRaWAN KEK | [Assets](./assets.md) |
| Who decrypts credentials to run plugins | [Workflow Engine](./workflow.md) |
| The broker whose server cert Vault signs | [MQTT Broker](./mqtt-broker.md) |
