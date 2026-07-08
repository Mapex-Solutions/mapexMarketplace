package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/workflowplugins/domain/entities"
	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
	"mapexmarketplace/src/shared/bundle"
)

// The catalog index is a bespoke search store, not entity CRUD: its queries need
// membership LIKE, cross-column OR search and DISTINCT facets that the generic
// sqlite model (equality-only filters) cannot express. So this adapter uses the
// manager's *sql.DB directly — a deliberate, documented exception to the
// "wrappers only" rule, scoped to this read-only derived index.

// Compile-time proof the adapter satisfies the repository port.
var _ repositories.WorkflowPluginCatalogRepository = (*adapter)(nil)

// New builds the plugin catalog repository over the shared SQLite index and the
// JSON catalog directory. The index is empty until Reload runs (in the module's
// repository init), keeping construction free of I/O.
func New(mgr *sqliteManager.SQLiteManager, dir string) repositories.WorkflowPluginCatalogRepository {
	return &adapter{db: mgr.DB(), dir: dir}
}

// Reload rebuilds the index from the per-vendor manifests: ensure the table,
// clear it, reload the catalog config, then insert every plugin in one
// transaction so a failed reload never leaves the index half-populated.
func (a *adapter) Reload(ctx context.Context) (int, error) {
	// Drop then recreate so the index always matches the current schema; an older
	// DB file would otherwise survive CREATE IF NOT EXISTS and fail the insert.
	if _, err := a.db.ExecContext(ctx, dropWorkflowPluginCatalog); err != nil {
		return 0, err
	}
	if _, err := a.db.ExecContext(ctx, ddlWorkflowPluginCatalog); err != nil {
		return 0, err
	}
	a.loadConfig()
	items, err := a.readManifests()
	if err != nil {
		return 0, err
	}
	a.warnUnknownCapabilities(items)
	count, err := a.replaceIndex(ctx, items)
	if err != nil {
		return 0, err
	}
	logger.Info(fmt.Sprintf("[REPO:WorkflowPluginCatalog] indexed count=%d dir=%s", count, a.dir))
	return count, nil
}

// Query returns the page of items matching the filter plus the total match count.
func (a *adapter) Query(ctx context.Context, filter repositories.CatalogFilter) ([]entities.CatalogItem, int, error) {
	where, args := buildWhere(filter)

	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableWorkflowPluginCatalog+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := "SELECT " + selectColumns + " FROM " + tableWorkflowPluginCatalog + where + " ORDER BY name_en LIMIT ? OFFSET ?"
	rows, err := a.db.QueryContext(ctx, listSQL, append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Facets returns the filter options actually present in the index, labelled and
// ordered by the catalog config. Categories/capabilities with zero plugins are
// omitted so the UI never shows a filter that yields nothing.
func (a *adapter) Facets(ctx context.Context) (repositories.FacetSet, error) {
	categories, err := a.presentColumn(ctx, "category")
	if err != nil {
		return repositories.FacetSet{}, err
	}
	capabilities, err := a.presentTokens(ctx, "capabilities")
	if err != nil {
		return repositories.FacetSet{}, err
	}
	return repositories.FacetSet{
		Categories:   filterFacets(a.config.Categories, categories),
		Capabilities: filterFacets(a.config.Capabilities, capabilities),
	}, nil
}

// GetInformation returns the plugin's information sheet read lazily from disk.
func (a *adapter) GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	return a.readWorkflowPluginFile(vendor, slug, fileInformation)
}

// GetEvents returns the plugin's events catalog read lazily from disk.
func (a *adapter) GetEvents(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	return a.readWorkflowPluginFile(vendor, slug, fileEvents)
}

// GetAsset returns a bundle asset (icon, image) and its content type, guarding
// the asset path against traversal outside the plugin folder.
func (a *adapter) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	dir, err := a.pluginDir(vendor, slug)
	if err != nil {
		return nil, "", err
	}
	full := filepath.Join(dir, filepath.Clean("/"+name))
	if full != dir && !bundle.PathWithin(dir, full) {
		return nil, "", repositories.ErrNotFound
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, "", repositories.ErrNotFound
	}
	return data, contentTypeFor(full), nil
}

// loadConfig reads catalog_config.json into memory; a missing file leaves the
// facets empty rather than failing the reload.
func (a *adapter) loadConfig() {
	a.config = catalogConfig{}
	data, err := os.ReadFile(filepath.Join(a.dir, fileCatalogConfig))
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &a.config); err != nil {
		logger.Error(err, "[REPO:WorkflowPluginCatalog] parse catalog config")
	}
}

// readManifests reads every vendors/*/catalog.json and flattens it into index
// rows, stamping each item with its vendor identity.
func (a *adapter) readManifests() ([]entities.CatalogItem, error) {
	matches, err := filepath.Glob(filepath.Join(a.dir, "vendors", "*", "catalog.json"))
	if err != nil {
		return nil, err
	}
	items := []entities.CatalogItem{}
	for _, file := range matches {
		data, err := os.ReadFile(file)
		if err != nil {
			logger.Error(err, "[REPO:WorkflowPluginCatalog] read manifest "+file)
			continue
		}
		var manifest vendorCatalog
		if err := json.Unmarshal(data, &manifest); err != nil {
			logger.Error(err, "[REPO:WorkflowPluginCatalog] parse manifest "+file)
			continue
		}
		for _, item := range manifest.Items {
			items = append(items, entities.CatalogItem{
				ID:                  item.ID,
				Vendor:              manifest.Vendor.Slug,
				VendorName:          manifest.Vendor.Name,
				PluginID:            item.PluginID,
				Slug:                item.Slug,
				NameEN:              item.Name.get(defaultLang),
				NamePT:              item.Name.get("pt-BR"),
				DescriptionEN:       item.Description.get(defaultLang),
				DescriptionPT:       item.Description.get("pt-BR"),
				Category:            item.Category,
				Capabilities:        item.Capabilities,
				Tags:                item.Tags,
				Icon:                item.Icon,
				Color:               item.Color,
				Image:               item.Image,
				RequiresCredentials: item.RequiresCredentials,
				NodeCount:           item.NodeCount,
				TriggerCount:        item.TriggerCount,
				HasEvents:           item.HasEvents,
				HasImage:            item.HasImage,
			})
		}
	}
	return items, nil
}

// replaceIndex inserts every item in one transaction. Reload recreates the table
// (DROP + CREATE) immediately before this, so it is already empty — no clear step
// is needed here.
func (a *adapter) replaceIndex(ctx context.Context, items []entities.CatalogItem) (int, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	for i := range items {
		item := items[i]
		if _, err := tx.ExecContext(ctx, insertWorkflowPluginCatalog,
			item.ID, item.Vendor, item.VendorName, item.PluginID, item.Slug, item.NameEN, item.NamePT,
			item.DescriptionEN, item.DescriptionPT, item.Category, wrapTokens(item.Capabilities), wrapTokens(item.Tags),
			item.Icon, item.Color, item.Image, boolToInt(item.RequiresCredentials), item.NodeCount, item.TriggerCount,
			boolToInt(item.HasEvents), boolToInt(item.HasImage),
		); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(items), nil
}

// presentColumn returns the distinct non-empty values of a scalar column. The
// column name is an internal constant, never user input.
func (a *adapter) presentColumn(ctx context.Context, column string) (map[string]bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT DISTINCT "+column+" FROM "+tableWorkflowPluginCatalog)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	present := map[string]bool{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != "" {
			present[value] = true
		}
	}
	return present, rows.Err()
}

// presentTokens returns the set of tokens present across all rows of a
// comma-wrapped multi-value column. The column name is an internal constant.
func (a *adapter) presentTokens(ctx context.Context, column string) (map[string]bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT "+column+" FROM "+tableWorkflowPluginCatalog)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	present := map[string]bool{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, token := range parseTokens(raw) {
			present[token] = true
		}
	}
	return present, rows.Err()
}

// warnUnknownCapabilities logs capabilities a plugin declares that are absent
// from the catalog config, keeping the vocabulary consistent without failing the
// reload.
func (a *adapter) warnUnknownCapabilities(items []entities.CatalogItem) {
	known := map[string]bool{}
	for _, option := range a.config.Capabilities {
		known[option.Value] = true
	}
	for _, item := range items {
		for _, capability := range item.Capabilities {
			if !known[capability] {
				logger.Warn(fmt.Sprintf("[REPO:WorkflowPluginCatalog] unknown capability=%q id=%s (add it to catalog_config.json)", capability, item.ID))
			}
		}
	}
}

// pluginDir resolves a plugin's folder, rejecting unsafe vendor/slug segments.
func (a *adapter) pluginDir(vendor, slug string) (string, error) {
	if !safeSegment(vendor) || !safeSegment(slug) {
		return "", repositories.ErrNotFound
	}
	return filepath.Join(a.dir, "vendors", vendor, slug), nil
}

// readWorkflowPluginFile reads a JSON bundle file from a plugin folder, mapping a missing
// file to the not-found sentinel.
func (a *adapter) readWorkflowPluginFile(vendor, slug, name string) (json.RawMessage, error) {
	dir, err := a.pluginDir(vendor, slug)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, repositories.ErrNotFound
	}
	return json.RawMessage(data), nil
}

// scanItems materializes index rows into catalog entities.
func scanItems(rows *sql.Rows) ([]entities.CatalogItem, error) {
	items := []entities.CatalogItem{}
	for rows.Next() {
		var (
			item                                     entities.CatalogItem
			capabilities, tags                       string
			requiresCredentials, hasEvents, hasImage int
		)
		if err := rows.Scan(
			&item.ID, &item.Vendor, &item.VendorName, &item.PluginID, &item.Slug, &item.NameEN, &item.NamePT,
			&item.DescriptionEN, &item.DescriptionPT, &item.Category, &capabilities, &tags, &item.Icon, &item.Color, &item.Image,
			&requiresCredentials, &item.NodeCount, &item.TriggerCount, &hasEvents, &hasImage,
		); err != nil {
			return nil, err
		}
		item.Capabilities = parseTokens(capabilities)
		item.Tags = parseTokens(tags)
		item.RequiresCredentials = requiresCredentials == 1
		item.HasEvents = hasEvents == 1
		item.HasImage = hasImage == 1
		items = append(items, item)
	}
	return items, rows.Err()
}
