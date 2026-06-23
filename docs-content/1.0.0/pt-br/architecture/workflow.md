---
title: Workflow Engine
description: O cérebro de automação do MapexOS — um engine de DAG durável e nativo de NATS que decide e delega, sobrevivendo a restarts e retomando no meio do grafo.
---

# Workflow Engine

O Workflow Engine é o **cérebro de automação** do MapexOS: um runtime durável, baseado em DAG, que executa os workflows que os operadores desenham no editor visual. Seu traço definidor é como lida com side-effects — em vez de executar a **ação de saída** de um node inline, ele **resolve a ação, entrega a um serviço especializado, depois suspende e retoma no callback**: ações HTTP/MQTT de saída vão para o serviço de [Triggers](./triggers.md), nodes `code` vão para o [JS Workflow Execution](./js-workflow-execution.md), e a descriptografia de credenciais vai para o serviço Vault. O engine ainda faz as chamadas internas que a orquestração exige — chama o serviço Vault para descriptografar credenciais e roda um proxy HTTP para opções dinâmicas de plugin — mas **nunca bloqueia um worker à espera de uma ação de saída**. Essa decomposição, somada a um checkpoint por step no NATS KV, é o que mantém cada run **determinístico e recuperável de crash**: um node que dispara uma ação não segura o run aberto — ele estaciona, e um callback o desperta.

> **Porta** `5007` · **Go** (DDD + Hexagonal) · **NATS JetStream + KV** para o estado vivo (sem Redis) · **MongoDB** + **MinIO** + **TieredCache** · editor **Vue 3 / Quasar**.

---

## O modelo: Definition → Instance → Execution

| Entidade | O que é |
|---|---|
| **Definition** | O blueprint do DAG — nodes, edges e os scripts dos nodes. |
| **Instance** | Uma definition vinculada a inputs específicos (`externalInputs`) — a configuração para um tenant ou asset. |
| **Execution** | Um run real, com seu próprio estado mutável, status e histórico. |

A cardinalidade é `Definition 1→N Instance 1→N Execution`. Definitions e instances são cacheadas (TieredCache → MongoDB). O estado vivo de trabalho de uma execution roda no NATS KV; seu ciclo de vida é espelhado no MongoDB para a UI, e quando ela termina também é publicada no serviço de Events para armazenamento permanente (veja [Storage](#storage)).

---

## Como um run executa

O runtime percorre o DAG **um node por vez**. Após cada avanço ele faz um **checkpoint de toda a execution em um registro do NATS KV** (`exec.{uuid}`) que guarda o estado mutável completo — `State`, `NodeOutputs`, `NodeStates`, `ActiveNodeIDs`, `ExecutionPath`. Se um worker sofre crash, um restart recarrega esse registro e **retoma a partir do último node concluído**, não do começo.

```
 ┌─ execute() ──────────────────────────────────────────────┐
 │  pick next node → run executor → apply result            │
 │        │                                                  │
 │        ├── checkpoint(execution) → NATS KV  exec.{uuid}   │
 │        │                                                  │
 │        ├── more nodes & steps < 300 ? ──▶ loop inline     │
 │        ├── steps ≥ 300 ? ──────────────▶ re-enqueue RESUME│
 │        └── node suspended ? ───────────▶ park + dispatch  │
 └──────────────────────────────────────────────────────────┘
```

Um único ciclo roda até `MaxInlineSteps = 300` nodes inline; além disso o runtime **se reenfileira** no stream de resume em vez de segurar um worker.

---

## Suspensão e resume — um contrato assíncrono uniforme

Nodes que não conseguem terminar de forma síncrona **suspendem**, e o contrato é **orientado a dados**: um executor sinaliza suspensão simplesmente colocando uma chave `waitType` em seu `NodeState`. O walker faz o checkpoint e despacha por tipo:

| `waitType` | Desperta quando… | Usado por |
|---|---|---|
| **timer** | um timer do NATS Schedule dispara | `delay`, backoff de retry, expiração de wait |
| **signal** | um signal HTTP externo chega | `wait_signal` |
| **condition** | a condição é reavaliada como verdadeira | `wait_for` |
| **callback** | um serviço externo retorna um resultado | `code`, `subworkflow`, nodes de plugin |

Os despertares de callback e signal chegam no stream `WORKFLOW-RESUME`; os de timer disparam no stream `WORKFLOW-SCHEDULE` — ambos retomam exatamente a execution. Esse único contrato cobre timers, signals humanos/externos, condições por polling e callbacks entre serviços — sem tratamento especial por node.

---

## O catálogo de nodes — 17 executores core + plugins

Todo node `core/*` mapeia para um executor embutido (`registry_builder.go`); qualquer tipo que não seja `core/` é roteado para um único **executor de plugin genérico**.

| Grupo | Nodes |
|---|---|
| **Inline** | `start` · `end` · `condition` · `switch` · `set_state` · `log` · `goto` |
| **Async (suspendem)** | `delay` · `wait_signal` · `code` · `subworkflow` · `trigger_event` |
| **Controle de fluxo** | `fanout` · `merge` · `sequence` · `loop` · `wait_for` |
| **Plugin** | qualquer tipo `<plugin>/<action>` → o executor de plugin genérico |

*(O editor também oferece `text_note` e `group_frame` — decorações apenas visuais, sem executor.)*

---

## Condições — AND / OR / NAND / NOR + 18 operadores

As condições não são um único threshold. O engine avalia **grupos aninháveis** com um `EvaluateGroup` recursivo e de curto-circuito — `AND`, `OR`, `NAND` (`!AND`), `NOR` (`!OR`) — sobre **18 operadores**:

- **Comparação:** `equals`, `notEquals`, `greaterThan(Equals)`, `lessThan(Equals)`, `between`
- **String:** `contains`, `notContains`, `startsWith`, `endsWith`, `regex`
- **Data / hora:** `beforeDate`, `afterDate`, `beforeTime`, `afterTime`, `betweenDate`, `betweenTime`

O mesmo avaliador sustenta `condition`, `switch` (modo de match `first` / `all`) e `wait_for` (um grupo de condição única). Um field que não pode ser resolvido degrada para **não-match** em vez de lançar erro — comportamento previsível sob dados parciais.

---

## State — persistente vs efêmero

Dois escopos, e a distinção é essencial:

| Escopo | Tempo de vida | Mutado por |
|---|---|---|
| **`State`** | Persiste por todo o run | patches de `set_state`: `set` · `increment` · `decrement` · `append` · `remove` (um valor nil apaga a chave) |
| **`NodeStates[nodeId]`** | Efêmero, por node | índice de loop, contagem de branches do merge, tentativa de retry, info de wait — limpos quando o node conclui |

Os inputs de um node são lidos por um **resolvedor de valores com seis fontes**: `literal` (com interpolação de template `{{...}}`), `event`, `state`, `variable` (um alias de `state`), `input` (os `externalInputs` da instance) e `node_output` (`NodeOutputs[nodeId]`).

---

## Paralelismo e controle de fluxo

- **fanout → merge** — `fanout` (≤ 20 branches) cria goroutines, cada uma com uma cópia **isolada e deep-copy** do state/outputs, de modo que os branches não conseguem corromper uns aos outros. `merge` os une com a estratégia `all` / `any` / `first`. Se os branches suspendem, o fanout inteiro suspende e retoma por branch.
- **loop** — itera sobre um array resolvido (≤ 10 000 iterações), injetando `loop_item` / `loop_index`. O walker mantém uma **pilha de loop LIFO**, então loops aninhados e a continuação pós-loop funcionam corretamente.
- **switch** — ramificação multidirecional sobre o avaliador de condições compartilhado.
- **subworkflow** — roda outro workflow como um node (profundidade ≤ 10, com dedup), mapeando `inputMappings` na entrada e `outputMappings` na volta.

---

## Goto — links virtuais que mantêm o DAG limpo

`core/goto` vem em portais **sender / receiver** pareados, casados por `pairLabel`. Em tempo de build, o GraphBuilder **injeta um edge de adjacência** de cada sender para o seu receiver — de modo que o grafo no editor permanece acíclico e legível enquanto o run ainda pode "saltar" através dele. Um sender órfão falha rápido com `GOTO_NO_RECEIVER`. É isso que permite a um workflow grande evitar fios cruzando todo o canvas.

---

## Retry e tratamento de erros

Um node pode rotear falhas por um **edge de saída `error`** explícito. Com um `ErrorHandler` habilitado, o node **retenta com backoff exponencial** — `initialInterval × multiplier^attempt`, limitado a `MaxRetryDelaySeconds = 3600`, com limite rígido `MaxRetryAttempts = 10` — implementado como uma suspensão `timer` no NATS Schedule (para que os retries também sobrevivam a restarts).

---

## Nodes de plugin — pura orquestração

Um node de plugin (`<plugin>/<action>`, por exemplo `telegram/message`) roda pelo **PluginExecutor** genérico, que:

1. carrega o **manifest** do plugin,
2. resolve os valores de field do node e os templates `{{context.field}}` (sobre manifest / credentials / config / workflow / event),
3. **descriptografa as credenciais** pelo serviço Vault, e então
4. **suspende com um payload de ação pronto** que o serviço de Triggers executa.

O engine nunca abre o socket por conta própria — ele monta a ação e a entrega. Novos conectores são manifests, não código do engine.

---

## Entradas e saídas

**Consome (NATS JetStream):**

| Stream / subject | Propósito |
|---|---|
| `WORKFLOW-EXECUTION` · `…workflow.execution.>` | Comandos de execução, roteados **por um campo `mode`** (`newInstance`, `signal`, `signalOrStart`, `subworkflow`) — nunca pelo subject. Vindos do router e de subworkflows. |
| `WORKFLOW-RESUME` · `…workflow.resume.>` | Resultados de callback / timer / signal que despertam runs suspensos. |
| `WORKFLOW-SCHEDULE` · `…workflow.schedule.fired` | Disparos de timer do NATS Schedule (delay / retry / expiração de wait). |

**Produz (NATS):** `…trigger.workflow.execute` → **Triggers** (dispatch de plugin + `trigger_event`); `…workflow.js.code` → **JS Workflow Execution** (para nodes `code`); execuções de subworkflow por instance; eventos de ciclo de vida `workflow.state.>` (`WORKFLOW-STATE`) → o **archiver** interno do serviço; e, ao terminar, `…events.workflow` (`EVENTS-WORKFLOW-LOGS`) → o serviço de **Events** para armazenamento permanente.

**HTTP** (porta `5007`, sob `/api/v1`): `workflow_definitions`, `workflow_instances` (incl. `POST /:id/execute`), `workflow_executions`, `plugins` (incl. `/enabled`), `load_options` (dropdowns dinâmicos de plugin).

---

## Storage

Uma execution percorre **três tiers** — e o próprio runtime **só escreve no NATS KV e nos streams do NATS; ele nunca toca o MongoDB**. O `archiver` (o único escritor do MongoDB) consome os eventos de ciclo de vida `workflow.state.>` (`WORKFLOW-STATE`) e os persiste.

| Tier | Onde | Papel |
|---|---|---|
| **Estado vivo de trabalho** | **NATS KV** (`exec.{uuid}`, CAS nativo via revision — **sem Redis**) | A execution em andamento; deletada do KV assim que termina. |
| **Histórico quente (UI/consulta)** | **MongoDB** (`dev-workflow`), via o archiver | Um registro leve no `created`, atualizações de status em `waiting`/`resumed`, o documento completo ao terminar — mantido sob um **TTL** para que o console consiga listar e consultar runs recentes. |
| **Registro frio / permanente** | **serviço de Events → ClickHouse** (`EVENTS-WORKFLOW-LOGS`) | Ao terminar, o archiver também publica a execution finalizada no serviço de Events para armazenamento durável e analítico. |

Definitions, instances e manifests de plugin são servidos a partir de um **TieredCache** (L0 RAM → L1 disk) respaldado por **MongoDB**; os blobs das definitions ficam no **MinIO/S3**.

---

## Cobertura de testes

Toda capacidade nesta página é exercitada por testes unitários em Go **e** por uma suíte end-to-end completa de DAG (`runtime/e2e/dag/core/`): os 17 executores core mais o executor de plugin; os grupos AND/OR/NAND/NOR e os 18 operadores; o checkpoint por step no KV e o suspend/resume; os timers do NATS Schedule e o retry com backoff; fanout/merge, a pilha de loop, a injeção de edge do goto e a profundidade + dedup de subworkflow; a resolução de manifest de plugin; e os caminhos do archiver e do `load_options`.

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Os conceitos por trás dos nodes e do state | [Conceitos Fundamentais → Automação](../getting-started/concepts.md#automation) |
| Onde as ações de saída de fato disparam | [Triggers](./triggers.md) |
| Onde os nodes `code` rodam | [JS Workflow Execution](./js-workflow-execution.md) |
| Como os eventos chegam a um workflow | [Router](./router.md) |
