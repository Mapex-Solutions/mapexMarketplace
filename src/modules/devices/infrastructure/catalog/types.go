package catalog

import "database/sql"

// adapter implements the device catalog repository over the SQLite index (for
// search) plus the JSON catalog directory (for the heavy bundles, read lazily).
type adapter struct {
	db     *sql.DB
	dir    string
	config catalogConfig
}

// vendorCatalog is one vendor's boot-time manifest (vendors/{vendor}/catalog.json):
// the vendor identity plus the searchable metadata for each of its models.
type vendorCatalog struct {
	Vendor vendorInfo     `json:"vendor"`
	Items  []manifestItem `json:"items"`
}

// vendorInfo is the vendor identity shared by all the vendor's models.
type vendorInfo struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Site    string `json:"site"`
	Support string `json:"support"`
}

// manifestItem is one model's metadata as authored in the vendor manifest.
type manifestItem struct {
	ID           string        `json:"id"`
	Model        string        `json:"model"`
	Slug         string        `json:"slug"`
	Name         localizedText `json:"name"`
	Description  localizedText `json:"description"`
	Protocol     string        `json:"protocol"`
	ReadingTypes []string      `json:"readingTypes"`
	Tags         []string      `json:"tags"`
	Icon         string        `json:"icon"`
	HasCodec     bool          `json:"hasCodec"`
	HasManual    bool          `json:"hasManual"`
}

// catalogConfig is the per-marketplace config (catalog_config.json): the
// controlled vocabulary that labels the protocol and reading-type filter facets.
type catalogConfig struct {
	Protocols    []facetOption `json:"protocols"`
	ReadingTypes []facetOption `json:"readingTypes"`
}

// facetOption is one facet value with its display label and icon.
type facetOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Icon  string `json:"icon"`
}

// codecManifest mirrors a codec.json file inside a codecs/{id}/ folder.
type codecManifest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Official  bool   `json:"official"`
	Default   bool   `json:"default"`
	Target    string `json:"target"`
	Language  string `json:"language"`
	SourceURL string `json:"sourceUrl"`
	Files     struct {
		Decoder string `json:"decoder"`
		Encoder string `json:"encoder"`
	} `json:"files"`
}
