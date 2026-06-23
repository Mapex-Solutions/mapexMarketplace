---
title: Por que MapexOS?
description: O padrão que a gente via se repetir — e a decisão de construir uma plataforma
  governada única em vez de remontar, mais uma vez, o mesmo encanamento multi-tenant a partir
  de ferramentas separadas.
---

# Por que MapexOS?

> **IoT em primeiro lugar, mas não só IoT.**
> O MapexOS não enxerga dispositivos ou sensores — ele enxerga **Assets**.
> Qualquer fonte. Qualquer protocolo. Uma única abstração.

Esta página é o *porquê* — o raciocínio por trás da plataforma. Para o *o quê*, comece por
[O que é o MapexOS?](./what-is-mapexos.md). Se você está decidindo se o MapexOS serve para o tipo de
operação que você toca, continue lendo.

---

## O padrão que a gente via se repetir

Depois de anos construindo soluções de IoT, backends distribuídos e plataformas operacionais, um padrão
não parava de se repetir: sempre que o objetivo era uma operação **completa**, o ecossistema open source
tinha uma ferramenta excelente para cada peça isolada — e cada uma resolvia só uma **peça**.

Uma ferramenta é forte em conectividade. Outra em dashboards. Outra em fluxos de automação. Outra em
identidade. Outra em armazenamento. Todas realmente boas no seu pedaço.

Mas uma operação enterprise de verdade precisa delas *juntas e governadas* — integradas, multi-organização,
com permissões, templates, assets, logs persistentes, validação de payload, regras de negócio,
observabilidade, APIs, um broker, e a capacidade de escalar. Nenhuma ferramenta sozinha cobre isso, então
a resposta era sempre a mesma: **juntar as peças.**

---

## Juntar funciona — até um certo ponto

Para uma operação pequena, compor ferramentas best-of-breed funciona. Você liga um serviço de ingestão a
um motor de fluxos, coloca um servidor de identidade na frente, guarda as regras num banco, adiciona uma
ferramenta de automação, e entrega.

Por um tempo isso te leva a algum lugar. Mas conforme a operação cresce — mais tenants, mais sites, mais
dispositivos, mais parceiros de integração — as **costuras entre as ferramentas** viram o trabalho.

| Uma vitória no mês 1… | …um imposto de manutenção no mês 12. |
|---|---|
| "O motor de fluxos liga o broker a tudo." | Toda integração agora vive num fluxo que ninguém quer encostar. |
| "O servidor de identidade nos dá auth de graça." | Organizações, sites e zonas vivem em grupos de auth que não mapeiam para mais nada. |
| "A ferramenta de automação cuida da nossa lógica." | A lógica de negócio agora está dividida entre a ferramenta de automação, o banco de regras e um script que ninguém lembra que roda. |
| "Multi-tenancy a gente adiciona depois." | "Depois" quer dizer reescrever o modelo de identidade debaixo de carga. |

Nenhuma dessas ferramentas é o problema — cada uma é excelente no que faz. O custo está no **espaço entre
elas**, e ele acumula:

- **Dependência de vários runtimes** para o comportamento central da plataforma.
- **Integrações difíceis de manter**, porque os contratos entre as ferramentas nunca foram explícitos.
- **Regras de negócio espalhadas** por três ou quatro runtimes.
- **Hierarquias organizacionais que não encaixam** no formato que os clientes reais têm
  (vendor → customer → site → building → zone).
- **Modelos de permissão que não compõem** ao longo da cadeia de ferramentas.
- **Nenhum reaproveitamento** entre clientes — cada cliente novo é um projeto novo.
- **Uma arquitetura que foi montada, nunca projetada de ponta a ponta.**

É esse espaço-entre-as-ferramentas que o MapexOS elimina.

---

## A decisão

Tem um momento — depois da terceira ou quarta vez que você reconstrói a mesma plataforma multi-tenant,
multi-site, orientada a template e com log de auditoria sobre uma pilha de cola diferente — em que você
para e pergunta:

> *"Por que eu estou reconstruindo o mesmo encanamento toda vez?"*

Essa pergunta é o motivo de o MapexOS existir.

O MapexOS não começou para ser "mais um projeto de IoT open source". Ele começou para ser uma **plataforma
única, aberta e de nível enterprise** que entrega o ciclo completo de uma operação conectada — ingestão,
validação, processamento, regras de negócio, organização de assets, observabilidade, integração — pronto
de fábrica, para que a próxima frota de verdade não comece do zero.

---

## Quatro compromissos que assumimos cedo

Toda decisão de design remete a um destes.

### 1. Multi-tenancy é a camada zero, não um remendo

Clientes reais tocam operações multi-tenant: um vendor atende vários customers, um customer tem vários
sites, um site tem buildings, floors, zones. Permissões, retenção, visibilidade e templates têm que seguir
essa árvore.

No MapexOS, cada evento, Asset, template, regra, workflow e trigger carrega a sua identidade
organizacional (`orgId` + um `pathKey` hierárquico) desde o instante em que entra. Não existe um modo
single-tenant adaptado depois — a árvore de organizações é a primeira coisa que a plataforma conhece.

### 2. Os workflows têm que sobreviver a reinícios de processo

Quando uma ordem de manutenção está pela metade e um servidor reinicia, ela tem que terminar — não sumir.
Quando um timer de escalação está no meio do caminho durante um deploy, ele tem que continuar contando.

O MapexOS traz um **workflow engine durável embutido na plataforma**. Cada passo de cada workflow tem
checkpoint; um worker que cai retoma no meio do grafo. A durabilidade é parte do engine, não um sistema
separado pendurado.

### 3. A plataforma tem que estender sem fazer redeploy

Operadores adicionam integrações o tempo todo — notificações, gateways de pagamento, ITSM, bancos. Uma
integração nova não pode significar um release de frontend.

O MapexOS usa um **modelo de plugin**: os novos tipos de nó de workflow são descritos por manifests, e o
editor os carrega em tempo de execução — então um conector pode aparecer no editor sem recompilar a
plataforma.

### 4. Roteamento é configuração, não código

Para onde um evento vai — storage, workflow, trigger, notificação — é definido por política, não por um
deploy. As regras de match vivem num banco e são aplicadas em tempo de execução: mude um limite,
redirecione os eventos de um tenant para um destino novo, adicione um sink — sem migração de schema, sem
rebuild.

---

## O que muda para a sua operação

Esses compromissos viram resultados que o negócio enxerga, não só o time de engenharia.

| O que costumava ser… | …vira… |
|---|---|
| Um cliente novo exige uma instalação nova. | Um cliente novo é um nó novo na árvore de organizações. |
| Um fabricante novo de dispositivo leva um trimestre para integrar. | Um fabricante novo é um único Asset Template novo. |
| Todo alerta é uma duplicata, um falso positivo, ou os dois. | Os alertas respeitam cooldowns, janelas de persistência e ciclos de vida de incidente por design. |
| Evidência de compliance exige montagem manual de log. | Evidência de compliance é uma consulta ao event store. |
| Um conector novo exige um release de frontend. | Um conector novo é um manifest de plugin. |
| A lógica vive em runtimes que ninguém ousa mexer. | A lógica vive em workflows com definições versionadas, execução reproduzível e uma única trilha de auditoria. |

A troca: mais disciplina na camada de plataforma, e muito menos acoplamento na camada de aplicação.

---

## Por que isso importa agora: IA precisa de uma fundação

A maioria das organizações fala em "aplicar IA aos nossos dados". O que elas costumam ter é **telemetria
fragmentada em sistemas que não conversam entre si.**

IA sobre dado fragmentado é, na melhor das hipóteses, barulhenta. Antes que qualquer agente de IA possa ser
confiado para apontar uma anomalia, recomendar uma intervenção ou tomar uma ação autônoma, os eventos que o
alimentam precisam ser ingeridos de todas as fontes, **normalizados** em um único contrato, **roteados**
para o lugar certo, **governados** com fronteiras de organização explícitas, **armazenados** com retenção
defensável, e **pesquisáveis** entre payloads heterogêneos.

Essa camada — eventos limpos, governados e consultáveis — é a fundação de que a IA precisa.
**O MapexOS está sendo construído para ser essa fundação.**

---

## Para onde o roadmap aponta *(planejado)*

Hoje o MapexOS ingere por HTTP, MQTT e LoRaWAN, roda workflows duráveis e o engine de roteamento, e entrega
a stack completa de governança multi-tenant. A direção daqui em diante:

| Direção *(planejado)* | O que isso te daria |
|---|---|
| **Mais protocolos** (ex.: CoAP) | Alcançar mais dispositivos de baixo consumo e de rede restrita sem novos gateways. |
| **Um catálogo consultável sobre datasets governados** | Tornar os eventos descobríveis entre tenants e janelas de tempo. |
| **Formato de tabela aberto para o event store** | Datasets de vida longa que sistemas de analytics e IA leem direto, sem um export customizado. |
| **Superfícies de orquestração de IA** | Nós de workflow que conduzem agentes de IA em cima de eventos limpos e governados. |

A tese é proposital: **a IA deve operar sobre eventos limpos, governados e contextualizados — não sobre
dado bruto e fragmentado.**

---

## Uma plataforma, não uma linha de produtos

O MapexOS é um único ecossistema aberto, não um conjunto de tiers pagos.

- **Um único repositório de código** para os serviços de backend e o frontend (`mapexOS`).
- **Uma única distribuição de deploy** para self-hosting (`mapexOSDeploy`).
- **Uma única licença** — Business Source License 1.1 — cobrindo uso self-hosted, modificação e contribuição.
- **Nenhum caminho de código separado de "edição enterprise".** As mesmas imagens rodam um Raspberry Pi num
  site remoto e um cluster regional atrás de um load balancer.

Na **Change Date**, a BSL converte para **Apache 2.0**. A única restrição hoje é sobre *hospedar*
comercialmente o MapexOS para terceiros — não sobre rodá-lo para a sua própria operação.

---

## Quando o MapexOS é uma ótima escolha

O MapexOS é a plataforma certa se vários destes descrevem você:

- Você opera **múltiplos clientes, sites ou tenants** e precisa de isolamento real entre eles.
- Você **integra dispositivos, fabricantes ou sistemas de terceiros com frequência**, e esse custo hoje é
  alto demais.
- Você precisa de **automação que sobrevive a quedas** — workflows que retomam depois de crashes, retries
  que respeitam os limites downstream, uma trilha de auditoria sobre cada ação.
- Você precisa de **uma plataforma só para telemetria IoT e integração com sistemas de negócio**, não duas
  stacks.
- Você precisa **self-hostar** numa infraestrutura que você controla — por compliance, custo, latência ou
  soberania de dados.
- Você está construindo a **fundação de eventos sobre a qual as futuras capacidades de IA vão rodar**, e
  quer ela aberta e inspecionável.

Se só um ou dois se aplicam, o MapexOS provavelmente é exagero — comece com a ferramenta mínima que resolve
o problema imediato. Se três ou mais se aplicam, o custo de *não* ter o MapexOS provavelmente já está
aparecendo na sua operação.

---

## Para onde ir agora

| Se você quer… | Vá para |
|---|---|
| Entender como ele é construído | [Visão Geral da Arquitetura](../architecture/overview.md) |
| Aprender o vocabulário | [Conceitos Centrais](../getting-started/concepts.md) |
| Rodar na sua máquina | [Quickstart](../getting-started/quickstart.md) |
