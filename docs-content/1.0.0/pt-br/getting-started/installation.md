---
title: Instalação
description: Suba toda a stack do MapexOS em uma única máquina com um só comando — pré-requisitos, acesso, o que fica rodando e o caminho até produção.
---

# Instalação

A plataforma inteira — cada serviço e sua infraestrutura — sobe a partir de uma única distribuição em Docker Compose: [`mapexOSDeploy`](https://github.com/Mapex-Solutions/mapexOSDeploy). Imagens multi-arch pré-buildadas, um comando.

## Pré-requisitos

- **Docker** e **Docker Compose v2**.
- Um host com aproximadamente **4 cores / 8 GB de RAM**. A stack é ajustada para caber em um notebook (veja [Pegada de recursos](#pegada-de-recursos)).
- Linux, macOS (Apple Silicon) ou Windows + Docker Desktop. As imagens são publicadas para **`linux/amd64`** e **`linux/arm64`**, então a mesma stack roda em um Raspberry Pi 4/5.

## Subir a stack

```bash
git clone https://github.com/Mapex-Solutions/mapexOSDeploy.git
cd mapexOSDeploy
docker compose up -d
```

Aguarde **~2 minutos** pela inicialização do primeiro boot — o replica-set do Mongo, os streams do NATS, os buckets do MinIO e as tabelas do ClickHouse são provisionados automaticamente.

## Acesso

| Superfície | URL | Credenciais |
|---|---|---|
| **Frontend** | <http://localhost> | `admin@mapex.local` / `mapex@123` *(troque no primeiro login)* |
| **Grafana** | <http://localhost:3001> | `admin` / `admin` — 8 dashboards pré-carregados |
| **Console do MinIO** | <http://localhost:9001> | `mapex_admin` / `mapex_admin_secret_change_me` |

Para servir a UI a outras máquinas da sua rede, defina o host público:

```bash
MAPEXOS_PUBLIC_HOST=192.168.0.50 docker compose up -d
```

> Uma linha vermelha de `[SECURITY WARNING]` por serviço nos logs é **esperada** no modo local — ela está apenas avisando que a stack está rodando com os defaults de dev. Veja [Indo para produção](#indo-para-produção) para endurecer a configuração.

## O que fica rodando

**Serviços da aplicação** (código-fonte em [`mapexOS`](https://github.com/Mapex-Solutions/mapexOS)):

| Serviço | Porta | Propósito |
|---|---|---|
| Frontend | 80 | Console Vue 3 + Quasar |
| mapex-iam | 5000 | Users, organizations, roles, auth |
| http-gateway | 5001 | Ingestão de webhooks, registry de datasources |
| assets | 5002 | Assets, templates, campos EVA, PKI de dispositivos + presence |
| router | 5003 | Roteamento de eventos, regras de match, fan-out |
| events | 5004 | Armazenamento no ClickHouse, retenção |
| triggers | 5006 | Executors de saída (HTTP, MQTT, NATS, …) |
| workflow | 5007 | Workflow Engine durável de DAGs + plugins |
| mapex-vault | 5010 | Vault de credenciais, autoridade de PKI |
| js-executor | 8000 | Isolates V8 para scripts de asset |
| js-wf-executor | 8001 | Isolates V8 para nodes de code de workflow |

**Infraestrutura:**

| Serviço | Porta | Imagem |
|---|---|---|
| MongoDB | 27017 | `mongo:7.0.34` (replica-set `rs0`) |
| Redis | 6379 | `redis:7.4-alpine` |
| ClickHouse | 8123 / 9440 | `clickhouse/clickhouse-server` |
| MinIO | 9000 / 9001 | `minio/minio` |
| NATS | 4222 / 8222 | `nats:2-alpine` (JetStream) |
| MQTT Broker | 1883 | `mapexos/mapex-broker-mqtt` |
| Prometheus | 9090 | `prom/prometheus` |
| Grafana | 3001 | `grafana/grafana` |

## Pegada de recursos

Todo serviço de longa duração traz um bloco `deploy.resources.limits` **dimensionado para caber em um único notebook de ~4 cores / 8 GB** — os limites somam aproximadamente **10,5 vCPU / ~5 GB de RAM**. Esses são **tetos, não reservas**: o Docker nunca reserva os cores ou a RAM, então os totais podem ultrapassar o host e a stack continua rodando tranquilamente — o scheduler compartilha os cores reais no tempo e containers ociosos não custam nada. MongoDB e ClickHouse ainda são ajustados internamente (`--wiredTigerCacheSizeGB`, `max_server_memory_usage`) para que seus caches respeitem o limite do container. Os one-shots de init são deixados sem limite de propósito.

## Parar e resetar

```bash
docker compose down       # stop containers, keep data
docker compose down -v    # stop + drop volumes (full reset)
```

## Indo para produção

Os defaults existem para que a stack caiba em um notebook — **não** para que ela aguente carga. Para produção:

1. Crie um arquivo de env de verdade: `cp infra/envs/production.example.env infra/envs/production.env` e então preencha todos os valores (o template inclui geradores, ex.: `openssl rand -hex 32` para o `AUTH_SECRET`).
2. Aponte os arquivos de compose para o `production.env` e defina `GO_ENV=prod`.
3. Rode em um **cluster Kubernetes**, onde você dimensiona `resources.requests`/`limits` (e HPA/VPA) à demanda real — dê a MongoDB/ClickHouse/Prometheus uma folga de verdade.

Todo serviço Mapex tem um **security guard** que se recusa a subir quando `GO_ENV`/`NODE_ENV` não é um valor de dev **e** alguma variável sensível (`AUTH_SECRET`, `INTERNAL_API_KEY`, `NATS_PASSWORD`, …) está faltando ou ainda é igual ao seu default de dev — um erro fatal e barulhento em vez de um boot silencioso com um segredo vazado.

> O `infra/envs/production.env` é ignorado pelo git. Nunca faça commit dele.

## Próximo

→ [Quickstart](./quickstart.md) — conecte seu primeiro sensor de ponta a ponta.
