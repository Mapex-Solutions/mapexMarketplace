package di

import (
	"go.uber.org/dig"

	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
)

// AssetTemplatesServiceDI declares the asset templates service dependencies as
// port interfaces, resolved by the DIG container.
type AssetTemplatesServiceDI struct {
	dig.In

	Repo repositories.AssetTemplateCatalogRepository
}
