---
title: Execução de JS em Workflows
description: O substrato de computação para os nós de código do workflow — roda o JavaScript de cada nó em um isolate V8 endurecido e devolve o resultado ao Workflow Engine para retomar o DAG.
---

# Execução de JS em Workflows

A Execução de JS em Workflows é o **substrato de computação para os nós `code` do workflow**. Quando um [workflow](./workflow.md) chega a um nó que roda JavaScript customizado, o engine não o executa inline — ele **suspende o nó**, despacha o script para cá e estaciona. Este serviço roda o código em um sandbox V8 endurecido e **publica o resultado de volta no subject de resume do workflow**, acordando exatamente aquele nó para que o DAG continue. É o mesmo contrato de suspender → rodar em outro lugar → retomar que você viu nos [Triggers](./triggers.md), aplicado à computação dentro do grafo.

Ele compartilha seu **sandbox engine** com a [Execução de JS](./js-execution.md) — os mesmos isolates V8 do `isolated-vm` sobre um pool de workers Piscina, os mesmos limites rígidos (**heap de 32 MB**, **timeout de 10 segundos**, isolate reciclado a cada 10.000 execuções, sem acesso ao Node, à rede ou ao filesystem). O que muda é o **contrato**: este serviço roda *um script arbitrário do usuário por nó*, com o estado do workflow em escopo, e não o pipeline fixo de decode→validação→transform. Como no seu irmão, o código roda sobre o **V8 completo** — o **ECMAScript (ES2015+)** moderno é nativo (classes, arrow functions, destructuring, `Map`/`Set`, os built-ins padrão), limitado apenas pelo orçamento de **32 MB** do isolate.

> **Porta** `8001` · **Node.js + TypeScript** · isolates V8 do **`isolated-vm`** sobre **Piscina** · movido inteiramente por NATS — sem superfície HTTP funcional · stateless.

---

## O contrato do nó de código

Cada requisição injeta quatro globais no isolate, dando ao script o contexto do workflow em execução:

| Global | O que carrega |
|---|---|
| `event` | O event que dispara a execução. |
| `state` | O estado persistente do workflow. |
| `inputs` | Os inputs já resolvidos do nó. |
| `nodes` | As saídas dos nós anteriores. |

O script retorna duas coisas — **`output`** (consumido pelos nós seguintes) e **`statePatch`** (mesclado ao estado do workflow). Há duas formas de retorná-los, e o engine aceita as duas:

```js
// explicit
result = { output: { celsius }, statePatch: { lastReading: celsius } };

// or convenience — a bare object return is wrapped as { output: <value>, statePatch: {} }
return { celsius };
```

O serviço então publica um `WorkflowScriptCallback` — `{ status: 'success' | 'error', output?, statePatch?, error? }` — no `callbackSubject` que o runtime forneceu, no stream de **resume** do workflow. **Exatamente um callback é publicado por requisição**, de sucesso ou de erro (incluindo um `SCRIPT_NOT_FOUND` quando o source de um nó está ausente), de modo que um nó de workflow nunca fica pendurado esperando por um resultado que não vai chegar.

---

## Cold starts mais rápidos — cache de bytecode do V8

Diferente do pipeline de assets por event, este serviço **persiste o bytecode V8 compilado entre cold starts**. Numa compilação fresca, o worker emite o `cachedData` do script, que é armazenado (fire-and-forget) em um **cache de bytecode** em camadas (RAM → disco → MinIO); na próxima vez que um worker compilar aquele nó — mesmo em outro pod após um restart — ele devolve o bytecode ao V8 para uma compilação de **~1–5 ms** em vez de **~10–50 ms**. O bytecode é **escopado por definição** e reaproveitado em todas as instâncias em execução de um workflow; em caso de incompatibilidade de versão do V8, ele silenciosamente recai numa compilação fresca. Source e bytecode são buscados **em paralelo** para manter o caminho frio curto.

O próprio source JavaScript do nó vem de um cache em camadas (`{orgId}/{workflowId}/scripts/{nodeId}`), mantido atualizado por um broadcast **FANOUT** que o Workflow Engine emite quando uma definição muda — então editar um nó de código entra em vigor sem restart.

---

## Tratamento de falhas

O engine trata código mal-comportado como rotina, com a mesma disciplina do executor de assets:

- Um script que **esgota o heap de 32 MB** descarta seu isolate; o worker reporta a condição de out-of-memory e a mensagem recebe **NACK para retry com backoff** num isolate novo.
- **Qualquer outro erro de script** publica um callback de *erro* no workflow e dá **ACK** na mensagem — o nó do workflow recebe a falha na sua aresta `error`, em vez de o script ser retentado para sempre.

---

## Entradas e saídas

**Consome (NATS JetStream):** `workflow.js.code` (stream `JSWORKFLOWEXECUTOR-CODE`, consumer de fila com balanceamento de carga) — uma mensagem por disparo de nó de código; e `fanout.workflow.definition.invalidate` (FANOUT) para descartar sources/bytecode em cache obsoletos.

**Produz (NATS):** um `WorkflowScriptCallback` no `callbackSubject` fornecido pelo runtime, no stream de **resume** do workflow. O serviço não fixa nenhum subject de saída no código — ele ecoa de volta para onde o runtime mandou.

Ele não expõe **nenhuma API HTTP funcional** — apenas `/health` e `/metrics`. Toda interação é via NATS, movida pelo Workflow Engine.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| O engine que despacha estes nós | [Workflow Engine](./workflow.md) |
| O sandbox irmão para payloads de dispositivos | [Execução de JS](./js-execution.md) |
