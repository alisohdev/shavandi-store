package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/cart"
)

type cartStub struct {
	user     string
	quantity int
	calls    int
}

func (s *cartStub) Get(_ context.Context, user string) (cart.Snapshot, error) {
	s.user = user
	return cart.Snapshot{Lines: []cart.Line{}}, nil
}
func (s *cartStub) Set(_ context.Context, user, variant string, quantity int) error {
	s.user = user
	s.quantity = quantity
	s.calls++
	return nil
}
func TestCartRoutesUseSessionIdentityAndBrowserGuards(t *testing.T) {
	store := &cartStub{}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "customer-a", Role: "customer", Active: true}}}
	h := NewHandler(nil, nil, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithCart(store))
	call := func(method, body, origin, csrf string, authenticated bool) *httptest.ResponseRecorder {
		path := "/api/v1/cart"
		if method == "POST" {
			path += "/lines/0192bbd1-9e2d-4a45-9baf-37f4d6280201"
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		if authenticated {
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "", "", "", false); w.Code != 401 {
		t.Fatalf("guest: %d", w.Code)
	}
	for _, body := range []string{`{}`, `{"quantity":10}`, `{"quantity":1,"user_id":"customer-b"}`, `{"quantity":1,"price":1}`} {
		if w := call("POST", body, "https://store.example", "signed-csrf", true); w.Code != 400 {
			t.Fatalf("invalid input %s: %d", body, w.Code)
		}
	}
	if w := call("POST", `{"quantity":2}`, "https://evil.example", "signed-csrf", true); w.Code != 403 {
		t.Fatalf("origin: %d", w.Code)
	}
	if w := call("POST", `{"quantity":2}`, "https://store.example", "bad-csrf", true); w.Code != 403 {
		t.Fatalf("csrf: %d", w.Code)
	}
	if store.calls != 0 {
		t.Fatal("rejected mutation reached persistence")
	}
	if w := call("POST", `{"quantity":2}`, "https://store.example", "signed-csrf", true); w.Code != 200 || store.user != "customer-a" || store.quantity != 2 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	a.session.User.ID = "customer-b"
	if w := call("GET", "", "", "", true); w.Code != 200 || store.user != "customer-b" {
		t.Fatal("cart identity not from session")
	}
}
