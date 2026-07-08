package services

import (
	"context"
	"encoding/json"
	"testing"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"

	"mapexmarketplace/src/modules/assettemplates/application/di"
	"mapexmarketplace/src/modules/assettemplates/application/dtos"
	"mapexmarketplace/src/modules/assettemplates/domain/entities"
	"mapexmarketplace/src/modules/assettemplates/domain/repositories"
)

// mockCatalogRepo is an inline mock of the catalog repository port; each field
// drives the matching method's return, and lastFilter/lastFacetSel capture the
// arguments the service passed down.
type mockCatalogRepo struct {
	items        []entities.CatalogItem
	total        int
	lastFilter   repositories.CatalogFilter
	facets       repositories.FacetSet
	lastFacetSel repositories.FacetSelection
	raw          json.RawMessage
	rawErr       error
	asset        []byte
	assetType    string
	reloadN      int
}

func (m *mockCatalogRepo) Query(_ context.Context, f repositories.CatalogFilter) ([]entities.CatalogItem, int, error) {
	m.lastFilter = f
	return m.items, m.total, nil
}
func (m *mockCatalogRepo) Facets(_ context.Context, sel repositories.FacetSelection) (repositories.FacetSet, error) {
	m.lastFacetSel = sel
	return m.facets, nil
}
func (m *mockCatalogRepo) GetInformation(_ context.Context, _, _ string) (json.RawMessage, error) {
	return m.raw, m.rawErr
}
func (m *mockCatalogRepo) GetItem(_ context.Context, _, _ string) (*entities.CatalogItem, error) {
	if m.rawErr != nil {
		return nil, m.rawErr
	}
	return &entities.CatalogItem{MarketplaceGuid: "guid-x", Sha256: "sha-x"}, nil
}
func (m *mockCatalogRepo) GetAsset(_ context.Context, _, _, _ string) ([]byte, string, error) {
	return m.asset, m.assetType, m.rawErr
}
func (m *mockCatalogRepo) Reload(_ context.Context) (int, error) {
	return m.reloadN, nil
}

func TestAssetTemplatesService_ListResolvesPaging(t *testing.T) {
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
			svc := New(di.AssetTemplatesServiceDI{Repo: repo})
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

func TestAssetTemplatesService_ListPassesFiltersAndMapsItems(t *testing.T) {
	repo := &mockCatalogRepo{
		total: 1,
		items: []entities.CatalogItem{{
			ID: "disruptive-technologies-temperature", Vendor: "disruptive-technologies", VendorName: "Disruptive Technologies",
			Model: "Temperature", Slug: "temperature", Category: "sensor", Version: "1", NameEN: "Temperature", FieldCount: 3, HasScripts: true,
		}},
	}
	svc := New(di.AssetTemplatesServiceDI{Repo: repo})
	res, err := svc.List(context.Background(), &dtos.CatalogQuery{
		Category: "sensor", Vendor: "disruptive-technologies", Model: "Temperature", Version: "1", Search: "temp",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.lastFilter.Category != "sensor" || repo.lastFilter.Vendor != "disruptive-technologies" ||
		repo.lastFilter.Model != "Temperature" || repo.lastFilter.Version != "1" || repo.lastFilter.Search != "temp" {
		t.Fatalf("filters not passed through: %+v", repo.lastFilter)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].Slug != "temperature" ||
		res.Items[0].FieldCount != 3 || !res.Items[0].HasScripts {
		t.Fatalf("unexpected mapped response: %+v", res)
	}
}

func TestAssetTemplatesService_ListResolvesLocalizedName(t *testing.T) {
	repo := &mockCatalogRepo{
		total: 1,
		items: []entities.CatalogItem{{Slug: "temperature", NameEN: "Temperature", NamePT: "Temperatura"}},
	}
	svc := New(di.AssetTemplatesServiceDI{Repo: repo})

	// pt-BR resolves the localized name; the default request falls back to en-US.
	ptRes, err := svc.List(context.Background(), &dtos.CatalogQuery{Lang: "pt-BR"})
	if err != nil {
		t.Fatalf("List pt-BR: %v", err)
	}
	if ptRes.Items[0].Name != "Temperatura" {
		t.Fatalf("pt-BR name = %q, want Temperatura", ptRes.Items[0].Name)
	}
	enRes, err := svc.List(context.Background(), &dtos.CatalogQuery{Lang: "en-US"})
	if err != nil {
		t.Fatalf("List en-US: %v", err)
	}
	if enRes.Items[0].Name != "Temperature" {
		t.Fatalf("en-US name = %q, want Temperature", enRes.Items[0].Name)
	}
}

func TestAssetTemplatesService_FacetsThreadsSelectionAndMaps(t *testing.T) {
	repo := &mockCatalogRepo{
		facets: repositories.FacetSet{
			Categories: []repositories.Facet{{Value: "sensor", Label: "Sensor"}},
			Vendors:    []repositories.Facet{{Value: "disruptive-technologies", Label: "Disruptive Technologies"}},
			Models:     []repositories.Facet{{Value: "Temperature", Label: "Temperature"}, {Value: "CO2", Label: "CO2"}},
			Versions:   []repositories.Facet{{Value: "1", Label: "1"}},
		},
	}
	svc := New(di.AssetTemplatesServiceDI{Repo: repo})

	res, err := svc.Facets(context.Background(), "disruptive-technologies", "Temperature", "pt-BR")
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	// The drill-down selection (vendor + model) and locale must reach the repository.
	if repo.lastFacetSel.Vendor != "disruptive-technologies" || repo.lastFacetSel.Model != "Temperature" || repo.lastFacetSel.Lang != "pt-BR" {
		t.Fatalf("selection not passed to repo: %+v", repo.lastFacetSel)
	}
	// The drill-down Models level must be mapped onto the wire DTO in order.
	if len(res.Models) != 2 || res.Models[0].Value != "Temperature" || res.Models[1].Value != "CO2" {
		t.Fatalf("models not mapped to DTO: %+v", res.Models)
	}
	if len(res.Categories) != 1 || len(res.Vendors) != 1 || len(res.Versions) != 1 {
		t.Fatalf("facet levels not mapped: %+v", res)
	}
}

func TestAssetTemplatesService_GetInformationMapsNotFound(t *testing.T) {
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
			repo := &mockCatalogRepo{raw: json.RawMessage(`{"name":{"en-US":"Temperature"}}`), rawErr: tt.repoErr}
			svc := New(di.AssetTemplatesServiceDI{Repo: repo})
			_, _, _, err := svc.GetInformation(context.Background(), "disruptive-technologies", "temperature")
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
