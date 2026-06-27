package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"mapexmarketplace/src/modules/workflowplugins/domain/entities"
)

// ErrNotFound is returned when a catalog item or bundle file does not exist, so
// the application layer can map it to a 404 without importing the persistence
// or filesystem error.
var ErrNotFound = errors.New("catalog item not found")

// PluginCatalogRepository is the persistence port for the workflow plugin
// catalog. The index (searchable metadata) answers Query/Facets; the heavy
// bundles are read lazily from disk by id (vendor + slug).
type PluginCatalogRepository interface {
	// Query returns the page of items matching the filter plus the total count.
	Query(ctx context.Context, filter CatalogFilter) ([]entities.CatalogItem, int, error)

	// Facets returns the available filter options (categories, capabilities)
	// actually present in the current catalog.
	Facets(ctx context.Context) (FacetSet, error)

	// GetInformation returns the raw plugin_information.json for one plugin.
	GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetEvents returns the raw events.json for one plugin.
	GetEvents(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetAsset returns a bundle asset (icon, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)

	// Reload rebuilds the index from the JSON catalog and returns the item count.
	Reload(ctx context.Context) (int, error)
}
