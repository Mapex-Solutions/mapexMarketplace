---
title: Triggers
description: O serviço que fala com o mundo externo — um único ponto de saída, oito conectores de protocolo, disparado de forma autônoma a partir do Router ou integrado a workflows por um resume totalmente assíncrono.
---

# Triggers

O Triggers é **como o MapexOS alcança o mundo externo**. Tudo o que vem antes — ingest, transform, route, automate — acontece dentro da plataforma; o Triggers é o **único ponto de saída** onde um event vira uma ação real contra um sistema externo: um webhook HTTP, um publish MQTT, um e-mail, uma mensagem no Slack. É o único serviço do MapexOS que faz I/O de saída.

Ele alcança o mundo externo de **duas formas**, e roda em **dois modos** — por conta própria, movido pelo Router, ou integrado ao Workflow engine por um resume totalmente assíncrono. O mesmo catálogo de conectores sustenta os dois.

> **Porta** `5006` · **Go** (DDD + Hexagonal) · **8 conectores de protocolo** atrás de uma porta · consumers **NATS JetStream** + **MongoDB** (fonte da verdade) + **Redis** (cache-aside) · instrumentação **Prometheus**.

---

## Duas formas de alcançar o mundo externo

| Forma | O que é |
|---|---|
| **Stored triggers** | Uma definição persistida e reutilizável — o catálogo configurado de efeitos colaterais. Criada pela API REST, armazenada no MongoDB, cacheada no Redis e **disparada por id**. É o que o Router usa: "uma regra deu match → dispara o trigger `X`." |
| **Generic connector calls** | Uma ação totalmente resolvida que chega via NATS e é despachada direto para um conector — **sem consulta ao banco**. É o caminho que dá vida aos **plugins**: a ação é montada em outro lugar (o Workflow engine) e o Triggers simplesmente a executa. |

Os dois caminhos convergem para o mesmo **catálogo de conectores**. A diferença é apenas *de onde vem a ação* — uma definição armazenada buscada por id, ou uma ação pronta entregue na rede.

---

## Dois modos de operação

### Standalone — movido pelo Router

O Router avalia regras de match contra cada event e faz fan-out para os triggers que devem disparar. Ele publica em `trigger.router.execute`; o Triggers busca cada stored trigger por id, resolve seus placeholders e o despacha. Sem workflow, sem orquestração — um reflexo direto de **event → ação**.

```
Event → Router (match rules, fan-out) → trigger.router.execute → Triggers → external system
```

### Integrado — movido pelo Workflow engine

Um nó de workflow que precisa de uma ação de saída **não a executa inline**. O engine suspende o nó, entrega a ação resolvida ao Triggers em `trigger.workflow.execute` e **estaciona** — passando um `callbackSubject` que aponta de volta para seu próprio stream de resume. O Triggers executa e então publica o resultado nesse subject, que acorda exatamente aquele nó e dá continuidade ao fluxo.

```
Workflow node suspends (waitType: callback)
   │  resolved action + callbackSubject  →  trigger.workflow.execute
   ▼
Triggers   (dispatch through the connector → outbound I/O)
   │  result  →  callbackSubject  (workflow.resume.*)
   ▼
Workflow RESUME  →  the parked node wakes and the flow continues
```

É isso que torna a execução de workflow **100% assíncrona**: o worker de workflow nunca bloqueia esperando por uma chamada externa. Ele estaciona, o Triggers conversa com o mundo, e o resume retoma de onde parou. (Veja a outra metade desse contrato em [Workflow Engine → suspensão e resume](./workflow.md).)

O caminho do workflow carrega dois modos num único subject, roteados por um campo `mode`: `trigger` (disparar um stored trigger por id) e `plugin` (rodar uma ação totalmente resolvida, sem busca).

---

## O catálogo de conectores — 8 adapters, uma porta

Todo conector implementa uma única porta `TriggerExecutor` e é registrado por uma string de tipo na inicialização. O dispatch é uma consulta em runtime por `triggerType` — **um novo conector é um novo executor, não uma mudança no serviço**.

| Categoria | Conectores |
|---|---|
| **Técnicos** | `http` · `mqtt` · `rabbitmq` · `nats` · `websocket` |
| **Comunicação** | `email` · `teams` · `slack` |

Se um event nomear um tipo sem executor registrado, a mensagem é rejeitada para a dead-letter queue, em vez de ser silenciosamente descartada.

---

## Templates que descem pela árvore organizacional

Um trigger nem sempre é um objeto de nível folha. Seu escopo é um de três níveis, então uma integração pode ser **definida uma vez e herdada**, em vez de copiada para cada site:

| Nível | Significado |
|---|---|
| **System** (`isSystem`) | Um template global do MapexOS — sem organização, disponível em todo lugar. |
| **Template** (`isTemplate`) | Um template de vendor / customer — **herdado por todo descendente** na hierarquia organizacional. |
| **Local** | Um trigger específico de organização, escopado ao seu `orgId` e `pathKey`. |

Defina uma escalada no Slack ou um webhook de ITSM uma única vez no nível de vendor, e cada customer, site e floor abaixo dele herda a mesma ação — sem duplicá-la sessenta vezes.

---

## Resolução de placeholders — triggers são templates

Uma config de trigger carrega placeholders `{{path.to.field}}` que são resolvidos contra o payload do event de entrada por navegação dot-path, logo antes da execução. Um único stored trigger atende milhares de events, cada um preenchido com seus próprios dados.

Dado um payload de event:

```json
{ "assetName": "Pump-A3", "data": { "temperature": 87.5 } }
```

e um template de corpo de trigger:

```json
{ "text": "Alert: {{assetName}} reached {{data.temperature}} C" }
```

o corpo despachado fica:

```json
{ "text": "Alert: Pump-A3 reached 87.5 C" }
```

---

## Como um batch executa — três fases

O Triggers puxa trabalho em batches e processa cada batch em **três fases deliberadas**, de modo que uma rajada de ações custe uma ida à rede, e não uma por mensagem:

```
 Phase 1 — execute in parallel     every message runs on its own goroutine,
                                    bounded by a worker semaphore (default 50)
 Phase 2 — flush once              a single FlushConnection pushes all the
                                    fire-and-forget publishes to the wire
                                    in one TCP round
 Phase 3 — settle sequentially     Ack / Nack / Reject each message by its result
```

O semáforo limita a concorrência para que um endpoint lento não esgote o processo; o flush único transforma N publishes em uma ida e volta; a fase de settle é onde o destino de cada mensagem é decidido.

---

## Tratamento de falhas — mapeado para a semântica do JetStream

Os desfechos mapeiam de forma limpa para o modelo de entrega do JetStream, de modo que os retries sejam significativos e becos sem saída não sejam retentados para sempre:

| Desfecho | O que acontece |
|---|---|
| **Erro permanente** (mensagem malformada, conector desconhecido, config ruim / placeholder não resolvido) | **Reject** → direto para a dead-letter queue. Sem retries inúteis. |
| **Erro transiente** (a chamada de saída falhou, a config não pôde ser buscada) | **Nack** → o JetStream reentrega; esgotadas as tentativas de entrega, a mensagem cai na dead-letter queue. |
| **Trigger desabilitado** | **Ack** sem ação — um no-op, não um erro. |

Todo desfecho é registrado (veja [Observabilidade](#observabilidade)), então uma mensagem na DLQ é sempre explicada por uma linha de auditoria persistida.

---

## Seguro por padrão

A borda de saída é onde uma plataforma é mais fácil de enfraquecer — então os conectores não enfraquecem. O TLS é **verificado, não pulado** (`InsecureSkipVerify` fica desligado), cada conector impõe seus próprios timeouts de connect e publish (ex.: MQTT connect 10s / publish 5s), e as credenciais são fornecidas por conector (HTTP `Authorization` / Bearer, TLS do broker). O MapexOS não entrega nenhum conector que desabilite a verificação de certificado.

---

## Governança e multi-tenancy

A superfície de CRUD é governada de ponta a ponta. Toda rota é **permission-gated** (`TriggerList`, `TriggerRead`, `TriggerCreate`, `TriggerUpdate`, `TriggerDelete`) e **ciente de cobertura**: um middleware hierárquico escopa toda listagem à fatia da árvore organizacional que o chamador tem permissão de ver, por `pathKey`. As configs de trigger são lidas através de uma camada **cache-aside** (Redis, TTL de 60 minutos) e invalidadas em update ou delete, então a execução raramente toca o MongoDB e ainda assim se mantém atualizada.

---

## Entradas e saídas

**Consome (NATS JetStream, stream `TRIGGERS`):**

| Subject | Origem | Propósito |
|---|---|---|
| `trigger.router.execute` | Router | Disparar um stored trigger por id (modo standalone). |
| `trigger.workflow.execute` | Workflow | Execução movida por workflow, roteada por `mode` (`trigger` / `plugin`). |

**Produz:**

- **Efeitos colaterais de saída** para sistemas externos através do conector selecionado — o trabalho de fato do serviço.
- **Event de auditoria** → `events.trigger` (por execução: trigger, type, success, duração, error), de-duplicado no JetStream por um message id `{eventTrackerId}-triggerlog`, consumido pelo serviço [Events](./events.md) → ClickHouse.
- **Resume de workflow** → o `callbackSubject` dinâmico no stream de resume do workflow, acordando o nó estacionado.

**HTTP** (porta `5006`, sob `/api/v1/triggers`): `GET /`, `GET /counter`, `POST /`, `GET /:id`, `PATCH /:id`, `DELETE /:id`.

---

## Observabilidade

Toda execução é medida. Counters e histogramas do Prometheus acompanham o tamanho do batch, a **duração do executor por tipo de conector**, hit/miss de cache, resoluções de placeholder, desfechos de publish e contagens de DLQ — e o **caminho movido por workflow carrega seu próprio conjunto dedicado de métricas** (incluindo os publishes de resume), de modo que o tráfego standalone e o integrado nunca se misturem. Combinado com a linha de auditoria idempotente por execução, você consegue responder *o que disparou, para onde foi o tempo e por quê* — por protocolo e por origem.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Quem decide que um trigger deve disparar | [Router](./router.md) |
| O contrato assíncrono do lado do workflow | [Workflow Engine](./workflow.md) |
| Onde as linhas de auditoria são armazenadas e consultadas | [Events](./events.md) |
| Contexto completo da plataforma | [Visão Geral da Arquitetura](./overview.md) |
