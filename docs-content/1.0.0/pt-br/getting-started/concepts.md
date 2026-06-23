---
title: Conceitos Fundamentais
description: O vocabulário do MapexOS — os blocos de construção que todo operador, integrador e arquiteto precisa conhecer antes de se aprofundar.
---

# Conceitos Fundamentais

Esta página define os blocos de construção do MapexOS — as palavras usadas em todo o resto da documentação. Leia uma vez antes de seguir adiante e volte sempre que um termo não estiver claro.

> **O que já é realidade hoje.** O MapexOS ingere dados por **HTTP, MQTT e LoRaWAN**, executa o pipeline de transformar → rotear → automatizar → armazenar e entrega toda a pilha de governança multi-tenant. Protocolos adicionais (como o CoAP) e as camadas de catálogo/IA estão no roadmap.

---

## As cinco camadas do MapexOS

Tudo no MapexOS pertence a uma das cinco camadas. O restante da página detalha cada uma delas.

| Camada | O que é | Conceitos-chave |
|---|---|---|
| **Tenancy e identidade** | Quem pode fazer o quê, restrito a qual parte da organização | Organization · User · Group · Role |
| **Fontes de dados** | O que produz eventos | Asset · Asset Template · Dynamic Fields (EVA) |
| **Pipeline de eventos** | O caminho que todo evento percorre | Standard Event · Router · Route Group |
| **Automação** | O que acontece depois que um evento chega | Workflow · Node · Trigger · Plugin |
| **Armazenamento e segredos** | Onde vivem o estado e as credenciais | Events Service · Persistent Log · Credential · Vault |

---

## Tenancy e identidade

### Organization

Uma **organization** é uma unidade de propriedade dentro do MapexOS. Todo evento, asset, template, workflow e regra pertence a exatamente uma organization. Permissões, retenção e visibilidade seguem essa fronteira.

As organizations formam uma **árvore hierárquica**, e cada nó carrega um `pathKey` desnormalizado — um prefixo que codifica sua posição na árvore, de modo que a plataforma consiga resolver "tudo abaixo desta org" com uma consulta de prefixo rápida, em vez de uma varredura recursiva. Os rótulos de nível são convenções; a plataforma não se importa com o nome que você dá a eles:

```
Vendor → Customer → Site → Building → Floor → Zone
```

Em todos os casos as regras são as mesmas:

- Cada nível herda o contexto do seu pai.
- Templates, regras e assets definidos em uma organization pai podem ser **compartilhados árvore abaixo** com os descendentes.
- Retenção (TTL), permissões e fronteiras de auditoria são aplicadas **na organization dona do dado**.

### User, Group, Role

- Um **user** é uma identidade individual que se autentica e opera a plataforma. Um user pode pertencer a várias organizations, com papéis diferentes em cada uma.
- Um **group** é uma coleção nomeada de users; as permissões concedidas a um group valem para todos os membros.
- Um **role** é um conjunto reutilizável de permissões, **definido dentro de uma organization** — o role *Operator* no *Customer A* e o role *Operator* no *Customer B* são definições separadas. É isso que impede que tenants vazem permissões uns para os outros.

A concessão de acesso completa é uma tripla — **assignee + role + organization** — e a herança de roles árvore abaixo é controlada por organization através de uma `rolePolicy` (os roles de um pai só alcançam um descendente quando esse descendente opta por recebê-los):

| Assignee (user ou group) | Role | Organization (escopo) | Acesso efetivo |
|---|---|---|---|
| João | Operator | *Site: Planta São Paulo* | Operar assets na Planta São Paulo |
| *Grupo de Manutenção* | Technician | *Building: Armazém Principal* | Cada membro mantém os assets daquele building |

---

## Fontes de dados

### Asset

Um **Asset** é qualquer coisa que produz eventos no MapexOS. O nome é deliberadamente mais amplo do que "dispositivo":

- Um sensor IoT, um gateway ou uma máquina reportando seu estado.
- Uma plataforma de terceiros enviando webhooks, ou uma aplicação customizada emitindo eventos de negócio.
- Uma área ou zona lógica que agrega outros assets.

Todo asset pertence a uma organization, usa um **Asset Template** e carrega sua identidade, sua localização opcional na árvore da org, metadados customizados e credenciais de conectividade.

### Asset Template

Um **Asset Template** define como uma *classe* de asset é integrada — de modo que onboardar um novo vendor ou modelo é **configuração**, não um novo ramo de código. Um único template pode atender centenas ou milhares de assets em muitas organizations.

Cada template carrega três scripts que rodam **em sequência** para cada payload recebido — executados em um isolate V8 isolado em sandbox — além de um script de teste opcional usado durante a autoria:

| Script | Propósito |
|---|---|
| **Preprocessor** | Decodificar / normalizar o formato de transporte (parsear binário, Base64, descomprimir) — opcional |
| **Validation** | Rejeitar entradas malformadas (campos obrigatórios, faixas, schema) |
| **Conversion** | Produzir o Standard Event — mapear o payload do vendor para o contrato do MapexOS |

```
Raw payload → Preprocessor → Validation → Conversion → Standard Event
```

Os templates podem ser do tipo **system** (que acompanham o MapexOS), **organization** (privados) ou **shared** (criados por uma org e compartilhados árvore abaixo).

### Dynamic Fields (EVA)

Um template pode declarar **Dynamic Fields** — valores tipados extraídos do payload. Cada campo tem um **name** lógico, um **type**, um **payload path** e um **`fieldId`** numérico estável e imutável (um template pode conter até 200):

```json
{ "sensor": { "readings": { "temp": 23.5, "humidity": 65 } } }
```

| fieldId | Field name | Type | Payload path |
|---|---|---|---|
| 1 | `temperature` | number | `sensor.readings.temp` |
| 2 | `humidity` | number | `sensor.readings.humidity` |

O `fieldId` é a ideia central: os eventos são armazenados em **colunas tipadas indexadas por esse id**, então um valor mantém seu significado para sempre, mesmo que o rótulo mude. É isso que torna eventos de vendors diferentes **consultáveis como um único conjunto** — você pede `temperature` em toda a frota, independentemente de como cada payload bruto nomeou esse campo.

---

## Pipeline de eventos

### Standard Event

Todo payload que entra no MapexOS — por HTTP, MQTT ou LoRaWAN — é convertido pelo seu template em um único **Standard Event**. Esse é o contrato exato, validado por schema, sobre o qual o resto da plataforma opera:

```ts
{
  eventType: string,        // classification, e.g. "telemetry.temperature"
  eventId: string,          // unique id for this event
  data: Record<string, any>,// the normalized payload
  metadata?: Record<string, any>, // optional context (org, asset, location, correlation ids)
  created: string           // ISO 8601 timestamp
}
```

Roteamento, automação, armazenamento e auditoria operam todos sobre Standard Events — nunca sobre payloads brutos de vendors.

### Router e Route Group

O **Router** recebe Standard Events e despacha cada um para um ou mais destinos, **em paralelo**, com base em regras configuradas. Um mesmo evento pode ser persistido, enviado a um workflow e encaminhado a um trigger no mesmo passo.

Um **Route Group** é um conjunto nomeado de regras de roteamento anexado à configuração de um asset. Cada regra é uma **condição de match** somada a um **tipo de destino**:

- Uma **condição de match** é uma lista de cláusulas — um campo em dot-path (ex.: `data.temperature`), um de oito operadores (`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `nin`) e um valor — combinadas com uma política de `all` (AND) ou `any` (OR).
- Um **tipo de destino** roteia o evento correspondente para um destino como o **events store**, um **workflow**, um **trigger** ou um sink de **lake-house**. Toda decisão de roteamento também gera um registro de **routing-history**.

```
Always                     → events store   (persist)
If data.temperature ≥ 80   → workflow        (handle escalation)
If data.alert in ["high"]  → trigger         (notify operations)
```

---

## Automação

É aqui que a plataforma transforma telemetria em ação de negócio.

### Workflow

Um **workflow** é um grafo acíclico dirigido (DAG) de **nodes** que roda quando disparado por um evento ou por uma chamada de API. Os workflows são **duráveis**: cada passo tem checkpoint em um key-value store do NATS, então um workflow que está no meio do caminho quando um worker cai **retoma a partir do último node**, não do começo. Timers, retries e waits também sobrevivem a reinícios. A lógica com estado — condições, contadores, cooldowns, janelas de persistência — vive inteiramente dentro do workflow (nos nodes `condition` e `set_state`); não há um serviço de regras separado.

Um workflow tem três entidades, em ordem crescente de concretude: uma **Definition** (o blueprint do DAG), uma **Instance** (uma definition vinculada a entradas específicas) e uma **Execution** (uma execução real, com seu próprio estado, status e histórico) — `Definition 1→N Instance 1→N Execution`.

O **estado do workflow** vem em dois sabores, e a distinção importa:

| Estado | Escopo | Uso |
|---|---|---|
| **State** (variáveis) | **Persistente** por toda a execução | Contadores, valores acumulados, flags — mutados por `set_state` (set / increment / append / remove) |
| **NodeStates** | **Efêmero**, por node | Índice de loop, tentativa de retry, info de wait — limpos quando o node conclui |

### Node

Um **node** é uma unidade de trabalho em um workflow. O MapexOS entrega **17 tipos de node nativos** e aceita novos como plugins:

| Grupo | Nodes |
|---|---|
| **Entrada e saída** | `trigger_event`, `start`, `end` |
| **Lógica e dados** | `condition`, `switch`, `set_state`, `log`, `code` (JavaScript do usuário em um isolate V8) |
| **Controle de fluxo** | `fanout`, `merge`, `sequence`, `loop`, `goto`, `subworkflow` |
| **Tempo e sinais** | `delay`, `wait_signal`, `wait_for` |

Dois deles são recursos de assinatura:

- **Condições** (`condition`, `switch`, `wait_for`) avaliam **grupos aninháveis de AND / OR / NAND / NOR** com 18 operadores (comparação, string, datetime, faixa) — o suficiente para expressar lógica operacional real, não apenas um único limiar.
- O **Goto** permite referenciar um passo pelo nome através de **"portais" emparelhados de sender/receiver**, em vez de arrastar um fio pelo canvas, mantendo um DAG grande legível.

### Trigger

Duas coisas no MapexOS são chamadas de "trigger" — e elas não são a mesma:

| Termo | O que significa |
|---|---|
| **Workflow trigger** (node `trigger_event`) | O node de entrada de um workflow — define o que o dispara |
| **Trigger** (executor) | Uma **ação de saída** independente, disparada pelo router ou de dentro de um workflow |

Um trigger executor entrega uma ação por um de **oito transportes nativos**:

| Categoria | Executors |
|---|---|
| **Técnicos** | HTTP, MQTT, RabbitMQ, NATS, WebSocket |
| **Comunicação** | Email, Microsoft Teams, Slack |

### Plugin

Um **plugin** adiciona novos tipos de node de workflow ou de trigger **sem rebuildar o frontend**. Um plugin é um **manifest** declarativo (credenciais, busca dinâmica de opções, tipos de node e seus campos, templates de operação); o editor de workflow o carrega em runtime e renderiza o formulário automaticamente. O modelo é inspirado no ecossistema de nodes do n8n, adaptado às restrições de durabilidade de workflow e multi-tenancy do MapexOS — de modo que um novo conector (um ITSM, um payment gateway, um banco de dados) é um manifest, não um release de plataforma.

---

## Armazenamento e segredos

### Events Service e Persistent Log

O **Events Service** é o sink terminal: ele consome Standard Events do backbone NATS e os armazena no **ClickHouse**, nas colunas EVA tipadas descritas acima, com **retenção (TTL) configurável por organization**. Como toda avaliação e ação também é registrada, o resultado é um **persistent log** — uma trilha imutável e consultável do *que aconteceu*, pesquisável por asset, por site, por janela de tempo, por resultado. A evidência de compliance vira uma consulta, não uma montagem manual.

### Credential e Vault

Uma **credential** é uma referência a um segredo — um token de API, um par usuário/senha, uma chave privada, um certificado. O **Vault** armazena credenciais com **criptografia de envelope** (uma chave por registro envolvida por uma master key, AES-256-GCM) e nunca serializa o texto plano em uma resposta de API. Ele também emite material de chave da plataforma e assina **certificados X.509 por asset** para a conectividade dos dispositivos. O data plane (workflows, triggers, templates) só enxerga referências como `{{credential.<id>.<field>}}`, resolvidas em runtime.

---

## Como os conceitos se conectam

```text
Organization
  ├── Users / Groups / Roles                (who can do what, where — scoped by pathKey)
  └── Assets
        └── use → Asset Template
                  ├── scripts → Preprocessor → Validation → Conversion
                  └── defines → Dynamic Fields (EVA, typed, fieldId-keyed)

Event flow
  Raw payload
    → Asset Template pipeline
    → Standard Event { eventType, eventId, data, metadata, created }
    → Router  (fan-out by Route Group rules: dot-path + operators, all/any)
        → Events Service     (persist in ClickHouse + EVA query + per-org TTL)
        → Workflow           (durable DAG of nodes, checkpointed)
              ├─ condition / switch   (AND/OR/NAND/NOR groups, 18 operators)
              ├─ set_state            (persistent State) · NodeStates (ephemeral)
              ├─ code                 (user JS in a V8 isolate)
              ├─ goto                 (sender/receiver portals)
              └─ trigger_event / plugin nodes  (act)
        → Trigger            (direct outbound action — 8 executors)

Secrets
  Workflow / Trigger / Template
    → {{credential.<id>.<field>}}
        → Vault (envelope-encrypted; device X.509 PKI; never plaintext on the data plane)
```

---

## Resumo

| Conceito | Propósito |
|---|---|
| **Organization** | Unidade multi-tenant hierárquica (com `pathKey`); a fronteira de permissões, retenção e visibilidade |
| **User / Group / Role** | Identidade e controle de acesso, com escopo por organization |
| **Asset** | Uma entidade que produz dados (sensor, máquina, aplicação, área) |
| **Asset Template** | Integração reutilizável: Preprocessor → Validation → Conversion + Dynamic Fields |
| **Dynamic Fields (EVA)** | Extrações tipadas e indexadas por `fieldId` — tornam eventos heterogêneos consultáveis como um único conjunto |
| **Standard Event** | O contrato normalizado `{eventType, eventId, data, metadata, created}` sobre o qual todo serviço opera |
| **Router / Route Group** | Motor de fan-out; regras fazem match em campos dot-path (8 operadores, all/any) para tipos de destino |
| **Workflow** | DAG durável de nodes que automatiza a resposta a um evento |
| **State / NodeStates** | Variáveis persistentes da execução vs. dados efêmeros por node |
| **Node** | Uma unidade de trabalho — 17 tipos nativos (condition, switch, set_state, code, goto, fanout, loop, …) mais plugins |
| **Trigger** | O node de entrada de um workflow, ou uma ação de saída independente (8 executors) |
| **Plugin** | Extensão servida por manifest que adiciona nodes/triggers sem um release de frontend |
| **Events Service / Persistent Log** | Armazenamento em ClickHouse com retenção por org e uma trilha de auditoria imutável |
| **Credential / Vault** | Referências de segredo resolvidas por um vault de criptografia de envelope que também roda a PKI dos dispositivos |

---

## Para onde ir em seguida

| Para se aprofundar em… | Vá para |
|---|---|
| O pipeline e o store de eventos | [Architecture → Events](../architecture/events.md) |
| Identidade e multi-tenancy | [Architecture → mapexIam](../architecture/mapexiam.md) |
| Assets, templates e backend de dispositivos | [Architecture → Assets](../architecture/assets.md) |
| Como a plataforma é construída | [Architecture Overview](../architecture/overview.md) |
