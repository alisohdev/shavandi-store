//go:build integration

package postgres

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/httpapi"
)

type capturingSMS struct {
	mu    sync.Mutex
	codes map[string]string
	count int
	fail  bool
}

func (sms *capturingSMS) SendOTP(_ context.Context, phone, code string) error {
	sms.mu.Lock()
	defer sms.mu.Unlock()
	sms.count++
	sms.codes[phone] = code
	if sms.fail {
		return errors.New("private-provider-error")
	}
	return nil
}
func (sms *capturingSMS) code(phone string) string {
	sms.mu.Lock()
	defer sms.mu.Unlock()
	return sms.codes[phone]
}
func testAuthKey() []byte { return bytes.Repeat([]byte{17}, 32) }
func authVerifier(label, value string) []byte {
	mac := hmac.New(sha256.New, testAuthKey())
	_, _ = mac.Write([]byte(label + "\x00" + value))
	return mac.Sum(nil)
}
func wrongOTP(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}
func authService(t *testing.T, pool *pgxpool.Pool, sender auth.SMS) *auth.Service {
	t.Helper()
	service, err := auth.NewService(NewAuthStore(pool), sender, testAuthKey())
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func releaseCooldown(t *testing.T, pool *pgxpool.Pool, phone string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "UPDATE shavandi.auth_otp_targets SET requested_at = now() - interval '61 seconds' WHERE phone = $1", phone); err != nil {
		t.Fatal(err)
	}
}
func requestOTP(t *testing.T, service *auth.Service, phone, ip string) auth.Challenge {
	t.Helper()
	challenge, err := service.RequestCustomerOTP(context.Background(), phone, ip)
	if err != nil {
		t.Fatalf("request OTP: %v", err)
	}
	if challenge.ExpiresIn != 300 || challenge.ResendAfter != 60 || len(challenge.ID) != 43 {
		t.Fatal("unexpected challenge contract")
	}
	return challenge
}

func TestRealPostgresAuthenticationTransitions(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	sms := &capturingSMS{codes: map[string]string{}}
	service := authService(t, pool, sms)
	ctx := context.Background()

	t.Run("customer verification rotation logout and verifier-only storage", func(t *testing.T) {
		phone := "+989120000001"
		ip := "192.0.2.1"
		challenge := requestOTP(t, service, phone, ip)
		var before int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM shavandi.users WHERE phone=$1", phone).Scan(&before); err != nil || before != 0 {
			t.Fatal("customer created before phone possession was verified")
		}
		code := sms.code(phone)
		if _, err := service.VerifyOTP(ctx, challenge.ID, wrongOTP(code), "customer", ip, ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatalf("wrong OTP error = %v", err)
		}
		login, err := service.VerifyOTP(ctx, challenge.ID, code, "customer", ip, "")
		if err != nil {
			t.Fatal(err)
		}
		if login.User.Phone != phone || login.User.Role != "customer" || login.IsAdmin() || len(login.Token) != 43 {
			t.Fatal("incorrect customer session")
		}
		if _, err := service.Authenticate(ctx, login.Token); err != nil {
			t.Fatal(err)
		}
		if _, err := service.VerifyOTP(ctx, challenge.ID, code, "customer", ip, ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatalf("replay error = %v", err)
		}
		var storedID, storedCode, storedToken []byte
		var attempts int
		if err := pool.QueryRow(ctx, "SELECT id_verifier,code_verifier,attempts FROM shavandi.auth_otp_challenges WHERE phone=$1", phone).Scan(&storedID, &storedCode, &attempts); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(storedID, authVerifier("challenge", challenge.ID)) || !bytes.Equal(storedCode, authVerifier("otp", challenge.ID+":"+code)) || attempts != 2 {
			t.Fatal("challenge verifier or durable attempt count is incorrect")
		}
		if err := pool.QueryRow(ctx, "SELECT token_verifier FROM shavandi.auth_sessions WHERE user_id=$1", login.User.ID).Scan(&storedToken); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(storedToken, authVerifier("session", login.Token)) || bytes.Equal(storedToken, []byte(login.Token)) {
			t.Fatal("session does not use a verifier")
		}
		releaseCooldown(t, pool, phone)
		newer := requestOTP(t, service, phone, ip)
		rotated, err := service.VerifyOTP(ctx, newer.ID, sms.code(phone), "customer", ip, login.Token)
		if err != nil {
			t.Fatal(err)
		}
		if rotated.Token == login.Token {
			t.Fatal("session token did not rotate")
		}
		if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatal("previous session survived rotation")
		}
		if err := service.Logout(ctx, rotated.Token, ip); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Authenticate(ctx, rotated.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatal("logged-out session remained valid")
		}
	})

	t.Run("concurrent correct verification is single use", func(t *testing.T) {
		phone := "+989120000002"
		challenge := requestOTP(t, service, phone, "192.0.2.2")
		code := sms.code(phone)
		start := make(chan struct{})
		results := make(chan error, 2)
		for index := 0; index < 2; index++ {
			go func(index int) {
				<-start
				_, err := service.VerifyOTP(ctx, challenge.ID, code, "customer", fmt.Sprintf("192.0.2.%d", 100+index), "")
				results <- err
			}(index)
		}
		close(start)
		successes := 0
		for index := 0; index < 2; index++ {
			err := <-results
			if err == nil {
				successes++
			} else if !errors.Is(err, auth.ErrOTP) {
				t.Fatalf("concurrent verification: %v", err)
			}
		}
		if successes != 1 {
			t.Fatalf("concurrent OTP produced %d sessions", successes)
		}
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM shavandi.auth_sessions s JOIN shavandi.users u ON u.id=s.user_id WHERE u.phone=$1", phone).Scan(&count); err != nil || count != 1 {
			t.Fatal("OTP single-use database invariant failed")
		}
	})

	t.Run("concurrent failures cannot exceed the challenge attempt bound", func(t *testing.T) {
		phone := "+989120000007"
		challenge := requestOTP(t, service, phone, "192.0.2.7")
		code := wrongOTP(sms.code(phone))
		start := make(chan struct{})
		results := make(chan error, 8)
		for index := 0; index < 8; index++ {
			go func(index int) {
				<-start
				_, err := service.VerifyOTP(ctx, challenge.ID, code, "customer", fmt.Sprintf("198.51.100.%d", index+1), "")
				results <- err
			}(index)
		}
		close(start)
		for index := 0; index < 8; index++ {
			if err := <-results; !errors.Is(err, auth.ErrOTP) {
				t.Fatalf("concurrent wrong OTP: %v", err)
			}
		}
		var attempts int
		if err := pool.QueryRow(ctx, "SELECT attempts FROM shavandi.auth_otp_challenges WHERE id_verifier=$1", authVerifier("challenge", challenge.ID)).Scan(&attempts); err != nil || attempts != 5 {
			t.Fatalf("concurrent attempts=%d, err=%v", attempts, err)
		}
	})

	t.Run("five attempts remain exhausted across replays", func(t *testing.T) {
		phone := "+989120000003"
		challenge := requestOTP(t, service, phone, "192.0.2.3")
		code := sms.code(phone)
		for attempt := 0; attempt < 5; attempt++ {
			if _, err := service.VerifyOTP(ctx, challenge.ID, wrongOTP(code), "customer", "192.0.2.3", ""); !errors.Is(err, auth.ErrOTP) {
				t.Fatalf("attempt %d: %v", attempt, err)
			}
		}
		if _, err := service.VerifyOTP(ctx, challenge.ID, code, "customer", "192.0.2.3", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatalf("exhausted OTP error = %v", err)
		}
		var attempts int
		var consumed bool
		if err := pool.QueryRow(ctx, "SELECT attempts,consumed_at IS NOT NULL FROM shavandi.auth_otp_challenges WHERE id_verifier=$1", authVerifier("challenge", challenge.ID)).Scan(&attempts, &consumed); err != nil || attempts != 5 || !consumed {
			t.Fatal("exhausted attempts were not committed")
		}
		releaseCooldown(t, pool, phone)
		replacement := requestOTP(t, service, phone, "192.0.2.33")
		for attempt := 0; attempt < 4; attempt++ {
			_, _ = service.VerifyOTP(ctx, replacement.ID, wrongOTP(sms.code(phone)), "customer", "192.0.2.33", "")
		}
		_, err := service.VerifyOTP(ctx, replacement.ID, sms.code(phone), "customer", "192.0.2.33", "")
		var limited *auth.RateLimitError
		if !errors.As(err, &limited) {
			t.Fatalf("resend reset global phone verification bound: %v", err)
		}
	})

	t.Run("cooldown resend invalidation OTP expiry and session expiry", func(t *testing.T) {
		phone := "+989120000004"
		challenge := requestOTP(t, service, phone, "192.0.2.4")
		oldCode := sms.code(phone)
		_, err := service.RequestCustomerOTP(ctx, phone, "192.0.2.4")
		var limited *auth.RateLimitError
		if !errors.As(err, &limited) || limited.RetryAfter <= 0 || limited.RetryAfter > time.Minute {
			t.Fatalf("resend cooldown: %v", err)
		}
		releaseCooldown(t, pool, phone)
		replacement := requestOTP(t, service, phone, "192.0.2.4")
		if _, err := service.VerifyOTP(ctx, challenge.ID, oldCode, "customer", "192.0.2.4", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatal("replaced OTP remained valid")
		}
		if _, err := pool.Exec(ctx, "UPDATE shavandi.auth_otp_challenges SET created_at=now()-interval '6 minutes', expires_at=now()-interval '1 minute' WHERE id_verifier=$1", authVerifier("challenge", replacement.ID)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.VerifyOTP(ctx, replacement.ID, sms.code(phone), "customer", "192.0.2.4", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatal("expired OTP remained valid")
		}
		other := "+989120000005"
		issued := requestOTP(t, service, other, "192.0.2.5")
		login, err := service.VerifyOTP(ctx, issued.ID, sms.code(other), "customer", "192.0.2.5", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE shavandi.auth_sessions SET created_at=now()-interval '13 hours',expires_at=now()-interval '1 hour' WHERE token_verifier=$1", authVerifier("session", login.Token)); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatal("expired session remained valid")
		}
	})

	t.Run("singleton administrator requires password then distinct SMS challenge", func(t *testing.T) {
		phone := "+989120000010"
		password := "correct horse battery staple"
		admin, err := service.ProvisionAdmin(ctx, phone, password)
		if err != nil {
			t.Fatal(err)
		}
		if admin.Role != "admin" || strings.Contains(admin.PasswordHash, password) || !auth.VerifyPassword(admin.PasswordHash, password) {
			t.Fatal("administrator password storage is invalid")
		}
		if _, err := service.ProvisionAdmin(ctx, "+989120000011", password); !errors.Is(err, auth.ErrAdminExists) {
			t.Fatalf("second administrator error = %v", err)
		}
		for _, input := range []string{phone, "+989120000099"} {
			if _, err := service.BeginAdminLogin(ctx, input, "wrong password", "192.0.2.10"); !errors.Is(err, auth.ErrCredentials) {
				t.Fatalf("bad/unknown admin credentials: %v", err)
			}
		}
		ordinary := requestOTP(t, service, phone, "192.0.2.10")
		if _, err := service.VerifyOTP(ctx, ordinary.ID, sms.code(phone), "customer", "192.0.2.10", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatal("customer OTP authenticated the administrator")
		}
		adminChallenge, err := service.BeginAdminLogin(ctx, phone, password, "192.0.2.10")
		if err != nil {
			t.Fatal(err)
		}
		code := sms.code(phone)
		if _, err := service.VerifyOTP(ctx, adminChallenge.ID, code, "customer", "192.0.2.10", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatal("admin OTP crossed into customer flow")
		}
		login, err := service.VerifyOTP(ctx, adminChallenge.ID, code, "admin", "192.0.2.10", "")
		if err != nil {
			t.Fatal(err)
		}
		if !login.IsAdmin() {
			t.Fatal("complete administrator MFA did not establish admin assurance")
		}
		if _, err := service.ProvisionAdmin(ctx, phone, "a new strong administrator password"); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatal("password reset did not revoke existing sessions")
		}
	})

	t.Run("disabled account request is indistinguishable and cannot sign in", func(t *testing.T) {
		phone := "+989120000020"
		challenge := requestOTP(t, service, phone, "192.0.2.20")
		login, err := service.VerifyOTP(ctx, challenge.ID, sms.code(phone), "customer", "192.0.2.20", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE shavandi.users SET active=false WHERE id=$1", login.User.ID); err != nil {
			t.Fatal(err)
		}
		releaseCooldown(t, pool, phone)
		next := requestOTP(t, service, phone, "192.0.2.20")
		if _, err := service.VerifyOTP(ctx, next.ID, sms.code(phone), "customer", "192.0.2.20", ""); !errors.Is(err, auth.ErrOTP) {
			t.Fatal("disabled account authenticated")
		}
		if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Fatal("disabled account retained session access")
		}
	})

	t.Run("provider error and default sender fail closed", func(t *testing.T) {
		failing := authService(t, pool, &capturingSMS{codes: map[string]string{}, fail: true})
		for _, phone := range []string{"+989120000030", "+989120000010"} {
			releaseCooldown(t, pool, phone)
			if _, err := failing.RequestCustomerOTP(ctx, phone, "192.0.2.30"); !errors.Is(err, auth.ErrSMSUnavailable) {
				t.Fatalf("provider failure: %v", err)
			}
		}
		unavailable := authService(t, pool, nil)
		if _, err := unavailable.RequestCustomerOTP(ctx, "+989120000031", "192.0.2.31"); !errors.Is(err, auth.ErrSMSUnavailable) {
			t.Fatalf("default sender: %v", err)
		}
		var active int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM shavandi.auth_otp_challenges WHERE phone IN ('+989120000030','+989120000031') AND (delivered OR consumed_at IS NULL)").Scan(&active); err != nil || active != 0 {
			t.Fatal("SMS failure left a usable challenge")
		}
	})

	t.Run("request limits persist across sessions and IP limits cover new phones", func(t *testing.T) {
		phone := "+989120000040"
		requestOTP(t, service, phone, "192.0.2.40")
		for attempt := 0; attempt < 4; attempt++ {
			_, _ = service.RequestCustomerOTP(ctx, phone, "192.0.2.40")
		}
		_, err := service.RequestCustomerOTP(ctx, phone, "192.0.2.41")
		var limited *auth.RateLimitError
		if !errors.As(err, &limited) || limited.RetryAfter < 59*time.Minute {
			t.Fatalf("shared phone request limit: %v", err)
		}
		for index := 0; index < 20; index++ {
			phone := fmt.Sprintf("+989120001%03d", index)
			requestOTP(t, service, phone, "192.0.2.50")
		}
		if _, err := service.RequestCustomerOTP(ctx, "+989120001099", "192.0.2.50"); !errors.As(err, &limited) {
			t.Fatalf("per-IP request limit: %v", err)
		}
		var auditCount int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM shavandi.auth_audit_events WHERE event_type='rate_limited'").Scan(&auditCount); err != nil || auditCount < 2 {
			t.Fatal("rate denials were not audited")
		}
	})
}

func TestRealPostgresAuthenticationHTTPFlow(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	sms := &capturingSMS{codes: map[string]string{}}
	service := authService(t, pool, sms)
	handler := httpapi.NewHandler(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), httpapi.WithAuthentication(service, httpapi.AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}))
	boot := httptest.NewRecorder()
	handler.ServeHTTP(boot, httptest.NewRequest(http.MethodGet, "/api/v1/auth/csrf", nil))
	var csrf struct {
		Token string `json:"csrf_token"`
	}
	if err := json.Unmarshal(boot.Body.Bytes(), &csrf); err != nil {
		t.Fatal(err)
	}
	csrfCookie := boot.Result().Cookies()[0]
	post := func(path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "https://store.example")
		request.Header.Set("X-CSRF-Token", csrf.Token)
		request.AddCookie(csrfCookie)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	requested := post("/api/v1/auth/customer/otp/request", `{"phone":"09120000060"}`, nil)
	if requested.Code != 202 {
		t.Fatalf("HTTP request OTP: %d %s", requested.Code, requested.Body.String())
	}
	var challenge auth.Challenge
	if err := json.Unmarshal(requested.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"challenge_id": challenge.ID, "code": sms.code("+989120000060")})
	verified := post("/api/v1/auth/customer/otp/verify", string(data), nil)
	if verified.Code != 200 {
		t.Fatalf("HTTP verify: %d %s", verified.Code, verified.Body.String())
	}
	cookie := verified.Result().Cookies()[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatal("unsafe production session cookie")
	}
	if strings.Contains(verified.Body.String(), cookie.Value) || strings.Contains(verified.Body.String(), sms.code("+989120000060")) {
		t.Fatal("authentication secret appeared in response JSON")
	}
	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	sessionRequest.AddCookie(cookie)
	live := httptest.NewRecorder()
	handler.ServeHTTP(live, sessionRequest)
	if live.Code != 200 {
		t.Fatalf("HTTP persisted session: %d", live.Code)
	}
	adminRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/admin/session", nil)
	adminRequest.AddCookie(cookie)
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, adminRequest)
	if denied.Code != 403 {
		t.Fatal("customer cookie reached administrator route")
	}
	loggedOut := post("/api/v1/auth/logout", `{}`, cookie)
	if loggedOut.Code != 204 {
		t.Fatalf("HTTP logout: %d", loggedOut.Code)
	}
	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, sessionRequest)
	if stale.Code != 401 {
		t.Fatal("server accepted revoked HTTP session cookie")
	}
}

type interposingAuthStore struct {
	auth.Store
	calls        int
	beforeSecond func()
}

func (store *interposingAuthStore) WithTx(ctx context.Context, action func(auth.Tx) error) error {
	store.calls++
	if store.calls == 2 {
		store.beforeSecond()
	}
	return store.Store.WithTx(ctx, action)
}

func TestRealPostgresConcurrentAdminProvisionAndPasswordChange(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "INSERT INTO shavandi.users(phone,role) VALUES('+989120000080','admin')"); err == nil {
		t.Fatal("database allowed NULL administrator password")
	}
	sms := &capturingSMS{codes: map[string]string{}}
	service := authService(t, pool, sms)
	password := "original strong administrator password"
	type result struct {
		user auth.User
		err  error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, phone := range []string{"+989120000080", "+989120000081"} {
		go func(phone string) {
			<-start
			user, err := service.ProvisionAdmin(ctx, phone, password)
			results <- result{user, err}
		}(phone)
	}
	close(start)
	var admin auth.User
	successes := 0
	for index := 0; index < 2; index++ {
		outcome := <-results
		if outcome.err == nil {
			admin = outcome.user
			successes++
		} else if !errors.Is(outcome.err, auth.ErrAdminExists) {
			t.Fatalf("concurrent provisioning: %v", outcome.err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent provisioning produced %d administrators", successes)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM shavandi.users WHERE role='admin'").Scan(&count); err != nil || count != 1 {
		t.Fatal("singleton administrator index failed")
	}
	resetPassword := "changed strong administrator password"
	wrapped := &interposingAuthStore{Store: NewAuthStore(pool), beforeSecond: func() {
		if _, err := service.ProvisionAdmin(ctx, admin.Phone, resetPassword); err != nil {
			t.Fatal(err)
		}
	}}
	interposed, err := auth.NewService(wrapped, sms, testAuthKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interposed.BeginAdminLogin(ctx, admin.Phone, password, "192.0.2.80"); !errors.Is(err, auth.ErrCredentials) {
		t.Fatalf("stale password verification survived concurrent reset: %v", err)
	}
	if sms.count != 0 {
		t.Fatal("stale verified password sent an administrator SMS")
	}
	challenge, err := service.BeginAdminLogin(ctx, admin.Phone, resetPassword, "192.0.2.80")
	if err != nil {
		t.Fatal(err)
	}
	code := sms.code(admin.Phone)
	if _, err := service.ProvisionAdmin(ctx, admin.Phone, "third strong administrator password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.VerifyOTP(ctx, challenge.ID, code, "admin", "192.0.2.80", ""); !errors.Is(err, auth.ErrOTP) {
		t.Fatal("password change did not invalidate an outstanding admin OTP")
	}
}
