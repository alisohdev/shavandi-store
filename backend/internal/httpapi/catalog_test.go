package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
)

type stubCatalogReader struct {
	categories      []catalog.Category
	products        []catalog.PublicProduct
	cursor          string
	product         catalog.PublicProduct
	productErr      error
	filter          catalog.ProductFilter
	adminProducts   []catalog.Product
	adminProduct    catalog.Product
	adminCategories []catalog.Category
}

func (reader *stubCatalogReader) ListAllCategories(context.Context) ([]catalog.Category, error) {
	return reader.adminCategories, nil
}
func (reader *stubCatalogReader) GetCategory(_ context.Context, id catalog.ID) (catalog.Category, error) {
	for _, category := range reader.adminCategories {
		if category.ID == id {
			return category, nil
		}
	}
	return catalog.Category{}, catalog.ErrNotFound
}
func (reader *stubCatalogReader) CreateCategory(_ context.Context, category catalog.Category) (catalog.Category, error) {
	for _, existing := range reader.adminCategories {
		if existing.Slug == category.Slug {
			return catalog.Category{}, catalog.ErrConflict
		}
	}
	if category.ParentID != nil {
		found := false
		for _, existing := range reader.adminCategories {
			if existing.ID == *category.ParentID {
				found = true
			}
		}
		if !found {
			return catalog.Category{}, catalog.ErrInvalid
		}
	}
	category.ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280055"
	category.Version = 1
	reader.adminCategories = append(reader.adminCategories, category)
	return category, nil
}
func (reader *stubCatalogReader) EditCategory(_ context.Context, category catalog.Category, version int64) (catalog.Category, error) {
	for i, existing := range reader.adminCategories {
		if existing.ID == category.ID {
			if existing.Version != version {
				return catalog.Category{}, catalog.ErrVersionConflict
			}
			category.Version = version + 1
			reader.adminCategories[i] = category
			return category, nil
		}
	}
	return catalog.Category{}, catalog.ErrNotFound
}
func (reader *stubCatalogReader) ChangePublication(_ context.Context, id catalog.ID, version int64, target catalog.PublicationState) (catalog.Product, error) {
	p := reader.adminProduct
	if p.ID != id {
		return catalog.Product{}, catalog.ErrNotFound
	}
	if p.Version != version {
		return catalog.Product{}, catalog.ErrVersionConflict
	}
	if target == catalog.Published {
		if err := catalog.ValidatePublishedProduct(p); err != nil {
			return catalog.Product{}, err
		}
	}
	if p.PublicationState == target || (p.PublicationState == catalog.Hidden && target == catalog.Hidden) || (p.PublicationState == catalog.Published && target == catalog.Published) {
		return catalog.Product{}, catalog.ErrInvalidTransition
	}
	if p.PublicationState == catalog.Draft && target != catalog.Published || p.PublicationState == catalog.Published && target != catalog.Hidden || p.PublicationState == catalog.Hidden && target != catalog.Published {
		return catalog.Product{}, catalog.ErrInvalidTransition
	}
	p.PublicationState = target
	p.Version++
	reader.adminProduct = p
	return p, nil
}

func TestAdminCatalogReadRoutesRequireMFAAdminAndIncludeUnpublishedProducts(t *testing.T) {
	draft := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280001", Slug: "draft", PublicationState: catalog.Draft}
	hidden := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280002", Slug: "hidden", PublicationState: catalog.Hidden}
	reader := &stubCatalogReader{adminProducts: []catalog.Product{draft, hidden}, adminProduct: draft}
	admin := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "admin", Role: "admin", Active: true}, Method: "admin_password_sms"}}
	handler := NewHandler(nil, reader, testLogger(), WithAuthentication(admin, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader))

	request := func(path string, token bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token {
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if got := request("/api/v1/admin/products", false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", got.Code)
	}
	adminList := request("/api/v1/admin/products", true)
	if adminList.Code != http.StatusOK || !strings.Contains(adminList.Body.String(), `"PublicationState":"draft"`) || !strings.Contains(adminList.Body.String(), `"PublicationState":"hidden"`) {
		t.Fatalf("admin list = %d %s", adminList.Code, adminList.Body.String())
	}
	if adminList.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admin list cache policy = %q", adminList.Header().Get("Cache-Control"))
	}
	detail := request("/api/v1/admin/products/"+string(draft.ID), true)
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"PublicationState":"draft"`) {
		t.Fatalf("admin detail = %d %s", detail.Code, detail.Body.String())
	}
	missing := request("/api/v1/admin/products/0192bbd1-9e2d-4a45-9baf-37f4d6280003", true)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing detail status = %d: %s", missing.Code, missing.Body.String())
	}

	admin.session = auth.Session{User: auth.User{ID: "customer", Role: "customer", Active: true}, Method: "customer_otp"}
	if got := request("/api/v1/admin/products", true); got.Code != http.StatusForbidden {
		t.Fatalf("customer status = %d", got.Code)
	}
	admin.session = auth.Session{User: auth.User{ID: "admin", Role: "admin", Active: true}, Method: "password"}
	if got := request("/api/v1/admin/products", true); got.Code != http.StatusForbidden {
		t.Fatalf("admin without MFA status = %d", got.Code)
	}
}

func TestAdminProductCreateAndPatchRequireBrowserSecurityAndPreserveState(t *testing.T) {
	const categoryID = "0192bbd1-9e2d-4a45-9baf-37f4d6280001"
	initial := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280002", Slug: "shirt", Name: "Shirt", Description: "Description", PrimaryCategory: categoryID, RegularPriceToman: 1000, PublicationState: catalog.Hidden, Version: 4, Variants: []catalog.Variant{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280003", SKU: "sku", Active: true}}}
	reader := &stubCatalogReader{adminProduct: initial}
	authn := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	handler := NewHandler(nil, reader, testLogger(), WithAuthentication(authn, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader))
	post := func(body, origin, csrf string, role string) *httptest.ResponseRecorder {
		authn.session.User.Role = role
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	valid := `{"name":"New shirt","slug":"new-shirt","description":"Details","category_id":"` + categoryID + `","regular_price_toman":1200}`
	if got := post(valid, "https://store.example", "signed-csrf", "admin"); got.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", got.Code, got.Body.String())
	}
	if reader.adminProduct.PublicationState != catalog.Draft || reader.adminProduct.ID == "" {
		t.Fatalf("created product = %#v", reader.adminProduct)
	}
	reader.adminProduct = initial
	patchBody := `{"version":4,"name":"Edited shirt"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+string(initial.ID), strings.NewReader(patchBody))
	req.Header.Set("Origin", "https://store.example")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "signed-csrf")
	req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
	req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"PublicationState":"hidden"`) || reader.adminProduct.Name != "Edited shirt" {
		t.Fatalf("patch = %d %s", w.Code, w.Body.String())
	}
	if got := post(valid, "https://evil.example", "signed-csrf", "admin"); got.Code != http.StatusForbidden {
		t.Fatalf("bad origin = %d", got.Code)
	}
	if got := post(valid, "https://store.example", "bad-csrf", "admin"); got.Code != http.StatusForbidden {
		t.Fatalf("bad csrf = %d", got.Code)
	}
	if got := post(valid, "https://store.example", "signed-csrf", "customer"); got.Code != http.StatusForbidden {
		t.Fatalf("customer = %d", got.Code)
	}
	noCookie := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(valid))
	noCookie.Header.Set("Origin", "https://store.example")
	noCookie.Header.Set("Content-Type", "application/json")
	noCookie.Header.Set("X-CSRF-Token", "signed-csrf")
	noCookie.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, noCookie)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", unauth.Code)
	}
	for _, body := range []string{`{"name":"bad","slug":"bad","description":"x","category_id":"` + categoryID + `","regular_price_toman":-1}`, `{"extra":1}`, `{"name":"x"}`} {
		if got := post(body, "https://store.example", "signed-csrf", "admin"); got.Code != http.StatusBadRequest {
			t.Fatalf("invalid body status=%d body=%s", got.Code, got.Body.String())
		}
	}
}

func TestAdminVariantRoutesUseVersionedProductEditsAndRetireDefaultOnConversion(t *testing.T) {
	productID := "0192bbd1-9e2d-4a45-9baf-37f4d6280001"
	defaultID := "0192bbd1-9e2d-4a45-9baf-37f4d6280002"
	reader := &stubCatalogReader{adminProduct: catalog.Product{ID: catalog.ID(productID), Slug: "simple", Name: "Simple", Description: "Details", PrimaryCategory: "0192bbd1-9e2d-4a45-9baf-37f4d6280003", RegularPriceToman: 1000, Version: 1, PublicationState: catalog.Draft, Variants: []catalog.Variant{{ID: catalog.ID(defaultID), SKU: "default", Active: true}}}}
	authn := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	handler := NewHandler(nil, reader, testLogger(), WithAuthentication(authn, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader))
	do := func(method, path, body string, valid bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", "https://store.example")
		req.Header.Set("Content-Type", "application/json")
		if valid {
			req.Header.Set("X-CSRF-Token", "signed-csrf")
		}
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	added := do(http.MethodPost, "/api/v1/admin/products/"+productID+"/variants", `{"version":1,"size":"M","color":"Blue"}`, true)
	if added.Code != http.StatusCreated || reader.adminProduct.Version != 2 || reader.adminProduct.Variants[0].Active || reader.adminProduct.Variants[0].ID != catalog.ID(defaultID) || len(reader.adminProduct.Variants) != 2 {
		t.Fatalf("variant add/conversion = %d %s %#v", added.Code, added.Body.String(), reader.adminProduct.Variants)
	}
	variantID := string(reader.adminProduct.Variants[1].ID)
	updated := do(http.MethodPatch, "/api/v1/admin/products/"+productID+"/variants/"+variantID, `{"version":2,"size":"L","regular_price_override_toman":1500}`, true)
	if updated.Code != http.StatusOK || reader.adminProduct.Variants[1].Size == nil || *reader.adminProduct.Variants[1].Size != "L" || reader.adminProduct.Variants[1].RegularPriceOverrideToman == nil {
		t.Fatalf("variant update = %d %s", updated.Code, updated.Body.String())
	}
	deactivated := do(http.MethodDelete, "/api/v1/admin/products/"+productID+"/variants/"+variantID, `{"version":3}`, true)
	if deactivated.Code != http.StatusOK || reader.adminProduct.Variants[1].Active {
		t.Fatalf("variant deactivate = %d %s", deactivated.Code, deactivated.Body.String())
	}
	if missingCSRF := do(http.MethodDelete, "/api/v1/admin/products/"+productID+"/variants/"+variantID, `{"version":4}`, false); missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("delete without csrf=%d", missingCSRF.Code)
	}
}

func (reader *stubCatalogReader) ListAllProducts(context.Context) ([]catalog.Product, error) {
	return reader.adminProducts, nil
}
func (reader *stubCatalogReader) GetProduct(_ context.Context, id catalog.ID) (catalog.Product, error) {
	if reader.adminProduct.ID != id {
		return catalog.Product{}, catalog.ErrNotFound
	}
	return reader.adminProduct, nil
}
func (reader *stubCatalogReader) CreateProductDraft(_ context.Context, p catalog.Product) (catalog.Product, error) {
	p.Variants = []catalog.Variant{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280098", SKU: "default-test-sku", Active: true}}
	p.ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280099"
	p.Version = 1
	p.PublicationState = catalog.Draft
	if err := catalog.ValidateProduct(p); err != nil {
		return catalog.Product{}, err
	}
	reader.adminProduct = p
	reader.adminProducts = append(reader.adminProducts, p)
	return p, nil
}
func (reader *stubCatalogReader) EditProduct(_ context.Context, id catalog.ID, version int64, p catalog.Product) (catalog.Product, error) {
	if reader.adminProduct.ID != id {
		return catalog.Product{}, catalog.ErrNotFound
	}
	if reader.adminProduct.Version != version {
		return catalog.Product{}, catalog.ErrVersionConflict
	}
	variants := append([]catalog.Variant(nil), reader.adminProduct.Variants...)
	for _, change := range p.Variants {
		found := false
		for i := range variants {
			if variants[i].ID == change.ID {
				variants[i] = change
				found = true
				break
			}
		}
		if !found {
			change.ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280097"
			change.SKU = "added-test-sku"
			variants = append(variants, change)
		}
	}
	p.ID = id
	p.Version = version + 1
	p.PublicationState = reader.adminProduct.PublicationState
	p.Variants = variants
	reader.adminProduct = p
	for i := range reader.adminProducts {
		if reader.adminProducts[i].ID == id {
			reader.adminProducts[i] = p
		}
	}
	return p, nil
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

func TestAdminCategoryManagementRoutes(t *testing.T) {
	root := catalog.Category{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280001", Name: "Root", Slug: "root", Active: true, Version: 1}
	inactive := catalog.Category{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280002", Name: "Old", Slug: "old", Active: false, DisplayOrder: 4, Version: 1}
	reader := &stubCatalogReader{categories: []catalog.Category{root}, adminCategories: []catalog.Category{root, inactive}}
	authn := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	h := NewHandler(nil, reader, testLogger(), WithAuthentication(authn, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader))
	request := func(method, path, body, origin, csrf string, authCookie bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		if method != "GET" {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-CSRF-Token", csrf)
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		}
		if authCookie {
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	list := request(http.MethodGet, "/api/v1/admin/categories", "", "", "", true)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"Active":false`) {
		t.Fatalf("admin categories=%d %s", list.Code, list.Body.String())
	}
	created := request(http.MethodPost, "/api/v1/admin/categories", `{"name":"Child","slug":"child","parent_id":"`+string(root.ID)+`","display_order":2}`, "https://store.example", "signed-csrf", true)
	if created.Code != http.StatusCreated {
		t.Fatalf("create category=%d %s", created.Code, created.Body.String())
	}
	if !reader.adminCategories[2].Active {
		t.Fatal("new category should be active by default")
	}
	updated := request(http.MethodPatch, "/api/v1/admin/categories/"+string(root.ID), `{"version":1,"name":"Renamed","display_order":3}`, "https://store.example", "signed-csrf", true)
	if updated.Code != http.StatusOK || reader.adminCategories[0].Name != "Renamed" || reader.adminCategories[0].DisplayOrder != 3 {
		t.Fatalf("edit category=%d %s", updated.Code, updated.Body.String())
	}
	badParent := request(http.MethodPost, "/api/v1/admin/categories", `{"name":"Bad","slug":"bad","parent_id":"0192bbd1-9e2d-4a45-9baf-37f4d6280099"}`, "https://store.example", "signed-csrf", true)
	if badParent.Code != http.StatusBadRequest {
		t.Fatalf("invalid parent=%d", badParent.Code)
	}
	dup := request(http.MethodPost, "/api/v1/admin/categories", `{"name":"Dup","slug":"root"}`, "https://store.example", "signed-csrf", true)
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate slug=%d", dup.Code)
	}
	if got := request(http.MethodPost, "/api/v1/admin/categories", `{"name":"New","slug":"new"}`, "https://evil.example", "signed-csrf", true); got.Code != http.StatusForbidden {
		t.Fatalf("origin=%d", got.Code)
	}
	if got := request(http.MethodPost, "/api/v1/admin/categories", `{"name":"New","slug":"new"}`, "https://store.example", "wrong", true); got.Code != http.StatusForbidden {
		t.Fatalf("csrf=%d", got.Code)
	}
	authn.session.User.Role = "customer"
	if got := request(http.MethodGet, "/api/v1/admin/categories", "", "", "", true); got.Code != http.StatusForbidden {
		t.Fatalf("customer=%d", got.Code)
	}
	if got := request(http.MethodGet, "/api/v1/admin/categories", "", "", "", false); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated=%d", got.Code)
	}
	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))
	if public.Code != http.StatusOK || strings.Contains(public.Body.String(), `"slug":"old"`) {
		t.Fatalf("public categories changed: %d %s", public.Code, public.Body.String())
	}
}

func TestAdminProductPublicationRoutesValidateVersionAndTransition(t *testing.T) {
	product := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280101", Slug: "publishable", Name: "Shirt", Description: "Details", PrimaryCategory: "0192bbd1-9e2d-4a45-9baf-37f4d6280102", RegularPriceToman: 1000, PublicationState: catalog.Draft, Version: 1, Variants: []catalog.Variant{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280103", SKU: "sku", Active: true}}, Images: []catalog.Image{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280104", AssetRef: "/images/shirt.webp"}}}
	reader := &stubCatalogReader{adminProduct: product}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	h := NewHandler(nil, reader, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader))
	call := func(action string, v int, origin, csrf string, authCookie bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/"+string(product.ID)+"/"+action, strings.NewReader(`{"version":`+strconv.Itoa(v)+`}`))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		if authCookie {
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	if w := call("publish", 1, "https://store.example", "signed-csrf", true); w.Code != http.StatusOK || reader.adminProduct.PublicationState != catalog.Published {
		t.Fatalf("publish %d %s", w.Code, w.Body.String())
	}
	if w := call("hide", 2, "https://store.example", "signed-csrf", true); w.Code != http.StatusOK || reader.adminProduct.PublicationState != catalog.Hidden {
		t.Fatalf("hide %d %s", w.Code, w.Body.String())
	}
	if w := call("publish", 3, "https://store.example", "signed-csrf", true); w.Code != http.StatusOK || reader.adminProduct.PublicationState != catalog.Published {
		t.Fatalf("republish %d %s", w.Code, w.Body.String())
	}
	if w := call("hide", 1, "https://store.example", "signed-csrf", true); w.Code != http.StatusConflict {
		t.Fatalf("stale version %d", w.Code)
	}
	if w := call("hide", 4, "https://evil.example", "signed-csrf", true); w.Code != http.StatusForbidden {
		t.Fatalf("origin %d", w.Code)
	}
	if w := call("hide", 4, "https://store.example", "invalid", true); w.Code != http.StatusForbidden {
		t.Fatalf("csrf %d", w.Code)
	}
	if w := call("hide", 4, "https://store.example", "signed-csrf", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated %d", w.Code)
	}
	a.session.User.Role = "customer"
	if w := call("hide", 4, "https://store.example", "signed-csrf", true); w.Code != http.StatusForbidden {
		t.Fatalf("customer %d", w.Code)
	}
}
