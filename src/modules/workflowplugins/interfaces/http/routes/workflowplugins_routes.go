package routes

import (
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/workflowplugins/application/ports"
	"mapexmarketplace/src/modules/workflowplugins/interfaces/http/handlers"
)

// RegisterRoutes registers the workflow plugin marketplace HTTP routes.
//
// Following Hexagonal Architecture, this function accepts the service port
// interface rather than a concrete service implementation.
//
// Base path: /api/v1/workflow_plugins
//
//	GET  /                          - List plugins (filters + pagination)
//	GET  /facets                    - Available filter options
//	GET  /:vendor/:slug             - WorkflowPlugin information sheet
//	GET  /:vendor/:slug/events      - WorkflowPlugin events catalog
//	GET  /:vendor/:slug/assets/*    - WorkflowPlugin bundle asset (icon, image)
//
// Parameters:
//   - group: the router group to register the routes on
//   - service: the WorkflowPluginsServicePort implementation
func RegisterRoutes(group web.Router, service ports.WorkflowPluginsServicePort) {
	// The catalog is read-only and only changes on redeploy, so let browsers
	// and any CDN cache the responses briefly. Static bundle assets override
	// this with a longer window in their handler.
	group.Use(func(c *web.Ctx) error {
		c.Set("Cache-Control", "public, max-age=300")
		return c.Next()
	})

	group.Get("/", handlers.ListWorkflowPlugins(service))
	group.Get("/facets", handlers.GetFacets(service))

	group.Get("/:vendor/:slug", handlers.GetInformation(service))
	group.Get("/:vendor/:slug/events", handlers.GetEvents(service))
	group.Get("/:vendor/:slug/assets/*", handlers.GetAsset(service))
}
