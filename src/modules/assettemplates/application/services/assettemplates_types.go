package services

import "mapexmarketplace/src/modules/assettemplates/application/di"

// AssetTemplatesService serves the asset template catalog over the repository
// port: it resolves listing filters, maps the index rows to wire DTOs, and
// streams the heavy bundle read lazily from disk.
type AssetTemplatesService struct {
	deps di.AssetTemplatesServiceDI
}
