# E2E Test Coverage: Devices Catalog

**Suite:** `devices.catalog`
**Service:** mapexMarketplace device catalog — `GET /api/v1/devices` (public, read-only)
**Run:** `npm run test:e2e -- -g "devices.catalog"`

---

## Tests

| # | Test | Category | What it covers |
|---|------|----------|----------------|
| 1 | `list: returns a paginated page in the envelope shape` | Listing | `{status,errors,data:{items,total,page,perPage}}`; `perPage` honoured; non-empty catalog. |
| 2 | `list.pagination: page 2 returns different items than page 1` | Listing | Page 2 has no id overlap with page 1. |
| 3 | `list.filter.protocol: every item matches the protocol` | Filter | `?protocol=lorawan` → every returned item is LoRaWAN. |
| 4 | `list.filter.manufacturer: every item belongs to the vendor` | Filter | `?manufacturer=<vendor>` → results scoped to that vendor. |
| 5 | `facets: exposes the flat and drill-down filter options` | Facets | `protocols`, `readingTypes`, `manufacturers`, `models` all present and non-empty. |
| 6 | `facets.drilldown: models narrow to the picked manufacturer` | Drill-down | `?manufacturer=<vendor>` narrows `models` to a strict subset; `manufacturers`/`protocols` stay unchanged (orthogonal). |
| 7 | `facets.drilldown: narrowed models match the vendor listing` | Drill-down | Every model in `list?manufacturer=<vendor>` appears in the vendor's `facets.models` — ties facets to real listing. |
| 8 | `facets.drilldown: unknown manufacturer yields no models` | Drill-down | A non-existent vendor returns zero models (no empty-combo leakage). |
| 9 | `detail: information returns the model sheet` | Detail | `GET /:vendor/:slug` returns a non-empty information JSON. |
| 10 | `detail.notFound: unknown model returns 404` | Detail | Unknown vendor/slug → 404. |

The drill-down tests (6–8) are the regression guard for the `vendor → model` facet cascade.

---

## Run Commands

```bash
# install once (downloads Playwright)
npm install

# all marketplace e2e (boots the Go service via webServer, or reuses a running one)
npm run test:e2e

# just the catalog suite / a category
npm run test:e2e -- -g "devices.catalog"
npm run test:e2e -- -g "facets.drilldown"

# point at an already-running service on another port
MARKETPLACE_PORT=6066 npm run test:e2e

# open the HTML report
npm run test:e2e:report
```

The `webServer` block in `playwright.config.ts` runs `go run ./src/main.go` from the repo
root with `CATALOG_DIR=./catalog`. `reuseExistingServer` reuses a local instance if one is
already listening, so you can also start it yourself:
`HTTP_PORT=6060 CATALOG_DIR=./catalog GO_ENV=dev go run ./src/main.go`.

---

## File Structure

```
e2e/
  playwright.config.ts            # API-mode config; boots the Go service
  package.json                    # @playwright/test
  tsconfig.json
  fixtures/
    catalog.fixture.ts            # provides the `catalog` API resource (public, no auth)
  devices/
    devices-catalog.spec.ts       # test cases (10 tests)
    devices-catalog.resource.ts   # API resource (page-object analog)
    devices-catalog.data.ts       # wire types + helpers
    COVERAGE.md                   # this file
```

---

## API Resource: `DevicesCatalogResource`

| Method | Endpoint | Description |
|--------|----------|-------------|
| `list(query)` | `GET /api/v1/devices` | Paginated catalog cards; unwraps `.data`. |
| `facets(query)` | `GET /api/v1/devices/facets` | Filter options; `manufacturer` narrows `models`. |
| `information(vendor, slug)` | `GET /api/v1/devices/:vendor/:slug` | Raw model information sheet. |
| `getRaw(path)` | `GET /api/v1/devices{path}` | Unparsed response for status/negative assertions. |

---

## Known Limitations

- Assertions are **relative** (subset/shrinks/non-empty), not exact counts, so the suite
  stays green as the committed catalog grows.
- `simulator` and `codecs` detail endpoints are not yet asserted — a future test can add
  `GET /:vendor/:slug/simulator` and `/:vendor/:slug/codecs`.
- The suite targets the **devices** catalog. When `workflow_plugins` and `asset_templates`
  catalogs are wired, mirror this folder (`e2e/workflow_plugins/`, `e2e/asset_templates/`)
  with the same resource/spec/data/COVERAGE shape — the `asset_templates` suite gets the
  `vendor → model → version` drill-down.
```
