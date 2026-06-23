---
title: HTTP Gateway
description: A porta de entrada do MapexOS — uma borda de ingestão HTTP autenticada onde cada fonte externa entra com sua própria política de autenticação, recebe um correlation id e é admitida no pipeline com o payload mais enxuto possível.
---

# HTTP Gateway

O HTTP Gateway é a **porta de entrada** do MapexOS. É por aqui que o mundo externo *entra*: webhooks, telemetria de dispositivos e chamadas de API vindas de gateways e sistemas de negócio chegam por HTTP. Sua função é estreita e essencial — **autenticar cada requisição conforme a política da sua fonte, gerar um correlation id e admitir um payload limpo e mínimo no pipeline**. Ele não decodifica, não transforma e não roteia; autentica e encaminha, depois repassa para os estágios de [JS execution](./js-execution.md) → [Router](./router.md) via NATS.

Tudo o que o gateway faz se apoia em um único conceito: a **DataSource**.

> **Porta** `5001` · **Go** (DDD + Hexagonal) · **MongoDB** (registro de DataSources) + **Redis** (cache-aside) · publica em **NATS JetStream** · sem NATS de entrada — toda a entrada é HTTP.

---

## A DataSource — um ponto de entrada governado

Toda integração externa é modelada como uma **DataSource**: um registro de configuração que diz *quem* pode enviar dados, *como* se autentica e *a qual* asset os dados pertencem. Uma frota de dispositivos publicando com uma API key, um sistema parceiro usando OAuth2, um gateway on-prem restrito a uma faixa de IP — cada um é a sua própria DataSource, governada de forma independente.

| Campo | Papel |
|---|---|
| `auth` | A estratégia de autenticação dessa fonte (veja abaixo). |
| `assetBind` | Como um payload de entrada é mapeado para um Asset. |
| `enabled` | Um kill switch — uma fonte desabilitada é recusada antes que qualquer outra coisa rode. |
| `orgId` / `pathKey` | O tenant ao qual essa fonte pertence — a âncora de confiança. |

As DataSources são gerenciadas por uma superfície REST governada (`/api/v1/data_sources`, com controle de permissão e escopo de organização, com paginação, filtragem e projeção) e lidas no caminho crítico de ingestão por uma camada de **cache-aside no Redis** (TTL de 24 horas, com métricas de hit/miss), de modo que admitir uma requisição quase nunca toca o MongoDB.

---

## Autenticação por fonte na borda

O gateway não impõe um esquema de autenticação global único. **Cada DataSource carrega o seu próprio**, e o middleware de ingestão decide com base nele a cada requisição:

| Estratégia | Como autentica |
|---|---|
| `apiKey` | Uma chave em um header ou campo de query configurado. |
| `jwt` | Um JWT assinado validado contra um segredo compartilhado (header configurável). |
| `oauth2` | Um bearer token validado contra um endpoint JWKS (no estilo OIDC). |
| `ip_whitelist` | O IP do chamador precisa estar dentro das faixas CIDR permitidas. |
| `none` | Ingestão aberta, para fontes em rede confiável. |

Antes de qualquer uma dessas etapas rodar, uma **DataSource desabilitada é rejeitada de imediato** — o kill switch não custa nada. O mesmo vale para um tipo de autenticação desconhecido: recusado, nunca admitido por padrão.

```
POST /api/v1/events?ds={id}
   │
   ├─ resolve DataSource (Redis → MongoDB)
   ├─ disabled?            → 403   (+ audit)
   ├─ authenticate by type → 401 on failure (+ audit)
   └─ admitted             → publish to the pipeline → 201
```

---

## Tráfego rejeitado é registrado, não apenas descartado

Segurança em uma borda de ingestão vale tanto quanto a sua visibilidade. **Toda rejeição** — uma fonte desabilitada, uma chave inválida, um IP fora da faixa — dispara um `RawEventDTO{ success: false }` para `events.raw` em uma goroutine desacoplada, sem atrasar o `401`/`403` que é devolvido ao chamador. Esse registro chega ao ClickHouse pelo serviço de [Events](./events.md), de modo que tentativas falhas e forjadas viram uma **trilha de auditoria consultável**, e não um descarte silencioso.

---

## O correlation id nasce aqui

A cada requisição aceita, o gateway gera um novo **`eventTrackerId`** (um UUID) e o anexa ao payload. Esse id então percorre js-execution → router → triggers → workflow → storage — é o fio condutor que torna um evento **rastreável de ponta a ponta**. Quando o serviço de Events permite reconstruir toda a jornada de um evento por um único id, é *aqui* que esse id começa.

---

## Um repasse deliberadamente mínimo

O que o gateway encaminha para o pipeline é intencionalmente enxuto. O payload publicado em `processor.js.execute` carrega apenas:

- `sourceType: "http"` — a origem,
- o `orgId` e o `assetBind` da DataSource — o suficiente para resolver o Asset,
- o corpo bruto do `event`,
- o `eventTrackerId`.

Ele deliberadamente omite o nome, a descrição e o `pathKey` do asset — **js-execution lê isso do cache do Asset, a fonte da verdade.** Nada que a plataforma já conhece é duplicado no fio; a borda permanece fina e o Asset permanece a autoridade.

---

## Heartbeats — presença sem spoofing

Além da telemetria, o gateway aceita um **heartbeat** explícito (`POST /api/v1/heartbeat?ds={id}`, corpo `{ assetUUID }`) que governa o estado online de um asset. Ele publica em `asset.heartbeat.{orgId}` para o monitor de saúde do Assets. O ponto crucial: o **`orgId` e o `pathKey` vêm da DataSource já resolvida (pós-autenticação), nunca do corpo da requisição** — então um corpo forjado não consegue fazer um tenant reportar presença em nome de outro.

---

## Entradas e saídas

**HTTP (entrada):**

| Rota | Resultado |
|---|---|
| `POST /api/v1/events?ds={id}` | Admite telemetria → `201 { success: true }`. |
| `POST /api/v1/heartbeat?ds={id}` | Governa presença → `200 { success: true }`. |
| `/api/v1/data_sources` (CRUD) | Gerencia fontes — com autenticação e controle de permissão. |

**NATS (saída):**

| Subject | Propósito |
|---|---|
| `processor.js.execute` | Evento admitido → o pipeline de js-execution (JetStream, com ack). |
| `events.raw` | Registro de auditoria de falha de autenticação (fire-and-forget). |
| `asset.heartbeat.{orgId}` | Sinal de presença → o monitor de saúde do Assets (publicação core). |

O gateway consome **nada** do NATS — toda a sua superfície de entrada é HTTP.

---

## Observabilidade

As métricas do Prometheus cobrem os resultados de autenticação **por estratégia** (incluindo o gate de `disabled`), a duração da autenticação, o hit/miss do cache de DataSource e os resultados de publicação — além de `/health` e `/swagger`. Você consegue ver, por fonte e por tipo de autenticação, exatamente o que está sendo admitido e o que está sendo barrado.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| O que acontece com um evento admitido | [JS Execution](./js-execution.md) |
| Onde vive o modelo de asset | [Visão Geral da Arquitetura](./overview.md) |
| Onde ficam os registros de tráfego rejeitado | [Events](./events.md) |
