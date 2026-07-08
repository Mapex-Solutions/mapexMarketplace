package repositories

// CatalogFilter is the resolved query the repository runs against the index:
// empty string fields are ignored; Limit/Offset drive pagination.
type CatalogFilter struct {
	Protocol     string
	ReadingType  string
	Manufacturer string
	Model        string
	Search       string
	Limit        int
	Offset       int
}

// Facet is one selectable filter value with its display label and icon.
type Facet struct {
	Value string
	Label string
	Icon  string
}

// FacetSet groups the filter options the catalog exposes.
type FacetSet struct {
	Protocols     []Facet
	ReadingTypes  []Facet
	Manufacturers []Facet
	Models        []Facet
}

// FacetSelection narrows the drill-down facets by the user's current pick.
// Empty fields impose no narrowing, so a zero selection yields the top level.
type FacetSelection struct {
	// Manufacturer narrows Models to that vendor (drill-down: vendor -> model).
	Manufacturer string
}
