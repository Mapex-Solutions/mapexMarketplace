---
title: Visão Geral da Arquitetura
description: Como o MapexOS é construído — princípios de design, o pipeline de eventos, multi-tenancy, durabilidade e os 10 serviços que compõem a plataforma.
---

# Visão Geral da Arquitetura

> **IoT-first, mas não limitado a IoT.**
> O MapexOS não enxerga devices nem sensores — ele enxerga **Assets**.
> Qualquer origem. Qualquer protocolo. Uma única abstração.
>
> **Conecte. Automatize. Escale.** — a plataforma aberta para integração de dados e automação inteligente.

Leia esta página se você está avaliando o MapexOS para uma carga de produção. Todo o resto da documentação remete a uma das ideias desta página.

O MapexOS trata cada sinal de entrada — um device, um gateway, uma API, um webhook de terceiros, um app interno — como um **Asset event**. A partir dessa única abstração, ele entrega aos operadores um pipeline para **ingerir, validar, normalizar, rotear, armazenar e automatizar** esse sinal. A plataforma é **self-hosted, multi-tenant, multi-arch**, e foi construída para a mesma disciplina operacional de qualquer outro sistema de produção da sua stack.

```
   Sources                       MapexOS                         Destinations
   ───────                       ───────                         ────────────
   Devices ──┐                                              ┌── Webhooks / APIs
   Gateways ─┤   Ingest → Validate → Transform → Route →    ├── Slack / Teams / Email
   APIs ─────┼──        Store / Notify / Automate           ├── NATS / MQTT
   Apps ─────┤                                              └── Custom plugins
   3rd-party ┘
```

---

## TL;DR — o pitch de 60 segundos

- **Uma única abstração para toda origem.** Devices, gateways, APIs, apps e webhooks de terceiros viram todos **Asset events** — um payload normalizado com `orgId`, `eventId`, `data`, `metadata`.
- **Dez serviços escaláveis de forma independente** (8 em Go, 2 em Node) unidos por um único contrato de evento no **NATS JetStream**.
- **Um Workflow Engine durável e recuperável a falhas** — um runtime de DAG com retries, timers e sub-workflows; cada step tem checkpoint no NATS KV, então uma execução retoma após um restart em vez de recomeçar do zero.
- **Um modelo de Plugin no estilo n8n** — novos nós de Workflow chegam como manifests declarativos servidos pelo plugin registry; o editor os carrega em runtime, então novos conectores entram em cena sem rebuild do frontend.
- **O código de decode / validate / transform roda em sandbox** dentro de **V8 isolates** (`isolated-vm` sobre um pool de workers); um cache de read-model em camadas (**L0 RAM → L1 disk → L2 MinIO/S3 → Fallback (reconstrói o cache)**) mantém a config quente rápida em toda a plataforma.
- **Multi-tenant desde a camada zero.** `orgId + pathKey` se propaga por todo contrato; templates e políticas herdam para baixo na árvore de organizações.
- **Secrets nunca ficam em texto puro.** Envelope encryption no `mapexVault`; o data plane vê apenas referências como `secrets.slack.token`.
- **Um único comando para subir a plataforma.** `docker compose up -d` a partir do `mapexOSDeploy`; imagens multi-arch para servidores Linux, Apple Silicon, Windows + Docker Desktop e Raspberry Pi 4/5.
- **Licenciado sob BSL 1.1.** Faça self-host, modifique e execute para a sua própria organização; a distribuição comercial em SaaS é reservada. Converte para Apache 2.0 na Change Date.

Se algum desses pontos for um impeditivo — ou inegociável — o resto desta página mostra onde cada um deles vive no código.

---

## O que o MapexOS é — e o que ele não é

**O MapexOS é:**

- **IoT-first, mas não limitado a IoT.** Devices e sensores são cidadãos de primeira classe, mas toda outra origem — REST API, webhook, app interno, SaaS de terceiros — entra pela mesma abstração de **Asset event**.
- **Um runtime de automação durável.** Um Workflow Engine baseado em DAG, com execução determinística, retries com backoff, timers, sub-workflows e Triggers idempotentes — o estado tem checkpoint no NATS KV a cada step e sobrevive a restarts de processo.
- **Um control plane multi-tenant.** Hierarquia de organizações, RBAC, membership de grupos e herança de templates são preocupações de primeira classe — não puxadinhos.
- **Um router orientado a políticas.** Para onde um evento vai — storage, lake house, Workflow, Trigger — é *configuração*, não código. As regras de match vivem no MongoDB e se aplicam em runtime (sem migração de schema, sem redeploy); as regras são lidas através de um cache com uma janela curta de refresh.
- **Um editor extensível.** Novos nós de Workflow chegam como **Plugins no estilo n8n** descritos por manifests que o editor carrega em runtime; novos conectores entram no editor sem um release de frontend.
- **Self-hostable, multi-arch.** Um único `docker compose up -d` sobe a stack. Imagens para `linux/amd64` e `linux/arm64` — roda em servidores Linux, Apple Silicon, Windows + Docker Desktop e Raspberry Pi 4/5.

**O MapexOS NÃO é:**

- ❌ **Não é um banco de dados de séries temporais.** A persistência é delegada ao **ClickHouse**; o MapexOS é o pipeline que o alimenta e a superfície de consulta que o interpreta.
- ❌ **Não é mais um produto de "ingestão-e-dashboard para IoT".** A maioria das plataformas nesse espaço para na ingestão e em um gráfico. O MapexOS trata a ingestão como a parte fácil e investe seu orçamento de complexidade em **roteamento, Workflows duráveis, extensibilidade via Plugin e multi-tenancy** — as partes que de fato quebram em escala.
- ❌ **Não é um SaaS gerenciado.** A plataforma é self-hosted, container-first. A distribuição comercial hospedada é uma oferta separada da Mapex, reservada sob a **BSL 1.1**.

> **Por que a distinção importa.** Uma demo com um widget de temperatura não é uma plataforma de frota. A parte difícil de operar cargas reais começa *depois* que o dado pousa — e é exatamente aí que o MapexOS gasta sua engenharia.

---

## O pipeline de eventos

Todo evento que entra no MapexOS percorre o mesmo pipeline canônico. **Cada seta neste diagrama é um NATS subject sob um stream do JetStream, persistido e replayable.**

```
   INGEST                    TRANSFORM                ROUTE                    ACT  (fan-out, 0..N)
   ──────                    ─────────                ─────                    ────────────────────

  ┌───────────────┐                                                      ┌───────────────────────────┐
  │ http_gateway  │──┐    ┌───────────────┐     ┌───────────────┐   ┌───▶│ events    :5004 → ClickHo │
  │ :5001         │  │    │ js-executor   │     │ router        │   │    └───────────────────────────┘
  └───────────────┘  │    │ V8   :8000    │     │ :5003         │   │    ┌───────────────────────────┐
                     ├───▶│ decode →      │────▶│ match-rule    │───┤───▶│ workflow  :5007 → DAG      │
  ┌───────────────┐  │    │ validate →    │     │ evaluation    │   │    └───────────────────────────┘
  │ Mosquitto +   │  │    │ transform     │     │ + fan-out     │   │    ┌───────────────────────────┐
  │ broker plugin │──┘    └───────────────┘     └───────────────┘   ├───▶│ triggers  :5006 → 8 execs │
  └───────────────┘             │                                   │    └───────────────────────────┘
   authenticate +         Standard Event +                          │    ┌───────────────────────────┐
   persist raw copy       EVA dynamic fields                        └───▶│ lake-house   sink         │
                                                                         └───────────────────────────┘
```

**Sources** — Devices (HTTP / MQTT) e webhooks/APIs de terceiros entram todos no *Ingest*. **Cada seta é um subject do NATS JetStream** (persistido e replayable): a ingestão emite `events.raw` e `processor.js.execute`; o router faz fan-out sobre `events.save`, `workflow.execution.router`, `trigger.router.execute`, `events.lake_house` e `events.router` (history).

O pipeline tem **seis estágios**, cada um pertencente a um serviço diferente e cada um escalável de forma independente:

| Estágio | Serviço dono | O que acontece | Saída |
|---|---|---|---|
| **1. Ingest** | `http_gateway`, MQTT broker | Autentica o produtor, anexa `orgId` + `dataSourceId`, persiste uma cópia bruta de auditoria | `events.raw`, `processor.js.execute` |
| **2. Decode** | `js-executor` | Roda o script `decode` do asset em um V8 isolate para transformar os bytes do fabricante em um objeto JSON | payload `decoded` |
| **3. Validate** | `js-executor` | Roda o script `validate` — rejeita entradas malformadas antes que cheguem ao router | payload `validated` |
| **4. Transform** | `js-executor` | Roda o script `transform` — emite o **Standard Event** + EVA dynamic fields | Standard Event `transformed` |
| **5. Route** | `router` | Avalia as regras de match sobre o Standard Event; faz fan-out para 0..N subjects downstream | subjects por tipo |
| **6. Act** | `events`, `workflow`, `triggers` | Persiste, automatiza ou notifica — cada consumer é independente | linhas no ClickHouse, execuções de Workflow, chamadas de Trigger |

> **Por que cada step é seu próprio serviço.** Decode-validate-transform é CPU-bound e se beneficia do pooling de V8. O roteamento é pesado em avaliação de regras e precisa de lookups quentes de assets. O storage é I/O-bound no ClickHouse. Os Workflows são stateful, com timers de vida longa. Separar isso significa **escalar o ponto quente, não a plataforma inteira.**

---

## O ecossistema MapexOS

O MapexOS é um **monorepo** (`mapexOS`) com os serviços de backend e o frontend em Vue 3, cercado por alguns **repositórios satélites**. A maioria dos usuários só precisa do repo de deploy; os demais são para contribuidores, operadores de broker, LoRaWAN e integradores Go.

| Repositório | Papel |
|---|---|
| **[`mapexOS`](https://github.com/Mapex-Solutions/mapexOS)** | Monorepo: os serviços de backend e o frontend em Vue 3. |
| **[`mapexOSDeploy`](https://github.com/Mapex-Solutions/mapexOSDeploy)** | Distribuição Docker Compose que puxa imagens multi-arch pré-construídas do Docker Hub. **Comece por aqui para rodar a plataforma.** |
| **[`mapexMQTTBroker`](https://github.com/Mapex-Solutions/mapexMQTTBroker)** | O MQTT broker de produção — Eclipse Mosquitto v2 mais um plugin Go interno que cuida de auth, ACL, presence e ingress em um único `.so`. |
| **[`mapexLNS`](https://github.com/Mapex-Solutions/mapexLNS)** | LoRaWAN Network Server — uma camada fina sobre o The Things Stack (papéis GS/NS/JS) que despacha os uplinks decifrados para o NATS. |
| **[`mapexGoKit`](https://github.com/Mapex-Solutions/mapexGoKit)** | SDK Go compartilhado usado por todo serviço Go — middleware HTTP, clients de NATS/Mongo/ClickHouse/MinIO/Redis, observabilidade, validação, contratos. |
| **[`mapexDevicesSimulator`](https://github.com/Mapex-Solutions/mapexDevicesSimulator)** | Simulador desktop que injeta tráfego real de HTTP/MQTT/LoRaWAN na plataforma sem hardware. |

> **O MQTT broker é seu próprio repo por um motivo.** Cada decisão de CONNECT/PUBLISH é tomada **localmente** no plugin do broker, a partir de um cache de três camadas (**Pebble → MinIO/S3 → fallback HTTP** para o serviço `assets`). Sem ida e volta no caminho crítico. É essa garantia que permite a um único broker carregar dezenas de milhares de conexões simultâneas de devices sem que o control plane vire o gargalo.

---

## Princípios de design — e os tradeoffs que aceitamos

A arquitetura existe para responder a seis perguntas. Cada linha mostra a pergunta, o princípio que escolhemos, o que **abrimos mão** para consegui-lo e onde a escolha aparece no código.

| Pergunta que o princípio responde | Escolha que fizemos | Do que abrimos mão | Onde vive |
|---|---|---|---|
| *Como os serviços permanecem desacoplados conforme a plataforma cresce?* | **NATS JetStream** como sistema de registro dos eventos; HTTP apenas para o control plane | Request/response síncrono entre serviços | Todo fluxo de eventos entre serviços usa subjects em `packages/contracts/services/*/events` |
| *Como entregamos código de cliente com segurança, sem escapes de sandbox?* | **V8 isolates** sem bindings de Node, sem filesystem, sem rede | "Simplesmente escreva qualquer módulo Node" — só ES6+ é permitido | `js-executor`, `js-workflow-executor` |
| *Como os scripts rodam rápido em escala?* | **Compilação para bytecode + um cache em camadas** (L0 RAM → L1 disk → L2 MinIO/S3 → Fallback) | Orçamento de memória — a camada de RAM é limitada por pod | Camada de cache em camadas usada por `assets`, `workflow`, `js-executor` |
| *Como as execuções de Workflow sobrevivem a uma falha?* | **Estado do DAG com checkpoint no NATS KV**, arquivado no MongoDB ao terminar | Custo de storage — cada step é persistido | `workflow/src/modules/runtime` + `archiver` |
| *Como servimos muitos tenants com segurança em uma única stack?* | **`orgId + pathKey` em todo contrato**, RBAC aplicado centralmente, templates propagam árvore abaixo | Um modelo plano — não existe "modo single-tenant" | `mapexIam` está em todo caminho de código |
| *Como os operadores adicionam novas integrações sem redeploy?* | **Manifests de Plugin** que o editor carrega em runtime a partir do plugin registry | Os operadores confiam no registry que configuram | `workflow/src/modules/plugins` |

Esses tradeoffs não são adaptações posteriores — são a razão pela qual o codebase tem a cara que tem.

---

## Os 10 serviços — em um relance

```
   ┌─────────────────────────────────────────────────────────────────────────┐
   │                            CONTROL PLANE                                │
   │   mapexIam      mapexVault      assets       (frontend: mapexOS SPA)    │
   │   (auth/orgs)   (secrets/PKI)   (templates)                             │
   └─────────────────────────────────────────────────────────────────────────┘
   ┌─────────────────────────────────────────────────────────────────────────┐
   │                              DATA PLANE                                 │
   │                                                                         │
   │   http_gateway  ──┐                                                     │
   │                   ├─→  js-executor  ──→  router  ──┬─→  events          │
   │   MQTT broker  ──┘                                  ├─→  workflow       │
   │                                                     └─→  triggers       │
   │                                                                         │
   │              js-workflow-executor  (V8 for workflow code nodes)         │
   └─────────────────────────────────────────────────────────────────────────┘
```

| Serviço | Porta | Stack | Papel | Storage que possui |
|---|---|---|---|---|
| **mapexIam** | 5000 | Go | Organizações, usuários, roles, grupos, auth JWT, cache de RBAC | `dev-mapexos` (Mongo) + Redis DB 5 |
| **http_gateway** | 5001 | Go | Ingestão de webhooks, 5 estratégias de auth, registry de datasources | `dev-http_gateway` (Mongo) |
| **assets** | 5002 | Go | Assets, templates, EVA fields, backend de auth/ACL do MQTT | `dev-assets` (Mongo) |
| **router** | 5003 | Go | Avaliação de match-rule, fan-out para destinos downstream | `dev-router` (Mongo) |
| **events** | 5004 | Go | Storage no ClickHouse, 7 consumers NATS, queries EVA, TTL | `mapexos` (ClickHouse) + `dev-events` (Mongo) |
| **triggers** | 5006 | Go | 8 executors outbound: HTTP, MQTT, RabbitMQ, NATS, WebSocket, Email, Teams, Slack | `dev-triggers` (Mongo) |
| **workflow** | 5007 | Go | Runtime de DAG durável e recuperável a falhas, 17 tipos de nó core + nós de Plugin, checkpointing em NATS KV | `dev-workflow` (Mongo) + NATS KV |
| **mapexVault** | 5010 | Go | Credenciais com envelope encryption, autoridade PKI | `dev-vault` (Mongo) |
| **js-executor** | 8000 | Node + V8 | Isolates de Decode → Validate → Transform para eventos de IoT | MinIO/S3 (cache L2) |
| **js-workflow-executor** | 8001 | Node + V8 | V8 isolates para os nós `Code` de Workflow | MinIO/S3 (cache L2) |

Cada serviço segue o mesmo layout interno — **DDD + hexagonal**, módulos em `src/modules/`, ports/adapters separando o domínio da infraestrutura. É essa uniformidade que permite a um novo contribuidor cair em qualquer serviço e se localizar em minutos.

### Edge servers — os três pontos de ingress

Além dos dez serviços core, a plataforma tem **três edges de ingress** — e dois deles vivem em **seus próprios repositórios** com sua própria tecnologia, porque o edge de protocolo tem características operacionais muito diferentes:

| Edge | Transporte | Stack | Stream de ingress |
|---|---|---|---|
| **[HTTP Gateway](./http-gateway.md)** | HTTP / HTTPS | Go (Fiber) — um serviço core | `processor.js.execute` |
| **[MQTT Broker](./mqtt-broker.md)** | MQTT `1883` / `8883` | Eclipse Mosquitto v2 + um plugin Go | `mqtt.data.>` |
| **[LNS](./lns.md)** | LoRaWAN (UDP `1700` / WS `1887`) | Camada sobre o The Things Stack (Go) | `lorawan.data.>` |

Todos os três autenticam **no edge** a partir da mesma projeção autorada pelo Assets, e convergem para o `js-executor` para a normalização — cada um em seu próprio stream de protocolo.

---

## Multi-tenancy by design

Todo contrato carrega o contexto de tenant. Não é um header HTTP que se perde no caminho — é um campo tipado em todo DTO que cruza a fronteira de um serviço.

### A árvore de organizações

```
RootOrg
  ├── Tenant-A           (pathKey: "rootOrg.tenantA")
  │     ├── Region-EU    (pathKey: "rootOrg.tenantA.regionEu")
  │     └── Region-US    (pathKey: "rootOrg.tenantA.regionUs")
  └── Tenant-B           (pathKey: "rootOrg.tenantB")
```

| Preocupação | Como funciona |
|---|---|
| **Identidade** | `orgId` (UUID imutável) + `pathKey` (string hierárquica). Todo evento, asset, template, regra, Workflow e Trigger carrega os dois. |
| **Autorização** | Roles de RBAC são *definidos por org* — não existem roles globais de "admin" ou "viewer". O `mapexIam` avalia a matriz de acesso em cada chamada. |
| **Herança** | Templates e políticas declarados em uma org pai propagam para os descendentes, a menos que sejam explicitamente sobrescritos. |
| **Isolamento de estado** | Todas as chaves de estado são escopadas: `state:{orgId}:{workflowId}:{var}`. Não há keyspace compartilhado entre tenants. |
| **Isolamento de storage** | Lógico, não físico — mesmo banco Mongo, mesmo cluster ClickHouse. O isolamento duro é obtido implantando múltiplas stacks. |
| **Retenção** | O TTL é configurável **por org × por stream** (raw, processed, router history). A janela de compliance de um tenant não impõe custo a outro. |

> **Por que lógico e não físico?** Bancos físicos por tenant foram considerados e descartados — o custo operacional de replicar Mongo, ClickHouse, NATS e Redis por tenant é justamente o que torna a maioria das plataformas "multi-tenant" inviável para tenants SMB. O isolamento lógico, com filtragem disciplinada por `orgId` imposta no pacote de contratos, dá aos operadores a *opção* de fazer sharding depois sem mudanças de código.

---

## Durabilidade e confiabilidade

O que significa, concretamente, "durável por padrão"?

| Camada | Garantia | Mecanismo |
|---|---|---|
| **Ingestão** | Assim que o `http_gateway` retorna `2xx`, o evento está persistido no JetStream | `events.raw` é um stream do JetStream com storage em arquivo |
| **Decode / validate / transform** | Entrega at-least-once ao `js-executor` | Consumer NATS JS com `Ack` explícito após transform bem-sucedido |
| **Roteamento** | Regras de match avaliadas contra o snapshot mais recente do asset, nunca um obsoleto | O `router` lê via TieredCache (L0/L1) com invalidação por FANOUT em mutações de asset |
| **Execução de Workflow** | Um worker que falhou retoma o DAG em pleno voo após o restart | Checkpoint por step no `NATS KV` (`exec.{uuid}`); os timers do NATS Schedule sobrevivem a restarts |
| **Triggers** | Chamadas outbound que falham fazem retry com backoff exponencial e caem em uma DLQ | Política de retry do NATS JS + subjects de DLQ por Trigger |
| **Storage** | Nenhum evento pousa no ClickHouse duas vezes para o mesmo `eventId` | O serviço `events` deduplica em `(orgId, eventId)` via batch insert com ReplacingMergeTree |
| **Replay** | Operadores podem reprocessar eventos históricos sem escrever código | Os consumers do JetStream suportam `DeliverByStartTime` / `DeliverByStartSeq` |

> **O que isto não é.** O MapexOS não promete *exactly-once* ponta a ponta. Ele promete **at-least-once com chaves de idempotência estáveis** (`eventId`, `workflowUUID`, `executionStepId`). Os consumers downstream precisam respeitar essas chaves para deduplicar.

---

## Escala horizontal — o que escalar, e por qual sinal

O MapexOS escala adicionando instâncias do **serviço específico** que é o gargalo. Cada serviço tem um vetor de escala claro e um gargalo claro.

| Serviço | Escale por | Sinal a observar |
|---|---|---|
| `http_gateway` | Pods stateless atrás de LB | Requisições/segundo, latência p99 |
| MQTT broker | Cluster Mosquitto + NATS leaf nodes | Conexões simultâneas, CPU do broker |
| `js-executor` | Pool de workers + pool de isolates | Scripts/segundo, saturação do pool de isolates |
| `router` | Concorrência do NATS pull consumer | Mensagens pendentes em `events.processed` |
| `workflow` | Pool de workers + contagem de partições NATS | Mensagens pendentes em `workflow.execution.router`, tamanho do NATS KV |
| `triggers` | Pool de workers por executor | Profundidade da fila de retry, latência das chamadas downstream |
| `events` | Consumers JS paralelos + batchers de insert do ClickHouse | Throughput de insert, latência do batch |
| ClickHouse | Réplicas (leitura) + shards (escrita) | I/O de disco, lag de replicação |
| NATS JetStream | Tamanho do cluster + réplicas de stream | Pressão de storage, ack pendente |
| MongoDB | Replica set; faça sharding quando o working set exceder a RAM | Razão working set vs RAM |

> **Por que não auto-escalar tudo a partir de um único sinal?** Cada estágio tem um gargalo diferente — o `http_gateway` é request-bound, o `js-executor` é CPU-bound, o `events` é disk-bound. Uma única política de auto-scaling para toda a plataforma escalaria errado a maioria dos estágios na maior parte do tempo. Expomos sinais por estágio e deixamos os operadores decidirem.

---

## Arquitetura de dados — operacional, analítico, quente, frio

```
  ┌─ OPERATIONAL ───────────────┐   ┌─ ANALYTICAL ────────────────┐
  │  MongoDB                    │   │  ClickHouse                 │
  │  configs · governance       │   │  events.raw · processed     │
  │  templates · rules          │   │  router history · trig logs │
  └─────────────────────────────┘   └─────────────────────────────┘

  ┌─ HOT STATE ─────────────────┐   ┌─ OBJECT STORAGE ────────────┐
  │  Redis                      │   │  MinIO / S3                 │
  │  auth cache · rule counters │   │  read-models · definitions  │
  ├─────────────────────────────┤   │  artifacts · L2 cache       │
  │  NATS KV                    │   └─────────────────────────────┘
  │  workflow exec state        │
  │  plugin manifest cache      │   ┌─ EVENT BACKBONE ────────────┐
  └─────────────────────────────┘   │  NATS JetStream             │
                                     │  every cross-service        │
                                     │  subject · streams · replay │
                                     └─────────────────────────────┘
```

| Storage | Papel | O que vive lá |
|---|---|---|
| **MongoDB** | Fonte operacional da verdade | Orgs, usuários, roles, assets, templates, datasources, route groups, definições de Workflow, configs de Trigger, credenciais (criptografadas) |
| **ClickHouse** | Event store analítico | Eventos brutos, eventos processados, histórico de execução do router, logs de Workflow, logs de Trigger |
| **Redis** | Estado compartilhado de baixa latência | Cache de autorização / permissões (DB 5 — compartilhado entre serviços) |
| **NATS KV** | Estado quente por execução | Estado de execução de Workflow ao vivo (`exec.{uuid}`), cache de manifests de Plugin |
| **NATS JetStream** | Backbone de eventos | Todo subject e stream de evento entre serviços |
| **MinIO / S3** | Objeto + cache L2 | Artefatos de bytecode, definições de Workflow, assets de Plugin |

> **A disciplina.** Cada serviço tem **exatamente um** banco Mongo autoritativo — `dev-assets`, `dev-router`, `dev-workflow`, etc. Leituras entre serviços passam pelo NATS ou pela API de contratos, nunca por uma collection Mongo compartilhada. É isso que mantém os serviços implantáveis de forma independente.

---

## Estratégia de caching — performance previsível no caminho crítico

Orçamentos apertados de latência no caminho crítico (lookups de asset do router, resolução de definição de Workflow, carregamento de script) são servidos por um **TieredCache** uniforme — sempre os mesmos quatro níveis: **L0 RAM → L1 disk → L2 MinIO/S3 → Fallback (reconstrói o cache)**.

```
L0  RAM                 (per-pod, ~MB, sub-ms)
 │
L1  disk (NVMe / SSD)   (per-pod, ~GB, ~1 ms)
 │
L2  MinIO / S3          (cluster-wide, unbounded, ~10 ms)
 │
Fallback                (rebuild the cache from the source of truth)
```

> **L2 é qualquer object storage compatível com S3.** A camada L2 fala a **API S3**, então funciona com **MinIO, AWS S3, DigitalOcean Spaces** ou qualquer outro serviço compatível com a API S3 — escolha aquele que o seu deploy já executa. Em um miss de L2, o **Fallback** re-busca no serviço dono (via HTTP) e **reconstrói o cache** para que a próxima leitura volte a ser quente.

Onde é usado:

- O **`js-executor`** compila e faz cache do bytecode dos scripts de decode/validate/transform por worker, para reexecução rápida.
- O **`router`** faz cache dos snapshots de asset usados na avaliação de match-rule.
- O **`workflow`** faz cache de `WorkflowDefinition`, `WorkflowInstance` e manifests de Plugin.
- O **`assets`** serve as decisões de ACL de MQTT CONNECT/PUBLISH ao plugin do broker a partir do cache — sem ida e volta ao Mongo no caminho crítico.

A invalidação é **event-driven** sobre NATS — uma atualização de definição faz fan-out para todo pod que mantém a entrada de cache. Não há chute de TTL.

---

## Extensibilidade — três planos, três contratos

Os operadores estendem o MapexOS de três formas independentes, cada uma com seu próprio contrato e seu próprio raio de impacto.

| Plano | O que você estende | Onde roda | Contrato |
|---|---|---|---|
| **Templates** | Decode / validate / transform por asset | V8 isolate do `js-executor` | JavaScript ES6+, sem bindings de Node |
| **Nós de código de Workflow** | Lógica customizada dentro de uma execução de Workflow | V8 isolate do `js-workflow-executor` | JavaScript ES6+, o mesmo contrato de isolate |
| **Plugins** | Novos tipos de nó de Workflow no editor visual (estilo n8n) | Frontend (UI) + workflow (registry de manifests) | Manifest JSON declarativo servido pelo plugin registry |

Cada plano roda em sandbox em uma fronteira diferente:

- **Templates** veem um evento de cada vez e nunca alcançam o filesystem ou a rede do host.
- **Nós de código de Workflow** veem o contexto do Workflow e saem de forma limpa nos limites de memória/timeout.
- **Plugins** carregam sua UI a partir da URL do marketplace que o operador configura — o operador decide em quem confia.

Essa divisão é a razão pela qual **novos conectores entram no editor sem um release de frontend**, e pela qual **nova lógica de integração chega como um template, não como uma mudança de código**.

---

## Baseline de segurança

| Preocupação | Como o MapexOS resolve |
|---|---|
| **Autenticação (norte-sul)** | O `http_gateway` suporta API key, JWT, IP allowlist, OAuth 2.0 e basic auth por datasource |
| **Autenticação (leste-oeste)** | Chamadas serviço-a-serviço carregam um JWT assinado pelo `mapexIam`; os consumers validam via o JWKS do IAM |
| **Autorização** | Roles de RBAC por org, avaliados centralmente pelo `mapexIam`, com cache no Redis DB 5 |
| **Isolamento de tenant** | `orgId + pathKey` imposto em todo contrato, validado em toda fronteira |
| **Armazenamento de secrets** | O `mapexVault` guarda credenciais sob envelope encryption (AES-256-GCM); o data plane vê apenas referências como `secrets.<id>.<field>` |
| **PKI** | O `mapexVault` é a certificate authority — o material de mTLS é provisionado, não colado em configs |
| **Auditoria** | Toda execução do router escreve uma linha de history; toda chamada de Trigger escreve uma linha de log; ambas consultáveis via o serviço events |
| **Sandboxing** | Todo código de cliente roda em V8 isolates com limites explícitos de tempo e memória |
| **Criptografia em trânsito** | Todos os endpoints de NATS, Mongo, ClickHouse, Redis e MinIO são TLS-capable; o HTTP gateway termina TLS no LB |

O threat model assume que o operador da plataforma é confiável, que os scripts fornecidos pelo cliente são não confiáveis e que os tenants não podem observar os dados uns dos outros. As fronteiras impõem esse modelo.

---

## Observabilidade

O MapexOS foi feito para ser **diagnosticável de fora.**

| Sinal | Onde encontrar |
|---|---|
| **Logs JSON estruturados** | stdout de todo serviço; correlação por `eventId` / `workflowUUID` |
| **Métricas Prometheus** | `/metrics` em todo serviço Go; contagens de pendentes por NATS-consumer; saturação por pool de isolates |
| **Health probes** | `/health` (liveness) e `/ready` (readiness) em todo serviço |
| **Traces distribuídos** | OpenTelemetry-ready (`traceparent` W3C propagado pelos headers do NATS) |
| **Logs de auditoria persistentes** | Histórico de execução do router e logs de Trigger são consultáveis no ClickHouse |
| **Debug por asset** | Flag opt-in `debugEnabled` por asset escreve histórico detalhado do pipeline no ClickHouse — desligada por padrão para controlar custo |

> **Por que o debug é opt-in.** Persistir cada step de cada evento de cada asset dominaria o storage do ClickHouse. Assets de produção emitem apenas erros; o debug entra em cena com uma flag acionada durante a investigação de um incidente.

---

## Topologias de deploy

O MapexOS é entregue como Docker Compose para single-node e como imagens de container para qualquer orquestrador. O formato do deploy é função do throughput, não de um SKU de produto diferente.

| Topologia | Caso de uso | O que muda |
|---|---|---|
| **Single-node** | POC, edge gateway, demo | Um arquivo Compose, todos os serviços em um host |
| **Control plane em HA** | Produção, regional | Cluster NATS (3+ nós), Mongo replica set, múltiplos pods de `mapexIam` e frontend |
| **Data plane com sharding** | Ingestão de alto throughput | Múltiplas réplicas de `http_gateway` e `js-executor`; NATS leaf nodes para o edge MQTT |
| **Edge + central** | Deploys em campo | MQTT broker + NATS leaf no edge; plataforma completa no data center |

Não existe um caminho de código de "edição Enterprise" — os mesmos binários rodam todas as topologias.

---

## Como ler o resto da documentação

Leitores diferentes se importam com partes diferentes desta página. Aqui está para onde ir em seguida com base no que você veio buscar.

| Se você está avaliando o MapexOS como um... | Leia a seguir |
|---|---|
| **Arquiteto de plataforma** | [Conceitos Core](../getting-started/concepts.md) → [Events](./events.md) → [mapexIam](./mapexiam.md) |
| **SRE / operador** | [Instalação](../getting-started/installation.md) → [Observabilidade](#observabilidade) → [Quickstart](../getting-started/quickstart.md) |
| **Revisor de segurança** | [Arquitetura de segurança](./security.md) → [mapexIam](./mapexiam.md) |
| **Desenvolvedor de integração** | [Assets](./assets.md) → [Execução JS](./js-execution.md) → [HTTP Gateway](./http-gateway.md) |
| **Autor de Workflow** | [Conceitos Core](../getting-started/concepts.md) → [Execução JS](./js-execution.md) → [Triggers](./triggers.md) |

---

## Deep-dives dos serviços

Cada capacidade tem sua própria página, agrupada do jeito que a plataforma é construída.

**Serviços Go**
- [**mapexIam**](./mapexiam.md) — organizações, usuários, roles, JWT, caches de RBAC + coverage
- [**Router**](./router.md) — regras de match, fan-out, route groups
- [**Workflow Engine**](./workflow.md) — runtime de DAG durável, condições, suspend/resume
- [**Triggers**](./triggers.md) — executors outbound, retries, DLQ
- [**Events**](./events.md) — storage no ClickHouse, queries EVA, retenção
- [**Assets**](./assets.md) — assets, templates, EVA fields, PKI de device, presence
- [**Vault**](./vault.md) — secrets com envelope encryption, KEKs, a certificate authority

**Serviços JS**
- [**Execução JS**](./js-execution.md) — V8 isolates que normalizam os payloads dos devices
- [**Execução de Workflow JS**](./js-workflow-execution.md) — V8 isolates para os nós de código de Workflow

**Edge servers**
- [**HTTP Gateway**](./http-gateway.md) — ingestão de webhooks, auth por origem, registry de datasources
- [**MQTT Broker**](./mqtt-broker.md) — Mosquitto + plugin Go: auth local, ACL, presence, ingress
- [**LNS**](./lns.md) — LoRaWAN network server, uma camada sobre o The Things Stack

**Cross-cutting**
- [**Segurança**](./security.md) — identidade, isolamento, secrets e o threat model
