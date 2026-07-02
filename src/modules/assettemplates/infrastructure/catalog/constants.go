package catalog

// tableAssetTemplateCatalog is the SQLite index table backing the asset template
// marketplace.
const tableAssetTemplateCatalog = "asset_template_catalog"

// ddlAssetTemplateCatalog creates the index table. Scalar columns are queried
// directly. Rebuilt from JSON on boot.
const ddlAssetTemplateCatalog = `CREATE TABLE IF NOT EXISTS asset_template_catalog (
	id TEXT PRIMARY KEY,
	marketplace_guid TEXT NOT NULL,
	vendor TEXT NOT NULL,
	vendor_name TEXT NOT NULL,
	model TEXT NOT NULL,
	version TEXT NOT NULL,
	slug TEXT NOT NULL,
	name_en TEXT NOT NULL,
	name_pt TEXT NOT NULL,
	description_en TEXT NOT NULL,
	description_pt TEXT NOT NULL,
	category TEXT NOT NULL,
	icon TEXT NOT NULL,
	image TEXT NOT NULL,
	has_image INTEGER NOT NULL,
	field_count INTEGER NOT NULL,
	has_scripts INTEGER NOT NULL,
	sha256 TEXT NOT NULL
)`

// dropAssetTemplateCatalog drops the index table so Reload always rebuilds it
// with the current schema. The table is a derived index (rebuilt from JSON every
// boot), so dropping it loses nothing and avoids stale-schema drift from an older
// DB file.
const dropAssetTemplateCatalog = `DROP TABLE IF EXISTS asset_template_catalog`

// insertAssetTemplateCatalog inserts one index row.
const insertAssetTemplateCatalog = `INSERT INTO asset_template_catalog
	(id, marketplace_guid, vendor, vendor_name, model, version, slug, name_en, name_pt, description_en, description_pt, category, icon, image, has_image, field_count, has_scripts, sha256)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// selectColumns is the column list returned by listing queries, in scan order.
const selectColumns = "id, marketplace_guid, vendor, vendor_name, model, version, slug, name_en, name_pt, description_en, description_pt, category, icon, image, has_image, field_count, has_scripts, sha256"

// fileCatalogConfig is the per-marketplace config (facet vocabulary) at the
// asset templates catalog root.
const fileCatalogConfig = "catalog_config.json"

// fileInformation is the heavy detail bundle within an asset template folder.
const fileInformation = "asset_template_information.json"
