package catalog

// tableWorkflowPluginCatalog is the SQLite index table backing the plugin marketplace.
const tableWorkflowPluginCatalog = "workflow_plugin_catalog"

// ddlWorkflowPluginCatalog creates the index table. capabilities and tags are stored as
// comma-wrapped token strings (",action,trigger,") so a single LIKE filters by
// membership; scalar columns are queried directly. Rebuilt from JSON on boot.
const ddlWorkflowPluginCatalog = `CREATE TABLE IF NOT EXISTS workflow_plugin_catalog (
	id TEXT PRIMARY KEY,
	vendor TEXT NOT NULL,
	vendor_name TEXT NOT NULL,
	plugin_id TEXT NOT NULL,
	slug TEXT NOT NULL,
	name_en TEXT NOT NULL,
	name_pt TEXT NOT NULL,
	description_en TEXT NOT NULL,
	description_pt TEXT NOT NULL,
	category TEXT NOT NULL,
	capabilities TEXT NOT NULL,
	tags TEXT NOT NULL,
	icon TEXT NOT NULL,
	color TEXT NOT NULL,
	image TEXT NOT NULL,
	requires_credentials INTEGER NOT NULL,
	node_count INTEGER NOT NULL,
	trigger_count INTEGER NOT NULL,
	has_events INTEGER NOT NULL,
	has_image INTEGER NOT NULL
)`

// dropWorkflowPluginCatalog drops the index table so Reload always rebuilds it with the
// current schema. The table is a derived index (rebuilt from JSON every boot), so
// dropping it loses nothing and avoids stale-schema drift from an older DB file.
const dropWorkflowPluginCatalog = `DROP TABLE IF EXISTS workflow_plugin_catalog`

// insertWorkflowPluginCatalog inserts one index row.
const insertWorkflowPluginCatalog = `INSERT INTO workflow_plugin_catalog
	(id, vendor, vendor_name, plugin_id, slug, name_en, name_pt, description_en, description_pt, category, capabilities, tags, icon, color, image, requires_credentials, node_count, trigger_count, has_events, has_image)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// selectColumns is the column list returned by listing queries, in scan order.
const selectColumns = "id, vendor, vendor_name, plugin_id, slug, name_en, name_pt, description_en, description_pt, category, capabilities, tags, icon, color, image, requires_credentials, node_count, trigger_count, has_events, has_image"

// fileCatalogConfig is the per-marketplace config (facet vocabulary) at the
// plugins catalog root.
const fileCatalogConfig = "catalog_config.json"

// Bundle file names within a plugin folder.
const (
	fileInformation = "plugin_information.json"
	fileEvents      = "events.json"
)
