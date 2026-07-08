package catalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/assettemplates/domain/entities"
	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
	"mapexmarketplace/src/shared/bundle"
)

// The catalog index is a bespoke search store, not entity CRUD: its queries need
// cross-column OR search and DISTINCT facets that the generic sqlite model
// (equality-only filters) cannot express. So this adapter uses the manager's
// *sql.DB directly — a deliberate, documented exception to the "wrappers only"
// rule, scoped to this read-only derived index.

// Compile-time proof the adapter satisfies the repository port.
var _ repositories.AssetTemplateCatalogRepository = (*adapter)(nil)

// New builds the asset template catalog repository over the shared SQLite index
// and the JSON catalog directory. The index is empty until Reload runs (in the
// module's repository init), keeping construction free of I/O.
func New(mgr *sqliteManager.SQLiteManager, dir string) repositories.AssetTemplateCatalogRepository {
	return &adapter{db: mgr.DB(), dir: dir}
}

// Reload rebuilds the index from the per-vendor manifests: ensure the table,
// clear it, reload the catalog config, then insert every template in one
// transaction so a failed reload never leaves the index half-populated.
func (a *adapter) Reload(ctx context.Context) (int, error) {
	// Drop then recreate so the index always matches the current schema; an older
	// DB file would otherwise survive CREATE IF NOT EXISTS and fail the insert.
	if _, err := a.db.ExecContext(ctx, dropAssetTemplateCatalog); err != nil {
		return 0, err
	}
	if _, err := a.db.ExecContext(ctx, ddlAssetTemplateCatalog); err != nil {
		return 0, err
	}
	a.loadConfig()
	items, err := a.readManifests()
	if err != nil {
		return 0, err
	}
	a.warnUnknownCategories(items)
	a.verifyChecksums(items)
	count, err := a.replaceIndex(ctx, items)
	if err != nil {
		return 0, err
	}
	logger.Info(fmt.Sprintf("[REPO:AssetTemplateCatalog] indexed count=%d dir=%s", count, a.dir))
	return count, nil
}

// Query returns the page of items matching the filter plus the total match count.
func (a *adapter) Query(ctx context.Context, filter repositories.CatalogFilter) ([]entities.CatalogItem, int, error) {
	where, args := buildWhere(filter)

	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableAssetTemplateCatalog+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := "SELECT " + selectColumns + " FROM " + tableAssetTemplateCatalog + where + " ORDER BY name_en LIMIT ? OFFSET ?"
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

// Facets returns the filter options actually present in the index. Categories is
// the flat facet labelled and ordered by the catalog config (resolved by lang);
// vendors/models/versions are the three drill-down levels, each narrowed by the
// level above it.
func (a *adapter) Facets(ctx context.Context, sel repositories.FacetSelection) (repositories.FacetSet, error) {
	categories, err := a.presentColumn(ctx, "category")
	if err != nil {
		return repositories.FacetSet{}, err
	}
	vendors, err := a.distinctVendors(ctx)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	models, err := a.distinctModels(ctx, sel.Vendor)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	versions, err := a.distinctVersions(ctx, sel.Vendor, sel.Model)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	return repositories.FacetSet{
		Categories: filterCategoryFacets(a.config.Categories, categories, sel.Lang),
		Vendors:    vendors,
		Models:     models,
		Versions:   versions,
	}, nil
}

// GetInformation returns the template's information sheet read lazily from disk.
func (a *adapter) GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	return a.readTemplateFile(vendor, slug, fileInformation)
}

// GetItem returns the single indexed row for one template, or ErrNotFound. It
// reuses the listing column list and scanner for consistency, with an exact
// (vendor, slug) match instead of the filtered/paginated list query.
func (a *adapter) GetItem(ctx context.Context, vendor, slug string) (*entities.CatalogItem, error) {
	rows, err := a.db.QueryContext(ctx,
		"SELECT "+selectColumns+" FROM "+tableAssetTemplateCatalog+" WHERE vendor = ? AND slug = ? LIMIT 1",
		vendor, slug,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, repositories.ErrNotFound
	}
	return &items[0], nil
}

// GetAsset returns a bundle asset (icon, image) and its content type, guarding
// the asset path against traversal outside the template folder.
func (a *adapter) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	dir, err := a.templateDir(vendor, slug)
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
		logger.Error(err, "[REPO:AssetTemplateCatalog] parse catalog config")
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
			logger.Error(err, "[REPO:AssetTemplateCatalog] read manifest "+file)
			continue
		}
		var manifest vendorCatalog
		if err := json.Unmarshal(data, &manifest); err != nil {
			logger.Error(err, "[REPO:AssetTemplateCatalog] parse manifest "+file)
			continue
		}
		for _, item := range manifest.Items {
			items = append(items, entities.CatalogItem{
				ID:              item.ID,
				MarketplaceGuid: item.MarketplaceGuid,
				Vendor:          manifest.Vendor.Slug,
				VendorName:      manifest.Vendor.Name,
				Model:           item.Model,
				Version:         item.Version,
				Slug:            item.Slug,
				NameEN:          item.Name.get(defaultLang),
				NamePT:          item.Name.get("pt-BR"),
				DescriptionEN:   item.Description.get(defaultLang),
				DescriptionPT:   item.Description.get("pt-BR"),
				Category:        item.Category,
				Icon:            item.Icon,
				Image:           item.Image,
				HasImage:        item.HasImage,
				FieldCount:      item.FieldCount,
				HasScripts:      item.HasScripts,
				Sha256:          item.Sha256,
			})
		}
	}
	return items, nil
}

// verifyChecksums warns when an indexed template's published sha256 no longer
// matches its on-disk information sheet. That sha256 is served as an integrity
// header the installer hard-verifies against the exact bytes it receives, so a
// drift (a bundle edited without regenerating catalog.json) would make every
// client reject an otherwise-valid template. Catching it at index build surfaces
// the drift in the logs before any install starts failing.
func (a *adapter) verifyChecksums(items []entities.CatalogItem) {
	for i := range items {
		item := items[i]
		if item.Sha256 == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.dir, "vendors", item.Vendor, item.Slug, fileInformation))
		if err != nil {
			continue
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(data))
		if actual != item.Sha256 {
			logger.Warn(fmt.Sprintf("[REPO:AssetTemplateCatalog] sha256 drift for %s/%s: catalog=%s file=%s (regenerate catalog.json)", item.Vendor, item.Slug, item.Sha256, actual))
		}
	}
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
		if _, err := tx.ExecContext(ctx, insertAssetTemplateCatalog,
			item.ID, item.MarketplaceGuid, item.Vendor, item.VendorName, item.Model, item.Version, item.Slug,
			item.NameEN, item.NamePT, item.DescriptionEN, item.DescriptionPT, item.Category,
			item.Icon, item.Image, boolToInt(item.HasImage), item.FieldCount, boolToInt(item.HasScripts), item.Sha256,
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

// distinctVendors returns the vendors present in the index as facets, labelled by
// vendor name. This is the top of the drill-down (vendor -> model -> version).
func (a *adapter) distinctVendors(ctx context.Context) ([]repositories.Facet, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT DISTINCT vendor, vendor_name FROM "+tableAssetTemplateCatalog+" ORDER BY vendor_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facets := []repositories.Facet{}
	for rows.Next() {
		var value, label string
		if err := rows.Scan(&value, &label); err != nil {
			return nil, err
		}
		facets = append(facets, repositories.Facet{Value: value, Label: label})
	}
	return facets, rows.Err()
}

// distinctModels returns the model names present in the index as facets, the
// second drill-down level. A non-empty vendor narrows the list to that vendor
// (drill-down: vendor -> model); empty returns every model.
func (a *adapter) distinctModels(ctx context.Context, vendor string) ([]repositories.Facet, error) {
	query := "SELECT DISTINCT model FROM " + tableAssetTemplateCatalog
	args := []any{}
	if vendor != "" {
		query += " WHERE vendor = ?"
		args = append(args, vendor)
	}
	query += " ORDER BY model"

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facets := []repositories.Facet{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		facets = append(facets, repositories.Facet{Value: name, Label: name})
	}
	return facets, rows.Err()
}

// distinctVersions returns the versions present in the index as facets, the third
// drill-down level. A non-empty model narrows the list to that model (and vendor
// when set); empty returns every version.
func (a *adapter) distinctVersions(ctx context.Context, vendor, model string) ([]repositories.Facet, error) {
	query := "SELECT DISTINCT version FROM " + tableAssetTemplateCatalog
	conditions := []string{}
	args := []any{}
	if vendor != "" {
		conditions = append(conditions, "vendor = ?")
		args = append(args, vendor)
	}
	if model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, model)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY version"

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facets := []repositories.Facet{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		facets = append(facets, repositories.Facet{Value: name, Label: name})
	}
	return facets, rows.Err()
}

// presentColumn returns the distinct non-empty values of a scalar column. The
// column name is an internal constant, never user input.
func (a *adapter) presentColumn(ctx context.Context, column string) (map[string]bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT DISTINCT "+column+" FROM "+tableAssetTemplateCatalog)
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

// warnUnknownCategories logs categories a template declares that are absent from
// the catalog config, keeping the vocabulary consistent without failing the
// reload.
func (a *adapter) warnUnknownCategories(items []entities.CatalogItem) {
	known := map[string]bool{}
	for _, option := range a.config.Categories {
		known[option.Value] = true
	}
	for _, item := range items {
		if item.Category != "" && !known[item.Category] {
			logger.Warn(fmt.Sprintf("[REPO:AssetTemplateCatalog] unknown category=%q id=%s (add it to catalog_config.json)", item.Category, item.ID))
		}
	}
}

// templateDir resolves a template's folder, rejecting unsafe vendor/slug segments.
func (a *adapter) templateDir(vendor, slug string) (string, error) {
	if !safeSegment(vendor) || !safeSegment(slug) {
		return "", repositories.ErrNotFound
	}
	return filepath.Join(a.dir, "vendors", vendor, slug), nil
}

// readTemplateFile reads a JSON bundle file from a template folder, mapping a
// missing file to the not-found sentinel.
func (a *adapter) readTemplateFile(vendor, slug, name string) (json.RawMessage, error) {
	dir, err := a.templateDir(vendor, slug)
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
			item                 entities.CatalogItem
			hasImage, hasScripts int
		)
		if err := rows.Scan(
			&item.ID, &item.MarketplaceGuid, &item.Vendor, &item.VendorName, &item.Model, &item.Version, &item.Slug,
			&item.NameEN, &item.NamePT, &item.DescriptionEN, &item.DescriptionPT, &item.Category,
			&item.Icon, &item.Image, &hasImage, &item.FieldCount, &hasScripts, &item.Sha256,
		); err != nil {
			return nil, err
		}
		item.HasImage = hasImage == 1
		item.HasScripts = hasScripts == 1
		items = append(items, item)
	}
	return items, rows.Err()
}
