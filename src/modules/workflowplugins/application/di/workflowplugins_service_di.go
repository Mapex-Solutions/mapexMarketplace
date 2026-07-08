package di

import (
	"go.uber.org/dig"

	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
)

// WorkflowPluginsServiceDI declares the plugins service dependencies as port interfaces,
// resolved by the DIG container.
type WorkflowPluginsServiceDI struct {
	dig.In

	Repo repositories.WorkflowPluginCatalogRepository
}
