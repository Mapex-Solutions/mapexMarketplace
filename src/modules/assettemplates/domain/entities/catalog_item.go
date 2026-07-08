package entities

// CatalogItem is the indexed metadata for one marketplace asset template: enough
// to list, filter and locate its bundle on disk (vendor + slug resolve the
// folder). The heavy bundle (asset_template_information, image) is never held here.
type CatalogItem struct {
	ID              string
	MarketplaceGuid string
	Vendor          string
	VendorName      string
	Model           string
	Version         string
	Slug            string
	// Name and Description are indexed per locale; the service resolves each by the
	// request's lang. en-US is always present and acts as the fallback.
	NameEN        string
	NamePT        string
	DescriptionEN string
	DescriptionPT string
	Category      string
	Icon          string
	Image         string
	HasImage      bool
	// FieldCount is the number of fields the template defines; HasScripts flags
	// whether the template ships scripts.
	FieldCount int
	HasScripts bool
	Sha256     string
}
