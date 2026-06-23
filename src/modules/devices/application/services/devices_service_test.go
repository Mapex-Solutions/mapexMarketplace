package services

import (
	"context"
	"encoding/json"
	"testing"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"

	"mapexmarketplace/src/modules/devices/application/di"
	"mapexmarketplace/src/modules/devices/application/dtos"
	"mapexmarketplace/src/modules/devices/domain/entities"
	"mapexmarketplace/src/modules/devices/domain/repositories"
)

// mockCatalogRepo is an inline mock of the catalog repository port; each field
// drives the matching method's return, and lastFilter captures what Query saw.
type mockCatalogRepo struct {
	items      []entities.CatalogItem
	total      int
	lastFilter repositories.CatalogFilter
	facets     repositories.FacetSet
	raw        json.RawMessage
	rawErr     error
	reloadN    int
	codecs     []entities.Codec
}

func (m *mockCatalogRepo) Query(_ context.Context, f repositories.CatalogFilter) ([]entities.CatalogItem, int, error) {
	m.lastFilter = f
	return m.items, m.total, nil
}
func (m *mockCatalogRepo) Facets(_ context.Context) (repositories.FacetSet, error) {
	return m.facets, nil
}
func (m *mockCatalogRepo) GetInformation(_ context.Context, _, _ string) (json.RawMessage, error) {
	return m.raw, m.rawErr
}
func (m *mockCatalogRepo) GetSimulator(_ context.Context, _, _ string) (json.RawMessage, error) {
	return m.raw, m.rawErr
}
func (m *mockCatalogRepo) GetAsset(_ context.Context, _, _, _ string) ([]byte, string, error) {
	return []byte(m.raw), "text/plain", m.rawErr
}
func (m *mockCatalogRepo) Reload(_ context.Context) (int, error) {
	return m.reloadN, nil
}
func (m *mockCatalogRepo) ListCodecs(_ context.Context, _, _ string) ([]entities.Codec, error) {
	return m.codecs, m.rawErr
}

func TestDevicesService_ListResolvesPaging(t *testing.T) {
	tests := []struct {
		name        string
		query       *dtos.CatalogQuery
		wantLimit   int
		wantOffset  int
		wantPage    int
		wantPerPage int
	}{
		{name: "defaults", query: &dtos.CatalogQuery{}, wantLimit: 24, wantOffset: 0, wantPage: 1, wantPerPage: 24},
		{name: "second page", query: &dtos.CatalogQuery{Page: 2, PerPage: 10}, wantLimit: 10, wantOffset: 10, wantPage: 2, wantPerPage: 10},
		{name: "perPage capped", query: &dtos.CatalogQuery{Page: 1, PerPage: 500}, wantLimit: 100, wantOffset: 0, wantPage: 1, wantPerPage: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCatalogRepo{}
			svc := New(di.DevicesServiceDI{Repo: repo})
			res, err := svc.List(context.Background(), tt.query)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if repo.lastFilter.Limit != tt.wantLimit || repo.lastFilter.Offset != tt.wantOffset {
				t.Fatalf("filter limit/offset = %d/%d, want %d/%d", repo.lastFilter.Limit, repo.lastFilter.Offset, tt.wantLimit, tt.wantOffset)
			}
			if res.Page != tt.wantPage || res.PerPage != tt.wantPerPage {
				t.Fatalf("response page/perPage = %d/%d, want %d/%d", res.Page, res.PerPage, tt.wantPage, tt.wantPerPage)
			}
		})
	}
}

func TestDevicesService_ListPassesFiltersAndMapsItems(t *testing.T) {
	repo := &mockCatalogRepo{
		total: 1,
		items: []entities.CatalogItem{{
			ID: "milesight-em300-th", Vendor: "milesight", VendorName: "Milesight", Model: "EM300-TH",
			Slug: "em300-th", Protocol: "lorawan", ReadingTypes: []string{"temperature", "humidity"},
		}},
	}
	svc := New(di.DevicesServiceDI{Repo: repo})
	res, err := svc.List(context.Background(), &dtos.CatalogQuery{Protocol: "lorawan", ReadingType: "humidity", Manufacturer: "Milesight", Search: "em"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.lastFilter.Protocol != "lorawan" || repo.lastFilter.ReadingType != "humidity" ||
		repo.lastFilter.Manufacturer != "Milesight" || repo.lastFilter.Search != "em" {
		t.Fatalf("filters not passed through: %+v", repo.lastFilter)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].Slug != "em300-th" ||
		len(res.Items[0].ReadingTypes) != 2 {
		t.Fatalf("unexpected mapped response: %+v", res)
	}
}

func TestDevicesService_GetInformationMapsNotFound(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		wantErr bool
		want404 bool
	}{
		{name: "found", repoErr: nil, wantErr: false},
		{name: "not found maps to 404", repoErr: repositories.ErrNotFound, wantErr: true, want404: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockCatalogRepo{raw: json.RawMessage(`{"model":"EM300-TH"}`), rawErr: tt.repoErr}
			svc := New(di.DevicesServiceDI{Repo: repo})
			_, err := svc.GetInformation(context.Background(), "milesight", "em300-th")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.want404 {
					serverErr, ok := err.(*customErrors.ServerCustomError)
					if !ok || serverErr.Code != status.NOT_FOUND {
						t.Fatalf("expected 404 ServerCustomError, got %T %v", err, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDevicesService_CodecsMapsToDTO(t *testing.T) {
	repo := &mockCatalogRepo{codecs: []entities.Codec{
		{ID: "milesight-official", Name: "Milesight (official)", Official: true, Default: true, Target: "ttn", Path: "codecs/milesight-official"},
		{ID: "chirpstack-v4", Name: "ChirpStack v4", Source: "community", Target: "chirpstack", Path: "codecs/chirpstack-v4"},
	}}
	svc := New(di.DevicesServiceDI{Repo: repo})
	out, err := svc.Codecs(context.Background(), "milesight", "em300-th")
	if err != nil {
		t.Fatalf("Codecs: %v", err)
	}
	if len(out) != 2 || out[0].ID != "milesight-official" || !out[0].Official || out[1].Target != "chirpstack" {
		t.Fatalf("unexpected codecs mapping: %+v", out)
	}
}
