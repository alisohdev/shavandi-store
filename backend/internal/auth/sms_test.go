package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProviderContracts(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.iranpayamak.com" || r.Header.Get("Api-Key") != "secret" {
			t.Fatal("IranPayamak request")
		}
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		if body["recipient"] != "09123456789" || body["number_format"] != "english" {
			t.Fatal(body)
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{"status":"success","data":123}`))}, nil
	})}
	p := IranPayamakProvider{APIKey: "secret", Pattern: "approved", Line: "line", Domain: "better-buy.ir", Client: client}
	if e := p.SendOTP(ctx, "+989123456789", "123456"); e != nil {
		t.Fatal(e)
	}
	client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if e := r.ParseForm(); e != nil {
			t.Fatal(e)
		}
		if r.URL.Host != "rest.payamak-panel.com" || r.Form.Get("text") != "123456;123456" || r.Form.Get("bodyId") != "42" {
			t.Fatal("MeliPayamak request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"Value":"1234567890123456","RetStatus":1}`))}, nil
	})
	m := MeliPayamakProvider{Username: "user", Password: "secret", Pattern: "42", Client: client}
	if e := m.SendOTP(ctx, "+989123456789", "123456"); e != nil {
		t.Fatal(e)
	}
	for _, body := range []string{`{"RetStatus":1,"Value":"2"}`, `{"RetStatus":0,"Value":"1234567890123456"}`, `bad-json`} {
		client.Transport = roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		if e := m.SendOTP(ctx, "+989123456789", "123456"); e == nil {
			t.Fatal("accepted provider failure")
		}
	}
}

func TestSMSConfiguration(t *testing.T) {
	for _, key := range []string{"SMS_PROVIDER", "OTP_SMS_DOMAIN", "SMS_TEMPLATE_WEBOTP_APPROVED", "IRANPAYAMAK_API_KEY", "IRANPAYAMAK_PATTERN_CODE", "IRANPAYAMAK_SENDER_LINE", "MELIPAYAMAK_USERNAME", "MELIPAYAMAK_API_KEY", "MELIPAYAMAK_PASSWORD", "MELIPAYAMAK_PATTERN_ID"} {
		t.Setenv(key, "")
	}
	if _, err := SMSFromEnv(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMS_PROVIDER", "mock")
	if _, err := SMSFromEnv(); err == nil {
		t.Fatal("production mock accepted")
	}
	t.Setenv("SMS_PROVIDER", "iranpayamak")
	if _, err := SMSFromEnv(); err == nil {
		t.Fatal("missing template accepted")
	}
	t.Setenv("OTP_SMS_DOMAIN", "store.example")
	t.Setenv("SMS_TEMPLATE_WEBOTP_APPROVED", "true")
	if _, err := SMSFromEnv(); err == nil {
		t.Fatal("missing credentials accepted")
	}
	t.Setenv("IRANPAYAMAK_API_KEY", "secret")
	t.Setenv("IRANPAYAMAK_PATTERN_CODE", "pattern")
	t.Setenv("IRANPAYAMAK_SENDER_LINE", "line")
	if _, err := SMSFromEnv(); err != nil {
		t.Fatal(err)
	}
}
