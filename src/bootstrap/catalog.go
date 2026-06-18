package bootstrap

import (
	"os"
	"path/filepath"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	config "github.com/Mapex-Solutions/mapexGoKit/microservices/config"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"go.uber.org/dig"
)

// InitCatalogIndex opens the SQLite search index and provides its manager to the
// container. The index is derived and disposable: modules rebuild their tables
// from the JSON catalog on boot, so this only prepares the connection.
func InitCatalogIndex(c *dig.Container) {
	path, _ := config.GetStringValue("catalog_index_path")
	if err := ensureParentDir(path); err != nil {
		logger.Panic("[INFRA:Sqlite] prepare index dir: " + err.Error())
	}

	mgr, err := sqliteManager.New(sqliteManager.Config{Path: path, ForeignKeys: false})
	if err != nil {
		logger.Panic("[INFRA:Sqlite] open index: " + err.Error())
	}
	if err := c.Provide(func() *sqliteManager.SQLiteManager { return mgr }); err != nil {
		logger.Panic("[INFRA:Sqlite] provide index manager: " + err.Error())
	}
}

// ensureParentDir creates the directory holding the index file when needed.
func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
