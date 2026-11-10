package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/orders"
)

type ordersStub struct {
	calls int
	owner string
}

func (s *ordersStub) Create(_ context.Context, user, _, _ string) (orders.Order, error) {
	s.calls++
	s.owner = user
	return orders.Order{ID: "order-1", Status: "pending_confirmation", Lines: []orders.Line{}}, nil
}
func (s *ordersStub) List(_ context.Context, user string) ([]orders.Order, error) {
	s.owner = user
	return []orders.Order{}, nil
}
func (s *ordersStub) Cancel(_ context.Context, user, _ string) (orders.Order, error) {
	s.calls++
	s.owner = user
	return orders.Order{}, nil
}
func TestOrderRoutesOwnIdentityAndRequireCSRF(t *testing.T) {
	store := &ordersStub{}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "customer-a", Role: "customer", Active: true}}}
	h := NewHandler(nil, nil, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithOrders(store))
	call := func(body, origin, csrf string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/orders", strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"key":"key","quote_token":"quote"}`
	if w := call(body, "https://store.example", "signed-csrf", false); w.Code != 401 {
		t.Fatalf("guest: %d", w.Code)
	}
	if w := call(body, "https://evil.example", "signed-csrf", true); w.Code != 403 {
		t.Fatal("bad origin accepted")
	}
	if w := call(body, "https://store.example", "bad", true); w.Code != 403 {
		t.Fatal("bad csrf accepted")
	}
	if w := call(`{"key":"key","quote_token":"quote","user_id":"other","subtotal_toman":1}`, "https://store.example", "signed-csrf", true); w.Code != 400 {
		t.Fatal("identity or price override accepted")
	}
	if store.calls != 0 {
		t.Fatal("rejected request reached store")
	}
	if w := call(body, "https://store.example", "signed-csrf", true); w.Code != 200 || store.owner != "customer-a" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("creation: %d", w.Code)
	}
}

func (s *ordersStub) SaveDelivery(_ context.Context, user, _ string, _ orders.Delivery) (orders.Order, error) {
	s.calls++
	s.owner = user
	return orders.Order{}, nil
}

func TestDeliveryRouteRequiresOwnerSessionAndCSRF(t *testing.T) {
	store := &ordersStub{}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "customer-a", Role: "customer", Active: true}}}
	h := NewHandler(nil, nil, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithOrders(store))
	call := func(body, csrf string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/orders/order-1/delivery", strings.NewReader(body))
		r.Header.Set("Origin", "https://store.example")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		if authenticated {
			r.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := call(`{}`, "signed-csrf", false); w.Code != 401 {
		t.Fatal("guest delivery accepted")
	}
	if w := call(`{}`, "bad", true); w.Code != 403 {
		t.Fatal("invalid csrf accepted")
	}
	if w := call(`{"user_id":"other","shipping_fee_toman":0}`, "signed-csrf", true); w.Code != 400 {
		t.Fatal("caller identity or fee accepted")
	}
	if store.calls != 0 {
		t.Fatal("rejected delivery reached store")
	}
	if w := call(`{"version":0,"recipient":"Test recipient"}`, "signed-csrf", true); w.Code != 200 || store.owner != "customer-a" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("delivery identity was not from session")
	}
}
