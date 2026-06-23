---
title: O que é o MapexOS?
description: A camada operacional self-hosted da sua operação conectada — transforme cada
  sinal de dispositivos, gateways, APIs e sistemas de negócio em um único fluxo governado de
  eventos que você ingere, automatiza, audita e escala, na infraestrutura que você controla.
---

# O que é o MapexOS?

> **A camada operacional da sua operação conectada.**
> IoT em primeiro lugar, mas não só IoT — o MapexOS não enxerga dispositivos, ele enxerga **Assets**:
> qualquer fonte, qualquer protocolo, um único modelo governado.
>
> **Conecte. Automatize. Escale.**

---

## A resposta curta

**O MapexOS é uma plataforma self-hosted e multi-tenant que transforma cada sinal que sua operação
produz — sensores, gateways, APIs, sistemas de negócio — em um único fluxo governado de eventos que
você ingere, normaliza, roteia, automatiza e audita, de ponta a ponta.** Uma plataforma só, rodando
na infraestrutura que você controla, no lugar de uma pilha de ferramentas costuradas umas nas outras.

Ela foi feita para organizações que já passaram do ponto de usar ferramentas avulsas e precisam de um
sistema que dá para *operar* de verdade: com identidade, permissões, multi-tenancy, durabilidade e
trilha de auditoria como partes nativas da plataforma — não como remendos pendurados em volta.

---

## O problema que ela elimina

Uma operação conectada séria normalmente acaba virando umas sete coisas amarradas entre si: uma
ferramenta para ingerir, outra para identidade, um banco para regras, uma fila para eventos, algo para
automação, um dashboard, e uma pilha de código de cola segurando tudo de pé. Cada uma é forte no seu
pedaço — e cada uma é um sistema para operar, proteger, integrar e manter em sincronia.

Funciona até a hora em que não funciona mais: integrações apodrecem, as regras de negócio acabam
morando em três lugares diferentes, a estrutura multi-organização é modelada projeto a projeto, e
ninguém consegue responder a uma solicitação de auditoria sem um malabarismo de planilha.

**O MapexOS condensa tudo isso em uma única arquitetura** — a mesma escalabilidade em cada capacidade,
sem duplicação de dados, e sem integrações para o seu time construir e manter na mão.

---

## Como funciona — um pipeline, de ponta a ponta

Cada sinal, venha de onde vier, segue o mesmo caminho governado. Cada etapa é um serviço real, em
execução.

| Etapa | O que ela faz pela sua operação |
|---|---|
| **Ingest** | Entrada autenticada por **HTTP**, **MQTT** e **LoRaWAN** (network server como overlay sobre o The Things Stack). Toda fonte é um ponto de entrada autenticado e governado. |
| **Transform** | A lógica de **decode → validate → convert** por Asset roda em JavaScript em sandbox, transformando o payload de qualquer fabricante em um único evento normalizado e tipado — sem um ramo de código por dispositivo. |
| **Route** | Os eventos são avaliados contra as suas regras e entregues só aos sistemas que precisam vê-los — sinal, não ruído. |
| **Automate** | Um **workflow engine durável** e os triggers de saída transformam eventos em ação: tickets, notificações, escalações, comandos para dispositivos. |
| **Store** | Cada evento é guardado em um store tipado e consultável, com **retenção por Asset** — uma trilha de auditoria completa e pesquisável, por padrão. |

---

## O que você realmente ganha

### Automatize a operação — não só os alertas

O workflow engine é um runtime durável de verdade, não uma caixinha de regras. Ele modela lógica
operacional real — **condições aninhadas AND / OR / NAND / NOR** com 18 operadores, **variáveis
persistentes e efêmeras**, loops, sub-workflows, timers, fan-out/merge e **nós de plugin instaláveis**.
Os fluxos **sobrevivem a reinícios de processo** salvando um checkpoint a cada passo, e continuam
legíveis em escala com **links Goto** em vez de fios cruzando o canvas. O mesmo engine que abre um
ticket de manutenção a partir de uma tendência de vibração consegue escalar um pedido de alto valor com
pagamento recusado — um único livro de regras para IoT e sistemas de negócio.

### Saiba o estado de tudo

Cada Asset reporta **online / offline e último contato de forma nativa**, calculado a partir de
heartbeats e da presença no broker. A operação enxerga o que está realmente ativo sem precisar montar
nenhum encanamento de heartbeat.

### Opere muitos tenants em uma plataforma só

Cada evento, Asset, template e regra pertence a uma **organização** em uma hierarquia sem limite de
profundidade (vendor → customer → site → building → floor → zone). Permissões, retenção e visibilidade
seguem essa árvore via controle de acesso baseado em papéis — **uma única instalação atende dezenas de
clientes**, sem nenhuma visibilidade compartilhada entre eles.

### Pronto para auditoria por padrão

Cada evento e cada ação fica guardado e consultável por Asset, por site, por janela de tempo. A
evidência de compliance vira uma consulta, não um projeto trimestral.

### Seguro e soberano

Os segredos são **cifrados com envelope encryption** e nunca saem numa resposta de API. Os dispositivos
recebem **certificados X.509 por Asset** de uma CA interna. O código do cliente roda em **sandboxes
isolados** com limites rígidos. E como o MapexOS é **self-hosted**, o dado operacional nunca sai da
infraestrutura que você controla.

### Integre em dias, não em trimestres

Um **Asset Template** reutilizável converte o que quer que a fonte mande em um evento padrão, então um
novo fabricante ou modelo é uma *configuração*, não um novo ramo de código — um único template servindo
centenas de Assets em centenas de sites.

---

## Na prática: o que muda

Um operador regional de cadeia fria tem 400 freezers espalhados por 60 sites. Antes do MapexOS, o sensor
de cada fabricante tinha o seu próprio portal, os alertas disparavam a cada abertura de porta, e os
auditores montavam a evidência na mão a cada trimestre.

No MapexOS, um único **Asset Template** normaliza cada marca no mesmo evento de temperatura; um
**workflow** correlaciona temperatura com o estado da porta e só escala depois que uma violação de
verdade se mantém; um **trigger** abre um único ticket de ITSM por incidente; o **event store** guarda
cada avaliação — pesquisável e defensável; e um segundo operador roda na **mesma instalação**, com
isolamento total.

O resultado que os times de operação relatam: solicitações de auditoria respondidas em uma tarde, ruído
de alerta bem menor, e onboarding de fabricante medido em dias. **É esse resultado repetível que o
MapexOS existe para entregar.**

---

## Onde o MapexOS se encaixa

| Se você vinha usando… | …o MapexOS acrescenta |
|---|---|
| **Uma plataforma de IoT tradicional** (ingestão + dashboards + alertas) | Workflows duráveis, governança multi-tenant, extensibilidade por plugin e automação de eventos de negócio — não só gráficos de sensor. |
| **Um pipeline caseiro** (broker + functions + um banco + cola) | Tudo que você ia acabar construindo de qualquer jeito: identidade, RBAC, auditoria, segredos, templates reutilizáveis, durabilidade de workflow e uma UI de operador. |
| **Um event broker genérico** (NATS, Kafka, RabbitMQ) | A camada de aplicação acima do transporte: ingestão, normalização, roteamento, automação durável, auditoria, identidade. |
| **Uma ferramenta de automação no-code** (Zapier, Make, n8n) | Isolamento multi-tenant, workflows duráveis que sobrevivem a reinícios, código em sandbox, self-hosting e credenciais guardadas no vault. |

O MapexOS não substitui o seu broker, banco ou dashboard. Ele é a camada governada que conecta tudo isso
e dá aos operadores um único lugar para controlar o que flui entre eles.

---

## Rode do seu jeito

O MapexOS é entregue como um monorepo de **dez serviços de backend** e um console em **Vue 3**, com
repositórios satélite para o broker MQTT, o network server LoRaWAN e o SDK compartilhado. A stack inteira
sobe com **um único comando** e é publicada em **multi-arch** — de um Raspberry Pi num site remoto a um
cluster `x86_64` no seu data center.

É distribuído sob a **Business Source License (BSL 1.1)**: leia o código, modifique e rode em produção
para a sua própria organização; na Change Date ele converte para Apache 2.0.

---

## Para onde ir agora

| Se você está… | Comece aqui |
|---|---|
| Avaliando o MapexOS | [Por que MapexOS?](./why-mapexos.md) — a dor que ele resolve e as decisões por trás |
| Aprendendo o vocabulário | [Conceitos Centrais](../getting-started/concepts.md) |
| Mapeando a arquitetura | [Visão Geral da Arquitetura](../architecture/overview.md) |
| Pronto para rodar | [Quickstart](../getting-started/quickstart.md) |
