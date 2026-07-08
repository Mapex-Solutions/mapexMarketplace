# Bounded Context: Workflow Plugins Marketplace

Last reviewed: 2026-07-08

## Purpose

Serve the **workflow plugin marketplace**: a read-only catalog of workflow
plugins (Telegram, Slack, Discord, ...) that client apps browse and filter to add
a ready-to-use integration. This context owns the catalog listing and the lazy
delivery of each plugin's bundle; it does not own the act of installing a plugin —
that belongs to the consuming app.

## Ubiquitous Language

- **Catalog item** — the searchable metadata for one plugin (vendor, category,
  capabilities, name, tags, node/trigger counts). The card shown in a listing.
- **Vendor manifest** — `vendors/{vendor}/catalog.json`, one per vendor, listing
  that vendor's plugins. The unit read at boot to build the index.
- **Bundle** — a plugin's heavy files, read lazily from disk: `plugin_information.json`
  (detail manifest), `events.json` (optional trigger catalog), and assets (icon).
- **Capability** — what a plugin can do in a workflow: `action` and/or `trigger`.
- **Index** — the in-process SQLite table (`workflow_plugin_catalog`) of catalog
  items, derived from the vendor manifests and rebuilt on boot. Disposable; the
  JSON catalog is the source of truth.
- **Catalog config** — the controlled vocabulary (`catalog_config.json`) that
  labels the category and capability filter facets.

## Driving Ports

- `WorkflowPluginsServicePort` — list (filter + paginate), facets, get
  information, get events, get asset.

## Driven Ports

- `WorkflowPluginCatalogRepository` — query/facets over the index, lazy
  bundle/asset reads from disk, and index rebuild.

## Invariants

- The JSON catalog under `catalog/workflow_plugins` is the source of truth; the
  SQLite index is always derived and may be rebuilt at any time without data loss.
- Boot reads only the per-vendor `catalog.json` manifests, never every bundle, so
  startup cost scales with the number of vendors, not the number of plugins.
- Bundle and asset reads are path-guarded to the requested plugin folder; vendor
  and slug segments and asset paths cannot traverse outside `catalog/workflow_plugins`.

## Cross-Context Interactions

- Consumed by client apps over HTTP (`/api/v1/workflow_plugins`).
- Shares the marketplace service host with the `devices` module; each is a thin
  module over the same catalog pattern with its own catalog root and index table.
