---
title: mapexIam
description: The identity and access control plane of MapexOS — users, the organization hierarchy, and the permission and coverage caches that gate and scope every request across every other service.
---

# mapexIam

mapexIam is the **identity and access control plane** of MapexOS — the foundation every other service stands on. It owns *who* users are, the *organization hierarchy* they belong to, and *what they may do*, and it computes the two caches that every other service reads to **authorize and scope each request**. Every time another page says a route is "permission-gated and coverage-aware," **this is the service that makes that true**. It sits off the telemetry path; it is the control plane that governs it.

> **Port** `5000` · **Go** (DDD + Hexagonal) · **MongoDB** (system of record) + **Redis** (sessions, locks, authorization & coverage caches) + **NATS JetStream** (self-consumed invalidation bus) · JWT + bcrypt.

---

## The organization tree — hierarchical multi-tenancy

Every tenant is a node in an **organization tree** with unbounded depth:

```
vendor → customer → site → building → floor → zone
```

Each organization — and every role, group, and membership scoped to it — carries a denormalized **`pathKey`** (e.g. `000001/000001/0001`) and a **`depth`**. That turns ancestry and descendant lookups into **prefix matches**, not recursive joins: "everything under this customer" is a single range query on `pathKey`. This is what lets **one deployment serve dozens of customers with zero shared visibility**, and stay fast as the tree grows.

---

## Two caches that gate every request — everywhere

Authorization in MapexOS is not re-evaluated from scratch on every call. mapexIam computes two Redis caches that the **shared middleware in every other service** reads:

| Cache | Answers | How it's keyed |
|---|---|---|
| **Authorization** | *What can this user **do** in this org?* — a resolved set of permission strings | a **versioned** pointer `auth:org:{orgId}:user:{userId}:v{n}` (version round-robins 1–100, 30-day TTL) |
| **Coverage** | *Which orgs can this user **see**?* — the reachable subtree | `coverage:user:{userId}` |

The versioned authorization key is the clever part: **invalidation just bumps the version number** — an O(1) operation — and old versions age out on their own. Both caches are built under a **Redis distributed lock with a poll-for-result** pattern, so a thundering herd of concurrent requests triggers exactly one rebuild, not one per request.

When you saw the Router, Triggers, Assets, Vault, and the rest describe their CRUD as *permission-gated and coverage-scoped*, **those middlewares are reading these two caches** — computed here, once, and refreshed on change.

---

## Recursive role inheritance — gated by `merge` / `strict`

The real RBAC rule is not "roles cascade down the tree" — it is **opt-in cascade**. A membership marked `scope = recursive` on an ancestor org contributes its roles to a descendant org **only if that descendant's `AccessPolicy.RolePolicy` is `merge`**. An org set to **`strict`** stops inheritance at its boundary — its access is exactly what's granted on it directly. Coverage expansion applies the same filter when walking the subtree.

So a vendor administrator's role can flow down to every site that opts into `merge`, while a sensitive site set to `strict` remains sealed — governed, not accidental.

---

## Groups, roles, and memberships

Access is assembled from three first-class pieces:

- A **membership** grants a **role** to an assignee that is **either a user or a group** (`assigneeType`). Resolving a user's permissions unions their **direct** memberships **and** the memberships of **every group they belong to** — group members live in their own junction collection, designed for 100K+ tenants rather than embedded arrays.
- A **role** is a named set of **permission strings** (e.g. `organizations.create`). Roles are **custom**, not a fixed enum, and scoped like the rest of the platform: **system** templates (global MAPEX), **vendor/customer templates** (`isTemplate`, shared down the tree), or **org-local**.

---

## Authentication

Login is **email + bcrypt password**, minting an **access JWT** and a **refresh JWT** (HS256). The refresh token is stored per session in Redis; `/auth/refresh` **rotates** it and validates the presented token against the cached one to block replay, and a disabled account is refused. `/auth/me/permissions` (reading an `X-Org-Context` header) returns the caller's resolved permissions and the cache version they were computed at.

---

## The self-consuming invalidation bus

Authorization is only as correct as it is fresh. Whenever a **role, group, membership, org access-policy, or org hierarchy** changes, mapexIam publishes a cache-invalidation event to a NATS JetStream stream (`MAPEXIAM-CACHE-INVALIDATION`). It **consumes its own stream** to rebuild the affected Redis caches — and every downstream service subscribes to the **same wildcard** to drop its stale copies. One change, and the whole platform's view of "who can do what, where" converges. It also publishes `organization.created` and list-name-update events so other services keep their denormalized references in sync.

---

## Inputs & outputs

**HTTP:**

- **Auth:** `POST /auth/login` · `/auth/logout` · `/auth/refresh`; `GET /auth/me/permissions` · `/auth/users/me/coverage`.
- **Internal (API-key):** `POST /internal/auth/build-authorization` · `/internal/auth/build-coverage` — the cache builders other services call.
- **Resources** under `/api/v1`: `organizations` (+ `/tree`, cursor-paginated), `users` (+ `/me`), `groups`, `memberships`, `roles`, `lists`, and an `onboarding` orchestrator — all permission-gated and coverage-scoped.

**NATS:** produces **and** consumes the `cache.invalidation.>` wildcard (`MAPEXIAM-CACHE-INVALIDATION`, with DLQ); publishes `organization.created` and list-name-update events for cross-service sync.

**Stores:** MongoDB (users, organizations, roles, groups, group-members, memberships, lists), Redis (sessions, locks, the two caches, counters).

---

## Where to go next

| To dive into… | Go to |
|---|---|
| How tenancy and RBAC shape the platform | [Architecture Overview](./overview.md) |
| The cross-cutting security model | [Security](./security.md) |
| Where these permissions are enforced | [Router](./router.md) · [Assets](./assets.md) |
