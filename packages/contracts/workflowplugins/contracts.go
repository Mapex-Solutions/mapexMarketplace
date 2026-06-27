// Package workflowplugins holds the cross-boundary contracts for the workflow
// plugin marketplace. These shapes are the wire API the browser clients consume;
// a TS schema counterpart mirrors them under the JS workspace when one exists
// (polyglot reciprocity).
package workflowplugins

// CatalogItem is one card in the plugin marketplace listing: the searchable
// metadata, never the heavy bundle (plugin_information/events live as files).
type CatalogItem struct {
	ID           string   `json:"id"`
	Vendor       string   `json:"vendor"`
	VendorName   string   `json:"vendorName"`
	PluginID     string   `json:"pluginId"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	Capabilities []string `json:"capabilities"`
	Tags         []string `json:"tags"`
	Icon         string   `json:"icon"`
	Color        string   `json:"color"`
	// Image is the plugin icon's bundle-relative path (e.g. "icon.svg"); the client
	// builds the asset URL from vendor+slug. Empty when none ships.
	Image               string `json:"image"`
	RequiresCredentials bool   `json:"requiresCredentials"`
	NodeCount           int    `json:"nodeCount"`
	TriggerCount        int    `json:"triggerCount"`
	HasEvents           bool   `json:"hasEvents"`
	HasImage            bool   `json:"hasImage"`
}

// CatalogQuery carries the listing filters and pagination read from the query
// string. Empty filters are ignored; Page is 1-based.
type CatalogQuery struct {
	Category   string `json:"category"`
	Capability string `json:"capability"`
	Tag        string `json:"tag"`
	Search     string `json:"search"`
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

// Facets are the available filter options the listing UI renders.
type Facets struct {
	Categories   []FacetOption `json:"categories"`
	Capabilities []FacetOption `json:"capabilities"`
}
