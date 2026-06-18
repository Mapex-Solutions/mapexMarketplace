package catalog

import (
	"testing"

	"mapexmarketplace/src/modules/devices/domain/repositories"
)

func TestWrapParseTokens_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantRaw string
	}{
		{name: "empty", tokens: nil, wantRaw: ""},
		{name: "single", tokens: []string{"temperature"}, wantRaw: ",temperature,"},
		{name: "many", tokens: []string{"temperature", "humidity"}, wantRaw: ",temperature,humidity,"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := wrapTokens(tt.tokens)
			if raw != tt.wantRaw {
				t.Fatalf("wrapTokens = %q, want %q", raw, tt.wantRaw)
			}
			got := parseTokens(raw)
			if len(got) != len(tt.tokens) {
				t.Fatalf("parseTokens len = %d, want %d", len(got), len(tt.tokens))
			}
			for i := range tt.tokens {
				if got[i] != tt.tokens[i] {
					t.Fatalf("parseTokens[%d] = %q, want %q", i, got[i], tt.tokens[i])
				}
			}
		})
	}
}

func TestBuildWhere(t *testing.T) {
	tests := []struct {
		name      string
		filter    repositories.CatalogFilter
		wantWhere string
		wantArgs  int
	}{
		{name: "empty", filter: repositories.CatalogFilter{}, wantWhere: "", wantArgs: 0},
		{name: "protocol", filter: repositories.CatalogFilter{Protocol: "lorawan"}, wantWhere: " WHERE protocol = ?", wantArgs: 1},
		{name: "reading type", filter: repositories.CatalogFilter{ReadingType: "humidity"}, wantWhere: " WHERE reading_types LIKE ?", wantArgs: 1},
		{name: "search expands across both names, both descriptions and model", filter: repositories.CatalogFilter{Search: "em"}, wantWhere: " WHERE (name_en LIKE ? OR name_pt LIKE ? OR description_en LIKE ? OR description_pt LIKE ? OR model LIKE ?)", wantArgs: 5},
		{name: "combined", filter: repositories.CatalogFilter{Protocol: "lorawan", Manufacturer: "Milesight"}, wantWhere: " WHERE protocol = ? AND vendor_name = ?", wantArgs: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			where, args := buildWhere(tt.filter)
			if where != tt.wantWhere {
				t.Fatalf("where = %q, want %q", where, tt.wantWhere)
			}
			if len(args) != tt.wantArgs {
				t.Fatalf("args = %d, want %d", len(args), tt.wantArgs)
			}
		})
	}
}

func TestSafeSegment(t *testing.T) {
	tests := []struct {
		seg  string
		want bool
	}{
		{seg: "milesight", want: true},
		{seg: "em300-th", want: true},
		{seg: "", want: false},
		{seg: "..", want: false},
		{seg: "a/b", want: false},
		{seg: "a\\b", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.seg, func(t *testing.T) {
			if got := safeSegment(tt.seg); got != tt.want {
				t.Fatalf("safeSegment(%q) = %v, want %v", tt.seg, got, tt.want)
			}
		})
	}
}

func TestContentTypeFor(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "codec/decoder.js", want: "text/javascript; charset=utf-8"},
		{path: "device_information.json", want: "application/json; charset=utf-8"},
		{path: "images/logo.svg", want: "image/svg+xml"},
		{path: "manual/file.bin", want: "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := contentTypeFor(tt.path); got != tt.want {
				t.Fatalf("contentTypeFor(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
