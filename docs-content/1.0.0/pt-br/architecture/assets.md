---
title: Assets
description: A fonte da verdade para tudo o que está conectado ao MapexOS — o modelo de asset e template, o schema de campos EVA, o control plane de PKI e presence dos dispositivos, e o cache de read-model a partir do qual todo outro serviço é reconstruído.
---

# Assets

Assets é a **fonte da verdade** para tudo o que está conectado ao MapexOS. Ele detém *o que* um asset é, *como* seus payloads viram eventos normalizados, *como* ele se autentica e *se está online neste exato momento*. Ele não ingere, transforma nem roteia eventos por conta própria — ele **configura as entidades sobre as quais o resto do pipeline atua** e **escreve o cache que todo outro serviço lê**. Quando o [MQTT broker](./mqtt-broker.md), o [LNS](./lns.md), o [Router](./router.md) e o [JS execution](./js-execution.md) precisam saber algo sobre um asset, eles leem do seu próprio tiered cache — e **o fundo de cada um desses caches faz fallback para o Assets**, o único lugar que sempre consegue reconstruir a verdade.

Ele carrega duas responsabilidades: **o modelo** (Assets + Asset Templates) e o **control plane dos dispositivos** (PKI + presence/saúde).

> **Port** `5002` · **Go** (DDD + Hexagonal) · **MongoDB** (write model) + **MinIO/S3** (projeções de read-model) + **Redis** (hot-state de saúde) · **mapexVault** para a CA de assinatura e a key-encryption key · quatro módulos: `assets`, `assettemplates`, `healthmonitor`, `mqttcerts`.

---

## Asset & Template — onboarding por configuração, não por código

O modelo tem duas metades:

- Um **Asset Template** é um blueprint reutilizável: sua **classificação** (categoria / fabricante / modelo / versão), o `assetIdPath` que localiza o id do dispositivo num payload, os **scripts de transformação** e o **schema de campos EVA**. Dois scripts são obrigatórios — `scriptValidator` e `scriptConversion` — e dois são opcionais — `scriptProcessor` (pré-processamento) e `scriptTest` (para o test runner do console).
- Um **Asset** é uma instância de dispositivo vinculada a um template, carregando sua identidade de protocolo, route groups, config de saúde e certificado ativo. O `assetUUID` é seu id de negócio estável e cross-platform.

Um único template atende milhares de assets espalhados por centenas de sites. Um novo vendor ou modelo é uma **configuração de template**, nunca um novo branch de código — é isso que transforma o onboarding de dispositivos de uma tarefa de engenharia numa tarefa de setup.

---

## Campos dinâmicos EVA — onde o `fieldId` nasce

Um template declara seus campos dinâmicos, e **é aqui que se origina o armazenamento colunar tipado** que você viu em [Events](./events.md). Cada campo recebe um id numérico com um contrato rígido:

- `fieldId` é um `uint16`, **imutável** depois de atribuído (um contador `nextFieldId` por template apenas incrementa — ids nunca são reutilizados),
- um `type` declarado (`number` · `string` · `bool` · `date` · `geo`),
- um `status` de soft-delete (um campo removido é depreciado, nunca apagado fisicamente),
- até **200 campos ativos** por template.

Essa imutabilidade *é* o contrato entre Assets e Events: o Assets atribui o `fieldId`, e o Events armazena valores sob ele em seus mapas EVA tipados. Como os ids nunca se movem, o histórico permanece coerente para sempre.

---

## A fonte da verdade — e o cache do qual todo serviço é reconstruído

Este é o centro de gravidade da arquitetura. A **cada mudança que afeta como um asset é lido ou autenticado**, o Assets escreve duas projeções desnormalizadas no MinIO/S3:

| Projeção | Path | Para |
|---|---|---|
| **Read-model completo** | `mapex-assets/{orgId}/{assetUUID}.json` | Router, JS-execution, Events — contexto completo do asset. |
| **Projeção slim de autenticação** | `mapex-asset-auth/{assetUUID}.json` | o plugin do MQTT broker e o LNS — apenas o necessário para autenticar. |

Em seguida, dispara uma invalidação **FANOUT** no NATS para que cada réplica de cada consumidor descarte sua cópia obsoleta. O read-model é sempre escrito **antes** da invalidação, então um serviço que refaz o fetch ao receber a invalidação enxerga dados frescos, nunca um buraco.

```
                 ┌──────────── Assets (source of truth) ────────────┐
   write change → │  MongoDB (write model)                          │
                 │     │ project                                     │
                 │     ▼                                             │
                 │  MinIO  mapex-assets/… · mapex-asset-auth/…       │
                 │     │ then                                        │
                 │     ▼                                             │
                 │  FANOUT invalidate ─────────────────────────────┐│
                 └──────────────────────────────────────────────────┘
                                                                    ▼
   consumer cache:   L0 RAM → L1 disk → L2 MinIO → L3 HTTP fallback → Assets
                     (the fallback always reconstructs from the source of truth)
```

Todo consumidor — broker, LNS, Router, JS-execution — roda seu próprio `TieredCache`, e **o último tier de todos eles faz fallback por HTTP para o Assets** (`GET /internal/assets/:assetUUID`), que repopula o cache inline. Não importa o quão frio um cache fique, sempre existe um lugar autoritativo a partir do qual reconstruí-lo.

---

## A autenticação acontece na borda — o Assets nunca autentica

O Assets **escreve** as credenciais, mas **nunca valida uma conexão** por conta própria. A projeção slim de autenticação já está na borda, então os servidores que terminam o protocolo decidem cada conexão **localmente**:

- O **plugin do [MQTT broker](./mqtt-broker.md)** decide cada `CONNECT` MQTT apenas a partir do bloco `mqtt` da projeção — comparando via bcrypt a senha apresentada contra o `passwordHash`, ou casando o serial do certificado do cliente contra o `currentCertSerial`.
- O **[LNS](./lns.md)** autentica joins LoRaWAN a partir do bloco `lorawan` da projeção — a identidade do dispositivo e suas chaves cifradas em envelope (gateways autenticam por serial de certificado).

**Não há chamada HTTP ao Assets por conexão** — o único caminho HTTP da borda → Assets é o fallback de cache-miss L3 acima. A autenticação é rápida, feita onde a conexão chega, e continua funcionando mesmo enquanto o Assets estiver brevemente indisponível.

---

## Control plane de PKI dos dispositivos

O Assets é uma autoridade certificadora local para dispositivos (módulo `mqttcerts`). Ele emite **certificados de cliente X.509** — **ECDSA P-256, serial de 128 bits, extended key usage de client-auth** — assinados por uma **CA intermediária buscada uma única vez no [mapexVault](./vault.md) e mantida em RAM** (`atomic.Pointer`, nunca escrita em disco; um gate `caReady` retorna `502` até a CA ser carregada, com retry de backoff em caso de falha).

- **Um certificado ativo por asset** — reemitir exige `force=true`, caso contrário `409`.
- O **PEM do certificado e da chave privada são retornados exatamente uma vez** no momento da emissão e **nunca persistidos** — o asset guarda apenas metadados (serial, fingerprint, subject CN, emitido/expira).
- A **revogação é um hard delete** mais uma linha de auditoria à prova de adulteração que expira automaticamente após **30 dias** (um índice TTL do MongoDB).

A mesma maquinaria emite certificados para gateways LoRaWAN.

---

## Saúde de conectividade & presence

O `healthmonitor` é um motor genuíno de tempo quase real que rastreia o **online / offline / last-seen** de cada asset. Ele aprende a liveness de duas formas:

- **Heartbeats** em `asset.heartbeat.>` — *implícitos* (o js-execution emite um por evento de dado) ou *explícitos* (um `POST /api/v1/heartbeat` HTTP, ou presence advisories do MQTT broker para assets MQTT) — atualizando um timestamp de last-seen no Redis.
- Um **scan periódico** que marca um asset como offline assim que ele fica silencioso além do seu threshold por `requiredMisses` varreduras consecutivas (`cutoff = now − threshold`).

Ele roda em três modos por asset — **disabled**, **monitor-only** (persiste estado) ou **monitor + route** — e a cada transição publica duas coisas: um `asset_status_save` para o [Events](./events.md) (o histórico de conectividade consultável) e, quando o asset os configura, um `route.execute` com `eventSource = healthStatus` para o [Router](./router.md) (que roteia mudanças de presence apenas para `trigger` / `workflow`). As transições são **exatamente-uma-vez entre réplicas** (um `SREM` atômico no Redis), com uma proteção anti-race para reconexões multi-broker. Habilitar o monitoramento de saúde exige ao menos um route group de offline/online — caso contrário, um `422` fail-fast.

---

## Identidade multi-protocolo & segredos

Um asset fala um de três protocolos, e **nenhum segredo é jamais armazenado em texto puro**:

| Protocolo | Tratamento de identidade & segredo |
|---|---|
| **HTTP** | Sem credencial de dispositivo — admitido no [HTTP Gateway](./http-gateway.md) pelo seu DataSource. |
| **MQTT** | `password` **xor** `cert` (mutuamente exclusivos). Senhas são **bcrypt-hashed**; certs vêm do plane de PKI. |
| **LoRaWAN** | Dispositivo (OTAA/ABP) ou gateway. As chaves root/de sessão do dispositivo são **cifradas em envelope com uma KEK do mapexVault** — apenas os quatro campos de envelope persistem; o consumidor em runtime é o [LNS](./lns.md). |

O texto puro (uma senha gerada, uma chave, um PEM de cert) é mostrado ao operador **uma única vez** no momento da criação/emissão e fica irrecuperável depois disso.

---

## Entradas & saídas

**HTTP (in):** CRUD de `/api/v1/assets` e `/api/v1/asset_templates` (JWT, gated por permissão e coverage); `/api/v1/mqtt_certs` e `/api/v1/gateway_certs` (emitir/revogar, gated na prontidão da CA); rotas internas com API-key para o fallback de cache L3 (`GET /internal/assets/:assetUUID`, `…/asset_auth/:assetUUID`, `…/scripts/:assetUUID`).

**NATS (consumido):** `asset.heartbeat.>`, presence advisories do MQTT, o scan periódico de saúde e atualizações de nomes de classificação.

**NATS (produzido):** `fanout.asset.invalidate` e `fanout.template.invalidate` (coerência de cache), `events.asset_status_save` (histórico de conectividade → Events) e `route.execute` (`eventSource=healthStatus` → Router) nas transições de saúde.

**Object storage:** as duas projeções no MinIO acima, mais os scripts de template (`{orgId|mapexos_public}/{templateId}.json`) para o JS-execution.

---

## Para onde ir em seguida

| Para se aprofundar em… | Vá para |
|---|---|
| Onde os scripts dos templates rodam | [JS Execution](./js-execution.md) |
| Onde os campos dinâmicos são armazenados & consultados | [Events](./events.md) |
| Onde as mudanças de presence são roteadas | [Router](./router.md) |
| A CA e a KEK por trás do control plane | [Vault](./vault.md) |
