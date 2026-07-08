package ports

import (
	"context"
	"encoding/json"

	"mapexmarketplace/src/modules/assettemplates/application/dtos"
)

// AssetTemplatesServicePort is the driving port for the asset template
// marketplace. It serves the searchable listing and resolves the heavy bundle
// (information, assets) by id; applying a chosen template is the consumer's job,
// not the catalog's.
type AssetTemplatesServicePort interface {
	// List returns a filtered, paginated page of catalog cards.
	List(ctx context.Context, query *dtos.CatalogQuery) (*dtos.CatalogListResponse, error)

	// Facets returns the available filter options for the listing UI, narrowed by
	// the drill-down selection (vendor, model) and localized by lang.
	Facets(ctx context.Context, vendor, model, lang string) (*dtos.Facets, error)

	// GetInformation returns the full information sheet for one asset template plus
	// its marketplaceGuid and sha256 (identity + integrity metadata the install
	// flow reads from the response headers). The raw bytes are verbatim.
	GetInformation(ctx context.Context, vendor, slug string) (raw json.RawMessage, marketplaceGuid, sha256 string, err error)

	// GetAsset returns a bundle asset (icon, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)
}
