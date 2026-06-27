package entities

// CatalogItem is the indexed metadata for one marketplace workflow plugin: enough
// to list, filter and locate its bundle on disk (vendor + slug resolve the
// folder). The heavy bundle (plugin_information, events, icon) is never held here.
type CatalogItem struct {
	ID         string
	Vendor     string
	VendorName string
	PluginID   string
	Slug       string
	// Name and Description are indexed per locale; the service resolves each by the
	// request's lang. en-US is always present and acts as the fallback.
	NameEN        string
	NamePT        string
	DescriptionEN string
	DescriptionPT string
	Category      string
	Capabilities  []string
	Tags          []string
	Icon          string
	Color         string
	Image         string
	// RequiresCredentials, NodeCount and TriggerCount describe the plugin's runtime
	// shape; HasEvents/HasImage flag the optional bundle files.
	RequiresCredentials bool
	NodeCount           int
	TriggerCount        int
	HasEvents           bool
	HasImage            bool
}
