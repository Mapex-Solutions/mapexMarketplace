# Bounded Context: Devices Marketplace

Last reviewed: 2026-06-15

## Purpose

Serve the **device marketplace**: a read-only catalog of IoT devices that client
apps (the device simulator UI today) browse and filter to add a ready-to-use
device. This context owns the catalog listing and the lazy delivery of each
device's bundle; it does not own the act of installing a device — that belongs to
the consuming app.

## Ubiquitous Language

- **Catalog item** — the searchable metadata for one device model (vendor,
  protocol, reading types, manufacturer, name, tags). The card shown in a listing.
- **Vendor manifest** — `vendors/{vendor}/catalog.json`, one per vendor, listing
  that vendor's models. The unit read at boot to build the index.
- **Bundle** — a model's heavy files, read lazily from disk: `device_information.json`
  (detail sheet), `device_simulator.json` (install template), and assets (codec,
  manual, images).
- **Index** — the in-process SQLite table of catalog items, derived from the
  vendor manifests and rebuilt on boot / on refresh. Disposable; the JSON catalog
  is the source of truth.
- **Catalog config** — the controlled vocabulary (`catalog_config.json`) that
  labels the protocol and reading-type filter facets.

## Driving Ports

- `DevicesServicePort` — list (filter + paginate), facets, get information, get
  simulator, get asset, refresh.

## Driven Ports

- `DeviceCatalogRepository` — query/facets over the index, lazy bundle/asset
  reads from disk, and index rebuild.

## Invariants

- The JSON catalog under `catalog/devices` is the source of truth; the SQLite
  index is always derived and may be rebuilt at any time without data loss.
- Boot reads only the per-vendor `catalog.json` manifests, never every bundle, so
  startup cost scales with the number of vendors, not the number of devices.
- Bundle and asset reads are path-guarded to the requested model folder; vendor
  and slug segments and asset paths cannot traverse outside `catalog/devices`.

## Cross-Context Interactions

- Consumed by the device simulator UI over HTTP (`/api/v1/devices`).
- Shares the marketplace service host with the future `plugins` and
  `asset_templates` modules; each is a thin module over the same catalog pattern
  with its own catalog root and index table.
