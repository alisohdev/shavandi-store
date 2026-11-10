package payments

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestZarinpalAmountsAuthorityAndVerification(t *testing.T) {
	authority := "A" + strings.Repeat("0", 35)
	mode := "request"
	code := 100
	gateway := &Zarinpal{MerchantID: "0192bbd1-9e2d-4a45-9baf-37f4d6280901", Environment: "production", PublicOrigin: "https://store.example"}
	gateway.Client = &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "payment.zarinpal.com" || !strings.HasSuffix(r.URL.Path, "/"+mode+".json") || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("wrong provider request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["amount"] != float64(2350000) || body["merchant_id"] != gateway.MerchantID {
			t.Fatal("provider amount changed")
		}
		if mode == "request" {
			if body["currency"] != "IRR" || body["callback_url"] != "https://store.example/api/v1/payments/zarinpal/callback" {
				t.Fatal("currency or callback incorrect")
			}
		} else if body["authority"] != authority {
			t.Fatal("verification authority changed")
		}
		response := `{"data":{"code":100,"authority":"` + authority + `"},"errors":[]}`
		if mode == "verify" {
			response = `{"data":{"code":` + stringCode(code) + `,"ref_id":123456789},"errors":[]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	got, err := gateway.Request(context.Background(), 2350000, "0192bbd1-9e2d-4a45-9baf-37f4d6280902")
	if err != nil || got != authority {
		t.Fatal(err)
	}
	if gateway.Redirect(got) != "https://payment.zarinpal.com/pg/StartPay/"+authority {
		t.Fatal("unsafe redirect")
	}
	mode = "verify"
	for _, value := range []int{100, 101} {
		code = value
		ref, e := gateway.Verify(context.Background(), 2350000, authority)
		if e != nil || ref != 123456789 {
			t.Fatalf("verify code %d: %v", code, e)
		}
	}
	code = -21
	if _, err = gateway.Verify(context.Background(), 2350000, authority); !errors.Is(err, ErrNotPaid) {
		t.Fatal("unpaid accepted")
	}
	gateway.Client.Transport = transport(func(*http.Request) (*http.Response, error) { return nil, errors.New("secret provider response") })
	if _, err = gateway.Request(context.Background(), 2350000, "0192bbd1-9e2d-4a45-9baf-37f4d6280902"); err != ErrUnavailable {
		t.Fatal("provider details exposed")
	}
	if gateway.Redirect("https://evil.example") != "" {
		t.Fatal("authority injection accepted")
	}
}
func stringCode(value int) string {
	if value == 100 {
		return "100"
	}
	if value == 101 {
		return "101"
	}
	return "-21"
}
func TestPaymentConfigurationDisabledAndSandboxGates(t *testing.T) {
	for _, key := range []string{"PAYMENT_PROVIDER", "ZARINPAL_MERCHANT_ID", "ZARINPAL_ENV", "PAYMENT_PUBLIC_ORIGIN", "APP_ENV"} {
		t.Setenv(key, "")
	}
	if gateway, err := FromEnvironment(); err != nil || gateway != nil {
		t.Fatal("payments not disabled by default")
	}
	t.Setenv("PAYMENT_PROVIDER", "zarinpal")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("missing merchant accepted")
	}
	t.Setenv("ZARINPAL_MERCHANT_ID", "0192bbd1-9e2d-4a45-9baf-37f4d6280901")
	t.Setenv("ZARINPAL_ENV", "sandbox")
	t.Setenv("PAYMENT_PUBLIC_ORIGIN", "http://127.0.0.1:4321")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("production sandbox accepted")
	}
	t.Setenv("APP_ENV", "development")
	if _, err := FromEnvironment(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZARINPAL_ENV", "production")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("production HTTP accepted")
	}
	t.Setenv("PAYMENT_PUBLIC_ORIGIN", "https://store.example")
	if _, err := FromEnvironment(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAYMENT_PUBLIC_ORIGIN", "https://store.example/evil")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("non-origin callback accepted")
	}
}

func TestZarinpalRejectsUnverifiedMalformedAndRedirectedResponses(t *testing.T) {
	authority := "A" + strings.Repeat("0", 35)
	for _, test := range []struct {
		status int
		body   string
	}{{200, `{"data":{"code":101}}`}, {200, `{"data":{"code":100,"ref_id":0}}`}, {200, `{"data":{"code":-21,"ref_id":123}}`}, {200, `not-json`}, {302, `{"data":{"code":100,"ref_id":123}}`}, {500, `{"data":{"code":100,"ref_id":123}}`}} {
		gateway := &Zarinpal{Environment: "production", Client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: test.status, Header: http.Header{"Location": []string{"https://evil.example"}}, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}}
		if _, err := gateway.Verify(context.Background(), 10000, authority); err == nil {
			t.Fatalf("accepted invalid payment evidence: %d %s", test.status, test.body)
		}
	}
}
