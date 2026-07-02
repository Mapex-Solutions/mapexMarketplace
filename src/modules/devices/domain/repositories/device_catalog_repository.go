package repositories

import (
	"context"
	"encoding/json"
	"errors"

	"mapexmarketplace/src/modules/devices/domain/entities"
)

// ErrNotFound is returned when a catalog item or bundle file does not exist, so
// the application layer can map it to a 404 without importing the persistence
// or filesystem error.
var ErrNotFound = errors.New("catalog item not found")

// DeviceCatalogRepository is the persistence port for the device catalog. The
// index (searchable metadata) answers Query/Facets; the heavy bundles are read
// lazily from disk by id (vendor + slug).
type DeviceCatalogRepository interface {
	// Query returns the page of items matching the filter plus the total count.
	Query(ctx context.Context, filter CatalogFilter) ([]entities.CatalogItem, int, error)

	// Facets returns the available filter options actually present in the catalog.
	// The selection narrows the drill-down levels (e.g. manufacturer -> models).
	Facets(ctx context.Context, sel FacetSelection) (FacetSet, error)

	// ListCodecs returns the codecs shipped with one model, read from its
	// codecs/{id}/codec.json folders.
	ListCodecs(ctx context.Context, vendor, slug string) ([]entities.Codec, error)

	// GetInformation returns the raw device_information.json for one model.
	GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetSimulator returns the raw device_simulator.json (install template).
	GetSimulator(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetAsset returns a bundle asset (codec, manual, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)

	// Reload rebuilds the index from the JSON catalog and returns the item count.
	Reload(ctx context.Context) (int, error)
}
