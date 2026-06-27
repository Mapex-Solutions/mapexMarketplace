package di

import (
	"go.uber.org/dig"

	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
)

// PluginsServiceDI declares the plugins service dependencies as port interfaces,
// resolved by the DIG container.
type PluginsServiceDI struct {
	dig.In

	Repo repositories.PluginCatalogRepository
}
