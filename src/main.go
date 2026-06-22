package main

import (
	"time"

	container "github.com/Mapex-Solutions/mapexGoKit/microservices/container"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/shutdown"

	"mapexmarketplace/src/bootstrap"
	appModule "mapexmarketplace/src/modules/app"
)

// main boots the marketplace catalog server: configuration and logging, the
// SQLite search index, the HTTP app (health + catalog modules), then blocks until
// a signal and shuts down gracefully. The service is stateless — the JSON catalog
// under catalog_dir is the source of truth; the index is rebuilt from it on boot.
func main() {
	container.InitContainer()
	c := container.GetContainer()

	bootstrap.InitConfig()
	bootstrap.InitLogger()

	bootstrap.InitCatalogIndex(c)

	app := bootstrap.InitFiber(c)
	bootstrap.InitHealth(app)
	bootstrap.InitHome(app)

	// Catalog modules build their index tables and register their routes.
	appModule.InitModule()

	sm := shutdown.New()
	bootstrap.InitShutdown(c, sm, app)

	addr := bootstrap.ListenAddress()
	go func() {
		if err := app.Listen(addr); err != nil {
			logger.Error(err, "[INFRA:HTTP] HTTP server stopped")
		}
	}()
	logger.Info("[INFRA:HTTP] mapex-marketplace listening on " + addr)

	sm.WaitForSignal(10 * time.Second)
}
