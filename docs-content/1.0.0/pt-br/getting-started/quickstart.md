---
title: Quickstart
description: Conecte um sensor de temperatura de ponta a ponta — de uma leitura bruta a um evento armazenado que você vê no Grafana — em poucos minutos.
---

# Quickstart

Leve um sensor de uma leitura bruta até um **evento armazenado e consultável** — o pipeline inteiro, em poucos minutos. Este é o caminho feliz guiado; os formulários campo a campo e o JSON pronto para colar ficam na pasta [`quickstart/`](https://github.com/Mapex-Solutions/mapexOSDeploy/tree/main/quickstart) do repositório de deploy.

**Antes de começar:** a stack está rodando e você está logado no frontend (`admin@mapex.local` / `mapex@123`). Se não estiver, veja [Instalação](./installation.md).

## O que você vai construir

Você vai criar quatro coisas no console e depois empurrar dados através delas:

```
Asset Template → Route Group → Datasource → Asset → (send readings) → Grafana
```

Escolha um caminho de ingestão — os passos são os mesmos; só o datasource muda:

| Caminho | Melhor para |
|---|---|
| **HTTP** | Dispositivos que dão POST de JSON em um webhook (REST, gateways, lambdas) |
| **MQTT** | Dispositivos que publicam em um broker MQTT (a maioria dos firmwares de prateleira) |

## 1 · Asset Template — *Temperature Sensor*

Um template define como uma classe de dispositivo é integrada. Crie um com os três scripts que transformam um payload bruto em um Standard Event:

- **Preprocessor** — normaliza o formato de transporte (opcional).
- **Validation** — rejeita leituras malformadas.
- **Conversion** — emite o Standard Event.

Declare um **dynamic field** `temperature` (type `number`) para que as leituras sejam armazenadas em uma coluna tipada e consultável. Veja [Conceitos Fundamentais → Asset Template](./concepts.md#fontes-de-dados).

## 2 · Route Group — *Save Temperature Events*

Um route group decide para onde vão os eventos que dão match. Crie um que **persista cada leitura no events store** (ClickHouse). Uma regra, um destino — é o suficiente para a primeira rodada.

## 3 · Datasource

**Caminho HTTP:** crie um datasource para obter uma **URL de webhook + API key** para a qual o dispositivo vai dar POST.
**Caminho MQTT:** o dispositivo publica no broker (`localhost:1883`) usando as credenciais do asset — nenhum datasource é necessário.

## 4 · Asset — *weather-http-001*

Crie o asset e vincule-o ao **template**, ao **route group** e ao **protocolo** (HTTP ou MQTT). É essa a coisa que produz eventos.

## 5 · Enviar leituras de teste

Use o script Node.js pronto da pasta quickstart para empurrar leituras fictícias:

```bash
cd mapexOSDeploy/quickstart/device-http   # or device-mqtt
node send-events.js                        # (device-mqtt: publish-events.js)
```

## 6 · Ver chegar

Abra o **Grafana** em <http://localhost:3001> (`admin` / `admin`) e filtre o dashboard de eventos pelo seu **`assetUUID`**. Cada leitura que você envia aparece em segundos — ingerida, validada, convertida, roteada e armazenada.

## Limpeza

Tudo o que você criou é dado comum da plataforma — apague pela UI (Assets, Datasources, Route Groups, Templates). Não há script de teardown.

## Para onde ir em seguida

Você rodou um dispositivo por uma rota. A partir daqui:

| Para aprender… | Vá para |
|---|---|
| O vocabulário por trás de cada passo | [Conceitos Fundamentais](./concepts.md) |
| Como a plataforma é construída | [Architecture Overview](../architecture/overview.md) |
| Por que ela é feita assim | [Why MapexOS?](../introduction/why-mapexos.md) |
