---
title: Segurança
description: Como o MapexOS protege uma operação conectada de ponta a ponta — identidade, multi-tenancy, secrets, confiança de device, autenticação no edge, código em sandbox e uma trilha de auditoria completa — e como endurecer um deploy.
---

# Segurança

A segurança no MapexOS não é um recurso pregado de lado — ela está **entremeada em cada camada**, porque o trabalho da plataforma é ser *operada*: com identidade, isolamento, secrets e auditoria como partes de primeira classe da arquitetura. Esta página é o **mapa**: ela mostra como essas camadas se encaixam e aponta para o serviço que impõe cada uma. Nada aqui é um "produto de segurança" à parte — é como o sistema é construído.

O formato é o de **defesa em profundidade**. Uma requisição é identificada, escopada a um tenant, autorizada, servida por um código do qual ela não consegue escapar, e registrada — e um secret ou uma chave de device é criptografado em repouso, entregue apenas a um serviço nomeado, e nunca devolvido em uma resposta de API.

| Camada | O que protege | Imposto por |
|---|---|---|
| **Identidade e acesso** | quem você é, o que você pode fazer, onde | [mapexIam](./mapexiam.md) |
| **Isolamento multi-tenant** | uma organização não pode ver a outra | [mapexIam](./mapexiam.md) · todo serviço |
| **Secrets e chaves** | credenciais, chaves de criptografia, a CA | [Vault](./vault.md) |
| **Confiança de device** | só devices reais conectam, como eles mesmos | [Assets](./assets.md) · [Edge Servers](#autenticacao-no-edge) |
| **Autenticação no edge** | toda conexão é comprovada antes da admissão | [HTTP Gateway](./http-gateway.md) · [MQTT Broker](./mqtt-broker.md) · [LNS](./lns.md) |
| **Isolamento de código** | o código de cliente não alcança o host | [Execução JS](./js-execution.md) · [Execução de Workflow JS](./js-workflow-execution.md) |
| **Auditoria e rastreabilidade** | um registro completo e consultável do que aconteceu | [Events](./events.md) |

---

## Identidade e acesso

Toda requisição é autenticada e autorizada contra o [mapexIam](./mapexiam.md), o control plane de identidade. Os usuários fazem login com **e-mail + senha bcrypt** e recebem um **access JWT** de vida curta mais um **refresh JWT**; os refresh tokens são armazenados por sessão e **rotacionados com proteção contra replay**. A autorização é **baseada em roles**: as permissões resolvidas de um usuário para uma organização são computadas uma vez e cacheadas, e **todo outro serviço lê esse cache** por meio de um middleware compartilhado — é por isso que cada serviço descreve suas rotas como *permission-gated*. Os roles cascateiam árvore abaixo apenas onde um descendente opta por isso (`merge` vs `strict`), de modo que o acesso herdado é deliberado, nunca acidental.

---

## Isolamento multi-tenant

Um único deploy do MapexOS serve muitas organizações com **zero visibilidade compartilhada**. Cada asset, evento, template, role e credencial é escopado a uma **organização** em uma hierarquia ilimitada (`vendor → customer → site → building → floor → zone`), carregada como um **`pathKey`** desnormalizado. Listagem, consulta e roteamento são filtrados pela **coverage** de quem chama — a fatia da árvore que ele tem permissão de alcançar — de modo que um tenant não consegue enumerar, ler ou rotear os dados de outro. O isolamento é uma propriedade do modelo de dados e do cache, não um filtro que alguém precisa lembrar de aplicar.

---

## Secrets e chaves

Nenhum secret jamais é armazenado em texto puro ou devolvido em uma resposta de API. O [Vault](./vault.md) é a **raiz de confiança**:

- **Credenciais** de terceiros são seladas com **envelope encryption de duas camadas** (uma master key envolve uma data key por registro; AES-256-GCM). Os campos criptografados fisicamente não conseguem serializar para uma resposta; o texto puro é entregue apenas por um endpoint interno oculto e protegido por API key, pela duração de uma única requisição.
- Serviços que precisam criptografar seus próprios secrets recebem uma **chave escopada por contexto (KEK)** e rodam a criptografia **localmente** — a master key nunca sai do Vault. É assim que o [Assets](./assets.md) sela as chaves de device LoRaWAN.
- O Vault também é a **certificate authority** (veja abaixo); suas chaves privadas de CA têm envelope encryption e são decifradas por requisição, nunca cacheadas.

---

## Confiança de device e PKI

Os devices provam quem são com credenciais que o [Assets](./assets.md) emite e o [Vault](./vault.md) enraíza. Um device MQTT se autentica por **senha bcrypt** ou por um **certificado cliente X.509 por device** (ECDSA P-256), assinado por uma **CA intermediária buscada no Vault** e mantida apenas em RAM. A chave privada de um certificado é devolvida **uma única vez** no momento da emissão e **nunca persistida** — só seus metadados são guardados — e a revogação é imediata. Os gateways LoRaWAN se autenticam por **mTLS** com o serial fixado a partir do asset; as chaves de device LoRaWAN têm **envelope encryption** com uma KEK do Vault. As credenciais de um device dão acesso exatamente a esse device — e a nada mais.

---

## Autenticação no edge

Toda conexão externa é comprovada **antes** que qualquer coisa que ela envie seja admitida, e a decisão é tomada **no edge em que ela pousa** — a partir da mesma projeção autorada pelo Assets, sem um gargalo central de auth:

- O **[HTTP Gateway](./http-gateway.md)** autentica cada origem pela sua própria política (API key, JWT, bearer OAuth2/JWKS, IP allow-list ou nenhuma); uma origem desabilitada é recusada antes que qualquer auth rode, e **toda rejeição é auditada**.
- O **[MQTT Broker](./mqtt-broker.md)** decide cada `CONNECT` localmente (bcrypt ou serial do certificado), **falha fechado** diante de uma entrada ausente ou de uma indisponibilidade do store, e impõe um ACL estrito: um device só pode tocar **seus próprios** tópicos — o acesso entre assets é negado no broker.
- O **[LNS](./lns.md)** só admite gateways **registrados** e tira a identidade e as chaves de cada device do modelo do Assets.

A tenancy não pode ser forjada no edge: o `orgId` de uma requisição vem da **origem autenticada**, nunca do corpo do payload.

---

## Execução de código em sandbox

O MapexOS roda **JavaScript autorado por clientes** — scripts de transform de template e nós de código de Workflow — e trata tudo isso como não confiável. Todo script roda dentro de um **V8 isolate do `isolated-vm`** ([Execução JS](./js-execution.md) · [Execução de Workflow JS](./js-workflow-execution.md)) com um **teto de memória** rígido, um **timeout de wall-clock** e **nenhum acesso a Node, à rede ou ao filesystem**. Um script pode computar e transformar, e nada além disso — ele não consegue alcançar o host, outro tenant ou o mundo externo.

---

## Auditoria e rastreabilidade

Tudo o que acontece é registrado e consultável. O [Events](./events.md) persiste cada evento e cada execução no ClickHouse com **retenção por organização**, então a evidência de compliance é uma query, não um projeto. Um único **`eventTrackerId`** acompanha um evento de ponta a ponta — ingest → route → trigger → workflow → storage — então a história completa é um único lookup. As decisões de roteamento são **explicáveis** (cada uma registra por que deu ou não deu match), e as conexões **rejeitadas** também são auditadas, não descartadas em silêncio.

---

## Segurança de transporte

Os transportes externos usam **TLS**: o edge HTTP sobre HTTPS, MQTT sobre TLS/mTLS (porta `8883`), LoRaWAN Basics Station sobre mTLS. A verificação de certificado está **ligada** — a plataforma não entrega nenhum conector ou client que a desabilite — e os conectores outbound carregam seu próprio TLS e timeouts por destino. As chamadas internas serviço-a-serviço rodam em uma rede confiável e são protegidas por API keys.

---

## Seguro por padrão

Os defaults existem para caber em um laptop, não para serem enviados para produção desprotegidos — então a plataforma **se recusa a rodar desprotegida**. Todo serviço Mapex tem um **guard de segurança no boot**: quando `GO_ENV` / `NODE_ENV` não é um valor de desenvolvimento **e** alguma variável sensível (`AUTH_SECRET`, `INTERNAL_API_KEY`, `NATS_PASSWORD`, a master key de credenciais, …) está faltando ou ainda igual ao seu default de dev, o serviço **falha rápido com um erro fatal estridente** em vez de subir com um secret vazado ou fraco. Em modo local, ele imprime um `[SECURITY WARNING]` por serviço, para que a postura de dev nunca seja silenciosa.

---

## Checklist de hardening

O MapexOS é **self-hosted**, então a postura final é sua. Antes da produção:

1. **Gere todo secret.** Substitua todos os defaults de dev — `AUTH_SECRET`, `INTERNAL_API_KEY`, a master key do Vault, senhas de datastore (ex.: `openssl rand -hex 32`). O guard de boot impõe isso; não brigue com ele.
2. **Defina `GO_ENV`/`NODE_ENV=prod`** e aponte os serviços para um arquivo de env real e ignorado pelo git (nunca o commite).
3. **Termine TLS em tudo** que for externo — HTTPS no gateway, mTLS no broker e no LNS — com certificados que você controla.
4. **Rode no Kubernetes** com resource requests/limits reais e network policies que fechem as portas internas para fora.
5. **Tranque os datastores** — MongoDB, ClickHouse, Redis, MinIO, NATS — na rede do cluster, com autenticação habilitada.
6. **Rotacione** os secrets de JWT, as API keys e a master key do Vault em um cronograma, e ajuste a **retenção** por organização às suas necessidades de compliance.
7. **Escope os operadores** com roles de menor privilégio na árvore de organizações, em vez de grants recursivos amplos.

---

## Reportando uma vulnerabilidade

Se você encontrar um problema de segurança, por favor divulgue-o de forma responsável aos mantenedores, em vez de abrir uma issue pública, e dê tempo para uma correção antes de qualquer discussão pública.

---

## Para onde ir em seguida

| Para mergulhar em… | Vá para |
|---|---|
| Identidade, roles e tenancy | [mapexIam](./mapexiam.md) |
| Secrets, chaves e a CA | [Vault](./vault.md) |
| Certificados de device e presence | [Assets](./assets.md) |
| O mapa completo da plataforma | [Visão Geral da Arquitetura](./overview.md) |
