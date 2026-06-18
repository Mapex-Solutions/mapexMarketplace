# Mapex Marketplace

Um único serviço Go que hospeda todos os marketplaces do MapexOS: **plugins de
workflow**, **devices** e (futuramente) **asset templates**. É um servidor de
catálogo sem estado — sem Mongo, Redis ou NATS. O catálogo JSON em `catalog/` é a
fonte da verdade; ao subir, o serviço lê um manifesto leve por fabricante e monta
um **índice de busca SQLite** em processo. Os bundles pesados (fichas de
informação, templates de instalação, codecs, imagens) são lidos do disco sob
demanda apenas quando solicitados, então o serviço escala para catálogos grandes
com um custo de inicialização pequeno e constante.

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
| GET | `/` | Listar + filtrar (`protocol`, `readingType`, `manufacturer`, `search`, `page`, `perPage`) |
| GET | `/facets` | Opções de filtro disponíveis |
| POST | `/refresh` | Reconstruir o índice a partir do disco |
| GET | `/:vendor/:slug` | Ficha de informação do modelo |
| GET | `/:vendor/:slug/simulator` | Template de instalação do modelo |
| GET | `/:vendor/:slug/assets/*` | Asset do bundle (codec, manual, imagem) |

`GET /health` é a sonda de liveness. Todas as respostas JSON usam o envelope
padrão `{ status, errors, data }`.

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
