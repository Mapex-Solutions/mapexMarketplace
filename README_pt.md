# Mapex Marketplace

> Um catálogo de dispositivos IoT reais — datasheet, manual, codec de payload e um
> perfil de simulador pronto pra rodar — pra que qualquer projeto MapexOS possa
> navegar, instalar e simular um dispositivo **sem precisar comprar o hardware**.

## Por que construímos

Construir e testar uma plataforma IoT geralmente começa com uma ordem de compra.
Você quer ver como sua stack se comporta com uma sonda de solo Dragino, um contador
de pessoas Milesight, um sensor de pressão NB-IoT — então compra um, espera o
frete, grava as chaves, sobe um gateway e sai garimpando nas wikis dos fabricantes
o datasheet, o manual do usuário e o decoder de payload que realmente bate com o
seu network server. Multiplique isso por cada modelo que você quer suportar e o
"vamos testar esse device" vira semanas e uma linha no orçamento. Pior: os links de
documentação apodrecem — o manual que você salvou vira um 404 um ano depois.

Queríamos o oposto. Abrir um catálogo, escolher um dispositivo e tê-lo se
comportando como o real em segundos — emitindo payloads com o **layout de bytes
real que o codec oficial dele espera** — com datasheet, manual e decoder logo ao
lado, coletados e versionados pra nunca sumirem.

É isso o Mapex Marketplace: um único serviço de catálogo, sem estado, que troca
"comprar hardware pra testar" por "navegar e simular".

## O que vem em cada dispositivo

- **Ficha de informação** — modelo, protocolo (LoRaWAN / NB-IoT / …), tipos de
  leitura, fabricante e uma descrição localizada (en-US / pt-BR).
- **Perfil de simulador** — uma definição de dispositivo pronta pra instalar cujos
  eventos emitem payloads com a estrutura real, **validada contra o decoder oficial
  do fabricante** — então o que você simula é o que um nó real enviaria.
- **Codecs** — os decoders de payload oficiais (TTN + ChirpStack) que acompanham o
  dispositivo, pra você plugar direto no seu network server.
- **Documentos** — datasheet e manual do usuário como **PDFs locais** (além do link
  online original), pra o catálogo sobreviver à queda do site do fabricante.

## Usado por todo o MapexOS

O marketplace é infraestrutura compartilhada do ecossistema MapexOS — um catálogo,
vários consumidores:

- **[Mapex Devices Simulator](https://github.com/Mapex-Solutions/mapexDevicesSimulator)**
  — navega neste catálogo, instala um dispositivo e simula dados e payloads reais
  via HTTP / MQTT / LoRaWAN sem ter um único sensor; datasheet, manual e codec ficam
  a um clique.
- **[MapexOS](https://github.com/Mapex-Solutions/mapexOS)** — a plataforma
  enterprise de IoT open source onde esses dispositivos rodam de fato.
- Foi feito pra hospedar **todos** os marketplaces do MapexOS a partir da mesma
  primitiva: plugins de workflow, devices hoje, asset templates a seguir.

## Veja em ação

Algumas imagens do [Mapex Devices Simulator](https://github.com/Mapex-Solutions/mapexDevicesSimulator)
consumindo este catálogo — conjunto completo em [`images/`](images).

**Navegar no catálogo** — filtre por protocolo, tipo de leitura ou fabricante.
![Catálogo do marketplace](images/marketplace-01.png)

**Detalhe do dispositivo** — visão geral com descrição, tags de tipo de leitura e o link do fabricante.
![Visão geral do dispositivo](images/marketplace-03.png)

**Codecs** — os decoders oficiais ChirpStack v4 e TTN que acompanham o dispositivo.
![Codecs do dispositivo](images/marketplace-04.png)

**Arquivos** — datasheet e manual do usuário, a um clique.
![Arquivos do dispositivo](images/marketplace-05.png)

**Dispositivo instalado** — configure os eventos e um agendamento automático.
![Editar eventos do dispositivo](images/marketplace-07.png)

**Console** — dispare eventos e veja as mensagens HTTP / MQTT / LoRaWAN ao vivo com payloads reais.
![Console ao vivo](images/marketplace-02.png)

**Logs & Eventos** — o histórico persistido de cada mensagem.
![Logs e eventos](images/marketplace-06.png)

## Como funciona

Um único serviço Go hospeda todos os marketplaces. **Sem Mongo, Redis ou NATS.**
O catálogo JSON em `catalog/` é a fonte da verdade; ao subir, o serviço lê um
manifesto leve por fabricante e monta um **índice de busca SQLite** em processo. Os
bundles pesados (fichas de informação, templates de instalação, codecs, imagens)
são lidos do disco sob demanda apenas quando solicitados, então o serviço escala
para catálogos grandes com um custo de inicialização pequeno e constante.

## Arquitetura

Go + Fiber, DDD + Hexagonal conforme o padrão `/go-arch` da Mapex. Cada
marketplace é um módulo fino sobre a mesma primitiva de catálogo:

```
src/
  main.go
  bootstrap/        config · fiber · health · catalog (índice SQLite) · shutdown
  shared/configuration
  modules/
    app/            laço de init dos módulos (repositories → services → interfaces)
    devices/        domain · application · infrastructure/catalog · interfaces/http
packages/contracts/ DTOs de contrato (o schema TS espelha estes)
catalog/            a fonte da verdade em JSON
  devices/
    catalog_config.json
    vendors/{vendor}/catalog.json          # lido no boot → índice
    vendors/{vendor}/{model}/              # bundles, servidos sob demanda
```

## API de Devices

Caminho base `/api/v1/devices`:

| Método | Caminho | Função |
|--------|---------|--------|
| GET | `/` | Listar + filtrar (`protocol`, `readingType`, `manufacturer`, `search`, `lang`, `page`, `perPage`) |
| GET | `/facets` | Opções de filtro disponíveis |
| POST | `/refresh` | Reconstruir o índice a partir do disco |
| GET | `/:vendor/:slug` | Ficha de informação do modelo |
| GET | `/:vendor/:slug/simulator` | Template de instalação do modelo |
| GET | `/:vendor/:slug/assets/*` | Asset do bundle (codec, manual, imagem) |

`GET /health` é a sonda de liveness. Todas as respostas JSON usam o envelope
padrão `{ status, errors, data }`. O texto dos cards é localizado no servidor via a
query `lang` (ex.: `?lang=pt-BR`); a ficha de informação carrega todos os idiomas e
o cliente escolhe um.

## Executar localmente

```bash
go build -o bin/marketplace ./src
./bin/marketplace            # serve em http://127.0.0.1:6060, lê ./catalog
```

Configuração via variáveis de ambiente (ver
`src/shared/configuration/application/config.go`): `HTTP_PORT` (6060),
`CATALOG_DIR` (`./catalog`), `CATALOG_INDEX_PATH` (`./data/catalog-index.db`),
`CORS_ORIGINS` (`*`).

## Dependências

O `mapexGoKit` é consumido do checkout irmão via diretivas `replace`
(`../mapexGoKit/*`), igual aos demais serviços Go da Mapex.

---

🇺🇸 English version: [README.md](README.md)
