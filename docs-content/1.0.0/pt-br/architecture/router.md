---
title: Router
description: O despachante do MapexOS — avalia regras de match condicionais contra cada evento e faz o fan-out para persistência, automação e notificação, com o roteamento decidido a partir do estado confiável do asset, não do payload.
---

# Router

O Router é o **despachante do MapexOS**. Ele fica no estágio de *route* — entre o transform a montante e o automate/persist a jusante — e responde a uma única pergunta para cada evento: **para onde isto vai?** Um único evento que chega aqui pode fazer fan-out para persistência, um data lake, uma notificação, um [trigger](./triggers.md) de saída e um [workflow](./workflow.md) — tudo ao mesmo tempo, ou para nenhum deles, decidido regra a regra.

É isso que torna a plataforma **dinâmica**: o comportamento de roteamento é **dado, não código**. Você muda para onde os eventos vão editando RouteGroups pela API — sem redeploy, sem restart. O mesmo fluxo de eventos alimenta topologias a jusante completamente diferentes para dois tenants, porque as regras de cada um dizem isso.

> **Porta** `5003` · **Go** (DDD + Hexagonal) · consumidor **NATS JetStream** WorkQueue · **MongoDB** (regras, fonte da verdade) + **Redis** (cache-aside) + **TieredCache** (RAM → Disk → MinIO) · instrumentação **Prometheus**.

---

## O modelo: RouteGroup → Routers → regras de match

| Conceito | O que é |
|---|---|
| **RouteGroup** | Um conjunto nomeado e versionado de Routers, vinculado a assets. Um asset pode carregar vários. |
| **Router** | Um destino — um `kind` mais uma condição `match` opcional. Se a condição passa, o evento vai para esse kind. |
| **Regra de match** | Uma condição: `{ field, operator, value }`. Os Routers combinam regras com uma política `all` (AND) ou `any` (OR). Um Router sem configuração de match **sempre dispara**. |

Esse é todo o dinamismo em três camadas: assets apontam para RouteGroups, RouteGroups contêm Routers, Routers carregam condições. Edite qualquer camada — como dado — e o roteamento muda no próximo evento.

---

## Regras de match — as condições, exatamente como são avaliadas

Uma regra é `{ field, operator, value }`. O **field** é um **dot-path** dentro do evento (`data.temperature`, `metadata.site`), resolvido percorrendo mapas aninhados — rápido e previsível. O **operator** é um de oito:

| Operator | Significado | Como compara |
|---|---|---|
| `eq` | igual a | igualdade profunda |
| `neq` | diferente de | igualdade profunda, negada |
| `gt` | maior que | numérico |
| `gte` | maior ou igual a | numérico |
| `lt` | menor que | numérico |
| `lte` | menor ou igual a | numérico |
| `in` | é um dentre | pertencimento a uma lista |
| `nin` | não é um dentre | pertencimento a uma lista, negado |

As regras combinam sob uma política:

| Política | Comportamento |
|---|---|
| `all` | toda regra precisa passar (AND) |
| `any` | ao menos uma regra precisa passar (OR) |

Um field que o evento não contém faz aquela regra falhar com `field not found` — comportamento previsível sob dados parciais, nunca um crash. Um Router sem regra alguma é **sempre permitido**.

```json
{
  "name": "Cold-chain breach",
  "routers": [
    {
      "kind": "workflow",
      "match": {
        "policy": "all",
        "rules": [
          { "field": "data.temperature", "operator": "gt",  "value": 8 },
          { "field": "data.doorState",   "operator": "eq",  "value": "open" }
        ]
      }
    }
  ]
}
```

Esse Router inicia um workflow somente quando a temperatura passa de 8 **e** a porta está aberta — todo outro evento passa por ele sem ser tocado.

---

## Os cinco destinos

Um Router que deu match publica o evento enriquecido no subject do seu kind. Esses cinco subjects são como o Router conecta o pipeline de ingestão a cada capacidade a jusante:

| Kind | Subject | Vai para |
|---|---|---|
| `save_event` | `events.save` | o serviço de [Events](./events.md) → ClickHouse (persistência) |
| `lake_house` | `events.lake_house` | o sink analítico do lake-house |
| `notification` | `events.notification` | o caminho de notificação |
| `trigger` | `trigger.router.execute` | o serviço de [Triggers](./triggers.md) (ação de saída autônoma) |
| `workflow` | `workflow.execution.router` | o engine de [Workflow](./workflow.md) (inicia um run durável) |

Um único evento pode dar match em vários Routers e fazer fan-out para vários kinds ao mesmo tempo. Cada dispatch é deduplicado no JetStream por um message id `{eventTrackerId}-{routerIndex}`, de modo que uma reentrega nunca roteia em duplicidade.

---

## O roteamento é decidido a partir do estado confiável, não do payload

O Router nunca lê *quais* RouteGroups se aplicam a partir da mensagem de entrada. Ele resolve o asset do evento pelo cache e pega os ids de RouteGroup **do próprio asset**. Um remetente não consegue forjar o próprio roteamento enfiando ids no payload — o roteamento é governado pela configuração do asset, que só a plataforma controla.

```
event { assetUUID }  →  resolve asset (cache)  →  asset.RouteGroupIds  →  evaluate
                              (source of truth — never the payload)
```

---

## Roteamento ciente de saúde

O Router distingue dois tipos de entrada por um discriminador `eventSource`:

- **`assetEvent`** — um evento de dados normal → usa os `RouteGroupIds` do asset.
- **`healthStatus`** — uma transição online/offline → usa os `HealthMonitor.OfflineRouteGroupIds` ou `OnlineRouteGroupIds` do asset, escolhidos conforme a transição.

Um dispositivo que fica offline pode, portanto, acionar um conjunto de rotas *diferente* do que sua telemetria aciona. As transições de saúde são deliberadamente permitidas a alcançar apenas os kinds **`trigger`** e **`workflow`** — uma mudança de presence escala ou automatiza; não inunda o lake-house nem o feed de notificações.

---

## Toda decisão é explicável

Para cada RouteGroup que avalia, o Router emite um **evento de histórico de roteamento** para `events.router`, registrando o resultado de cada Router e o **raciocínio condição a condição** por trás dele:

```
"data.temperature" greater than 8 → allowed
"data.doorState" equals "open"    → denied (field not found)
policy: all → 1/2 rules passed    → denied
```

Você sempre consegue responder *por que este evento disparou — ou não — aquele workflow?* sem reexecutar nada. O roteamento deixa de ser uma caixa-preta.

---

## Como um batch é roteado

O Router puxa eventos em batches (padrão `8000`, `NATS_BATCH_SIZE`) de uma **WorkQueue** do JetStream e processa cada batch em três fases:

```
 Phase 1 — route in parallel       a worker pool (NumCPU × 2) resolves each asset,
                                    evaluates its RouteGroups, and buffers the
                                    fan-out publishes
 Phase 2 — flush once              a single FlushConnection pushes every buffered
                                    publish to the wire in one round
 Phase 3 — settle sequentially     Ack / Nack / Reject each message by its result
```

Os resultados se mapeiam na semântica do JetStream: uma mensagem malformada ou sem `orgId` / `assetUUID` / `event` é **Rejected** direto para a dead-letter queue (retentar não resolve); uma falha transitória, como um cache miss, recebe **Nack** e é reentregue. A entrega via WorkQueue garante que cada evento seja roteado por exatamente uma réplica.

---

## Contexto do asset — um tiered cache mantido coerente

Rotear cada evento exige o asset completo, rápido. O Router o resolve por um **TieredCache** que cai progressivamente para tiers mais baratos-porém-mais-frios:

```
L0  RAM   (256 MB, 5 min)  →  L1  Disk (/tmp/mapexos/cache)  →  L2  MinIO/S3  →  HTTP fallback → Assets
```

Para manter o cache de cada réplica honesto, o Router roda consumidores **FANOUT**: quando o serviço de Assets altera um asset ou um template, ele transmite uma invalidação que cada réplica aplica ao seu L0/L1 local. Nenhuma réplica roteia com um asset desatualizado.

---

## Governança e multi-tenancy

Os RouteGroups são gerenciados por uma superfície REST governada. Toda rota é **controlada por permissão** e **ciente de cobertura** — as listagens são restritas à fatia da árvore da organização que o chamador pode enxergar, por `pathKey`. Como os triggers, um RouteGroup pode ser um template **system** (global), um **vendor/customer template** herdado pelos descendentes (`isTemplate`), ou **org-local** — de modo que uma política de roteamento definida uma vez no alto da hierarquia se aplica a cada site abaixo dela. As configs são lidas via cache-aside no Redis (TTL de 60 minutos) e aquecidas na escrita, então o caminho crítico de roteamento quase nunca toca o MongoDB.

---

## Entradas e saídas

**Consome (NATS JetStream):**

| Subject | Stream | Propósito |
|---|---|---|
| `route.execute` | `ROUTE-GROUPS` (WorkQueue + DLQ) | Um evento de asset ou transição de saúde a rotear. |
| `mapexos.fanout.asset.invalidate` | FANOUT | Despeja um asset do L0/L1 local. |
| `mapexos.fanout.template.invalidate` | FANOUT | Despeja um template do L0/L1 local. |

**Produz (NATS):** `events.save`, `events.lake_house`, `events.notification`, `trigger.router.execute`, `workflow.execution.router` e `events.router` (histórico de roteamento). Os dispatches são deduplicados por message id.

**HTTP** (porta `5003`):

- `GET/POST /api/v1/route_groups`, `GET /api/v1/route_groups/counter`, `GET/PATCH/DELETE /api/v1/route_groups/:id` — JWT, com controle de permissão e escopo de cobertura.
- `GET /internal/route_groups?ids=…&projection=…` — API-key, lookup em massa serviço a serviço.

---

## Observabilidade

Cada estágio é medido no Prometheus: eventos recebidos / roteados / falhos com histogramas de latência, hit/miss do TieredCache **por tier**, taxa de acerto do cache de RouteGroup, avaliações de match e seus resultados, e contagens/latência de publicação **por subject a jusante**. Aliado ao histórico de roteamento por decisão, você enxerga tanto o fluxo agregado quanto a razão por trás de qualquer decisão de roteamento individual.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Onde as ações de saída que deram match rodam | [Triggers](./triggers.md) |
| Onde os workflows que deram match executam | [Workflow Engine](./workflow.md) |
| Onde os eventos roteados são armazenados | [Events](./events.md) |
| Contexto completo da plataforma | [Visão Geral da Arquitetura](./overview.md) |
