package entities

// CatalogItem is the indexed metadata for one marketplace device: enough to list,
// filter and locate its bundle on disk (vendor + slug resolve the folder). The
// heavy bundle (information, simulator, codec, assets) is never held here.
type CatalogItem struct {
	ID         string
	Vendor     string
	VendorName string
	Model      string
	Slug       string
	// Name and Description are indexed per locale; the service resolves each by the
	// request's lang. en-US is always present and acts as the fallback. (The model
	// stays universal — only the human-facing parts are localized.)
	NameEN        string
	NamePT        string
	DescriptionEN string
	DescriptionPT string
	Protocol      string
	ReadingTypes  []string
	Tags          []string
	Icon          string
	HasCodec      bool
	HasManual     bool
}
