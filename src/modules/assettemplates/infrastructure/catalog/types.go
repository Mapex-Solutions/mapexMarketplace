package catalog

import "database/sql"

// adapter implements the asset template catalog repository over the SQLite index
// (for search) plus the JSON catalog directory (for the heavy bundles, read
// lazily).
type adapter struct {
	db     *sql.DB
	dir    string
	config catalogConfig
}

// vendorCatalog is one vendor's boot-time manifest (vendors/{vendor}/catalog.json):
// the vendor identity plus the searchable metadata for each of its templates.
type vendorCatalog struct {
	Vendor vendorInfo     `json:"vendor"`
	Items  []manifestItem `json:"items"`
}

// vendorInfo is the vendor identity shared by all the vendor's templates.
type vendorInfo struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Site    string `json:"site"`
	Support string `json:"support"`
}

// manifestItem is one asset template's metadata as authored in the vendor
// manifest.
type manifestItem struct {
	ID              string        `json:"id"`
	MarketplaceGuid string        `json:"marketplaceGuid"`
	Slug            string        `json:"slug"`
	Name            localizedText `json:"name"`
	Description     localizedText `json:"description"`
	Category        string        `json:"category"`
	Model           string        `json:"model"`
	Version         string        `json:"version"`
	Icon            string        `json:"icon"`
	Image           string        `json:"image"`
	HasImage        bool          `json:"hasImage"`
	FieldCount      int           `json:"fieldCount"`
	HasScripts      bool          `json:"hasScripts"`
	Sha256          string        `json:"sha256"`
}

// catalogConfig is the per-marketplace config (catalog_config.json): the
// controlled vocabulary that labels the category facet. Each category label is a
// bilingual map resolved by the request's lang.
type catalogConfig struct {
	Categories []categoryOption `json:"categories"`
}

// categoryOption is one category facet value with its bilingual label and icon.
type categoryOption struct {
	Value string        `json:"value"`
	Label localizedText `json:"label"`
	Icon  string        `json:"icon"`
}
