package bootstrap

import (
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	configApp "mapexmarketplace/src/shared/configuration/application"
)

// InitConfig loads configuration from env vars on top of the service defaults.
func InitConfig() {
	config.InitConfig(configApp.DefaultConfiguration)
}

// InitLogger initializes the structured logger. The level comes from log_level
// when set, otherwise debug in dev and info elsewhere.
func InitLogger() {
	serviceName, _ := config.GetStringValue("service_name")
	serviceVersion, _ := config.GetStringValue("service_version")
	goEnv, _ := config.GetStringValue("go_env")

	logger.InitLogger(logger.LoggerOptions{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Environment:    goEnv,
		Level:          resolveLogLevel(goEnv),
	})
}

// resolveLogLevel maps the configured level (or the environment) to a LogLevel.
func resolveLogLevel(goEnv string) logger.LogLevel {
	if lvl, _ := config.GetStringValue("log_level"); lvl != "" {
		switch lvl {
		case "debug":
			return logger.DebugLevel
		case "info":
			return logger.InfoLevel
		case "warn":
			return logger.WarnLevel
		case "error":
			return logger.ErrorLevel
		case "silent":
			return logger.DisabledLevel
		}
	}
	if goEnv == "" || goEnv == "dev" || goEnv == "development" {
		return logger.DebugLevel
	}
	return logger.InfoLevel
}
