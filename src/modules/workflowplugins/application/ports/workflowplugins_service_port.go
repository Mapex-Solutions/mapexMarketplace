package ports

import (
	"context"
	"encoding/json"

	"mapexmarketplace/src/modules/workflowplugins/application/dtos"
)

// WorkflowPluginsServicePort is the driving port for the workflow plugin marketplace. It
// serves the searchable listing and resolves the heavy bundles (information,
// events, assets) by id; installing a chosen plugin is the consumer's job, not
// the catalog's.
type WorkflowPluginsServicePort interface {
	// List returns a filtered, paginated page of catalog cards.
	List(ctx context.Context, query *dtos.CatalogQuery) (*dtos.CatalogListResponse, error)

	// Facets returns the available filter options for the listing UI.
	Facets(ctx context.Context) (*dtos.Facets, error)

	// GetInformation returns the full information sheet for one plugin.
	GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetEvents returns the events catalog for one plugin.
	GetEvents(ctx context.Context, vendor, slug string) (json.RawMessage, error)

	// GetAsset returns a bundle asset (icon, image) with its content type.
	GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error)
}
