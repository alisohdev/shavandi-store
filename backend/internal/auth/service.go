package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const otpLifetime = 5 * time.Minute
const resendCooldown = time.Minute

type Service struct {
	store         Store
	sms           SMS
	key           []byte
	now           func() time.Time
	dummyPassword string
	passwordSlots chan struct{}
}

func NewService(store Store, sms SMS, key []byte) (*Service, error) {
	if store == nil || len(key) < 32 {
		return nil, errors.New("authentication requires a store and at least 32 secret bytes")
	}
	if sms == nil {
		sms = UnavailableSMS{}
	}
	dummy, err := HashPassword("unused-dummy-credential-" + base64.RawURLEncoding.EncodeToString(key))
	if err != nil {
		return nil, err
	}
	return &Service{store: store, sms: sms, key: append([]byte(nil), key...), now: time.Now, dummyPassword: dummy, passwordSlots: make(chan struct{}, 2)}, nil
}

func (s *Service) digest(label, value string) []byte {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(label + "\x00" + value))
	return mac.Sum(nil)
}

func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func validToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 32 && base64.RawURLEncoding.EncodeToString(raw) == token
}

func (s *Service) NewCSRF() (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	return token + "." + base64.RawURLEncoding.EncodeToString(s.digest("csrf", token)), nil
}

func (s *Service) ValidCSRF(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !validToken(parts[0]) {
		return false
	}
	verifier, err := base64.RawURLEncoding.DecodeString(parts[1])
	return err == nil && hmac.Equal(verifier, s.digest("csrf", parts[0]))
}

func (s *Service) audit(ctx context.Context, tx Tx, event, phone, ip, userID string, now time.Time) error {
	return tx.Audit(ctx, Audit{Event: event, UserID: userID, SubjectKey: s.digest("subject", phone), IPKey: s.digest("ip", ip), At: now})
}

// Expected denials are committed so an attacker cannot roll back counters or audit events.
func (s *Service) limit(ctx context.Context, tx Tx, action, phone, ip string, now time.Time, phoneLimit, ipLimit int32, window time.Duration) (error, error) {
	var denied *RateLimitError
	for _, rule := range []struct {
		key   string
		limit int32
	}{{"ip:" + ip, ipLimit}, {"phone:" + phone, phoneLimit}} {
		started, attempts, err := tx.TakeRate(ctx, s.digest("rate:"+action, rule.key), now, window)
		if err != nil {
			return nil, err
		}
		if attempts > rule.limit {
			retry := started.Add(window).Sub(now)
			if denied == nil || retry > denied.RetryAfter {
				denied = &RateLimitError{RetryAfter: retry}
			}
		}
	}
	if denied != nil {
		return denied, s.audit(ctx, tx, "rate_limited", phone, ip, "", now)
	}
	return nil, nil
}

func (s *Service) RequestCustomerOTP(ctx context.Context, input, ip string) (Challenge, error) {
	phone, err := NormalizePhone(input)
	if err != nil {
		return Challenge{}, err
	}
	return s.requestOTP(ctx, phone, "customer", "", "", ip)
}

func (s *Service) BeginAdminLogin(ctx context.Context, input, password, ip string) (Challenge, error) {
	phone, err := NormalizePhone(input)
	if err != nil || len(password) == 0 || len(password) > 1024 {
		return Challenge{}, ErrCredentials
	}
	now := s.now().UTC()
	var user User
	var denied error
	err = s.store.WithTx(ctx, func(tx Tx) error {
		var err error
		denied, err = s.limit(ctx, tx, "password", phone, ip, now, 5, 30, 15*time.Minute)
		if err != nil || denied != nil {
			return err
		}
		user, err = tx.UserByPhone(ctx, phone)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	})
	if err != nil {
		return Challenge{}, err
	}
	if denied != nil {
		return Challenge{}, denied
	}
	encoded := s.dummyPassword
	if user.Role == "admin" && user.Active {
		encoded = user.PasswordHash
	}
	select {
	case s.passwordSlots <- struct{}{}:
	case <-ctx.Done():
		return Challenge{}, ctx.Err()
	}
	valid := VerifyPassword(encoded, password)
	<-s.passwordSlots
	if !valid || user.Role != "admin" || !user.Active {
		err = s.store.WithTx(ctx, func(tx Tx) error { return s.audit(ctx, tx, "password_failed", phone, ip, "", now) })
		if err != nil {
			return Challenge{}, err
		}
		return Challenge{}, ErrCredentials
	}
	return s.requestOTP(ctx, phone, "admin", user.ID, user.PasswordHash, ip)
}

func (s *Service) requestOTP(ctx context.Context, phone, purpose, userID, passwordHash, ip string) (Challenge, error) {
	id, err := newToken()
	if err != nil {
		return Challenge{}, err
	}
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return Challenge{}, err
	}
	code := fmt.Sprintf("%06d", number.Int64())
	now := s.now().UTC()
	otp := OTP{IDVerifier: s.digest("challenge", id), Phone: phone, Purpose: purpose, UserID: userID, CodeVerifier: s.digest("otp", id+":"+code), ExpiresAt: now.Add(otpLifetime)}
	var denied error
	err = s.store.WithTx(ctx, func(tx Tx) error {
		var err error
		denied, err = s.limit(ctx, tx, "otp-request", phone, ip, now, 5, 20, time.Hour)
		if err != nil || denied != nil {
			return err
		}
		last, err := tx.LockTarget(ctx, phone, purpose, now)
		if err != nil {
			return err
		}
		if retry := last.Add(resendCooldown).Sub(now); retry > 0 {
			denied = &RateLimitError{RetryAfter: retry}
			return s.audit(ctx, tx, "rate_limited", phone, ip, userID, now)
		}
		user, err := tx.UserByPhone(ctx, phone)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		// Customer issuance is identical for existing, unknown, and ineligible phones.
		// The verify transition checks the account role before it can create a session.
		if purpose == "admin" && (err != nil || user.ID != userID || user.Role != "admin" || !user.Active || user.PasswordHash != passwordHash) {
			denied = ErrCredentials
			return nil
		}
		if err := tx.TouchTarget(ctx, phone, purpose, now); err != nil {
			return err
		}
		if err := tx.InvalidateOTPs(ctx, phone, purpose, now); err != nil {
			return err
		}
		if err := tx.CreateOTP(ctx, otp, now); err != nil {
			return err
		}
		return s.audit(ctx, tx, "otp_requested", phone, ip, userID, now)
	})
	if err != nil {
		return Challenge{}, err
	}
	if denied != nil {
		return Challenge{}, denied
	}
	sendCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	sendErr := s.sms.SendOTP(sendCtx, phone, code)
	cancel()
	// If the request was cancelled, still revoke a pending challenge using a fresh bounded context.
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer finishCancel()
	err = s.store.WithTx(finishCtx, func(tx Tx) error {
		var consumed *time.Time
		event := "otp_sent"
		if sendErr != nil {
			at := s.now().UTC()
			consumed = &at
			event = "sms_failed"
		}
		if err := tx.FinishSMS(finishCtx, otp.IDVerifier, sendErr == nil, consumed); err != nil {
			return err
		}
		return s.audit(finishCtx, tx, event, phone, ip, userID, s.now().UTC())
	})
	if err != nil {
		return Challenge{}, err
	}
	if sendErr != nil {
		return Challenge{}, ErrSMSUnavailable
	}
	return Challenge{ID: id, ExpiresIn: int(otpLifetime / time.Second), ResendAfter: int(resendCooldown / time.Second)}, nil
}

func (s *Service) VerifyOTP(ctx context.Context, id, inputCode, purpose, ip, previousToken string) (Login, error) {
	code, err := normalizeCode(inputCode)
	if err != nil || !validToken(id) || (purpose != "customer" && purpose != "admin") {
		return Login{}, ErrOTP
	}
	now := s.now().UTC()
	token, err := newToken()
	if err != nil {
		return Login{}, err
	}
	var login Login
	var denied error
	err = s.store.WithTx(ctx, func(tx Tx) error {
		// The challenge key bounds unknown IDs as well as requests against a real phone.
		var err error
		denied, err = s.limit(ctx, tx, "otp-verify", id, ip, now, 15, 40, 15*time.Minute)
		if err != nil || denied != nil {
			return err
		}
		otp, err := tx.LockOTP(ctx, s.digest("challenge", id), now)
		if errors.Is(err, ErrNotFound) {
			denied = ErrOTP
			return s.audit(ctx, tx, "otp_failed", "", ip, "", now)
		}
		if err != nil {
			return err
		}
		denied, err = s.limit(ctx, tx, "otp-verify-phone", otp.Phone, ip, now, 10, 80, 15*time.Minute)
		if err != nil || denied != nil {
			return err
		}
		// A transaction may have waited for another verifier or a resend lock.
		now = s.now().UTC()
		if otp.Purpose != purpose || !otp.Delivered || otp.ConsumedAt != nil || !otp.ExpiresAt.After(now) || otp.Attempts >= 5 {
			denied = ErrOTP
			return s.audit(ctx, tx, "otp_failed", otp.Phone, ip, otp.UserID, now)
		}
		valid := subtle.ConstantTimeCompare(otp.CodeVerifier, s.digest("otp", id+":"+code)) == 1
		var consumed *time.Time
		if valid || otp.Attempts+1 >= 5 {
			consumed = &now
		}
		if err := tx.AttemptOTP(ctx, otp.IDVerifier, consumed); err != nil {
			return err
		}
		if !valid {
			denied = ErrOTP
			return s.audit(ctx, tx, "otp_failed", otp.Phone, ip, otp.UserID, now)
		}
		var user User
		if purpose == "customer" {
			user, err = tx.CreateCustomer(ctx, otp.Phone)
		} else {
			user, err = tx.UserByID(ctx, otp.UserID)
		}
		if err != nil {
			return err
		}
		if !user.Active || user.Role != purpose {
			denied = ErrOTP
			return s.audit(ctx, tx, "otp_failed", otp.Phone, ip, user.ID, now)
		}
		lifetime := 12 * time.Hour
		method := "customer_sms"
		if purpose == "admin" {
			lifetime = time.Hour
			method = "admin_password_sms"
		}
		session := Session{User: user, ExpiresAt: now.Add(lifetime), Method: method}
		if err := tx.CreateSession(ctx, s.digest("session", token), session, now); err != nil {
			return err
		}
		if validToken(previousToken) {
			if err := tx.RevokeSession(ctx, s.digest("session", previousToken), now); err != nil {
				return err
			}
		}
		if err := s.audit(ctx, tx, "login_succeeded", otp.Phone, ip, user.ID, now); err != nil {
			return err
		}
		login = Login{Session: session, Token: token}
		return nil
	})
	if err != nil {
		return Login{}, err
	}
	if denied != nil {
		return Login{}, denied
	}
	return login, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if !validToken(token) {
		return Session{}, ErrUnauthenticated
	}
	session, err := s.store.Session(ctx, s.digest("session", token), s.now().UTC())
	if errors.Is(err, ErrNotFound) {
		return Session{}, ErrUnauthenticated
	}
	return session, err
}

func (s *Service) Logout(ctx context.Context, token, ip string) error {
	if !validToken(token) {
		return nil
	}
	now := s.now().UTC()
	return s.store.WithTx(ctx, func(tx Tx) error {
		if err := tx.RevokeSession(ctx, s.digest("session", token), now); err != nil {
			return err
		}
		return s.audit(ctx, tx, "logout", "", ip, "", now)
	})
}

func (s *Service) ProvisionAdmin(ctx context.Context, input, password string) (User, error) {
	phone, err := NormalizePhone(input)
	if err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := s.now().UTC()
	var user User
	err = s.store.WithTx(ctx, func(tx Tx) error {
		// Serialize role changes with issuance and verification for either purpose.
		for _, purpose := range []string{"admin", "customer"} {
			if _, err := tx.LockTarget(ctx, phone, purpose, now); err != nil {
				return err
			}
		}
		var err error
		user, err = tx.ProvisionAdmin(ctx, phone, hash)
		if err != nil {
			return err
		}
		if err := tx.RevokeUserSessions(ctx, user.ID, now); err != nil {
			return err
		}
		for _, purpose := range []string{"admin", "customer"} {
			if err := tx.InvalidateOTPs(ctx, phone, purpose, now); err != nil {
				return err
			}
		}
		return s.audit(ctx, tx, "admin_provisioned", phone, "cli", user.ID, now)
	})
	return user, err
}
