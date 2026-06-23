---
title: Events
description: O sistema de registro do MapexOS — um sink ClickHouse que transforma cada sinal em um event tipado, consultável e retido por tenant, rastreável de ponta a ponta por um único tracking id.
---

# Events

O Events é o **sistema de registro** do MapexOS. É o fim da linha: todo outro estágio — ingest, transform, route, automate — entrega sua saída aqui, e o Events a persiste no **ClickHouse** como uma linha tipada e consultável. Ele não toma decisões e não publica nada de volta no bus; é puro **armazenamento e leitura**. Quando você pergunta *o que aconteceu, com qual asset, quando e por quê* — é este o serviço que responde.

Seu truque definidor é armazenar **dados sem schema em uma forma tipada e colunar**: campos arbitrários por vendor viram colunas tipadas e consultáveis sem migration, cada linha carrega sua própria retenção, e um único tracking id amarra toda a jornada de um event.

> **Porta** `5004` · **Go** (DDD + Hexagonal) · consumers em batch do **NATS JetStream** → **ClickHouse** (bulk insert, retenção por TTL) · **MongoDB** (catálogo de retenção) + **Redis** + **S3/MinIO** (cache de templates em camadas) · três módulos: `events`, `asset_status`, `retention`.

---

## EVA — armazenamento tipado para dados sem schema

Cada asset fala um payload diferente: um manda `temperature`, outro `co2`, um terceiro `doorState`. Armazenar isso como blobs JSON tornaria tudo inconsultável; armazenar uma coluna por campo significaria uma migration por dispositivo. O MapexOS não faz nem um nem outro. Ele usa **EVA** (Entity-Value-Attribute): cada campo de template recebe um `fieldId` numérico, e o valor pousa em uma de **quatro colunas MAP tipadas do ClickHouse**, chaveada por esse id:

| Coluna | Tipo | Carrega |
|---|---|---|
| `eva_number` | `map<uint16, float64>` | campos numéricos — temperatura, pressão, contagem |
| `eva_string` | `map<uint16, string>` | campos de texto — status, localização, tipo de dispositivo |
| `eva_bool` | `map<uint16, uint8>` | booleanos — alarme, online, ativo |
| `eva_date` | `map<uint16, datetime>` | timestamps — última atualização, horário do alarme |

O tipo vem do tipo declarado do campo no template do asset (com inferência como fallback; um campo geo armazena sua latitude como número *e* como string `"lat,lng"`). O resultado: um novo vendor ou modelo é uma **configuração de template**, nunca uma mudança de schema — e ainda assim todo campo dinâmico é armazenado de forma **tipada e colunar**, de modo que você possa filtrar e agregar sobre ele diretamente. O endpoint `POST /store/query` recebe uma lista de `EvaFilters` — condições sobre esses campos dinâmicos — e as empurra direto para o ClickHouse.

Para reverter os ids numéricos em nomes legíveis na leitura, o Events resolve o template do asset através de um **cache em camadas** (L0 RAM → L1 disco → L2 S3/MinIO → fallback HTTP para o serviço Assets), mantido coerente por um consumer **FANOUT** `template.invalidate`, para que uma edição de template upstream nunca sirva nomes obsoletos.

---

## Retenção por linha — cada linha sabe quanto tempo deve viver

Retenção não é uma única configuração global. **Cada linha é carimbada no momento da escrita com seu próprio `retention_days`**, resolvido por organização e por tabela a partir do módulo `retention` (um catálogo no MongoDB, cache-aside no Redis), e o **TTL** nativo do ClickHouse a expira automaticamente. Classes diferentes de dados vivem por períodos diferentes por padrão — payloads brutos e logs de debug por pouco tempo, históricos de execução por mais tempo, events processados por mais tempo ainda — e qualquer organização pode definir sua própria política:

| Classe de dado | Tempo de vida padrão |
|---|---|
| Ingest bruto · debug do JS-executor | 1 dia |
| Histórico de execução de Router / trigger / workflow | 7 dias |
| Events processados · dead-letter | 30 dias |

Uma nova organização é semeada com políticas padrão automaticamente (o módulo consome o event de ciclo de vida `organization.created`), e editar uma política emite a mudança de TTL correspondente no ClickHouse.

---

## O que ele armazena — um consumer por stream

O Events roda um consumer em batch por stream, cada um escrevendo sua própria tabela no ClickHouse:

| Stream | O que captura |
|---|---|
| `events.save` | **Events processados** — a tabela primária, com mapeamento EVA completo. |
| `events.raw` | Payloads de ingest brutos, como recebidos. |
| `events.logs.jsexecutor` | Saída de debug de decode/validate/convert do JS-executor. |
| `events.router` | Decisões do Router — o histórico de roteamento por event. |
| `events.trigger` | Desfechos de execução de triggers. |
| `events.workflow` (`EVENTS-WORKFLOW-LOGS`) | Execuções terminais de workflow. |
| `dlq` | Entradas de dead-letter de qualquer serviço. |

Juntos, eles formam uma trilha de auditoria completa e consultável: o próprio event processado, *e* o registro de como ele foi roteado, o que ele disparou e qual workflow ele rodou.

---

## Rastreável de ponta a ponta por um único id

Um único `eventTrackerId` (um UUID) é cunhado no ingest e propagado por todo serviço — route, trigger, workflow — e gravado em cada linha aqui. Então reconstruir toda a história de um event é uma única busca por chave, não um join entre sistemas: você consegue segui-lo do payload bruto, passando pela decisão de roteamento, até o trigger que disparou e o workflow que rodou. Os nomes (`asset_name`, `template_name`, …) são **desnormalizados no momento da escrita**, então as leituras nunca pagam por um join.

---

## Como um batch é persistido

Todos os consumers compartilham um pipeline genérico de três fases, afinado para o gosto do ClickHouse por escritas grandes:

```
 Phase 1 — parse in parallel     a bounded worker pool (NumCPU × 2) parses,
                                  validates, and EVA-maps each message
 Phase 2 — one bulk insert       every valid row goes to ClickHouse in a single
                                  bulk INSERT — minimal write amplification
 Phase 3 — settle per message    Ack on success · Nack (retry) on insert failure
                                  · Reject (→ DLQ) on a parse/validation failure
```

O consumer de dead-letter usa uma variante deliberada que **nunca dá Nack** — uma mensagem que ele não consegue parsear é reconhecida e pulada, em vez de retentada, de modo que a DLQ nunca possa se realimentar em loop.

---

## Além dos events — histórico de conectividade

Um segundo módulo, `asset_status`, persiste cada **transição online/offline** que um asset faz em sua própria tabela e serve uma **timeline de conectividade**: `GET /api/v1/events/connectivity_history` e `…/assets/:assetUUID/connectivity_history`, filtrável por janela de tempo e tipo de transição. A operação consegue ver exatamente quando um dispositivo caiu e voltou — um histórico de primeira classe, não algo reconstruído a partir de logs.

---

## Ler e gerenciar — a superfície HTTP

Todas as rotas de leitura são **paginadas por cursor, escopadas por organização e permission-gated**, sob `/api/v1/events`:

- Listas por stream: `GET /raw`, `/jsexec`, `/router`, `/trigger`, `/workflow`, `/dlq` (+ `/dlq/counts` agrupado por serviço).
- `POST /store/query` — events processados com `EvaFilters` opcionais (consulta por campo dinâmico).
- `GET /store/:eventTrackerId` — um event processado com seus ids EVA resolvidos para os nomes definidos no template.
- `GET /workflow/execution/:executionId` — um único registro de workflow.
- Timelines de conectividade (acima).

As políticas de retenção são gerenciadas sob `/api/v1/retention`: `GET /`, `GET /:id`, `PUT /` (upsert por `{orgId, type}`), `DELETE /:id`.

---

## Entradas e saídas

**Consome (NATS JetStream, pull em batch):** `events.save`, `events.raw`, `events.logs.jsexecutor`, `events.router`, `events.trigger`, `events.workflow`, `dlq`, `events.asset_status_save`, `organization.created`, e o FANOUT `template.invalidate`.

**Produz:** nada no bus. O Events é um sink terminal — seu único efeito de saída é uma mudança de TTL no ClickHouse quando uma política de retenção é editada.

---

## Observabilidade

Métricas Prometheus por consumer acompanham mensagens consumidas / parseadas / inseridas / falhas, **latência de bulk insert no ClickHouse por tabela**, hit/miss do cache de templates EVA e tamanhos de batch. O health expõe o grafo de dependências (MongoDB, Redis, NATS obrigatórios; ClickHouse e S3 degradam graciosamente — o serviço continua aceitando mensagens do NATS mesmo quando as escritas estão brevemente indisponíveis).

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Quem decide o que é persistido | [Router](./router.md) |
| Onde os campos dinâmicos são definidos | [Visão Geral da Arquitetura](./overview.md) |
| O que produz os históricos de execução | [Triggers](./triggers.md) · [Workflow Engine](./workflow.md) |
