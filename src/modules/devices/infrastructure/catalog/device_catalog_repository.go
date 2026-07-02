package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"mapexmarketplace/src/modules/devices/domain/entities"
	"mapexmarketplace/src/modules/devices/domain/repositories"
)

// The catalog index is a bespoke search store, not entity CRUD: its queries need
// membership LIKE, cross-column OR search and DISTINCT facets that the generic
// sqlite model (equality-only filters) cannot express. So this adapter uses the
// manager's *sql.DB directly — a deliberate, documented exception to the
// "wrappers only" rule, scoped to this read-only derived index.

// Compile-time proof the adapter satisfies the repository port.
var _ repositories.DeviceCatalogRepository = (*adapter)(nil)

// New builds the device catalog repository over the shared SQLite index and the
// JSON catalog directory. The index is empty until Reload runs (in the module's
// repository init), keeping construction free of I/O.
func New(mgr *sqliteManager.SQLiteManager, dir string) repositories.DeviceCatalogRepository {
	return &adapter{db: mgr.DB(), dir: dir}
}

// Reload rebuilds the index from the per-vendor manifests: ensure the table,
// clear it, reload the catalog config, then insert every model in one
// transaction so a failed reload never leaves the index half-populated.
func (a *adapter) Reload(ctx context.Context) (int, error) {
	// Drop then recreate so the index always matches the current schema; an older
	// DB file (pre-i18n columns) would otherwise survive CREATE IF NOT EXISTS and
	// fail the insert with "no column named name_en".
	if _, err := a.db.ExecContext(ctx, dropDeviceCatalog); err != nil {
		return 0, err
	}
	if _, err := a.db.ExecContext(ctx, ddlDeviceCatalog); err != nil {
		return 0, err
	}
	a.loadConfig()
	items, err := a.readManifests()
	if err != nil {
		return 0, err
	}
	a.warnUnknownReadingTypes(items)
	count, err := a.replaceIndex(ctx, items)
	if err != nil {
		return 0, err
	}
	logger.Info(fmt.Sprintf("[REPO:DeviceCatalog] indexed count=%d dir=%s", count, a.dir))
	return count, nil
}

// Query returns the page of items matching the filter plus the total match count.
func (a *adapter) Query(ctx context.Context, filter repositories.CatalogFilter) ([]entities.CatalogItem, int, error) {
	where, args := buildWhere(filter)

	var total int
	if err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableDeviceCatalog+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := "SELECT " + selectColumns + " FROM " + tableDeviceCatalog + where + " ORDER BY name_en LIMIT ? OFFSET ?"
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
// ordered by the catalog config. Protocols/reading types with zero devices are
// omitted so the UI never shows a filter that yields nothing.
func (a *adapter) Facets(ctx context.Context, sel repositories.FacetSelection) (repositories.FacetSet, error) {
	protocols, err := a.presentColumn(ctx, "protocol")
	if err != nil {
		return repositories.FacetSet{}, err
	}
	readingTypes, err := a.presentReadingTypes(ctx)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	manufacturers, err := a.distinctManufacturers(ctx)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	models, err := a.distinctModels(ctx, sel.Manufacturer)
	if err != nil {
		return repositories.FacetSet{}, err
	}
	return repositories.FacetSet{
		Protocols:     filterFacets(a.config.Protocols, protocols),
		ReadingTypes:  filterFacets(a.config.ReadingTypes, readingTypes),
		Manufacturers: manufacturers,
		Models:        models,
	}, nil
}

// ListCodecs reads the codecs/{id}/codec.json folders for a model, ordered with
// the default first, then official, then by name.
func (a *adapter) ListCodecs(ctx context.Context, vendor, slug string) ([]entities.Codec, error) {
	dir, err := a.modelDir(vendor, slug)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(dir, dirCodecs, "*", fileCodec))
	if err != nil {
		return nil, err
	}
	codecs := []entities.Codec{}
	for _, file := range matches {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var manifest codecManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			logger.Error(err, "[REPO:DeviceCatalog] parse codec "+file)
			continue
		}
		codecs = append(codecs, entities.Codec{
			ID:          manifest.ID,
			Name:        manifest.Name,
			Source:      manifest.Source,
			Official:    manifest.Official,
			Default:     manifest.Default,
			Target:      manifest.Target,
			Language:    manifest.Language,
			SourceURL:   manifest.SourceURL,
			Path:        filepath.ToSlash(filepath.Join(dirCodecs, filepath.Base(filepath.Dir(file)))),
			DecoderFile: manifest.Files.Decoder,
			EncoderFile: manifest.Files.Encoder,
		})
	}
	sortCodecs(codecs)
	return codecs, nil
}

// GetInformation returns the model's information sheet read lazily from disk.
func (a *adapter) GetInformation(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	return a.readModelFile(vendor, slug, fileInformation)
}

// GetSimulator returns the model's install template read lazily from disk.
func (a *adapter) GetSimulator(ctx context.Context, vendor, slug string) (json.RawMessage, error) {
	return a.readModelFile(vendor, slug, fileSimulator)
}

// GetAsset returns a bundle asset (codec, manual, image) and its content type,
// guarding the asset path against traversal outside the model folder.
func (a *adapter) GetAsset(ctx context.Context, vendor, slug, name string) ([]byte, string, error) {
	dir, err := a.modelDir(vendor, slug)
	if err != nil {
		return nil, "", err
	}
	full := filepath.Join(dir, filepath.Clean("/"+name))
	if full != dir && !pathWithin(dir, full) {
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
		logger.Error(err, "[REPO:DeviceCatalog] parse catalog config")
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
			logger.Error(err, "[REPO:DeviceCatalog] read manifest "+file)
			continue
		}
		var manifest vendorCatalog
		if err := json.Unmarshal(data, &manifest); err != nil {
			logger.Error(err, "[REPO:DeviceCatalog] parse manifest "+file)
			continue
		}
		for _, item := range manifest.Items {
			items = append(items, entities.CatalogItem{
				ID:            item.ID,
				Vendor:        manifest.Vendor.Slug,
				VendorName:    manifest.Vendor.Name,
				Model:         item.Model,
				Slug:          item.Slug,
				NameEN:        item.Name.get(defaultLang),
				NamePT:        item.Name.get("pt-BR"),
				DescriptionEN: item.Description.get(defaultLang),
				DescriptionPT: item.Description.get("pt-BR"),
				Protocol:      item.Protocol,
				ReadingTypes:  item.ReadingTypes,
				Tags:          item.Tags,
				Icon:          item.Icon,
				Image:         item.Image,
				HasCodec:      item.HasCodec,
				HasManual:     item.HasManual,
			})
		}
	}
	return items, nil
}

// replaceIndex clears the table and inserts every item in one transaction.
func (a *adapter) replaceIndex(ctx context.Context, items []entities.CatalogItem) (int, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+tableDeviceCatalog); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	for i := range items {
		item := items[i]
		if _, err := tx.ExecContext(ctx, insertDeviceCatalog,
			item.ID, item.Vendor, item.VendorName, item.Model, item.Slug, item.NameEN, item.NamePT,
			item.DescriptionEN, item.DescriptionPT, item.Protocol, wrapTokens(item.ReadingTypes), wrapTokens(item.Tags),
			item.Icon, item.Image, boolToInt(item.HasCodec), boolToInt(item.HasManual),
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

// distinctManufacturers returns the vendor names present in the index as facets.
func (a *adapter) distinctManufacturers(ctx context.Context) ([]repositories.Facet, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT DISTINCT vendor_name FROM "+tableDeviceCatalog+" ORDER BY vendor_name")
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

// distinctModels returns the model names present in the index as facets, the
// second drill-down level. A non-empty manufacturer narrows the list to that
// vendor (drill-down: vendor -> model); empty returns every model.
func (a *adapter) distinctModels(ctx context.Context, manufacturer string) ([]repositories.Facet, error) {
	query := "SELECT DISTINCT model FROM " + tableDeviceCatalog
	args := []any{}
	if manufacturer != "" {
		query += " WHERE vendor_name = ?"
		args = append(args, manufacturer)
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

// presentColumn returns the distinct non-empty values of a scalar column. The
// column name is an internal constant, never user input.
func (a *adapter) presentColumn(ctx context.Context, column string) (map[string]bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT DISTINCT "+column+" FROM "+tableDeviceCatalog)
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

// presentReadingTypes returns the set of reading types present across all rows,
// splitting the comma-wrapped column.
func (a *adapter) presentReadingTypes(ctx context.Context) (map[string]bool, error) {
	rows, err := a.db.QueryContext(ctx, "SELECT reading_types FROM "+tableDeviceCatalog)
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

// warnUnknownReadingTypes logs reading types a device declares that are absent
// from the catalog config, keeping the vocabulary consistent without failing the
// reload.
func (a *adapter) warnUnknownReadingTypes(items []entities.CatalogItem) {
	known := map[string]bool{}
	for _, option := range a.config.ReadingTypes {
		known[option.Value] = true
	}
	for _, item := range items {
		for _, readingType := range item.ReadingTypes {
			if !known[readingType] {
				logger.Warn(fmt.Sprintf("[REPO:DeviceCatalog] unknown readingType=%q id=%s (add it to catalog_config.json)", readingType, item.ID))
			}
		}
	}
}

// sortCodecs orders codecs with the default first, then official, then by name.
func sortCodecs(codecs []entities.Codec) {
	sort.SliceStable(codecs, func(i, j int) bool {
		if codecs[i].Default != codecs[j].Default {
			return codecs[i].Default
		}
		if codecs[i].Official != codecs[j].Official {
			return codecs[i].Official
		}
		return codecs[i].Name < codecs[j].Name
	})
}

// modelDir resolves a model's folder, rejecting unsafe vendor/slug segments.
func (a *adapter) modelDir(vendor, slug string) (string, error) {
	if !safeSegment(vendor) || !safeSegment(slug) {
		return "", repositories.ErrNotFound
	}
	return filepath.Join(a.dir, "vendors", vendor, slug), nil
}

// readModelFile reads a JSON bundle file from a model folder, mapping a missing
// file to the not-found sentinel.
func (a *adapter) readModelFile(vendor, slug, name string) (json.RawMessage, error) {
	dir, err := a.modelDir(vendor, slug)
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
			item                entities.CatalogItem
			readingTypes, tags  string
			hasCodec, hasManual int
		)
		if err := rows.Scan(
			&item.ID, &item.Vendor, &item.VendorName, &item.Model, &item.Slug, &item.NameEN, &item.NamePT,
			&item.DescriptionEN, &item.DescriptionPT, &item.Protocol, &readingTypes, &tags, &item.Icon, &item.Image, &hasCodec, &hasManual,
		); err != nil {
			return nil, err
		}
		item.ReadingTypes = parseTokens(readingTypes)
		item.Tags = parseTokens(tags)
		item.HasCodec = hasCodec == 1
		item.HasManual = hasManual == 1
		items = append(items, item)
	}
	return items, rows.Err()
}

// pathWithin reports whether target sits inside base (after cleaning), guarding
// asset reads against directory traversal.
func pathWithin(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) && !hasParentPrefix(rel)
}

// hasParentPrefix reports whether a relative path escapes upward ("../...").
func hasParentPrefix(rel string) bool {
	return rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator))
}
