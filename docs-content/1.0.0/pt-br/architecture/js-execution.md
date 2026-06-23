---
title: JS Execution
description: O motor de normalização do MapexOS — roda os scripts de template de cada asset dentro de isolates V8 endurecidos para transformar o payload bruto de qualquer vendor em um evento padrão e tipado, com segurança e em escala.
---

# JS Execution

JS Execution é o **motor de normalização** do MapexOS. É onde um payload bruto de dispositivo — seja qual for o formato que um vendor inventou — vira o único **Standard Event** da plataforma. Todo [Asset Template](./assets.md) carrega o JavaScript que sabe ler seus dispositivos; este serviço roda esse código, por evento, dentro de um **sandbox V8 endurecido**, e entrega o resultado normalizado ao [Router](./router.md). É o estágio de *transformação*: entre a ingestão — **qualquer fonte externa que entra na plataforma** ([HTTP](./http-gateway.md), [MQTT](./mqtt-broker.md), [LoRaWAN](./lns.md), e o que mais vier) — a montante e o roteamento a jusante. O motor é **agnóstico de fonte**: cada evento é etiquetado com seu `sourceType` (`http` · `mqtt` · `lorawan`), mas o **mesmo** pipeline de decode→validation→transform roda independentemente disso — então fazer o onboarding de um novo protocolo de ingestão não exige mudança alguma aqui.

Seu problema mais difícil é que o código que ele roda é **escrito por clientes**, não pela plataforma — então todo o design gira em torno de rodar código não confiável **com segurança, de forma determinística e rápido**.

> **Port** `8000` · **Node.js + TypeScript** (DDD + Hexagonal) · isolates V8 com **`isolated-vm`** sobre um worker pool **Piscina** · lê os scripts de asset/template de um tiered cache (MinIO como source of truth) · sem banco de dados próprio.

---

## O pipeline — decode → validation → transform

Cada evento roda um pipeline fixo de três fases, montado a partir dos scripts do template:

| Fase | Script do template | Papel |
|---|---|---|
| **decode** | `scriptProcessor` *(opcional)* | Faz o parse do payload bruto — desempacota um frame hex, separa um CSV, decodifica base64. |
| **validation** | `scriptValidator` *(opcional)* | Confere os dados decodificados contra o esperado antes que avancem. |
| **transform** | `scriptConversion` *(obrigatório)* | Emite o **Standard Event** — o formato normalizado e tipado que o resto do MapexOS fala. |

A saída JSON de cada fase vira o `payload` de entrada da fase seguinte; apenas a saída final de **transform** é o resultado padronizado. Como a lógica por dispositivo mora num template, o mesmo motor normaliza um sensor de solo LoRaWAN e uma catraca HTTP **sem nenhum branch por dispositivo na plataforma** — fazer o onboarding de um novo vendor é escrever três scripts pequenos, não mudar este serviço.

---

## Um sandbox de verdade — o código do cliente não consegue escapar

Todo script roda dentro de um **isolate V8 `isolated-vm`** numa thread de worker do Piscina, com limites rígidos:

- **teto de heap de 32 MB** por isolate,
- **timeout de execução de 10 segundos** por script,
- **sem acesso ao Node, à rede ou ao filesystem** — o worker importa apenas a primitiva de isolamento e `worker_threads`; nada da aplicação atravessa para dentro do sandbox.

Dentro desses limites o código roda em **V8 completo** — o **ECMAScript (ES2015+)** moderno está disponível nativamente, sem transpilação: classes, arrow functions, template literals, destructuring, `Map`/`Set` e os built-ins padrão. O único teto é o orçamento de memória de **32 MB** do isolate — um script que aloca além disso é parado, não truncado em silêncio.

Cada evento recebe um **contexto V8 fresco** (seu `payload` injetado por cópia e liberado em seguida), de modo que um evento nunca pode ver o estado de outro. O isolate em si é **reciclado a cada 10.000 eventos** para limitar a memória ao longo do tempo. Scripts compilados ficam em cache por worker e por template, então o caminho crítico compila o código de cada template uma vez, não uma vez por evento.

---

## Resiliente a código ruim

Código não confiável se comporta mal, e o motor trata isso como normal:

- Um script que **esgota o heap** descarta seu isolate; o worker reporta a condição de out-of-memory, **reconstrói um isolate fresco**, e o evento é **NACK'd para retry** — uma falha transitória, não um evento perdido.
- Um script com um **erro de parse ou de schema** é uma falha *permanente*: a mensagem é rejeitada (para a dead-letter queue) em vez de retentada, porque retentar não resolve.
- Uma falha registra em qual fase falhou (`decode` / `validation` / `transform`) e emite um log de debug para o serviço de [Events](./events.md), de modo que um script quebrado seja diagnosticável, não silencioso.

Scripts de validação ganham um helper injetado, **`$mv`** (MapexValidator), com checagens tipadas (`string` / `number` / `boolean` / `date` / `array` / `object`) e um ponto de entrada `validate(value, schema)` — para que os autores validem de forma declarativa em vez de escrever guards na mão.

---

## Construído para throughput

O motor puxa eventos em **lotes** e é ajustado por um único botão. O `CPU_LIMIT` deriva a quantidade de workers, o tamanho do lote e os limites de in-flight; o processo se recusa a subir se o `CPU_LIMIT` exceder as CPUs realmente disponíveis, então ele nunca consegue sobrecarregar seu host. Dentro de um lote ele **deduplica os lookups de asset**, roda cada evento pelo worker pool em paralelo e **faz o flush de todas as publicações a jusante de uma vez** antes de dar o ack — transformando uma rajada de eventos numa única rodada de escrita, em vez de uma por evento.

---

## Alimentado pela source of truth do Assets

O JS Execution não tem banco de dados próprio. Ele resolve o asset de cada evento e seus scripts de template a partir de um **tiered cache** (RAM → disk → MinIO), cuja source of truth é o read-model do [Assets](./assets.md). Dois consumidores **FANOUT** (`asset.invalidate`, `template.invalidate`) descartam entradas obsoletas no instante em que um asset ou template muda a montante — então uma edição de script entra em efeito sem restart, e nenhum worker jamais roda contra um template obsoleto.

---

## Entradas & saídas

**Consome (NATS JetStream, em lotes):**

| Subject | Fonte |
|---|---|
| `processor.js.execute` | Eventos de datasource do HTTP-gateway. |
| `mqtt.data.>` | Telemetria MQTT do [MQTT broker](./mqtt-broker.md). |
| `lorawan.data.>` | Uplinks LoRaWAN do [LNS](./lns.md). |
| `fanout.asset.invalidate` · `fanout.template.invalidate` | Coerência de cache. |

Cada borda de ingresso publica no **seu próprio stream de protocolo**, e toda mensagem carrega seu `orgId` / `assetUUID` e uma tag `sourceType` — então o motor resolve o asset e roda o pipeline idêntico não importa como o dado chegou.

**Produz (NATS, fire-and-forget, flushed uma vez por lote):**

| Subject | Propósito |
|---|---|
| `route.execute` | O evento padronizado (`eventSource: "assetEvent"`) → o [Router](./router.md). |
| `events.logs.jsexecutor` · `events.raw` | Logs de execução / debug → o serviço de [Events](./events.md). |
| `asset.heartbeat.{orgId}` | Um sinal implícito de liveness (quando o asset usa o modo de heartbeat implícito) → [Assets](./assets.md). |

**HTTP** (port `8000`): endpoints de **test** de script e de **sample-payload** usados pelo editor de template e pelo test runner, mais `/health` e `/metrics`.

---

## Para onde ir em seguida

| Para se aprofundar em… | Vá para |
|---|---|
| Onde os scripts e campos são definidos | [Assets](./assets.md) |
| O que admite os eventos que ele consome | [HTTP Gateway](./http-gateway.md) |
| Para onde sua saída é roteada | [Router](./router.md) |
