package services

import (
	"context"
	"encoding/json"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"

	"mapexmarketplace/src/modules/assettemplates/application/di"
	"mapexmarketplace/src/modules/assettemplates/application/dtos"
	"mapexmarketplace/src/modules/assettemplates/application/ports"
	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
)

// notFound is the 404 the global error handler renders as an envelope. It is
// raised when a requested bundle or template does not exist.
func notFound() error {
	return &customErrors.ServerCustomError{Code: status.NOT_FOUND, Errors: []string{"catalog item not found"}}
}

// Compile-time check that the service satisfies its port.
var _ ports.AssetTemplatesServicePort = (*AssetTemplatesService)(nil)

// New builds the asset templates catalog service over its injected repository port.
func New(deps di.AssetTemplatesServiceDI) ports.AssetTemplatesServicePort {
	return &AssetTemplatesService{deps: deps}
}

// List resolves the query into a repository filter, runs it against the index,
// and maps the matching rows into a paginated response.
func (s *AssetTemplatesService) List(ctx context.Context, query *dtos.CatalogQuery) (*dtos.CatalogListResponse, error) {
	filter := s.buildFilter(query)
	items, total, err := s.deps.Repo.Query(ctx, filter)
	if err != nil {
		return nil, err
	}
	return s.buildListResponse(query, items, total), nil
}

// Facets returns the listing filter options mapped to their wire DTO, threading
// the drill-down selection (vendor narrows models, model narrows versions) and
// the locale that resolves the category labels.
func (s *AssetTemplatesService) Facets(ctx context.Context, vendor, model, lang string) (*dtos.Facets, error) {
	set, err := s.deps.Repo.Facets(ctx, repositories.FacetSelection{Vendor: vendor, Model: model, Lang: lang})
	if err != nil {
		return nil, err
	}
	return s.mapFacets(set), nil
}

// GetInformation returns the template's information sheet, or 404 when unknown.
func (s *AssetTemplatesService) GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	raw, err := s.deps.Repo.GetInformation(ctx, vendor, slug)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	return raw, nil
}

// GetAsset returns a bundle asset and its content type, or 404 when unknown.
func (s *AssetTemplatesService) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	data, contentType, err := s.deps.Repo.GetAsset(ctx, vendor, slug, name)
	if err != nil {
		return nil, "", s.mapNotFound(err)
	}
	return data, contentType, nil
}
