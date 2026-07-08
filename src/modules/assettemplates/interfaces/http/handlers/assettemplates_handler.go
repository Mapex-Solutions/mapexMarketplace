package handlers

import (
	"strconv"

	response "github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/assettemplates/application/dtos"
	"mapexmarketplace/src/modules/assettemplates/application/ports"
	"mapexmarketplace/src/shared/bundle"
)

// ListTemplates returns a handler that lists catalog asset templates, reading the
// filters and pagination from the query string.
//
// Returns 200 OK with the paginated CatalogListResponse.
func ListTemplates(service ports.AssetTemplatesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		query := &dtos.CatalogQuery{
			Category: c.Query("category"),
			Vendor:   c.Query("vendor"),
			Model:    c.Query("model"),
			Version:  c.Query("version"),
			Search:   c.Query("search"),
			Lang:     c.Query("lang"),
			Page:     atoi(c.Query("page")),
			PerPage:  atoi(c.Query("perPage")),
		}
		retData, err := service.List(c.UserContext(), query)
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetFacets returns a handler that serves the listing filter options, threading
// the drill-down selection (vendor, model) and the locale that labels categories.
//
// Returns 200 OK with the Facets DTO.
func GetFacets(service ports.AssetTemplatesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.Facets(c.UserContext(), c.Query("vendor"), c.Query("model"), c.Query("lang"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetInformation returns a handler that serves an asset template's information sheet.
//
// Returns 200 OK with the raw information JSON, or 404 if the template is unknown.
func GetInformation(service ports.AssetTemplatesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, marketplaceGuid, sha256, err := service.GetInformation(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		// Serve the raw on-disk bundle verbatim (never the {status,errors,data}
		// envelope) so the install can hard-verify the published sha256 against these
		// exact bytes; the shared helper attaches the identity/integrity headers and
		// keeps the response out of shared caches that could strip them.
		return bundle.ServeVerifiableBundle(c, []byte(retData), marketplaceGuid, sha256)
	}
}

// GetAsset returns a handler that streams a bundle asset (icon, image) with its
// content type. The asset path is the wildcard segment.
//
// Returns 200 OK with the raw file, or 404 if the asset is unknown.
func GetAsset(service ports.AssetTemplatesServicePort) web.Handler {
	return func(c *web.Ctx) error {
		data, contentType, err := service.GetAsset(c.UserContext(), c.Params("vendor"), c.Params("slug"), c.Params("*"))
		if err != nil {
			return err
		}
		c.Set(contentTypeHeader, contentType)
		c.Set("Cache-Control", "public, max-age=86400") // bundle assets rarely change
		return c.Send(data)
	}
}

// atoi parses a query integer, treating any malformed value as zero so the
// service applies its defaults.
func atoi(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
