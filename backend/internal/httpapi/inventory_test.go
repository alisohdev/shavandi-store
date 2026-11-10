package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
)

type inventoryStub struct {
	calls int
	actor string
}

func (s *inventoryStub) SetStock(_ context.Context, _, _, actor string, _ int64, _ int, _ string) error {
	s.calls++
	s.actor = actor
	return nil
}
func TestInventoryRequiresMFAAndRejectsCallerIdentity(t *testing.T) {
	stock := &inventoryStub{}
	products := &stubCatalogReader{adminProduct: catalog.Product{ID: "product-1"}}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "admin-1", Role: "admin", Active: true}, Method: "admin_password_sms"}}
	h := NewHandler(nil, products, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(products), WithInventory(stock))
	call := func(body, csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/v1/admin/products/product-1/variants/variant-1/stock", strings.NewReader(body))
		r.Header.Set("Origin", "https://store.example")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(`{"version":1,"quantity":2,"reason":"Count","actor_id":"other"}`, "signed-csrf"); w.Code != 400 {
		t.Fatalf("identity override: %d", w.Code)
	}
	if w := call(`{"version":1,"quantity":2,"reason":"Count"}`, "bad-csrf"); w.Code != 403 {
		t.Fatalf("csrf: %d", w.Code)
	}
	a.session.Method = "customer_sms"
	if w := call(`{"version":1,"quantity":2,"reason":"Count"}`, "signed-csrf"); w.Code != 403 {
		t.Fatalf("MFA: %d", w.Code)
	}
	if stock.calls != 0 {
		t.Fatal("rejected request mutated stock")
	}
	a.session.Method = "admin_password_sms"
	if w := call(`{"version":1,"quantity":2,"reason":"Count"}`, "signed-csrf"); w.Code != 200 || stock.actor != "admin-1" {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
}
