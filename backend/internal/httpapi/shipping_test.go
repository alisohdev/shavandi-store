package httpapi

import (
	"context"
	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/shipping"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type shippingStub struct {
	saved int
	all   bool
}

func (s *shippingStub) List(_ context.Context, all bool) ([]shipping.Method, error) {
	s.all = all
	return []shipping.Method{}, nil
}
func (s *shippingStub) Save(_ context.Context, input shipping.Method) (shipping.Method, error) {
	s.saved++
	return input, nil
}
func TestShippingManagementRequiresMFAAndCSRF(t *testing.T) {
	store := &shippingStub{}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "admin", Role: "admin", Active: true}, Method: "admin_password_sms"}}
	h := NewHandler(nil, nil, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithShipping(store))
	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest("GET", "/api/v1/shipping-methods", nil))
	if public.Code != 200 || store.all {
		t.Fatal("public shipping list not active-only")
	}
	call := func(csrf string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/admin/shipping-methods", strings.NewReader(`{"name":"Postal","fee_toman":75000,"active":true,"version":0}`))
		r.Header.Set("Origin", "https://store.example")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call("bad"); w.Code != 403 {
		t.Fatal("bad csrf accepted")
	}
	a.session.User.Role = "customer"
	if w := call("signed-csrf"); w.Code != 403 {
		t.Fatal("customer changed shipping")
	}
	if store.saved != 0 {
		t.Fatal("unauthorized write reached store")
	}
	a.session.User.Role = "admin"
	if w := call("signed-csrf"); w.Code != 200 || store.saved != 1 {
		t.Fatal("admin shipping save failed")
	}
}
