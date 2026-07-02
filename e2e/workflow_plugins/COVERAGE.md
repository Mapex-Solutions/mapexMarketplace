# E2E Test Coverage: Workflow Plugins Catalog

**Suite:** `workflow_plugins.catalog`
**Service:** mapexMarketplace workflow plugins catalog — `GET /api/v1/workflow_plugins` (public, read-only)
**Run:** `npm run test:e2e -- -g "workflow_plugins.catalog"`

---

## Tests

| # | Test | Category | What it covers |
|---|------|----------|----------------|
| 1 | `list: returns a paginated page in the envelope shape` | Listing | `{status,errors,data:{items,total,page,perPage}}`; `perPage` honoured; non-empty catalog. |
| 2 | `list.filter.category: every item matches the category` | Filter | `?category=<x>` → every returned plugin is in that category. |
| 3 | `list.filter.capability: every item carries the capability` | Filter | `?capability=<x>` → every plugin's `capabilities` includes it. |
| 4 | `facets: exposes flat categories and capabilities (no drill-down)` | Facets | `categories` + `capabilities` present and non-empty. Plugins facets are **orthogonal** — no hierarchy. |
| 5 | `detail: information returns the plugin manifest` | Detail | `GET /:vendor/:slug` returns a non-empty manifest JSON. |
| 6 | `events: a plugin with events returns a non-empty catalog` | Detail | Finds a plugin with `hasEvents=true` (telegram today) and asserts `GET /:vendor/:slug/events` is non-empty. |
| 7 | `detail.notFound: unknown plugin returns 404` | Detail | Unknown vendor/slug → 404. |

Unlike `devices`, the plugins catalog has **no drill-down** — `category` and `capability` are
orthogonal flat filters (a plugin has one category and several capabilities; neither contains the other).

---

## Run Commands

```bash
npm run test:e2e -- -g "workflow_plugins.catalog"
npm run test:e2e -- -g "events"
```

---

## File Structure

```
e2e/workflow_plugins/
  plugins-catalog.spec.ts       # test cases (7 tests)
  plugins-catalog.resource.ts   # API resource (page-object analog)
  plugins-catalog.data.ts       # wire types + helpers
  COVERAGE.md                   # this file
```

---

## API Resource: `PluginsCatalogResource`

| Method | Endpoint | Description |
|--------|----------|-------------|
| `list(query)` | `GET /api/v1/workflow_plugins` | Paginated plugin cards; unwraps `.data`. |
| `facets()` | `GET /api/v1/workflow_plugins/facets` | Flat filter options (categories, capabilities). |
| `information(vendor, slug)` | `GET /api/v1/workflow_plugins/:vendor/:slug` | Raw plugin manifest. |
| `events(vendor, slug)` | `GET /api/v1/workflow_plugins/:vendor/:slug/events` | Trigger-events catalog. |
| `getRaw(path)` | `GET /api/v1/workflow_plugins{path}` | Unparsed response for status/negative assertions. |

---

## Known Limitations

- Assertions are **relative** (subset/non-empty), so the suite stays green as the catalog grows.
- The `/:vendor/:slug/assets/*` (icon/image) endpoint is not asserted yet.
- Only `telegram` currently ships an events catalog; test 6 finds the first plugin with
  `hasEvents=true` rather than hard-coding it.
```
