package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/catalog"
)

type stubCatalogReader struct {
	categories []catalog.Category
	products   []catalog.PublicProduct
	cursor     string
	product    catalog.PublicProduct
	productErr error
	filter     catalog.ProductFilter
}

func (reader *stubCatalogReader) ListActiveCategories(context.Context) ([]catalog.Category, error) {
	return reader.categories, nil
}

func (reader *stubCatalogReader) ListPublishedProducts(_ context.Context, filter catalog.ProductFilter) ([]catalog.PublicProduct, string, error) {
	reader.filter = filter
	return reader.products, reader.cursor, nil
}

func (reader *stubCatalogReader) GetPublishedProductBySlug(context.Context, string) (catalog.PublicProduct, error) {
	return reader.product, reader.productErr
}

func TestPublicCatalogRoutesUseExplicitReadContracts(t *testing.T) {
	reader := &stubCatalogReader{
		categories: []catalog.Category{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280001", Name: "Jerseys", Slug: "jerseys", Active: true}},
		products: []catalog.PublicProduct{{
			ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280002", Slug: "blue-shirt", Name: "Blue Shirt",
			Variants: []catalog.PublicVariant{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280003", Price: catalog.EffectivePrice{RegularToman: 1000}}},
		}},
		cursor:  "opaque-cursor",
		product: catalog.PublicProduct{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280002", Slug: "blue-shirt", Name: "Blue Shirt"},
	}
	handler := NewHandler(func(context.Context) error { return nil }, reader, testLogger())

	categories := httptest.NewRecorder()
	handler.ServeHTTP(categories, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))
	if categories.Code != http.StatusOK {
		t.Fatalf("categories status = %d: %s", categories.Code, categories.Body.String())
	}
	if strings.Contains(categories.Body.String(), "Version") || strings.Contains(categories.Body.String(), "CreatedAt") {
		t.Fatalf("category response leaked internal fields: %s", categories.Body.String())
	}
	var categoryBody struct {
		Categories []categoryResponse `json:"categories"`
	}
	if err := json.Unmarshal(categories.Body.Bytes(), &categoryBody); err != nil {
		t.Fatalf("decode category response: %v", err)
	}
	if len(categoryBody.Categories) != 1 || categoryBody.Categories[0].Slug != "jerseys" {
		t.Fatalf("category response = %#v", categoryBody)
	}

	products := httptest.NewRecorder()
	handler.ServeHTTP(products, httptest.NewRequest(http.MethodGet, "/api/v1/products?category=jerseys&limit=7", nil))
	if products.Code != http.StatusOK {
		t.Fatalf("products status = %d: %s", products.Code, products.Body.String())
	}
	if reader.filter.CategorySlug != "jerseys" || reader.filter.Limit != 7 || reader.filter.After != nil {
		t.Fatalf("products filter = %#v", reader.filter)
	}
	if !strings.Contains(products.Body.String(), `"next_cursor":"opaque-cursor"`) || strings.Contains(products.Body.String(), "SKU") {
		t.Fatalf("product list contract = %s", products.Body.String())
	}

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/products/blue-shirt", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"product":{"id"`) {
		t.Fatalf("product detail response = %d %s", detail.Code, detail.Body.String())
	}
}

func TestPublicCatalogRoutesRejectBadPaginationAndHideNonPublicProducts(t *testing.T) {
	reader := &stubCatalogReader{productErr: catalog.ErrNotFound}
	handler := NewHandler(func(context.Context) error { return nil }, reader, testLogger())

	for _, path := range []string{
		"/api/v1/products?limit=0",
		"/api/v1/products?limit=not-a-number",
		"/api/v1/products?cursor=broken",
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assertErrorResponse(t, response, http.StatusBadRequest, "invalid_request")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/products/draft-only", nil))
	assertErrorResponse(t, response, http.StatusNotFound, "not_found")

	mutation := httptest.NewRecorder()
	handler.ServeHTTP(mutation, httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(`{"name":"unsafe"}`)))
	assertErrorResponse(t, mutation, http.StatusMethodNotAllowed, "method_not_allowed")
}
