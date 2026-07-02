package services

import (
	"errors"

	"mapexmarketplace/src/modules/assettemplates/application/constants"
	"mapexmarketplace/src/modules/assettemplates/application/dtos"
	"mapexmarketplace/src/modules/assettemplates/domain/entities"
	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
)

// buildFilter normalizes the query into a repository filter, applying the
// pagination defaults and the perPage cap.
func (s *AssetTemplatesService) buildFilter(query *dtos.CatalogQuery) repositories.CatalogFilter {
	page, perPage := s.resolvePaging(query)
	return repositories.CatalogFilter{
		Category: query.Category,
		Vendor:   query.Vendor,
		Model:    query.Model,
		Version:  query.Version,
		Search:   query.Search,
		Limit:    perPage,
		Offset:   (page - 1) * perPage,
	}
}

// buildListResponse maps the matched index rows into a paginated response.
func (s *AssetTemplatesService) buildListResponse(query *dtos.CatalogQuery, items []entities.CatalogItem, total int) *dtos.CatalogListResponse {
	page, perPage := s.resolvePaging(query)
	out := make([]dtos.CatalogItem, 0, len(items))
	for i := range items {
		out = append(out, s.mapItem(&items[i], query.Lang))
	}
	return &dtos.CatalogListResponse{Items: out, Total: total, Page: page, PerPage: perPage}
}

// resolvePaging clamps the requested page/perPage into valid bounds.
func (s *AssetTemplatesService) resolvePaging(query *dtos.CatalogQuery) (int, int) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	perPage := query.PerPage
	if perPage <= 0 {
		perPage = constants.DefaultPerPage
	}
	if perPage > constants.MaxPerPage {
		perPage = constants.MaxPerPage
	}
	return page, perPage
}

// mapItem converts an index row into its wire DTO, resolving the card name and
// description for the requested locale (falling back to en-US).
func (s *AssetTemplatesService) mapItem(item *entities.CatalogItem, lang string) dtos.CatalogItem {
	return dtos.CatalogItem{
		ID:              item.ID,
		MarketplaceGuid: item.MarketplaceGuid,
		Slug:            item.Slug,
		Name:            resolveName(item, lang),
		Description:     resolveDescription(item, lang),
		Category:        item.Category,
		Vendor:          item.Vendor,
		VendorName:      item.VendorName,
		Model:           item.Model,
		Version:         item.Version,
		Icon:            item.Icon,
		Image:           item.Image,
		HasImage:        item.HasImage,
		FieldCount:      item.FieldCount,
		HasScripts:      item.HasScripts,
		Sha256:          item.Sha256,
	}
}

// resolveName picks the localized card name for the requested locale, falling
// back to en-US.
func resolveName(item *entities.CatalogItem, lang string) string {
	if lang == "pt-BR" && item.NamePT != "" {
		return item.NamePT
	}
	return item.NameEN
}

// resolveDescription picks the localized card description: the requested locale
// when present, otherwise the en-US fallback.
func resolveDescription(item *entities.CatalogItem, lang string) string {
	if lang == "pt-BR" && item.DescriptionPT != "" {
		return item.DescriptionPT
	}
	return item.DescriptionEN
}

// mapFacets converts the domain facet set into the wire DTO.
func (s *AssetTemplatesService) mapFacets(set repositories.FacetSet) *dtos.Facets {
	return &dtos.Facets{
		Categories: s.mapFacetList(set.Categories),
		Vendors:    s.mapFacetList(set.Vendors),
		Models:     s.mapFacetList(set.Models),
		Versions:   s.mapFacetList(set.Versions),
	}
}

// mapFacetList converts a slice of domain facets into wire options.
func (s *AssetTemplatesService) mapFacetList(facets []repositories.Facet) []dtos.FacetOption {
	out := make([]dtos.FacetOption, 0, len(facets))
	for _, f := range facets {
		out = append(out, dtos.FacetOption{Value: f.Value, Label: f.Label, Icon: f.Icon})
	}
	return out
}

// mapNotFound translates the repository's not-found sentinel into the HTTP 404;
// any other error passes through unchanged.
func (s *AssetTemplatesService) mapNotFound(err error) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return notFound()
	}
	return err
}
