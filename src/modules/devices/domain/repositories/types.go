package repositories

// CatalogFilter is the resolved query the repository runs against the index:
// empty string fields are ignored; Limit/Offset drive pagination.
type CatalogFilter struct {
	Protocol     string
	ReadingType  string
	Manufacturer string
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
}
