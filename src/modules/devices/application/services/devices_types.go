package services

import "mapexmarketplace/src/modules/devices/application/di"

// DevicesService serves the device catalog over the repository port: it resolves
// listing filters, maps the index rows to wire DTOs, and streams the heavy
// bundles read lazily from disk.
type DevicesService struct {
	deps di.DevicesServiceDI
}
