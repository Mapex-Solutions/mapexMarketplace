package services

import "mapexmarketplace/src/modules/workflowplugins/application/di"

// WorkflowPluginsService serves the workflow plugin catalog over the repository port: it
// resolves listing filters, maps the index rows to wire DTOs, and streams the
// heavy bundles read lazily from disk.
type WorkflowPluginsService struct {
	deps di.WorkflowPluginsServiceDI
}
