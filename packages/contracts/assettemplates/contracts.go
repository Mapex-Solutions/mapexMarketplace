// Package assettemplates holds the cross-boundary contracts for the asset
// template marketplace. These shapes are the wire API the browser clients
// consume; a TS schema counterpart mirrors them under the JS workspace when one
// exists (polyglot reciprocity).
package assettemplates

// CatalogItem is one card in the asset template marketplace listing: the
// searchable metadata, never the heavy bundle (asset_template_information lives
// as a file). Name and Description are already resolved to the request's locale.
type CatalogItem struct {
	ID              string `json:"id"`
	MarketplaceGuid string `json:"marketplaceGuid"`
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	Vendor          string `json:"vendor"`
	VendorName      string `json:"vendorName"`
	Model           string `json:"model"`
	Version         string `json:"version"`
	Icon            string `json:"icon"`
	// Image is the template image's bundle-relative path; the client builds the
	// asset URL from vendor+slug. Empty when none ships.
	Image      string `json:"image"`
	HasImage   bool   `json:"hasImage"`
	FieldCount int    `json:"fieldCount"`
	HasScripts bool   `json:"hasScripts"`
	Sha256     string `json:"sha256"`
}

// CatalogQuery carries the listing filters and pagination read from the query
// string. Empty filters are ignored; Page is 1-based.
type CatalogQuery struct {
	Category string `json:"category"`
	Vendor   string `json:"vendor"`
	Model    string `json:"model"`
	Version  string `json:"version"`
	Search   string `json:"search"`
	// Lang selects the locale for the resolved card name/description (e.g. "pt-BR");
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

// Facets are the available filter options the listing UI renders, the three
// drill-down levels (vendor -> model -> version) plus the flat category facet.
type Facets struct {
	Categories []FacetOption `json:"categories"`
	Vendors    []FacetOption `json:"vendors"`
	Models     []FacetOption `json:"models"`
	Versions   []FacetOption `json:"versions"`
}
