package routes

import (
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/assettemplates/application/ports"
	"mapexmarketplace/src/modules/assettemplates/interfaces/http/handlers"
)

// RegisterRoutes registers the asset template marketplace HTTP routes.
//
// Following Hexagonal Architecture, this function accepts the service port
// interface rather than a concrete service implementation.
//
// Base path: /api/v1/asset_templates
//
//	GET  /                          - List asset templates (filters + pagination)
//	GET  /facets                    - Available filter options (drill-down + categories)
//	GET  /:vendor/:slug             - Asset template information sheet
//	GET  /:vendor/:slug/assets/*    - Asset template bundle asset (icon, image)
//
// Parameters:
//   - group: the router group to register the routes on
//   - service: the AssetTemplatesServicePort implementation
func RegisterRoutes(group web.Router, service ports.AssetTemplatesServicePort) {
	// The catalog is read-only and only changes on redeploy, so let browsers
	// and any CDN cache the responses briefly. Static bundle assets override
	// this with a longer window in their handler.
	group.Use(func(c *web.Ctx) error {
		c.Set("Cache-Control", "public, max-age=300")
		return c.Next()
	})

	group.Get("/", handlers.ListTemplates(service))
	group.Get("/facets", handlers.GetFacets(service))

	group.Get("/:vendor/:slug", handlers.GetInformation(service))
	group.Get("/:vendor/:slug/assets/*", handlers.GetAsset(service))
}
