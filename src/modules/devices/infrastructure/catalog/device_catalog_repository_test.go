package catalog

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	sqliteManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/sqlite/manager"

	"mapexmarketplace/src/modules/devices/domain/repositories"
)

// newRealCatalogAdapter builds the catalog adapter over a throwaway SQLite file
// and indexes the repository's real devices catalog, so the facet queries run
// against the actual committed data rather than a hand-made fixture.
func newRealCatalogAdapter(t *testing.T) *adapter {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	// pkg dir: src/modules/devices/infrastructure/catalog -> repo root is five up.
	devicesDir := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "catalog", "devices")

	mgr, err := sqliteManager.New(sqliteManager.Config{Path: filepath.Join(t.TempDir(), "index.db"), ForeignKeys: false})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	a := &adapter{db: mgr.DB(), dir: devicesDir}
	if _, err := a.Reload(context.Background()); err != nil {
		t.Fatalf("reload index: %v", err)
	}
	return a
}

// TestFacets_ModelsDrillDownByManufacturer proves the vendor -> model drill-down:
// the Models facet narrows to the picked manufacturer, the orthogonal facets do
// not, and an unknown manufacturer yields no models.
func TestFacets_ModelsDrillDownByManufacturer(t *testing.T) {
	a := newRealCatalogAdapter(t)
	ctx := context.Background()

	all, err := a.Facets(ctx, repositories.FacetSelection{})
	if err != nil {
		t.Fatalf("facets (top level): %v", err)
	}
	if len(all.Manufacturers) == 0 || len(all.Models) == 0 {
		t.Fatalf("expected non-empty manufacturers and models, got %d/%d", len(all.Manufacturers), len(all.Models))
	}

	vendor := all.Manufacturers[0].Value
	narrowed, err := a.Facets(ctx, repositories.FacetSelection{Manufacturer: vendor})
	if err != nil {
		t.Fatalf("facets (manufacturer=%q): %v", vendor, err)
	}

	// The drill-down level must shrink and stay a subset of the full model set.
	if len(narrowed.Models) == 0 {
		t.Fatalf("expected models for %q, got none", vendor)
	}
	if len(narrowed.Models) >= len(all.Models) {
		t.Fatalf("expected narrowed models (%d) < all models (%d)", len(narrowed.Models), len(all.Models))
	}
	allModels := make(map[string]bool, len(all.Models))
	for _, m := range all.Models {
		allModels[m.Value] = true
	}
	for _, m := range narrowed.Models {
		if !allModels[m.Value] {
			t.Errorf("narrowed model %q is not in the full model set", m.Value)
		}
	}

	// Orthogonal facets are unaffected by the drill-down selection.
	if len(narrowed.Manufacturers) != len(all.Manufacturers) {
		t.Errorf("manufacturers must not narrow: all=%d narrowed=%d", len(all.Manufacturers), len(narrowed.Manufacturers))
	}
	if len(narrowed.Protocols) != len(all.Protocols) {
		t.Errorf("protocols must not narrow: all=%d narrowed=%d", len(all.Protocols), len(narrowed.Protocols))
	}

	// An unknown manufacturer yields no models (no empty-combo leakage).
	none, err := a.Facets(ctx, repositories.FacetSelection{Manufacturer: "__no_such_vendor__"})
	if err != nil {
		t.Fatalf("facets (unknown manufacturer): %v", err)
	}
	if len(none.Models) != 0 {
		t.Errorf("expected 0 models for unknown manufacturer, got %d", len(none.Models))
	}
}
