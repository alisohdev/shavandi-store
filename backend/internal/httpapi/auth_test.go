package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shavandi/store/backend/internal/auth"
)

type fakeAuthentication struct {
	calls   int
	err     error
	session auth.Session
}

func (f *fakeAuthentication) NewCSRF() (string, error)    { return "signed-csrf", nil }
func (f *fakeAuthentication) ValidCSRF(value string) bool { return value == "signed-csrf" }
func (f *fakeAuthentication) RequestCustomerOTP(context.Context, string, string) (auth.Challenge, error) {
	f.calls++
	return auth.Challenge{ID: "opaque-challenge", ExpiresIn: 300, ResendAfter: 60}, f.err
}
func (f *fakeAuthentication) BeginAdminLogin(context.Context, string, string, string) (auth.Challenge, error) {
	f.calls++
	return auth.Challenge{ID: "opaque-challenge", ExpiresIn: 300, ResendAfter: 60}, f.err
}
func (f *fakeAuthentication) VerifyOTP(context.Context, string, string, string, string, string) (auth.Login, error) {
	f.calls++
	return auth.Login{Session: f.session, Token: "private-session-token"}, f.err
}
func (f *fakeAuthentication) Authenticate(_ context.Context, token string) (auth.Session, error) {
	if token != "private-session-token" {
		return auth.Session{}, auth.ErrUnauthenticated
	}
	return f.session, f.err
}
func (f *fakeAuthentication) Logout(context.Context, string, string) error { f.calls++; return f.err }

func authTestHandler(f *fakeAuthentication) http.Handler {
	return NewHandler(nil, nil, testLogger(), WithAuthentication(f, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}))
}
func authPost(path, body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Origin", "https://store.example")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", "signed-csrf")
	request.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
	return request
}

func TestAuthRequiresExactOriginAndSignedCSRFCookie(t *testing.T) {
	tests := []struct {
		name   string
		change func(*http.Request)
		code   string
	}{
		{"missing Origin", func(r *http.Request) { r.Header.Del("Origin") }, "origin_not_allowed"},
		{"subdomain suffix", func(r *http.Request) { r.Header.Set("Origin", "https://store.example.evil") }, "origin_not_allowed"},
		{"different port", func(r *http.Request) { r.Header.Set("Origin", "https://store.example:444") }, "origin_not_allowed"},
		{"missing CSRF", func(r *http.Request) { r.Header.Del("X-CSRF-Token") }, "csrf_invalid"},
		{"missing cookie", func(r *http.Request) { r.Header.Del("Cookie") }, "csrf_invalid"},
		{"forged CSRF pair", func(r *http.Request) {
			r.Header.Set("X-CSRF-Token", "forged")
			r.Header.Set("Cookie", "__Host-shavandi_csrf=forged")
		}, "csrf_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeAuthentication{}
			handler := authTestHandler(fake)
			request := authPost("/api/v1/auth/customer/otp/request", `{"phone":"09123456789"}`)
			test.change(request)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			assertErrorResponse(t, response, 403, test.code)
			if fake.calls != 0 {
				t.Fatal("untrusted mutation reached authentication service")
			}
			if response.Header().Get("Access-Control-Allow-Origin") == "*" {
				t.Fatal("wildcard CORS enabled")
			}
		})
	}
}

func TestAuthCSRFBootstrapAndSecureCookies(t *testing.T) {
	fake := &fakeAuthentication{session: auth.Session{User: auth.User{ID: "user", Phone: "+989123456789", Role: "customer", Active: true}, ExpiresAt: time.Now().Add(time.Hour), Method: "customer_sms"}}
	handler := authTestHandler(fake)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	if response.Code != 200 {
		t.Fatalf("same-origin GET without Origin returned %d", response.Code)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("CSRF cookie missing")
	}
	cookie := cookies[0]
	if cookie.Name != "__Host-shavandi_csrf" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" {
		t.Fatalf("unsafe CSRF cookie: %#v", cookie)
	}
	verified := httptest.NewRecorder()
	handler.ServeHTTP(verified, authPost("/api/v1/auth/customer/otp/verify", `{"challenge_id":"id","code":"123456"}`))
	if verified.Code != 200 {
		t.Fatalf("verify returned %d: %s", verified.Code, verified.Body.String())
	}
	sessionCookie := verified.Result().Cookies()[0]
	if sessionCookie.Name != "__Host-shavandi_session" || !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe session cookie")
	}
	if strings.Contains(verified.Body.String(), "private-session-token") || strings.Contains(verified.Body.String(), "auth_method") {
		t.Fatal("private session fields leaked in JSON")
	}
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/admin/session", nil)
	sessionRequest.AddCookie(sessionCookie)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, sessionRequest)
	assertErrorResponse(t, denied, 403, "forbidden")
	fake.session.User.Role = "admin"
	denied = httptest.NewRecorder()
	handler.ServeHTTP(denied, sessionRequest)
	assertErrorResponse(t, denied, 403, "forbidden")
	fake.session.Method = "admin_password_sms"
	accepted := httptest.NewRecorder()
	handler.ServeHTTP(accepted, sessionRequest)
	if accepted.Code != 200 {
		t.Fatalf("MFA admin session returned %d", accepted.Code)
	}
	logout := authPost("/api/v1/auth/logout", `{}`)
	logout.AddCookie(sessionCookie)
	ended := httptest.NewRecorder()
	handler.ServeHTTP(ended, logout)
	if ended.Code != 204 || ended.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear session cookie")
	}
}

func TestAuthRejectsUnboundedOrUnexpectedJSON(t *testing.T) {
	for _, body := range []string{`{"phone":"09123456789","role":"admin"}`, `{"phone":"09123456789"} {}`, `{"phone":"` + strings.Repeat("x", 5000) + `"}`, `null`} {
		fake := &fakeAuthentication{}
		response := httptest.NewRecorder()
		authTestHandler(fake).ServeHTTP(response, authPost("/api/v1/auth/customer/otp/request", body))
		if response.Code != 400 || fake.calls != 0 {
			t.Fatalf("unsafe JSON status=%d calls=%d", response.Code, fake.calls)
		}
	}
}

func TestAuthErrorsAndCORSPreflight(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{{auth.ErrSMSUnavailable, 503, "sms_unavailable"}, {auth.ErrOTP, 401, "invalid_otp"}, {auth.ErrCredentials, 401, "invalid_credentials"}, {&auth.RateLimitError{RetryAfter: 1500 * time.Millisecond}, 429, "rate_limited"}, {errors.New("private database error"), 503, "auth_unavailable"}} {
		response := httptest.NewRecorder()
		authTestHandler(&fakeAuthentication{err: test.err}).ServeHTTP(response, authPost("/api/v1/auth/customer/otp/request", `{"phone":"09123456789"}`))
		assertErrorResponse(t, response, test.status, test.code)
		if test.status == 429 && response.Header().Get("Retry-After") != "2" {
			t.Fatal("Retry-After is not rounded integer seconds")
		}
		if strings.Contains(response.Body.String(), "private database") {
			t.Fatal("internal error leaked")
		}
	}
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/customer/otp/request", nil)
	request.Header.Set("Origin", "https://store.example")
	request.Header.Set("Access-Control-Request-Method", "POST")
	request.Header.Set("Access-Control-Request-Headers", "Content-Type, X-CSRF-Token")
	response := httptest.NewRecorder()
	authTestHandler(&fakeAuthentication{}).ServeHTTP(response, request)
	if response.Code != 204 || response.Header().Get("Access-Control-Allow-Origin") != "https://store.example" || response.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("exact-origin preflight failed: %d", response.Code)
	}
	request.Header.Set("Origin", "https://evil.example")
	response = httptest.NewRecorder()
	authTestHandler(&fakeAuthentication{}).ServeHTTP(response, request)
	assertErrorResponse(t, response, 403, "origin_not_allowed")
}
