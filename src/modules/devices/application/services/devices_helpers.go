package services

import (
	"errors"

	"mapexmarketplace/src/modules/devices/application/constants"
	"mapexmarketplace/src/modules/devices/application/dtos"
	"mapexmarketplace/src/modules/devices/domain/entities"
	"mapexmarketplace/src/modules/devices/domain/repositories"
)

// buildFilter normalizes the query into a repository filter, applying the
// pagination defaults and the perPage cap.
func (s *DevicesService) buildFilter(query *dtos.CatalogQuery) repositories.CatalogFilter {
	page, perPage := s.resolvePaging(query)
	return repositories.CatalogFilter{
		Protocol:     query.Protocol,
		ReadingType:  query.ReadingType,
		Manufacturer: query.Manufacturer,
		Search:       query.Search,
		Limit:        perPage,
		Offset:       (page - 1) * perPage,
	}
}

// buildListResponse maps the matched index rows into a paginated response.
func (s *DevicesService) buildListResponse(query *dtos.CatalogQuery, items []entities.CatalogItem, total int) *dtos.CatalogListResponse {
	page, perPage := s.resolvePaging(query)
	out := make([]dtos.CatalogItem, 0, len(items))
	for i := range items {
		out = append(out, s.mapItem(&items[i], query.Lang))
	}
	return &dtos.CatalogListResponse{Items: out, Total: total, Page: page, PerPage: perPage}
}

// resolvePaging clamps the requested page/perPage into valid bounds.
func (s *DevicesService) resolvePaging(query *dtos.CatalogQuery) (int, int) {
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

// mapItem converts an index row into its wire DTO, resolving the card
// description for the requested locale (falling back to en-US).
func (s *DevicesService) mapItem(item *entities.CatalogItem, lang string) dtos.CatalogItem {
	return dtos.CatalogItem{
		ID:           item.ID,
		Vendor:       item.Vendor,
		VendorName:   item.VendorName,
		Model:        item.Model,
		Slug:         item.Slug,
		Name:         resolveName(item, lang),
		Description:  resolveDescription(item, lang),
		Protocol:     item.Protocol,
		ReadingTypes: item.ReadingTypes,
		Tags:         item.Tags,
		Icon:         item.Icon,
		HasCodec:     item.HasCodec,
		HasManual:    item.HasManual,
	}
}

// resolveName picks the localized card name (model + localized category) for the
// requested locale, falling back to en-US.
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
func (s *DevicesService) mapFacets(set repositories.FacetSet) *dtos.Facets {
	return &dtos.Facets{
		Protocols:     s.mapFacetList(set.Protocols),
		ReadingTypes:  s.mapFacetList(set.ReadingTypes),
		Manufacturers: s.mapFacetList(set.Manufacturers),
	}
}

// mapFacetList converts a slice of domain facets into wire options.
func (s *DevicesService) mapFacetList(facets []repositories.Facet) []dtos.FacetOption {
	out := make([]dtos.FacetOption, 0, len(facets))
	for _, f := range facets {
		out = append(out, dtos.FacetOption{Value: f.Value, Label: f.Label, Icon: f.Icon})
	}
	return out
}

// mapCodecs converts the domain codecs into their wire DTOs.
func (s *DevicesService) mapCodecs(codecs []entities.Codec) []dtos.Codec {
	out := make([]dtos.Codec, 0, len(codecs))
	for i := range codecs {
		c := codecs[i]
		out = append(out, dtos.Codec{
			ID:          c.ID,
			Name:        c.Name,
			Source:      c.Source,
			Official:    c.Official,
			Default:     c.Default,
			Target:      c.Target,
			Language:    c.Language,
			SourceURL:   c.SourceURL,
			Path:        c.Path,
			DecoderFile: c.DecoderFile,
			EncoderFile: c.EncoderFile,
		})
	}
	return out
}

// mapNotFound translates the repository's not-found sentinel into the HTTP 404;
// any other error passes through unchanged.
func (s *DevicesService) mapNotFound(err error) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return notFound()
	}
	return err
}
