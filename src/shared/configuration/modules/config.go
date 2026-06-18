package configMod

import (
	devicesModule "mapexmarketplace/src/modules/devices"
)

// ModuleDefinition describes a marketplace module's init phases. Each phase
// resolves its own dependencies (the SQLite index manager, the fiber app) from
// the DI container, so the registry stays decoupled from concrete wiring.
type ModuleDefinition struct {
	Name             string
	InitRepositories func()
	InitServices     func()
	InitInterfaces   func()
}

// Modules lists the marketplace modules in init order. Plugins and asset
// templates join here as they are migrated; each is a thin module over the same
// catalog primitive.
var Modules = []ModuleDefinition{
	{
		Name:             "Devices",
		InitRepositories: devicesModule.InitRepositories,
		InitServices:     devicesModule.InitServices,
		InitInterfaces:   devicesModule.InitInterfaces,
	},
}
