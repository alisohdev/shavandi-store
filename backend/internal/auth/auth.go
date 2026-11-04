package auth

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid         = errors.New("invalid authentication input")
	ErrCredentials     = errors.New("invalid credentials")
	ErrOTP             = errors.New("invalid one-time code")
	ErrUnauthenticated = errors.New("authentication required")
	ErrForbidden       = errors.New("administrator authentication required")
	ErrSMSUnavailable  = errors.New("SMS delivery unavailable")
	ErrNotFound        = errors.New("authentication record not found")
	ErrAdminExists     = errors.New("the administrator identity has already been provisioned")
)

type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "authentication rate limit exceeded" }

type User struct {
	ID           string `json:"id"`
	Phone        string `json:"phone"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
	Active       bool   `json:"-"`
}

type Session struct {
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
	Method    string    `json:"-"`
}

func (s Session) IsAdmin() bool {
	return s.User.Active && s.User.Role == "admin" && s.Method == "admin_password_sms"
}

type Login struct {
	Session
	Token string `json:"-"`
}

type Challenge struct {
	ID          string `json:"challenge_id"`
	ExpiresIn   int    `json:"expires_in"`
	ResendAfter int    `json:"resend_after"`
}

type OTP struct {
	IDVerifier   []byte
	Phone        string
	Purpose      string
	UserID       string
	CodeVerifier []byte
	Attempts     int32
	Delivered    bool
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
}

// SMS sends only to the supplied verified-format phone. Implementations must not log codes.
type SMS interface {
	SendOTP(context.Context, string, string) error
}

// UnavailableSMS is the only default sender. A missing provider never authenticates anyone.
type UnavailableSMS struct{}

func (UnavailableSMS) SendOTP(context.Context, string, string) error { return ErrSMSUnavailable }

type Audit struct {
	Event      string
	UserID     string
	SubjectKey []byte
	IPKey      []byte
	At         time.Time
}

type Tx interface {
	UserByPhone(context.Context, string) (User, error)
	UserByID(context.Context, string) (User, error)
	CreateCustomer(context.Context, string) (User, error)
	ProvisionAdmin(context.Context, string, string) (User, error)
	TakeRate(context.Context, []byte, time.Time, time.Duration) (time.Time, int32, error)
	LockTarget(context.Context, string, string, time.Time) (time.Time, error)
	TouchTarget(context.Context, string, string, time.Time) error
	InvalidateOTPs(context.Context, string, string, time.Time) error
	CreateOTP(context.Context, OTP, time.Time) error
	LockOTP(context.Context, []byte, time.Time) (OTP, error)
	FinishSMS(context.Context, []byte, bool, *time.Time) error
	AttemptOTP(context.Context, []byte, *time.Time) error
	CreateSession(context.Context, []byte, Session, time.Time) error
	RevokeSession(context.Context, []byte, time.Time) error
	RevokeUserSessions(context.Context, string, time.Time) error
	Audit(context.Context, Audit) error
}

type Store interface {
	WithTx(context.Context, func(Tx) error) error
	Session(context.Context, []byte, time.Time) (Session, error)
}
