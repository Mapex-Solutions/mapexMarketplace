---
title: Vault
description: A raiz de confiança do MapexOS — um cofre de segredos cifrados em envelope, um serviço de distribuição de chaves e a autoridade certificadora por trás de toda identidade de dispositivo. Segredos entram, mas nunca são entregues de volta.
---

# Vault

mapexVault é a **raiz de confiança** do MapexOS. Ele guarda o que jamais pode vazar: **credenciais** de terceiros, as **chaves** que permitem a outros serviços cifrarem seus próprios segredos, e a **autoridade certificadora** por trás de toda identidade de dispositivo. Ele fica fora do caminho da telemetria — não ingere eventos nem roteia nada — mas o resto da segurança da plataforma é construído sobre ele. Sua regra mestra é simples: **um segredo entra, e o texto puro nunca volta numa resposta de API** — apenas para um serviço interno nomeado, por um canal interno autenticado, e somente enquanto a requisição durar.

Ele faz três trabalhos, em três módulos: **credentials**, **KEK** e **PKI**.

> **Port** `5010` · **Go** (DDD + Hexagonal) · **MongoDB** (cifrado em repouso) · **NATS JetStream** (refresh de token agendado) · cifragem em envelope **AES-256-GCM** · CA com **crypto/x509**. O material secreto é marcado com `json:"-"` — ele não consegue serializar para uma resposta.

---

## Credentials — cifragem em envelope de duas camadas

Uma credential é um segredo de terceiros — um token de API, um grant OAuth, um usuário/senha (`manual` · `oauth2` · `userAndPass`). Ela é armazenada sob **duas camadas de cifragem em envelope**:

```
 master key  ──wraps──▶  per-record DEK  ──encrypts──▶  the secret data
 (in Vault only)         (stored encrypted)              (AES-256-GCM)
```

Uma única **master key** (mantida apenas pelo Vault) envelopa uma **data-encryption key por registro**; essa DEK cifra o payload. Apenas os quatro campos de envelope são persistidos (`encryptedDEK`, `dekNonce`, `encryptedData`, `dataNonce`) — todos marcados com `json:"-"`, de modo que o segredo **fisicamente não pode** ser serializado numa resposta de API. O texto puro é entregue por um único caminho: um `GET /internal/credentials/:id/decrypt` oculto e gated por API-key, usado pelo motor de [Workflow](./workflow.md) para executar a action de um plugin — e mesmo ali ele só vive durante a requisição.

Como cada registro tem sua própria DEK, comprometer a chave de um registro expõe um registro — não o cofre inteiro.

---

## Refresh de token — agendado, nunca por polling

Credenciais OAuth e de token expiram, então o Vault as mantém vivas **sem um cron de polling**. Quando uma credential é armazenada, o Vault agenda uma única **mensagem NATS JetStream** para disparar em **`expiry − 15 minutos`**; quando dispara, o Vault executa a troca de token e **rearma o próximo timer**. Um **reconciler** de hora em hora resemeia qualquer timer perdido num restart ou drift do NATS, e credenciais já expiradas são reagendadas para refresh quase imediatamente no boot.

A parte engenhosa é a separação: o **`providerConfig`** de uma credential — *como* refrescá-la (o método HTTP, a URL, os headers e onde na resposta mora o novo token) — é armazenado **sem cifragem de propósito**. Os valores secretos são injetados apenas no momento da execução, através de placeholders `{{credential.*}}`. Assim, o worker de refresh sabe *onde e como* refrescar **sem nunca decifrar o segredo** para descobrir.

---

## KEK — deixe os serviços cifrarem localmente, sem a master key

Nem todo serviço deveria chamar o Vault para cifrar cada segredo, e nenhum deveria jamais segurar a master key. Por isso o Vault distribui **key-encryption keys**: uma chave AES-256 escopada por contexto (por exemplo `lorawan_device_keys`) servida por `GET /internal/kek/:context`. Um serviço busca sua KEK uma vez e então roda **sua própria cifragem em envelope local** com ela — do jeito que o [Assets](./assets.md) sela as chaves de dispositivo LoRaWAN antes de armazená-las.

A master key nunca sai do Vault; o serviço consumidor segura apenas uma chave escopada para seu próprio contexto. As KEKs são semeadas no momento do deploy, não criadas em runtime, e o endpoint retorna `503` até um contexto ser semeado — um claro "ainda não pronto" contra o qual o chamador pode tentar de novo.

---

## PKI — a autoridade certificadora sob demanda

O Vault é a **autoridade certificadora** sob toda identidade de dispositivo. Ele armazena a **CA root e a intermediária** com suas chaves privadas **cifradas em envelope em repouso**, e expõe três operações internas:

| Endpoint | Retorna | Consumido por |
|---|---|---|
| `GET /internal/pki/intermediate_ca_bundle` | o cert + a chave da CA intermediária | [Assets](./assets.md) — mantida em RAM para assinar certs de cliente por dispositivo |
| `GET /internal/pki/ca_chain` | a cadeia pública root + intermediária | qualquer um que verifique um cert do MapexOS |
| `POST /internal/pki/sign_server` | um certificado de servidor assinado | a identidade TLS do [MQTT broker](./mqtt-broker.md) |

A propriedade definidora: **a chave privada da CA é decifrada a cada requisição e descartada** — ela nunca fica em cache na memória entre chamadas, e espera-se que o chamador a descarte no instante em que a usa. Toda a história de device-PKI que você viu em [Assets](./assets.md) — certs ECDSA por dispositivo assinados por uma CA intermediária mantida em RAM — **tem sua raiz aqui**: o Assets busca o bundle uma vez no boot, e o Vault é de onde a confiança desse bundle realmente vem.

---

## Multi-tenant por padrão

Toda operação de credential externa é escopada ao `orgId` do chamador e ao `pathKey` hierárquico do contexto da requisição — uma organização nunca pode ler ou refrescar os segredos de outra. Templates de credential são excluídos das leituras de owned e não podem ser criados ou alternados pela API externa.

---

## Entradas & saídas

**HTTP (externo, JWT + permissões)** sob `/api/v1/credentials`: `POST` · `GET` · `GET /:id` · `PATCH /:id` · `DELETE /:id`.

**HTTP (interno, API-key, oculto):** `/internal/credentials/:id/decrypt` (→ Workflow), `/internal/kek/:context` (→ Assets), `/internal/pki/intermediate_ca_bundle` (→ Assets), `/internal/pki/ca_chain`, `/internal/pki/sign_server` (→ MQTT broker).

**NATS (auto-entregue):** agendamentos de refresh por credential em `MAPEXVAULT-SCHEDULE`, e o stream do reconciler que os mantém semeados.

Ele não publica nada no bus cross-service e não consome telemetria — o Vault é infraestrutura que os outros serviços *chamam*, não um estágio do pipeline.

---

## Para onde ir em seguida

| Para se aprofundar em… | Vá para |
|---|---|
| Quem usa a CA e a KEK LoRaWAN | [Assets](./assets.md) |
| Quem decifra credentials para executar plugins | [Workflow Engine](./workflow.md) |
| O broker cujo cert de servidor o Vault assina | [MQTT Broker](./mqtt-broker.md) |
