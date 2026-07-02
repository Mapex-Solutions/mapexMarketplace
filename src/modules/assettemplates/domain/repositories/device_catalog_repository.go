package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"mapexmarketplace/src/modules/assettemplates/domain/entities"
)

// ErrNotFound is returned when a catalog item or bundle file does not exist, so
// the application layer can map it to a 404 without importing the persistence
// or filesystem error.
var ErrNotFound = errors.New("catalog item not found")

// AssetTemplateCatalogRepository is the persistence port for the asset template
// catalog. The index (searchable metadata) answers Query/Facets; the heavy
// bundles are read lazily from disk by id (vendor + slug).
type AssetTemplateCatalogRepository interface {
	// Query returns the page of items matching the filter plus the total count.
	Query(ctx context.Context, filter CatalogFilter) ([]entities.CatalogItem, int, error)

	// Facets returns the available filter options (categories, vendors, models,
	// versions) actually present in the current catalog, narrowed by the selection.
	Facets(ctx context.Context, sel FacetSelection) (FacetSet, error)

	// GetInformation returns the raw asset_template_information.json for one template.
	GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetAsset returns a bundle asset (icon, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)

	// Reload rebuilds the index from the JSON catalog and returns the item count.
	Reload(ctx context.Context) (int, error)
}
