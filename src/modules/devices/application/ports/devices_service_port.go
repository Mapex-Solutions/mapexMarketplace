package ports

import (
	"context"
	"encoding/json"

	"mapexmarketplace/src/modules/devices/application/dtos"
)

// DevicesServicePort is the driving port for the device marketplace. It serves
// the searchable listing and resolves the heavy bundles (information, simulator,
// assets) by id; installing a chosen device is the consumer's job, not the
// catalog's.
type DevicesServicePort interface {
	// List returns a filtered, paginated page of catalog cards.
	List(ctx context.Context, query *dtos.CatalogQuery) (*dtos.CatalogListResponse, error)

	// Facets returns the available filter options for the listing UI.
	Facets(ctx context.Context) (*dtos.Facets, error)

	// Codecs returns the codecs available for one model.
	Codecs(ctx context.Context, vendor, slug string) ([]dtos.Codec, error)

	// GetInformation returns the full information sheet for one model.
	GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetSimulator returns the installable simulator template for one model.
	GetSimulator(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetAsset returns a bundle asset (codec, manual, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)

	// Refresh rebuilds the catalog index from disk and returns the item count.
	Refresh(ctx context.Context) (int, error)
}
