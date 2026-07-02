package repositories

// CatalogFilter is the resolved query the repository runs against the index:
// empty string fields are ignored; Limit/Offset drive pagination.
type CatalogFilter struct {
	Category string
	Vendor   string
	Model    string
	Version  string
	Search   string
	Limit    int
	Offset   int
}

// Facet is one selectable filter value with its display label and icon.
type Facet struct {
	Value string
	Label string
	Icon  string
}

// FacetSet groups the filter options the catalog exposes: the three drill-down
// levels (vendor -> model -> version) plus the flat category facet.
type FacetSet struct {
	Categories []Facet
	Vendors    []Facet
	Models     []Facet
	Versions   []Facet
}

// FacetSelection narrows the drill-down facets by the user's current pick.
// Empty fields impose no narrowing, so a zero selection yields the top level.
type FacetSelection struct {
	// Vendor narrows Models to that vendor (drill-down: vendor -> model).
	Vendor string
	// Model narrows Versions to that model (drill-down: model -> version).
	Model string
	// Lang selects the locale for the resolved category facet labels.
	Lang string
}
