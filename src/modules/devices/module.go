package devices

import (
	"context"
	"fmt"
	"path/filepath"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	container "github.com/Mapex-Solutions/mapexGoKit/microservices/container"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/devices/application/ports"
	service "mapexmarketplace/src/modules/devices/application/services"
	"mapexmarketplace/src/modules/devices/domain/repositories"
	catalogRepo "mapexmarketplace/src/modules/devices/infrastructure/catalog"
	"mapexmarketplace/src/modules/devices/interfaces/http/routes"
)

// InitRepositories registers the device catalog repository over the shared SQLite
// index and builds the index from the JSON catalog on boot.
func InitRepositories() {
	c := container.GetContainer()

	catalogDir, _ := config.GetStringValue("catalog_dir")
	devicesDir := filepath.Join(catalogDir, "devices")

	if err := c.Provide(func(mgr *sqliteManager.SQLiteManager) repositories.DeviceCatalogRepository {
		return catalogRepo.New(mgr, devicesDir)
	}); err != nil {
		logger.Panic("[MODULE:Devices] provide repository: " + err.Error())
	}

	if err := c.Invoke(func(repo repositories.DeviceCatalogRepository) {
		count, err := repo.Reload(context.Background())
		if err != nil {
			logger.Panic("[MODULE:Devices] build index: " + err.Error())
		}
		logger.Info(fmt.Sprintf("[MODULE:Devices] catalog indexed count=%d", count))
	}); err != nil {
		logger.Panic("[MODULE:Devices] resolve repository: " + err.Error())
	}

	logger.Info("[MODULE:Devices] Repositories registered")
}

// InitServices registers the devices catalog service.
func InitServices() {
	c := container.GetContainer()
	if err := c.Provide(service.New); err != nil {
		logger.Panic("[MODULE:Devices] provide service: " + err.Error())
	}
	logger.Info("[MODULE:Devices] Services registered")
}

// InitInterfaces creates the device route group and registers its HTTP routes
// over the resolved service port.
func InitInterfaces() {
	c := container.GetContainer()
	if err := c.Invoke(func(app *web.App, svc ports.DevicesServicePort) {
		group := app.Group("/api/v1/devices")
		routes.RegisterRoutes(group, svc)
		logger.Info("[MODULE:Devices] Routes registered")
	}); err != nil {
		logger.Panic("[MODULE:Devices] register interfaces: " + err.Error())
	}
}
