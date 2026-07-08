package workflowplugins

import (
	"context"
	"fmt"
	"path/filepath"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	container "github.com/Mapex-Solutions/mapexGoKit/microservices/container"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/workflowplugins/application/ports"
	service "mapexmarketplace/src/modules/workflowplugins/application/services"
	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
	catalogRepo "mapexmarketplace/src/modules/workflowplugins/infrastructure/catalog"
	"mapexmarketplace/src/modules/workflowplugins/interfaces/http/routes"
)

// InitRepositories registers the plugin catalog repository over the shared SQLite
// index and builds the index from the JSON catalog on boot.
func InitRepositories() {
	c := container.GetContainer()

	catalogDir, _ := config.GetStringValue("catalog_dir")
	pluginsDir := filepath.Join(catalogDir, "workflow_plugins")

	if err := c.Provide(func(mgr *sqliteManager.SQLiteManager) repositories.WorkflowPluginCatalogRepository {
		return catalogRepo.New(mgr, pluginsDir)
	}); err != nil {
		logger.Panic("[MODULE:WorkflowPlugins] provide repository: " + err.Error())
	}

	if err := c.Invoke(func(repo repositories.WorkflowPluginCatalogRepository) {
		count, err := repo.Reload(context.Background())
		if err != nil {
			logger.Panic("[MODULE:WorkflowPlugins] build index: " + err.Error())
		}
		logger.Info(fmt.Sprintf("[MODULE:WorkflowPlugins] catalog indexed count=%d", count))
	}); err != nil {
		logger.Panic("[MODULE:WorkflowPlugins] resolve repository: " + err.Error())
	}

	logger.Info("[MODULE:WorkflowPlugins] Repositories registered")
}

// InitServices registers the plugins catalog service.
func InitServices() {
	c := container.GetContainer()
	if err := c.Provide(service.New); err != nil {
		logger.Panic("[MODULE:WorkflowPlugins] provide service: " + err.Error())
	}
	logger.Info("[MODULE:WorkflowPlugins] Services registered")
}

// InitInterfaces creates the plugin route group and registers its HTTP routes
// over the resolved service port.
func InitInterfaces() {
	c := container.GetContainer()
	if err := c.Invoke(func(app *web.App, svc ports.WorkflowPluginsServicePort) {
		group := app.Group("/api/v1/workflow_plugins")
		routes.RegisterRoutes(group, svc)
		logger.Info("[MODULE:WorkflowPlugins] Routes registered")
	}); err != nil {
		logger.Panic("[MODULE:WorkflowPlugins] register interfaces: " + err.Error())
	}
}
