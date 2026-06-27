package services

import (
	"errors"

	"mapexmarketplace/src/modules/workflowplugins/application/constants"
	"mapexmarketplace/src/modules/workflowplugins/application/dtos"
	"mapexmarketplace/src/modules/workflowplugins/domain/entities"
	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
)

// buildFilter normalizes the query into a repository filter, applying the
// pagination defaults and the perPage cap.
func (s *PluginsService) buildFilter(query *dtos.CatalogQuery) repositories.CatalogFilter {
	page, perPage := s.resolvePaging(query)
	return repositories.CatalogFilter{
		Category:   query.Category,
		Capability: query.Capability,
		Tag:        query.Tag,
		Search:     query.Search,
		Limit:      perPage,
		Offset:     (page - 1) * perPage,
	}
}

// buildListResponse maps the matched index rows into a paginated response.
func (s *PluginsService) buildListResponse(query *dtos.CatalogQuery, items []entities.CatalogItem, total int) *dtos.CatalogListResponse {
	page, perPage := s.resolvePaging(query)
	out := make([]dtos.CatalogItem, 0, len(items))
	for i := range items {
		out = append(out, s.mapItem(&items[i], query.Lang))
	}
	return &dtos.CatalogListResponse{Items: out, Total: total, Page: page, PerPage: perPage}
}

// resolvePaging clamps the requested page/perPage into valid bounds.
func (s *PluginsService) resolvePaging(query *dtos.CatalogQuery) (int, int) {
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
func (s *PluginsService) mapItem(item *entities.CatalogItem, lang string) dtos.CatalogItem {
	return dtos.CatalogItem{
		ID:                  item.ID,
		Vendor:              item.Vendor,
		VendorName:          item.VendorName,
		PluginID:            item.PluginID,
		Slug:                item.Slug,
		Name:                resolveName(item, lang),
		Description:         resolveDescription(item, lang),
		Category:            item.Category,
		Capabilities:        item.Capabilities,
		Tags:                item.Tags,
		Icon:                item.Icon,
		Color:               item.Color,
		Image:               item.Image,
		RequiresCredentials: item.RequiresCredentials,
		NodeCount:           item.NodeCount,
		TriggerCount:        item.TriggerCount,
		HasEvents:           item.HasEvents,
		HasImage:            item.HasImage,
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
func (s *PluginsService) mapFacets(set repositories.FacetSet) *dtos.Facets {
	return &dtos.Facets{
		Categories:   s.mapFacetList(set.Categories),
		Capabilities: s.mapFacetList(set.Capabilities),
	}
}

// mapFacetList converts a slice of domain facets into wire options.
func (s *PluginsService) mapFacetList(facets []repositories.Facet) []dtos.FacetOption {
	out := make([]dtos.FacetOption, 0, len(facets))
	for _, f := range facets {
		out = append(out, dtos.FacetOption{Value: f.Value, Label: f.Label, Icon: f.Icon})
	}
	return out
}

// mapNotFound translates the repository's not-found sentinel into the HTTP 404;
// any other error passes through unchanged.
func (s *PluginsService) mapNotFound(err error) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return notFound()
	}
	return err
}
