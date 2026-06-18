package bootstrap

import (
	"context"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/shutdown"

	"go.uber.org/dig"
)

// InitShutdown registers graceful shutdown hooks: stop accepting HTTP first
// (priority 0), then close the index database (priority 5).
func InitShutdown(c *dig.Container, sm *shutdown.ShutdownManager, app *web.App) {
	sm.RegisterFunc("http", 0, func(ctx context.Context) error {
		return app.ShutdownWithContext(ctx)
	})

	if err := c.Invoke(func(db *sqliteManager.SQLiteManager) {
		sm.RegisterFunc("sqlite", 5, func(_ context.Context) error {
			return db.Close()
		})
	}); err != nil {
		logger.Error(err, "[INFRA:Shutdown] resolve index db for shutdown")
	}
}
