// Package bundle holds the cross-module primitives for serving marketplace
// catalog bundles: the directory-traversal guard for bundle-asset reads and the
// verifiable raw-bytes response. Every catalog module (devices, asset templates,
// workflow plugins) shares these so a hardening fix lands in one place instead of
// drifting across per-module copies.
package bundle

import "path/filepath"

// PathWithin reports whether target sits inside base (after cleaning). It is the
// single source for the "../ escape" check that guards every catalog module's
// GetAsset against reading files outside the resolved bundle folder.
func PathWithin(base, target string) bool {
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
