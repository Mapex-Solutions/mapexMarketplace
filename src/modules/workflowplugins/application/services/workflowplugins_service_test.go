package services

import (
	"context"
	"encoding/json"
	"testing"

	customErrors "github.com/Mapex-Solutions/mapexGoKit/microservices/http/customErrors"
	status "github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"

	"mapexmarketplace/src/modules/workflowplugins/application/di"
	"mapexmarketplace/src/modules/workflowplugins/application/dtos"
	"mapexmarketplace/src/modules/workflowplugins/domain/entities"
	"mapexmarketplace/src/modules/workflowplugins/domain/repositories"
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
	asset      []byte
	assetType  string
	reloadN    int
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
func (m *mockCatalogRepo) GetEvents(_ context.Context, _, _ string) (json.RawMessage, error) {
	return m.raw, m.rawErr
}
func (m *mockCatalogRepo) GetAsset(_ context.Context, _, _, _ string) ([]byte, string, error) {
	return m.asset, m.assetType, m.rawErr
}
func (m *mockCatalogRepo) Reload(_ context.Context) (int, error) {
	return m.reloadN, nil
}

func TestWorkflowPluginsService_ListResolvesPaging(t *testing.T) {
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
			svc := New(di.WorkflowPluginsServiceDI{Repo: repo})
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

func TestWorkflowPluginsService_ListPassesFiltersAndMapsItems(t *testing.T) {
	repo := &mockCatalogRepo{
		total: 1,
		items: []entities.CatalogItem{{
			ID: "acme-http", Vendor: "acme", VendorName: "ACME", PluginID: "http", Slug: "http",
			Category: "communication", Capabilities: []string{"http"}, NodeCount: 3, HasEvents: true, NameEN: "HTTP",
		}},
	}
	svc := New(di.WorkflowPluginsServiceDI{Repo: repo})
	res, err := svc.List(context.Background(), &dtos.CatalogQuery{Category: "communication", Capability: "http", Tag: "io", Search: "htt"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repo.lastFilter.Category != "communication" || repo.lastFilter.Capability != "http" ||
		repo.lastFilter.Tag != "io" || repo.lastFilter.Search != "htt" {
		t.Fatalf("filters not passed through: %+v", repo.lastFilter)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].Slug != "http" ||
		res.Items[0].PluginID != "http" || res.Items[0].NodeCount != 3 || !res.Items[0].HasEvents {
		t.Fatalf("unexpected mapped response: %+v", res)
	}
}

func TestWorkflowPluginsService_ListResolvesLocalizedName(t *testing.T) {
	repo := &mockCatalogRepo{
		total: 1,
		items: []entities.CatalogItem{{Slug: "http", NameEN: "HTTP Request", NamePT: "Requisicao HTTP"}},
	}
	svc := New(di.WorkflowPluginsServiceDI{Repo: repo})

	// pt-BR resolves the localized name; the default request falls back to en-US.
	ptRes, err := svc.List(context.Background(), &dtos.CatalogQuery{Lang: "pt-BR"})
	if err != nil {
		t.Fatalf("List pt-BR: %v", err)
	}
	if ptRes.Items[0].Name != "Requisicao HTTP" {
		t.Fatalf("pt-BR name = %q, want Requisicao HTTP", ptRes.Items[0].Name)
	}
	enRes, err := svc.List(context.Background(), &dtos.CatalogQuery{Lang: "en-US"})
	if err != nil {
		t.Fatalf("List en-US: %v", err)
	}
	if enRes.Items[0].Name != "HTTP Request" {
		t.Fatalf("en-US name = %q, want HTTP Request", enRes.Items[0].Name)
	}
}

func TestWorkflowPluginsService_FacetsMapsCategoriesAndCapabilities(t *testing.T) {
	repo := &mockCatalogRepo{
		facets: repositories.FacetSet{
			Categories:   []repositories.Facet{{Value: "communication", Label: "Communication"}},
			Capabilities: []repositories.Facet{{Value: "http", Label: "HTTP"}, {Value: "mqtt", Label: "MQTT"}},
		},
	}
	svc := New(di.WorkflowPluginsServiceDI{Repo: repo})

	res, err := svc.Facets(context.Background())
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if len(res.Categories) != 1 || res.Categories[0].Value != "communication" {
		t.Fatalf("categories not mapped: %+v", res.Categories)
	}
	if len(res.Capabilities) != 2 || res.Capabilities[0].Value != "http" || res.Capabilities[1].Value != "mqtt" {
		t.Fatalf("capabilities not mapped: %+v", res.Capabilities)
	}
}

func TestWorkflowPluginsService_GetInformationMapsNotFound(t *testing.T) {
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
			repo := &mockCatalogRepo{raw: json.RawMessage(`{"name":"HTTP Request"}`), rawErr: tt.repoErr}
			svc := New(di.WorkflowPluginsServiceDI{Repo: repo})
			_, err := svc.GetInformation(context.Background(), "acme", "http")
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
