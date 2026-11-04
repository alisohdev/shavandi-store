package auth

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

type unusedStore struct{}

func (unusedStore) WithTx(context.Context, func(Tx) error) error { panic("unexpected transaction") }
func (unusedStore) Session(context.Context, []byte, time.Time) (Session, error) {
	panic("unexpected session read")
}

func TestIranianPhoneNormalization(t *testing.T) {
	for _, input := range []string{"09123456789", "9123456789", "+989123456789", "00989123456789", "۰۹۱۲ ۳۴۵ ۶۷۸۹", "٠٩١٢٣٤٥٦٧٨٩", " (+98) 912-345-6789 "} {
		got, err := NormalizePhone(input)
		if err != nil || got != "+989123456789" {
			t.Errorf("NormalizePhone(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "+98912345678", "091234567890", "+9719123456789", "02112345678", "98+9123456789", "+98912345678a", "+98912345678۹+", "09\n123456789", strings.Repeat("0", 97)} {
		if _, err := NormalizePhone(input); err == nil {
			t.Errorf("accepted invalid phone %q", input)
		}
	}
}

func TestPasswordHashIsSaltedAndBounded(t *testing.T) {
	password := "correct horse battery staple"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatal("password hash does not have the required random salt and Argon2id parameters")
	}
	if !VerifyPassword(first, password) || VerifyPassword(first, "wrong password") {
		t.Fatal("password verification failed")
	}
	for _, encoded := range []string{"", password, strings.Replace(first, "m=65536", "m=999999999", 1), strings.Replace(first, "v=19", "v=18", 1), first + "$extra"} {
		if VerifyPassword(encoded, password) {
			t.Fatal("accepted malformed password hash")
		}
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("accepted short administrator password")
	}
	if _, err := HashPassword(strings.Repeat("x", 1025)); err == nil {
		t.Fatal("accepted unbounded password")
	}
}

func TestCSRFVerifierCannotBeForgedOrMovedBetweenSecrets(t *testing.T) {
	service, err := NewService(unusedStore{}, nil, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewService(unusedStore{}, nil, bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.NewCSRF()
	if err != nil {
		t.Fatal(err)
	}
	if !service.ValidCSRF(token) || other.ValidCSRF(token) {
		t.Fatal("CSRF token is not bound to its signing secret")
	}
	for _, bad := range []string{"", token + "x", token[:20], strings.Replace(token, ".", "x", 1), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if service.ValidCSRF(bad) {
			t.Fatal("accepted forged CSRF token")
		}
	}
	if _, err := NewService(unusedStore{}, nil, []byte("short")); err == nil {
		t.Fatal("accepted short signing secret")
	}
}

func TestAdminAuthorizationRequiresPasswordAndSMSAssurance(t *testing.T) {
	for _, session := range []Session{{User: User{Role: "admin", Active: true}, Method: "customer_sms"}, {User: User{Role: "customer", Active: true}, Method: "admin_password_sms"}, {User: User{Role: "admin", Active: false}, Method: "admin_password_sms"}} {
		if session.IsAdmin() {
			t.Fatal("insufficient session authorized as administrator")
		}
	}
	if !(Session{User: User{Role: "admin", Active: true}, Method: "admin_password_sms"}).IsAdmin() {
		t.Fatal("valid administrator assurance rejected")
	}
}
