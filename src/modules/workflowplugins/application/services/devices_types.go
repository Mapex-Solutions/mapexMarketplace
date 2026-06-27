package services

import "mapexmarketplace/src/modules/workflowplugins/application/di"

// PluginsService serves the workflow plugin catalog over the repository port: it
// resolves listing filters, maps the index rows to wire DTOs, and streams the
// heavy bundles read lazily from disk.
type PluginsService struct {
	deps di.PluginsServiceDI
}
