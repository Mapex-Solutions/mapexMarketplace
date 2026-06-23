package services

import (
	"context"
	"encoding/json"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"

	"mapexmarketplace/src/modules/devices/application/di"
	"mapexmarketplace/src/modules/devices/application/dtos"
	"mapexmarketplace/src/modules/devices/application/ports"
)

// notFound is the 404 the global error handler renders as an envelope. It is
// raised when a requested bundle or model does not exist.
func notFound() error {
	return &customErrors.ServerCustomError{Code: status.NOT_FOUND, Errors: []string{"catalog item not found"}}
}

// Compile-time check that the service satisfies its port.
var _ ports.DevicesServicePort = (*DevicesService)(nil)

// New builds the devices catalog service over its injected repository port.
func New(deps di.DevicesServiceDI) ports.DevicesServicePort {
	return &DevicesService{deps: deps}
}

// List resolves the query into a repository filter, runs it against the index,
// and maps the matching rows into a paginated response.
func (s *DevicesService) List(ctx context.Context, query *dtos.CatalogQuery) (*dtos.CatalogListResponse, error) {
	filter := s.buildFilter(query)
	items, total, err := s.deps.Repo.Query(ctx, filter)
	if err != nil {
		return nil, err
	}
	return s.buildListResponse(query, items, total), nil
}

// Facets returns the listing filter options mapped to their wire DTO.
func (s *DevicesService) Facets(ctx context.Context) (*dtos.Facets, error) {
	set, err := s.deps.Repo.Facets(ctx)
	if err != nil {
		return nil, err
	}
	return s.mapFacets(set), nil
}

// Codecs returns the codecs available for a model, or 404 when it is unknown.
func (s *DevicesService) Codecs(ctx context.Context, vendor, slug string) ([]dtos.Codec, error) {
	codecs, err := s.deps.Repo.ListCodecs(ctx, vendor, slug)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	return s.mapCodecs(codecs), nil
}

// GetInformation returns the model's information sheet, or 404 when unknown.
func (s *DevicesService) GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	raw, err := s.deps.Repo.GetInformation(ctx, vendor, slug)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	return raw, nil
}

// GetSimulator returns the model's install template, or 404 when unknown.
func (s *DevicesService) GetSimulator(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	raw, err := s.deps.Repo.GetSimulator(ctx, vendor, slug)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	return raw, nil
}

// GetAsset returns a bundle asset and its content type, or 404 when unknown.
func (s *DevicesService) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	data, contentType, err := s.deps.Repo.GetAsset(ctx, vendor, slug, name)
	if err != nil {
		return nil, "", s.mapNotFound(err)
	}
	return data, contentType, nil
}
