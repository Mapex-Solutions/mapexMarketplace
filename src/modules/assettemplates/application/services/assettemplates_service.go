package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

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

// GetInformation returns the template's information sheet plus its identity and
// integrity metadata (marketplaceGuid, sha256) for the install flow to consume,
// or 404 when unknown. The raw bytes are returned verbatim (never re-encoded) so
// the published sha256 stays verifiable against exactly what is served.
func (s *AssetTemplatesService) GetInformation(ctx context.Context, vendor, slug string) (raw json.RawMessage, marketplaceGuid, sha256 string, err error) {
	raw, err = s.deps.Repo.GetInformation(ctx, vendor, slug)
	if err != nil {
		return nil, "", "", s.mapNotFound(err)
	}
	item, err := s.deps.Repo.GetItem(ctx, vendor, slug)
	if err != nil {
		// The on-disk sheet read succeeded but the index has no row: the catalog
		// tree and the built index disagree (a partial deploy or a vendor
		// catalog.json that failed to parse). Surface the drift instead of letting
		// it look like a plain not-found, then still answer 404 since the index is
		// the source of truth for what is published.
		if errors.Is(err, repositories.ErrNotFound) {
			logger.Warn(fmt.Sprintf("[SERVICE:AssetTemplate] index/disk drift: %s/%s has an on-disk sheet but no catalog index row; regenerate the vendor catalog.json", vendor, slug))
		}
		return nil, "", "", s.mapNotFound(err)
	}
	return raw, item.MarketplaceGuid, item.Sha256, nil
}

// GetAsset returns a bundle asset and its content type, or 404 when unknown.
func (s *AssetTemplatesService) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	data, contentType, err := s.deps.Repo.GetAsset(ctx, vendor, slug, name)
	if err != nil {
		return nil, "", s.mapNotFound(err)
	}
	return data, contentType, nil
}
