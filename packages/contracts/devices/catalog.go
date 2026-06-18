// Package devices holds the cross-boundary contracts for the device marketplace.
// These shapes are the wire API the browser clients consume; a TS schema
// counterpart mirrors them under the JS workspace (polyglot reciprocity).
package devices

// CatalogItem is one card in the device marketplace listing: the searchable
// metadata, never the heavy bundle (information/simulator/codec live as files).
type CatalogItem struct {
	ID           string   `json:"id"`
	Vendor       string   `json:"vendor"`
	VendorName   string   `json:"vendorName"`
	Model        string   `json:"model"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Protocol     string   `json:"protocol"`
	ReadingTypes []string `json:"readingTypes"`
	Tags         []string `json:"tags"`
	Icon         string   `json:"icon"`
	HasCodec     bool     `json:"hasCodec"`
	HasManual    bool     `json:"hasManual"`
}

// CatalogQuery carries the listing filters and pagination read from the query
// string. Empty filters are ignored; Page is 1-based.
type CatalogQuery struct {
	Protocol     string `json:"protocol"`
	ReadingType  string `json:"readingType"`
	Manufacturer string `json:"manufacturer"`
	Search       string `json:"search"`
	// Lang selects the locale for the resolved card description (e.g. "pt-BR");
	// empty falls back to en-US.
	Lang    string `json:"lang"`
	Page    int    `json:"page"`
	PerPage int    `json:"perPage"`
}

// CatalogListResponse is a single page of catalog items plus the total match
// count for pagination.
type CatalogListResponse struct {
	Items   []CatalogItem `json:"items"`
	Total   int           `json:"total"`
	Page    int           `json:"page"`
	PerPage int           `json:"perPage"`
}

// FacetOption is a selectable filter value with its display label and icon.
type FacetOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Icon  string `json:"icon,omitempty"`
}

// Facets are the available filter options the listing UI renders.
type Facets struct {
	Protocols     []FacetOption `json:"protocols"`
	ReadingTypes  []FacetOption `json:"readingTypes"`
	Manufacturers []FacetOption `json:"manufacturers"`
}

// Codec describes one payload codec available for a model. The same device may
// expose several (vendor-official + community ports for different platforms); the
// client lists them and lets the user pick the one matching their network server.
type Codec struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Official    bool   `json:"official"`
	Default     bool   `json:"default"`
	Target      string `json:"target"`
	Language    string `json:"language"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	Path        string `json:"path"`
	DecoderFile string `json:"decoderFile,omitempty"`
	EncoderFile string `json:"encoderFile,omitempty"`
}
