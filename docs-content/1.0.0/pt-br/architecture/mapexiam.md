---
title: mapexIam
description: O control plane de identidade e controle de acesso do MapexOS — usuários, a hierarquia de organizações e os caches de permissão e coverage que protegem e escopam cada requisição em todos os demais serviços.
---

# mapexIam

O mapexIam é o **control plane de identidade e controle de acesso** do MapexOS — a fundação sobre a qual todo outro serviço se apoia. Ele é dono de *quem* são os usuários, da *hierarquia de organizações* à qual eles pertencem e *do que podem fazer*, e computa os dois caches que todo outro serviço lê para **autorizar e escopar cada requisição**. Toda vez que outra página diz que uma rota é "permission-gated e coverage-aware", **é este o serviço que torna isso verdade**. Ele fica fora do caminho de telemetria; é o control plane que o governa.

> **Porta** `5000` · **Go** (DDD + Hexagonal) · **MongoDB** (sistema de registro) + **Redis** (sessões, locks, caches de autorização e coverage) + **NATS JetStream** (barramento de invalidação auto-consumido) · JWT + bcrypt.

---

## A árvore de organizações — multi-tenancy hierárquica

Todo tenant é um nó em uma **árvore de organizações** de profundidade ilimitada:

```
vendor → customer → site → building → floor → zone
```

Cada organização — e cada role, grupo e membership escopado a ela — carrega um **`pathKey`** desnormalizado (ex.: `000001/000001/0001`) e um **`depth`**. Isso transforma lookups de ancestrais e descendentes em **prefix matches**, não em joins recursivos: "tudo abaixo deste customer" é uma única range query sobre `pathKey`. É isso que permite a **um único deploy servir dezenas de customers com zero visibilidade compartilhada**, e se manter rápido conforme a árvore cresce.

---

## Dois caches que protegem cada requisição — em todo lugar

A autorização no MapexOS não é reavaliada do zero a cada chamada. O mapexIam computa dois caches no Redis que o **middleware compartilhado em todo outro serviço** lê:

| Cache | Responde | Como é chaveado |
|---|---|---|
| **Autorização** | *O que este usuário pode **fazer** nesta org?* — um conjunto resolvido de strings de permissão | um ponteiro **versionado** `auth:org:{orgId}:user:{userId}:v{n}` (a versão faz round-robin de 1–100, TTL de 30 dias) |
| **Coverage** | *Quais orgs este usuário pode **ver**?* — a subárvore alcançável | `coverage:user:{userId}` |

A chave de autorização versionada é a parte engenhosa: **invalidar é só incrementar o número da versão** — uma operação O(1) — e as versões antigas expiram por conta própria. Ambos os caches são construídos sob um **lock distribuído do Redis com um padrão de poll-for-result**, então uma avalanche de requisições concorrentes dispara exatamente um rebuild, não um por requisição.

Quando você viu o Router, os Triggers, o Assets, o Vault e o resto descreverem seus CRUDs como *permission-gated e coverage-scoped*, **são esses middlewares lendo estes dois caches** — computados aqui, uma vez, e atualizados na mudança.

---

## Herança recursiva de roles — controlada por `merge` / `strict`

A regra real de RBAC não é "roles cascateiam árvore abaixo" — é **cascata opt-in**. Um membership marcado como `scope = recursive` em uma org ancestral contribui com seus roles para uma org descendente **apenas se o `AccessPolicy.RolePolicy` desse descendente for `merge`**. Uma org configurada como **`strict`** interrompe a herança na sua fronteira — seu acesso é exatamente o que é concedido diretamente sobre ela. A expansão de coverage aplica o mesmo filtro ao caminhar pela subárvore.

Assim, o role de um administrador de vendor pode fluir para baixo até cada site que opta por `merge`, enquanto um site sensível configurado como `strict` permanece selado — governado, não acidental.

---

## Grupos, roles e memberships

O acesso é montado a partir de três peças de primeira classe:

- Um **membership** concede um **role** a um destinatário que é **ou um usuário ou um grupo** (`assigneeType`). Resolver as permissões de um usuário faz a união dos seus memberships **diretos** **e** dos memberships de **todo grupo ao qual ele pertence** — os membros de grupo vivem em sua própria collection de junção, projetada para 100K+ tenants em vez de arrays embutidos.
- Um **role** é um conjunto nomeado de **strings de permissão** (ex.: `organizations.create`). Os roles são **customizados**, não um enum fixo, e escopados como o resto da plataforma: templates de **sistema** (MAPEX global), **templates de vendor/customer** (`isTemplate`, compartilhados árvore abaixo) ou **org-local**.

---

## Autenticação

O login é **e-mail + senha bcrypt**, cunhando um **access JWT** e um **refresh JWT** (HS256). O refresh token é armazenado por sessão no Redis; o `/auth/refresh` o **rotaciona** e valida o token apresentado contra o cacheado para bloquear replay, e uma conta desabilitada é recusada. O `/auth/me/permissions` (lendo um header `X-Org-Context`) retorna as permissões resolvidas de quem chama e a versão de cache em que foram computadas.

---

## O barramento de invalidação auto-consumido

A autorização só é tão correta quanto é fresca. Sempre que um **role, grupo, membership, access-policy de org ou hierarquia de org** muda, o mapexIam publica um evento de invalidação de cache em um stream do NATS JetStream (`MAPEXIAM-CACHE-INVALIDATION`). Ele **consome o seu próprio stream** para reconstruir os caches afetados no Redis — e todo serviço downstream assina o **mesmo wildcard** para descartar suas cópias obsoletas. Uma mudança, e a visão de toda a plataforma sobre "quem pode fazer o quê, onde" converge. Ele também publica eventos `organization.created` e de atualização de nome de lista para que outros serviços mantenham suas referências desnormalizadas em sincronia.

---

## Entradas e saídas

**HTTP:**

- **Auth:** `POST /auth/login` · `/auth/logout` · `/auth/refresh`; `GET /auth/me/permissions` · `/auth/users/me/coverage`.
- **Internal (API-key):** `POST /internal/auth/build-authorization` · `/internal/auth/build-coverage` — os construtores de cache que os outros serviços chamam.
- **Resources** sob `/api/v1`: `organizations` (+ `/tree`, com paginação por cursor), `users` (+ `/me`), `groups`, `memberships`, `roles`, `lists` e um orquestrador de `onboarding` — todos permission-gated e coverage-scoped.

**NATS:** produz **e** consome o wildcard `cache.invalidation.>` (`MAPEXIAM-CACHE-INVALIDATION`, com DLQ); publica eventos `organization.created` e de atualização de nome de lista para sincronização entre serviços.

**Stores:** MongoDB (users, organizations, roles, groups, group-members, memberships, lists), Redis (sessões, locks, os dois caches, contadores).

---

## Para onde ir em seguida

| Para mergulhar em… | Vá para |
|---|---|
| Como tenancy e RBAC moldam a plataforma | [Visão Geral da Arquitetura](./overview.md) |
| O modelo de segurança cross-cutting | [Segurança](./security.md) |
| Onde essas permissões são impostas | [Router](./router.md) · [Assets](./assets.md) |
