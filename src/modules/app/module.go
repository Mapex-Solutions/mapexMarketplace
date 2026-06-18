package app

import (
	"fmt"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	configMod "mapexmarketplace/src/shared/configuration/modules"
)

// InitModule runs each registered module's init phases in order across all
// modules: repositories, then services, then interfaces. Phases resolve their
// dependencies from the DI container, so this loop stays wiring-agnostic.
func InitModule() {
	for _, mod := range configMod.Modules {
		if mod.InitRepositories != nil {
			logger.Info(fmt.Sprintf("[MODULE:%s] Initializing repositories", mod.Name))
			mod.InitRepositories()
		}
	}
	for _, mod := range configMod.Modules {
		if mod.InitServices != nil {
			logger.Info(fmt.Sprintf("[MODULE:%s] Initializing services", mod.Name))
			mod.InitServices()
		}
	}
	for _, mod := range configMod.Modules {
		if mod.InitInterfaces != nil {
			logger.Info(fmt.Sprintf("[MODULE:%s] Initializing interfaces", mod.Name))
			mod.InitInterfaces()
		}
	}
}
