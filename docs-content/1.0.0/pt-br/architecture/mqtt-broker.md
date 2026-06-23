---
title: MQTT Broker
description: A borda de entrada MQTT do MapexOS — Eclipse Mosquitto com um único plugin Go que autentica cada conexão localmente a partir da projeção de Assets e converte o tráfego autorizado em eventos NATS estruturados.
---

# MQTT Broker

O MQTT Broker é o caminho pelo qual dispositivos que falam **MQTT** entram no MapexOS. Ele **não** é um microsserviço do MapexOS — é o **Eclipse Mosquitto 2.0** carregando um único **plugin Go** desenvolvido internamente (cgo) que transforma a borda MQTT em uma fonte de eventos **NATS** estruturados. O Mosquitto cuida do protocolo MQTT; o plugin cuida do MapexOS: ele **autentica cada conexão localmente**, autoriza cada tópico e encaminha cada mensagem autorizada para o NATS — de onde a [JS Execution](./js-execution.md) e o monitor de saúde de [Assets](./assets.md) a consomem.

É o lado broker da regra que você viu em [Assets](./assets.md): *Assets define as credenciais; a borda decide a conexão.* Esta é essa borda.

> Empacotado como a imagem Docker `mapexos/mapex-broker-mqtt` (operadores fazem pull, nunca build) · **Mosquitto 2.0.x** + um plugin **cgo** (libmosquitto + OpenSSL) · listeners **1883** (TCP) / **8883** (mTLS, detectado automaticamente) · **Pebble** (L1) + **MinIO** (L2) + **NATS Core** (nenhum JetStream produzido aqui).

---

## Um plugin, quatro hooks

O plugin registra exatamente quatro callbacks do Mosquitto — todo o comportamento do MapexOS se apoia neles:

| Hook | O que o plugin faz |
|---|---|
| `BASIC_AUTH` | Autentica a conexão contra a projeção de auth de Assets. |
| `ACL_CHECK` | Autoriza cada PUBLISH / SUBSCRIBE por tópico. |
| `MESSAGE` | Encaminha cada publish autorizado para o NATS. |
| `DISCONNECT` | Emite um aviso de presence. |

O Mosquitto 2.0.x não expõe um hook `CONNECT`, então um dispositivo entrando **online** é derivado da borda de **auth-success** — o broker nunca precisa de um sinal de conexão separado.

---

## Auth sem HTTP no caminho quente

Cada conexão é autenticada **localmente** — não há nenhuma chamada de volta para Assets por conexão. O plugin resolve as credenciais do dispositivo através de um **cache de três camadas, auto-recuperável**:

```
 CONNECT → L1 Pebble (KV embarcado em disco, sobrevive a restarts)
              ↓ miss
           L2 MinIO   (mapex-asset-auth/{assetUUID}.json — a projeção que Assets escreve)
              ↓ miss
           L3 HTTP    (GET /internal/asset_auth/:assetUUID → Assets)
```

Todo acerto em L2 ou L3 **aquece a L1**, então o caminho quente é puro disco local. Um miss completo **nega** (default-deny); uma indisponibilidade total do store **falha fechado**. A L1 carrega uma rede de segurança de TTL (padrão de 30 min), e um consumidor **FANOUT** descarta entradas obsoletas da L1 no instante em que Assets altera um dispositivo — de modo que o broker autentica rápido e nunca com credenciais desatualizadas.

As credenciais em si vêm direto do bloco `mqtt` da projeção de auth de [Assets](./assets.md). O broker aplica os **dois modos mutuamente exclusivos** que Assets declara:

- **password** — uma comparação local **bcrypt** contra `passwordHash` (executada na thread do broker, sem HTTP),
- **cert** — igualdade entre o serial do certificado do cliente e `currentCertSerial`.

Um asset em modo password apresentando um cert é negado, e vice-versa; um tipo de auth desconhecido é negado.

---

## Um dispositivo só pode tocar nos próprios tópicos

A autorização é uma verificação **puramente em Go, sem alocação** sobre um contrato estrito. O **username** MQTT é o **`assetUUID` puro** (globalmente único), e um dispositivo pode usar exatamente dois formatos de tópico:

| Tópico | Direção |
|---|---|
| `events/{assetUUID}/{eventType}` | o dispositivo **publica** telemetria |
| `commands/{assetUUID}/{commandType}` | o dispositivo **se inscreve** em comandos |

A regra decisiva: o token `assetUUID` no tópico **precisa ser igual ao username**. Um dispositivo fisicamente não consegue publicar nem ler nos tópicos de outro dispositivo — o acesso entre assets é negado no broker, em uma comparação de strings na casa dos sub-microssegundos, antes de qualquer coisa chegar ao NATS.

---

## Disponibilidade acima de exatidão — o publisher assíncrono

Um publish autorizado nunca pode deixar um backend lento travar um dispositivo. Por isso o plugin entrega cada mensagem a um **publisher assíncrono limitado**: um enfileiramento não-bloqueante em um channel de tamanho fixo, drenado por um pool de workers que publicam no NATS. Se a fila está cheia, a mensagem é **descartada e contabilizada** — nunca bloqueada, nunca repetida inline. Um NATS lento ou inacessível degrada para drops contabilizados, enquanto os **clientes MQTT seguem se conectando e publicando** sem serem afetados. Cada payload é copiado através da fronteira cgo antes de chegar a um worker, de modo que os buffers do próprio broker nunca são tocados fora da thread.

---

## O que ele emite — bytes crus, subjects estruturados

O broker **não decodifica nem transforma** payloads — ele encaminha os bytes crus do MQTT e deixa a [JS Execution](./js-execution.md) normalizá-los. Ele publica dois streams NATS (Core, fire-and-forget):

| Sinal | Subject | Consumido por |
|---|---|---|
| **Ingress** | `mqtt.data.{orgId}.{assetUUID}` | [JS Execution](./js-execution.md) (se inscreve em `mqtt.data.>`) |
| **Presence** | `mqtt.presence.advisory` (connect / disconnect) | o monitor de saúde de [Assets](./assets.md) |

Antes de publicar, o ingress aplica invariantes de segurança — descarta tokens de subject que são ilegais no NATS e payloads maiores que ~900 KiB, cada um como um aviso contabilizado em vez de um evento malformado.

**Consome:** `fanout.asset.invalidate` (de Assets) para remover entradas obsoletas da L1. O broker **não roda nenhum servidor HTTP próprio** — ele é um plugin, não um serviço.

---

## Para onde ir agora

| Para se aprofundar em… | Vá para |
|---|---|
| Quem define as credenciais que ele lê | [Assets](./assets.md) |
| O que normaliza os bytes que ele encaminha | [JS Execution](./js-execution.md) |
| A contraparte LoRaWAN na borda | [LNS](./lns.md) |
