package httpapi

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/auth"
)

type Authentication interface {
	NewCSRF() (string, error)
	ValidCSRF(string) bool
	RequestCustomerOTP(context.Context, string, string) (auth.Challenge, error)
	BeginAdminLogin(context.Context, string, string, string) (auth.Challenge, error)
	VerifyOTP(context.Context, string, string, string, string, string) (auth.Login, error)
	Authenticate(context.Context, string) (auth.Session, error)
	Logout(context.Context, string, string) error
}

type AuthOptions struct {
	AllowedOrigins []string
	CookieSecure   bool
}
type handlerOptions struct {
	authentication Authentication
	authOptions    AuthOptions
}
type HandlerOption func(*handlerOptions)

func WithAuthentication(service Authentication, options AuthOptions) HandlerOption {
	return func(settings *handlerOptions) { settings.authentication = service; settings.authOptions = options }
}

type authAPI struct {
	service Authentication
	options AuthOptions
}
type sessionContextKey struct{}

// RequireAuthentication can protect later routes without exposing an authAPI instance.
func RequireAuthentication(service Authentication, options AuthOptions) func(http.Handler) http.Handler {
	return (&authAPI{service: service, options: options}).RequireAuthentication
}

func registerAuthRoutes(router chi.Router, service Authentication, options AuthOptions) {
	api := &authAPI{service: service, options: options}
	router.Route("/api/v1/auth", func(router chi.Router) {
		router.Use(api.browserSecurity)
		router.Use(api.boundedContext)
		router.Options("/*", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
		router.Get("/csrf", api.csrf)
		router.Post("/customer/otp/request", api.customerRequest)
		router.Post("/customer/otp/verify", api.verify("customer"))
		router.Post("/admin/login", api.adminLogin)
		router.Post("/admin/otp/verify", api.verify("admin"))
		router.With(api.RequireAuthentication).Get("/session", api.session)
		router.With(api.RequireAuthentication, RequireAdmin).Get("/admin/session", api.session)
		router.Post("/logout", api.logout)
	})
}

func (api *authAPI) boundedContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (api *authAPI) cookieNames() (string, string) {
	if api.options.CookieSecure {
		return "__Host-shavandi_session", "__Host-shavandi_csrf"
	}
	return "shavandi_dev_session", "shavandi_dev_csrf"
}

func (api *authAPI) browserSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		for _, candidate := range api.options.AllowedOrigins {
			if origin == candidate {
				allowed = true
				break
			}
		}
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if (origin != "" && !allowed) || ((r.Method == http.MethodPost || r.Method == http.MethodOptions) && !allowed) {
			writeError(w, r, http.StatusForbidden, "origin_not_allowed", "The request origin is not allowed")
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Expose-Headers", "Retry-After")
		}
		if r.Method == http.MethodOptions {
			method := r.Header.Get("Access-Control-Request-Method")
			if method != "GET" && method != "POST" {
				writeError(w, r, http.StatusForbidden, "origin_not_allowed", "The requested method is not allowed")
				return
			}
			for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				header = strings.ToLower(strings.TrimSpace(header))
				if header != "" && header != "content-type" && header != "x-csrf-token" {
					writeError(w, r, http.StatusForbidden, "origin_not_allowed", "The requested header is not allowed")
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
		}
		if api.service == nil {
			writeAuthError(w, r, auth.ErrSMSUnavailable)
			return
		}
		if r.Method == http.MethodPost {
			_, csrfName := api.cookieNames()
			cookie, err := r.Cookie(csrfName)
			token := r.Header.Get("X-CSRF-Token")
			if err != nil || !api.service.ValidCSRF(token) || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) != 1 {
				writeError(w, r, http.StatusForbidden, "csrf_invalid", "Refresh the page and try again")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (api *authAPI) setCookie(w http.ResponseWriter, name, value string, expires time.Time, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: api.options.CookieSecure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge})
}

func (api *authAPI) csrf(w http.ResponseWriter, r *http.Request) {
	_, name := api.cookieNames()
	var token string
	if cookie, err := r.Cookie(name); err == nil && api.service.ValidCSRF(cookie.Value) {
		token = cookie.Value
	}
	if token == "" {
		var err error
		token, err = api.service.NewCSRF()
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
	}
	api.setCookie(w, name, token, time.Now().Add(12*time.Hour), 12*60*60)
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": token})
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, r, http.StatusUnsupportedMediaType, "invalid_request", "Use application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	data, err := io.ReadAll(r.Body)
	if err != nil || len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The authentication request is invalid")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The authentication request is invalid")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The authentication request is invalid")
		return false
	}
	return true
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if address := net.ParseIP(host); address != nil {
		return address.String()
	}
	return "unknown"
}

func (api *authAPI) customerRequest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone string `json:"phone"`
	}
	if !decodeAuthJSON(w, r, &input) {
		return
	}
	challenge, err := api.service.RequestCustomerOTP(r.Context(), input.Phone, remoteIP(r))
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, challenge)
}

func (api *authAPI) adminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if !decodeAuthJSON(w, r, &input) {
		return
	}
	challenge, err := api.service.BeginAdminLogin(r.Context(), input.Phone, input.Password, remoteIP(r))
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, challenge)
}

func (api *authAPI) verify(purpose string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			ID   string `json:"challenge_id"`
			Code string `json:"code"`
		}
		if !decodeAuthJSON(w, r, &input) {
			return
		}
		name, _ := api.cookieNames()
		previous := ""
		if cookie, err := r.Cookie(name); err == nil {
			previous = cookie.Value
		}
		login, err := api.service.VerifyOTP(r.Context(), input.ID, input.Code, purpose, remoteIP(r), previous)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		maxAge := int(math.Ceil(time.Until(login.ExpiresAt).Seconds()))
		if maxAge < 1 {
			maxAge = 1
		}
		api.setCookie(w, name, login.Token, login.ExpiresAt, maxAge)
		writeJSON(w, http.StatusOK, login.Session)
	}
}

// RequireAuthentication reads only the opaque cookie and the live database session.
func (api *authAPI) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, _ := api.cookieNames()
		cookie, err := r.Cookie(name)
		if err != nil {
			writeAuthError(w, r, auth.ErrUnauthenticated)
			return
		}
		session, err := api.service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, session)))
	})
}

// RequireAdmin must follow authentication and checks the completed password-and-SMS assurance.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())
		if !ok {
			writeAuthError(w, r, auth.ErrUnauthenticated)
			return
		}
		if !session.IsAdmin() {
			writeAuthError(w, r, auth.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(auth.Session)
	return session, ok
}

func (api *authAPI) session(w http.ResponseWriter, r *http.Request) {
	session, _ := SessionFromContext(r.Context())
	writeJSON(w, http.StatusOK, session)
}
func (api *authAPI) logout(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if !decodeAuthJSON(w, r, &input) {
		return
	}
	name, _ := api.cookieNames()
	if cookie, err := r.Cookie(name); err == nil {
		if err := api.service.Logout(r.Context(), cookie.Value, remoteIP(r)); err != nil {
			writeAuthError(w, r, err)
			return
		}
	}
	api.setCookie(w, name, "", time.Unix(1, 0), -1)
	w.WriteHeader(http.StatusNoContent)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var limited *auth.RateLimitError
	switch {
	case errors.As(err, &limited):
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		writeError(w, r, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Please try again later")
	case errors.Is(err, auth.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Enter a valid Iranian mobile number")
	case errors.Is(err, auth.ErrCredentials):
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "The credentials are invalid")
	case errors.Is(err, auth.ErrOTP):
		writeError(w, r, http.StatusUnauthorized, "invalid_otp", "The code is invalid or expired")
	case errors.Is(err, auth.ErrUnauthenticated):
		writeError(w, r, http.StatusUnauthorized, "authentication_required", "Sign in to continue")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "forbidden", "Administrator access is required")
	case errors.Is(err, auth.ErrSMSUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "sms_unavailable", "SMS sign-in is temporarily unavailable")
	default:
		writeError(w, r, http.StatusServiceUnavailable, "auth_unavailable", "Authentication is temporarily unavailable")
	}
}
