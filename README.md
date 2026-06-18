# Mapex Marketplace

A single Go service that hosts every MapexOS marketplace: **workflow plugins**,
**devices**, and (later) **asset templates**. It is a stateless catalog server —
no Mongo, Redis, or NATS. The JSON catalog under `catalog/` is the source of
truth; on boot the service reads one lightweight manifest per vendor and builds an
in-process **SQLite search index**. Heavy bundles (information sheets, install
templates, codecs, images) are read lazily from disk only when requested, so the
service scales to large catalogs with a small, constant startup cost.

## Architecture

Go + Fiber, DDD + Hexagonal per the Mapex `/go-arch` standard. Each marketplace
is a thin module over the same catalog primitive:

```
src/
  main.go
  bootstrap/        config · fiber · health · catalog (SQLite index) · shutdown
  shared/configuration
  modules/
    app/            module init loop (repositories → services → interfaces)
    devices/        domain · application · infrastructure/catalog · interfaces/http
packages/contracts/ wire DTOs (TS schema counterpart mirrors these)
catalog/            the JSON source of truth
  devices/
    catalog_config.json
    vendors/{vendor}/catalog.json          # read at boot → index
    vendors/{vendor}/{model}/              # bundles, served lazily
```

## Devices API

Base path `/api/v1/devices`:

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/` | List + filter (`protocol`, `readingType`, `manufacturer`, `search`, `page`, `perPage`) |
| GET | `/facets` | Available filter options |
| POST | `/refresh` | Rebuild the index from disk |
| GET | `/:vendor/:slug` | Model information sheet |
| GET | `/:vendor/:slug/simulator` | Model install template |
| GET | `/:vendor/:slug/assets/*` | Bundle asset (codec, manual, image) |

`GET /health` is the liveness probe. All JSON responses use the standard
`{ status, errors, data }` envelope.

## Run locally

```bash
go build -o bin/marketplace ./src
./bin/marketplace            # serves http://127.0.0.1:6060, reads ./catalog
```

Configuration is via env vars (see `src/shared/configuration/application/config.go`):
`HTTP_PORT` (6060), `CATALOG_DIR` (`./catalog`), `CATALOG_INDEX_PATH`
(`./data/catalog-index.db`), `CORS_ORIGINS` (`*`).

## Dependencies

`mapexGoKit` is consumed from the sibling checkout via `replace` directives
(`../mapexGoKit/*`), matching the other Mapex Go services.

---

🇧🇷 Versão em português: [README_pt.md](README_pt.md)
