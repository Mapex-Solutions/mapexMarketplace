package routes

import (
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

	"mapexmarketplace/src/modules/devices/application/ports"
	"mapexmarketplace/src/modules/devices/interfaces/http/handlers"
)

// RegisterRoutes registers the device marketplace HTTP routes.
//
// Following Hexagonal Architecture, this function accepts the service port
// interface rather than a concrete service implementation.
//
// Base path: /api/v1/devices
//
//	GET  /                          - List devices (filters + pagination)
//	GET  /facets                    - Available filter options
//	POST /refresh                   - Rebuild the catalog index from disk
//	GET  /:vendor/:slug             - Model information sheet
//	GET  /:vendor/:slug/simulator   - Model install template
//	GET  /:vendor/:slug/assets/*    - Model bundle asset (codec, manual, image)
//
// Parameters:
//   - group: the router group to register the routes on
//   - service: the DevicesServicePort implementation
func RegisterRoutes(group web.Router, service ports.DevicesServicePort) {
	group.Get("/", handlers.ListDevices(service))
	group.Get("/facets", handlers.GetFacets(service))
	group.Post("/refresh", handlers.RefreshCatalog(service))

	group.Get("/:vendor/:slug", handlers.GetInformation(service))
	group.Get("/:vendor/:slug/simulator", handlers.GetSimulator(service))
	group.Get("/:vendor/:slug/codecs", handlers.ListModelCodecs(service))
	group.Get("/:vendor/:slug/assets/*", handlers.GetAsset(service))
}
