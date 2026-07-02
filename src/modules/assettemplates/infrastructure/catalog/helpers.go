package catalog

import (
	"mime"
	"path/filepath"
	"strings"

	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
)

// boolToInt maps a bool to the SQLite integer the column stores.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// buildWhere assembles the WHERE clause and its args from a filter. Empty fields
// are skipped; it returns ("", nil) when no filter is set. Search matches the
// name, description and model columns (cross-column LIKE).
func buildWhere(filter repositories.CatalogFilter) (string, []any) {
	conditions := []string{}
	args := []any{}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}
	if filter.Vendor != "" {
		conditions = append(conditions, "vendor = ?")
		args = append(args, filter.Vendor)
	}
	if filter.Model != "" {
		conditions = append(conditions, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Version != "" {
		conditions = append(conditions, "version = ?")
		args = append(args, filter.Version)
	}
	if filter.Search != "" {
		conditions = append(conditions, "(name_en LIKE ? OR name_pt LIKE ? OR description_en LIKE ? OR description_pt LIKE ? OR model LIKE ?)")
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like, like, like)
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// safeSegment guards a path segment (vendor / slug) against traversal: a single
// clean name with no separators.
func safeSegment(seg string) bool {
	if seg == "" || seg == "." || seg == ".." {
		return false
	}
	return !strings.ContainsAny(seg, "/\\")
}

// contentTypeFor resolves a file's content type from its extension, defaulting to
// octet-stream for unknown kinds.
func contentTypeFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	}
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}

// filterCategoryFacets keeps the catalog config categories whose value is present
// in the index, resolving each label to the requested locale (en-US fallback) and
// preserving the config's icon and order.
func filterCategoryFacets(options []categoryOption, present map[string]bool, lang string) []repositories.Facet {
	out := make([]repositories.Facet, 0, len(options))
	for _, o := range options {
		if present[o.Value] {
			out = append(out, repositories.Facet{Value: o.Value, Label: o.Label.get(lang), Icon: o.Icon})
		}
	}
	return out
}
