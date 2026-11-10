package httpapi

import (
	"context"
	"github.com/shavandi/store/backend/internal/payments"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisabledPaymentsRejectInitiationAndCallback(t *testing.T) {
	h := authTestHandler(&fakeAuthentication{})
	config := httptest.NewRecorder()
	h.ServeHTTP(config, httptest.NewRequest("GET", "/api/v1/payments/config", nil))
	if config.Code != 200 || !strings.Contains(config.Body.String(), `"enabled":false`) {
		t.Fatal("payments enabled without credentials")
	}
	callback := httptest.NewRecorder()
	h.ServeHTTP(callback, httptest.NewRequest("GET", "/api/v1/payments/zarinpal/callback?Status=OK&Authority=forged", nil))
	if callback.Code != 503 {
		t.Fatal("unconfigured callback accepted")
	}
	guest := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/orders/anything/payment", strings.NewReader(`{"delivery_version":1,"expected_total_toman":1}`))
	request.Header.Set("Origin", "https://store.example")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "signed-csrf")
	request.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
	h.ServeHTTP(guest, request)
	if guest.Code != 401 {
		t.Fatalf("unauthenticated payment=%d", guest.Code)
	}
}

// The callback is deliberately session-free; only server-to-server verification can settle it.
type callbackStore struct {
	payments.Store
	attempt payments.Attempt
	settled int
}

func (s *callbackStore) ByAuthority(_ context.Context, authority string) (payments.Attempt, error) {
	if authority != s.attempt.Authority {
		return payments.Attempt{}, payments.ErrNotFound
	}
	return s.attempt, nil
}
func (s *callbackStore) Claim(context.Context, string) (bool, error) { return true, nil }
func (s *callbackStore) ReleaseClaim(context.Context, string) error  { return nil }
func (s *callbackStore) Settle(_ context.Context, _ string, ref int64) error {
	if ref != 123 {
		return payments.ErrInvalid
	}
	s.settled++
	s.attempt.Status = "succeeded"
	return nil
}

type callbackGateway struct {
	failed bool
	calls  int
}

func (g *callbackGateway) Request(context.Context, int64, string) (string, error) {
	return "", payments.ErrUnavailable
}
func (g *callbackGateway) Redirect(string) string { return "" }
func (g *callbackGateway) Verify(context.Context, int64, string) (int64, error) {
	g.calls++
	if g.failed {
		return 0, payments.ErrNotPaid
	}
	return 123, nil
}
func TestCallbackDoesNotTrustStatusOrNeedSession(t *testing.T) {
	authority := "A" + strings.Repeat("0", 35)
	store := &callbackStore{attempt: payments.Attempt{ID: "attempt", Authority: authority, Status: "ready", AmountRial: 2350000, Environment: "production", MerchantFingerprint: "merchant"}}
	gateway := &callbackGateway{failed: true}
	service := &payments.Service{Store: store, Gateway: gateway, Environment: "production", MerchantFingerprint: "merchant"}
	h := NewHandler(nil, nil, testLogger(), WithAuthentication(&fakeAuthentication{}, AuthOptions{}), WithPayments(service, "https://store.example"))
	call := func(status, auth string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/payments/zarinpal/callback?Status="+status+"&Authority="+auth, nil))
		if w.Code != 303 || w.Header().Get("Location") != "https://store.example/orders/?payment=returned" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("callback: %d %v", w.Code, w.Header())
		}
	}
	call("NOK", authority)
	if gateway.calls != 0 || store.settled != 0 {
		t.Fatal("NOK mutated payment")
	}
	call("OK", "forged")
	if gateway.calls != 0 || store.settled != 0 {
		t.Fatal("unknown authority accepted")
	}
	call("OK", authority)
	if gateway.calls != 1 || store.settled != 0 {
		t.Fatal("OK query trusted without verification")
	}
	gateway.failed = false
	call("OK", authority)
	call("OK", authority)
	if store.settled != 1 || gateway.calls != 2 {
		t.Fatal("callback duplicate settled twice")
	}
}
