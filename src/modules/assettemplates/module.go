package assettemplates

import (
	"context"
	"fmt"
	"path/filepath"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	container "github.com/Mapex-Solutions/mapexGoKit/microservices/container"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/assettemplates/application/ports"
	service "mapexmarketplace/src/modules/assettemplates/application/services"
	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
	catalogRepo "mapexmarketplace/src/modules/assettemplates/infrastructure/catalog"
	"mapexmarketplace/src/modules/assettemplates/interfaces/http/routes"
)

// InitRepositories registers the asset template catalog repository over the
// shared SQLite index and builds the index from the JSON catalog on boot.
func InitRepositories() {
	c := container.GetContainer()

	catalogDir, _ := config.GetStringValue("catalog_dir")
	templatesDir := filepath.Join(catalogDir, "asset_templates")

	if err := c.Provide(func(mgr *sqliteManager.SQLiteManager) repositories.AssetTemplateCatalogRepository {
		return catalogRepo.New(mgr, templatesDir)
	}); err != nil {
		logger.Panic("[MODULE:AssetTemplates] provide repository: " + err.Error())
	}

	if err := c.Invoke(func(repo repositories.AssetTemplateCatalogRepository) {
		count, err := repo.Reload(context.Background())
		if err != nil {
			logger.Panic("[MODULE:AssetTemplates] build index: " + err.Error())
		}
		logger.Info(fmt.Sprintf("[MODULE:AssetTemplates] catalog indexed count=%d", count))
	}); err != nil {
		logger.Panic("[MODULE:AssetTemplates] resolve repository: " + err.Error())
	}

	logger.Info("[MODULE:AssetTemplates] Repositories registered")
}

// InitServices registers the asset templates catalog service.
func InitServices() {
	c := container.GetContainer()
	if err := c.Provide(service.New); err != nil {
		logger.Panic("[MODULE:AssetTemplates] provide service: " + err.Error())
	}
	logger.Info("[MODULE:AssetTemplates] Services registered")
}

// InitInterfaces creates the asset template route group and registers its HTTP
// routes over the resolved service port.
func InitInterfaces() {
	c := container.GetContainer()
	if err := c.Invoke(func(app *web.App, svc ports.AssetTemplatesServicePort) {
		group := app.Group("/api/v1/asset_templates")
		routes.RegisterRoutes(group, svc)
		logger.Info("[MODULE:AssetTemplates] Routes registered")
	}); err != nil {
		logger.Panic("[MODULE:AssetTemplates] register interfaces: " + err.Error())
	}
}
