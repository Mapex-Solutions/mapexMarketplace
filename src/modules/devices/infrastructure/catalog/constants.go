package catalog

// tableDeviceCatalog is the SQLite index table backing the device marketplace.
const tableDeviceCatalog = "device_catalog"

// ddlDeviceCatalog creates the index table. reading_types and tags are stored as
// comma-wrapped token strings (",temperature,humidity,") so a single LIKE filters
// by membership; scalar columns are queried directly. Rebuilt from JSON on boot.
const ddlDeviceCatalog = `CREATE TABLE IF NOT EXISTS device_catalog (
	id TEXT PRIMARY KEY,
	vendor TEXT NOT NULL,
	vendor_name TEXT NOT NULL,
	model TEXT NOT NULL,
	slug TEXT NOT NULL,
	name_en TEXT NOT NULL,
	name_pt TEXT NOT NULL,
	description_en TEXT NOT NULL,
	description_pt TEXT NOT NULL,
	protocol TEXT NOT NULL,
	reading_types TEXT NOT NULL,
	tags TEXT NOT NULL,
	icon TEXT NOT NULL,
	image TEXT NOT NULL,
	has_codec INTEGER NOT NULL,
	has_manual INTEGER NOT NULL
)`

// dropDeviceCatalog drops the index table so Reload always rebuilds it with the
// current schema. The table is a derived index (rebuilt from JSON every boot), so
// dropping it loses nothing and avoids stale-schema drift from an older DB file
// (e.g. a pre-i18n table without the name_en/name_pt columns).
const dropDeviceCatalog = `DROP TABLE IF EXISTS device_catalog`

// insertDeviceCatalog inserts one index row.
const insertDeviceCatalog = `INSERT INTO device_catalog
	(id, vendor, vendor_name, model, slug, name_en, name_pt, description_en, description_pt, protocol, reading_types, tags, icon, image, has_codec, has_manual)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// selectColumns is the column list returned by listing queries, in scan order.
const selectColumns = "id, vendor, vendor_name, model, slug, name_en, name_pt, description_en, description_pt, protocol, reading_types, tags, icon, image, has_codec, has_manual"

// fileCatalogConfig is the per-marketplace config (facet vocabulary) at the
// devices catalog root.
const fileCatalogConfig = "catalog_config.json"

// Bundle file names within a model folder.
const (
	fileInformation = "device_information.json"
	fileSimulator   = "device_simulator.json"
)

// Codec bundle layout: codecs/{id}/codec.json.
const (
	dirCodecs = "codecs"
	fileCodec = "codec.json"
)
