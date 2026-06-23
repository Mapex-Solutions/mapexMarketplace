---
title: LNS
description: A borda LoRaWAN do MapexOS — um network server construído como uma camada fina sobre o The Things Stack, que extrai seus dispositivos do modelo de Assets e transforma uplinks decifrados em eventos NATS.
---

# LNS

O LNS (LoRaWAN Network Server) é o caminho pelo qual dispositivos **LoRaWAN** chegam ao MapexOS. Em vez de reimplementar uma stack LoRaWAN, ele **embarca o [The Things Stack](https://www.thethingsindustries.com/stack/)** — o motor LoRaWAN aberto e de nível produção — como dependência e roda apenas os papéis de protocolo de que precisa, encaixando o MapexOS em algumas costuras precisas. O The Things Stack cuida do protocolo de rádio; o MapexOS é dono de **identidade, chaves e fluxo de dados**: o LNS alimenta o TTS com seus dispositivos a partir do modelo de [Assets](./assets.md), e devolve os uplinks decifrados para o **NATS**, para a [JS Execution](./js-execution.md) normalizar — exatamente como as bordas de entrada HTTP e MQTT.

> Embarca o **The Things Stack v3** como dependência Go fixada (pinned), executado a partir de seu próprio composition root (o TTS nunca é forkado) · transportes de gateway **UDP** Semtech packet-forwarder (`:1700`) + **Basics Station** WebSocket (`:1887`) · **Redis** (sessão quente) + **MinIO**/**NVMe** (cache de config fria) · sem servidor HTTP, sem MongoDB.

---

## Camada sobreposta, não fork

O LNS inicia o The Things Stack a partir de **seu próprio composition root** e acende exatamente três papéis em um componente compartilhado:

- **Gateway Server (GS)** — termina o tráfego de gateway,
- **Network Server (NS)** — camada MAC, frame counters, agendamento,
- **Join Server (JS)** — join OTAA + derivação de chaves de sessão.

Ele deliberadamente **não** inicia o Application Server, o Identity Server nem o Console do TTS — porque o MapexOS já é dono dessas responsabilidades ([JS Execution](./js-execution.md) é o codec, [Assets](./assets.md) é o registro de dispositivos, e o frontend do MapexOS é o console). O TTS é uma dependência fixada, nunca editada; papéis não usados simplesmente nunca são construídos. O resultado é um motor LoRaWAN real e testado em batalha, com o MapexOS substituído apenas onde importa.

---

## Os dispositivos vêm de Assets, não de um banco do TTS

Este é o truque central. O LNS **substitui o banco de dispositivos do TTS** por um adaptador de **hidratação sob miss** (hydrate-on-miss):

```
 TTS pede um dispositivo
     │  HOT  → Redis session store (nsredis / jsredis)
     │  miss
     └─ COLD → a config fria de Assets via tiered cache
               (L1 NVMe → L2 MinIO mapex-asset-auth/{assetUUID}.json → Assets HTTP)
               → mapeado para um dispositivo TTS → cacheado → retornado
```

A identidade, a região, a classe e as **root keys** do dispositivo vêm do mesmo modelo de [Assets](./assets.md) que o resto da plataforma usa (`orgId` = aplicação TTS, `assetUUID` = dispositivo TTS). As root keys são **decifradas com uma key-encryption key buscada no [Vault](./vault.md)** no boot e mantidas apenas em RAM — o LNS nunca persiste chaves em texto puro. Tanto a ativação **OTAA** (1.0.3 e 1.1) quanto a **ABP** são suportadas, e um evento `fanout.asset.invalidate` remove ambas as camadas de cache no momento em que um dispositivo muda upstream.

---

## Estado frio vs. quente

A divisão é deliberada e estrutural:

| Estado | O que é | Onde vive |
|---|---|---|
| **Frio** | identidade, root keys, região, classe, sessão ABP — **somente leitura**, reconstruível a partir do asset | o modelo de Assets (cacheado) |
| **Quente** | frame counters, chaves de sessão derivadas, estado MAC — **descartável** | apenas no Redis |

Assim, o LNS não é dono de **nenhum banco de registro**. Perca o estado quente do Redis e um dispositivo simplesmente **refaz o join** (OTAA) ou se re-hidrata (ABP) — não há nada insubstituível para fazer backup.

---

## Uplinks decifrados viram eventos NATS

Quando o TTS valida o MIC de um uplink e o decifra via AES, o LNS despacha o **payload em texto puro** para o NATS — o TTS acredita estar alimentando um Application Server; está alimentando o pipeline do MapexOS. Ele publica subjects por dispositivo (NATS Core, fire-and-forget):

| Sinal | Subject | Vai para |
|---|---|---|
| **Uplink** | `lorawan.data.{orgId}.{assetUUID}` | [JS Execution](./js-execution.md) — normalizado como qualquer outra entrada |
| **Join** | `lorawan.join.{orgId}.{assetUUID}` | um sinal de join OTAA / presence |
| **Status de downlink** | `lorawan.downlink.status.{orgId}.{assetUUID}` | o ciclo de vida do comando (`queued` → `sent` → `ack`/`nack`/`failed`) |

O envelope do uplink carrega o `orgId`, o `assetUUID`, o `fPort`, o `fCnt` e o payload em texto puro em base64 — então a JS Execution resolve o asset e roda o **mesmo pipeline de decode → validação → transform** que roda para HTTP e MQTT. O LNS **não** decodifica payloads de aplicação; isso é trabalho da JS Execution.

---

## Gateways e downlinks

Gateways são **entidades de primeira classe, registradas**: apenas gateways conhecidos se conectam (`RequireRegisteredGateways`), cada um extraído da config fria de Assets com seu próprio **frequency plan** (multi-região, planos embarcados no binário). A autenticação de gateway é por **EUI** (UDP) ou **mTLS** (Basics Station) — o serial do certificado fixado a partir do asset, a cadeia de CA provisionada pela PKI do [Vault](./vault.md).

No caminho reverso, o LNS **consome** comandos de downlink do NATS (stream JetStream `LORAWAN_DOWNLINK`) e os empurra para a fila de downlink do Network Server, emitindo os eventos de status acima conforme o comando avança em seu ciclo de vida.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Onde a identidade e as chaves do dispositivo são definidas | [Assets](./assets.md) |
| A KEK e a CA de gateway por trás disso | [Vault](./vault.md) |
| O que normaliza os uplinks que ele emite | [JS Execution](./js-execution.md) |
| A contraparte MQTT na borda | [MQTT Broker](./mqtt-broker.md) |
