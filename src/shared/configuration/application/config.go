package configApp

import (
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
)

// DefaultConfiguration declares every config key for the marketplace service,
// with env-var overrides and defaults. The service is a stateless catalog server:
// it reads the JSON catalog under catalog_dir at boot, builds an in-process SQLite
// search index at catalog_index_path, and serves it over HTTP.
var DefaultConfiguration = []config.ConfigDefinition{
	/** Service identity */
	{Key: "service_name", Env: "SERVICE_NAME", Type: "string", Default: "mapex-marketplace"},
	{Key: "service_version", Env: "SERVICE_VERSION", Type: "string", Default: "0.1.0"},
	{Key: "go_env", Env: "GO_ENV", Type: "string", Default: "dev"},

	// log_level overrides the env-based default (debug in dev, info otherwise).
	{Key: "log_level", Env: "LOG_LEVEL", Type: "string", Default: ""},

	/** HTTP server */
	{Key: "http_port", Env: "HTTP_PORT", Type: "int", Default: 6060},
	{Key: "http_address", Env: "HTTP_ADDRESS", Type: "string", Default: "127.0.0.1"},
	{Key: "ctx_timeout", Env: "CTX_TIMEOUT", Type: "int", Default: 15},

	// catalog_dir is the root of the JSON catalog (source of truth). Each
	// marketplace lives under catalog_dir/{plugins,devices,asset_templates}.
	{Key: "catalog_dir", Env: "CATALOG_DIR", Type: "string", Default: "./catalog"},

	// catalog_index_path is the on-disk SQLite search index. It is derived and
	// disposable: rebuilt from catalog_dir on every boot.
	{Key: "catalog_index_path", Env: "CATALOG_INDEX_PATH", Type: "string", Default: "./data/catalog-index.db"},

	// cors_origins is the allowlist for browser clients (workflow UI, simulator UI).
	{Key: "cors_origins", Env: "CORS_ORIGINS", Type: "string", Default: "*"},
}
