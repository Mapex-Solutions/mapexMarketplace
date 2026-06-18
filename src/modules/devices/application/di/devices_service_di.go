package di

import (
	"go.uber.org/dig"

	"mapexmarketplace/src/modules/devices/domain/repositories"
)

// DevicesServiceDI declares the devices service dependencies as port interfaces,
// resolved by the DIG container.
type DevicesServiceDI struct {
	dig.In

	Repo repositories.DeviceCatalogRepository
}
