package handlers

import (
	"strconv"

	response "github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/workflowplugins/application/dtos"
	"mapexmarketplace/src/modules/workflowplugins/application/ports"
)

// ListWorkflowPlugins returns a handler that lists catalog plugins, reading the filters
// and pagination from the query string.
//
// Returns 200 OK with the paginated CatalogListResponse.
func ListWorkflowPlugins(service ports.WorkflowPluginsServicePort) web.Handler {
	return func(c *web.Ctx) error {
		query := &dtos.CatalogQuery{
			Category:   c.Query("category"),
			Capability: c.Query("capability"),
			Tag:        c.Query("tag"),
			Search:     c.Query("search"),
			Lang:       c.Query("lang"),
			Page:       atoi(c.Query("page")),
			PerPage:    atoi(c.Query("perPage")),
		}
		retData, err := service.List(c.UserContext(), query)
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetFacets returns a handler that serves the listing filter options.
//
// Returns 200 OK with the Facets DTO.
func GetFacets(service ports.WorkflowPluginsServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.Facets(c.UserContext())
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetInformation returns a handler that serves a plugin's information sheet.
//
// Returns 200 OK with the raw information JSON, or 404 if the plugin is unknown.
func GetInformation(service ports.WorkflowPluginsServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.GetInformation(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetEvents returns a handler that serves a plugin's events catalog.
//
// Returns 200 OK with the raw events JSON, or 404 if the plugin is unknown.
func GetEvents(service ports.WorkflowPluginsServicePort) web.Handler {
	return func(c *web.Ctx) error {
		retData, err := service.GetEvents(c.UserContext(), c.Params("vendor"), c.Params("slug"))
		if err != nil {
			return err
		}
		return response.Success(c, retData)
	}
}

// GetAsset returns a handler that streams a bundle asset (icon, image) with its
// content type. The asset path is the wildcard segment.
//
// Returns 200 OK with the raw file, or 404 if the asset is unknown.
func GetAsset(service ports.WorkflowPluginsServicePort) web.Handler {
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
