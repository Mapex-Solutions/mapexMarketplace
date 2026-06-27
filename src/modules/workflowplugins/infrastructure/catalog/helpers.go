package catalog

import (
	"mime"
	"path/filepath"
	"strings"

	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
)

// wrapTokens encodes a token slice as a comma-wrapped string (",a,b,") so a
// single LIKE '%,value,%' tests membership. An empty slice encodes to "".
func wrapTokens(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return "," + strings.Join(tokens, ",") + ","
}

// parseTokens decodes a comma-wrapped string back into its tokens, dropping the
// empty edges. Always returns a non-nil (possibly empty) slice.
func parseTokens(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// boolToInt maps a bool to the SQLite integer the column stores.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// buildWhere assembles the WHERE clause and its args from a filter. Empty fields
// are skipped; it returns ("", nil) when no filter is set.
func buildWhere(filter repositories.CatalogFilter) (string, []any) {
	conditions := []string{}
	args := []any{}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}
	if filter.Capability != "" {
		conditions = append(conditions, "capabilities LIKE ?")
		args = append(args, "%,"+filter.Capability+",%")
	}
	if filter.Tag != "" {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%,"+filter.Tag+",%")
	}
	if filter.Search != "" {
		conditions = append(conditions, "(name_en LIKE ? OR name_pt LIKE ? OR description_en LIKE ? OR description_pt LIKE ? OR tags LIKE ?)")
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

// filterFacets keeps the catalog config options whose value is present in the
// index, preserving the config's label, icon and order.
func filterFacets(options []facetOption, present map[string]bool) []repositories.Facet {
	out := make([]repositories.Facet, 0, len(options))
	for _, o := range options {
		if present[o.Value] {
			out = append(out, repositories.Facet{Value: o.Value, Label: o.Label, Icon: o.Icon})
		}
	}
	return out
}
