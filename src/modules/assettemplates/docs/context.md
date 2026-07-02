# Bounded Context: Asset Templates Marketplace

Last reviewed: 2026-06-29

## Purpose

Serve the **asset template marketplace**: a read-only catalog of asset templates
(vendor integrations such as Disruptive Technologies sensors) that client apps
browse and filter to provision a ready-to-use asset model. This context owns the
catalog listing and the lazy delivery of each template's bundle; it does not own
the act of applying a template — that belongs to the consuming app.

## Ubiquitous Language

- **Catalog item** — the searchable metadata for one asset template (vendor,
  model, version, category, name, field count). The card shown in a listing.
- **Vendor manifest** — `vendors/{vendor}/catalog.json`, one per vendor, listing
  that vendor's templates. The unit read at boot to build the index.
- **Bundle** — a template's heavy file, read lazily from disk:
  `asset_template_information.json` (the full template), plus optional assets
  (icon, image).
- **Drill-down** — the three-level facet cascade vendor -> model -> version: each
  level narrows the one below it.
- **Index** — the in-process SQLite table (`asset_template_catalog`) of catalog
  items, derived from the vendor manifests and rebuilt on boot. Disposable; the
  JSON catalog is the source of truth.
- **Catalog config** — the controlled vocabulary (`catalog_config.json`) that
  labels the category facet. Each category label is bilingual (en-US, pt-BR),
  resolved by the request's lang.

## Driving Ports

- `AssetTemplatesServicePort` — list (filter + paginate), facets (drill-down +
  localized categories), get information, get asset.

## Driven Ports

- `AssetTemplateCatalogRepository` — query/facets over the index, lazy
  bundle/asset reads from disk, and index rebuild.

## Invariants

- The JSON catalog under `catalog/asset_templates` is the source of truth; the
  SQLite index is always derived and may be rebuilt at any time without data loss.
- Boot reads only the per-vendor `catalog.json` manifests, never every bundle, so
  startup cost scales with the number of vendors, not the number of templates.
- The drill-down facets are contextual: models are narrowed by the selected
  vendor, versions by the selected model (and vendor).
- Bundle and asset reads are path-guarded to the requested template folder; vendor
  and slug segments and asset paths cannot traverse outside `catalog/asset_templates`.

## Cross-Context Interactions

- Consumed by client apps over HTTP (`/api/v1/asset_templates`).
- Shares the marketplace service host with the `devices` and `workflowplugins`
  modules; each is a thin module over the same catalog pattern with its own
  catalog root and index table.
