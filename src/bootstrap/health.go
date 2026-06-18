package bootstrap

import (
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	response "github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
)

// InitHealth registers GET /health, the liveness probe. It uses the standard
// envelope so clients treat it like every other endpoint; `data` carries the
// status and version.
func InitHealth(app *web.App) {
	app.Get("/health", func(c *web.Ctx) error {
		version, _ := config.GetStringValue("service_version")
		return response.Success(c, web.Map{
			"status":  "ok",
			"version": version,
		})
	})
}
