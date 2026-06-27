package catalog

import "database/sql"

// adapter implements the plugin catalog repository over the SQLite index (for
// search) plus the JSON catalog directory (for the heavy bundles, read lazily).
type adapter struct {
	db     *sql.DB
	dir    string
	config catalogConfig
}

// vendorCatalog is one vendor's boot-time manifest (vendors/{vendor}/catalog.json):
// the vendor identity plus the searchable metadata for each of its plugins.
type vendorCatalog struct {
	Vendor vendorInfo     `json:"vendor"`
	Items  []manifestItem `json:"items"`
}

// vendorInfo is the vendor identity shared by all the vendor's plugins.
type vendorInfo struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Site    string `json:"site"`
	Support string `json:"support"`
}

// manifestItem is one plugin's metadata as authored in the vendor manifest.
type manifestItem struct {
	ID                  string        `json:"id"`
	PluginID            string        `json:"pluginId"`
	Slug                string        `json:"slug"`
	Name                localizedText `json:"name"`
	Description         localizedText `json:"description"`
	Category            string        `json:"category"`
	Capabilities        []string      `json:"capabilities"`
	Tags                []string      `json:"tags"`
	Icon                string        `json:"icon"`
	Color               string        `json:"color"`
	Image               string        `json:"image"`
	RequiresCredentials bool          `json:"requiresCredentials"`
	NodeCount           int           `json:"nodeCount"`
	TriggerCount        int           `json:"triggerCount"`
	HasEvents           bool          `json:"hasEvents"`
	HasImage            bool          `json:"hasImage"`
}

// catalogConfig is the per-marketplace config (catalog_config.json): the
// controlled vocabulary that labels the category and capability filter facets.
type catalogConfig struct {
	Categories   []facetOption `json:"categories"`
	Capabilities []facetOption `json:"capabilities"`
}

// facetOption is one facet value with its display label and icon.
type facetOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}
