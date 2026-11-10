package payments

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

var authorityPattern = regexp.MustCompile(`^[AS][A-Za-z0-9]{35}$`)
var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type Zarinpal struct {
	MerchantID, Environment, PublicOrigin string
	Client                                *http.Client
}

func FromEnvironment() (gateway *Zarinpal, err error) {
	provider := os.Getenv("PAYMENT_PROVIDER")
	if provider == "" || provider == "disabled" {
		return nil, nil
	}
	if provider != "zarinpal" {
		return nil, errors.New("invalid PAYMENT_PROVIDER")
	}
	gateway = &Zarinpal{MerchantID: os.Getenv("ZARINPAL_MERCHANT_ID"), Environment: os.Getenv("ZARINPAL_ENV"), PublicOrigin: os.Getenv("PAYMENT_PUBLIC_ORIGIN")}
	if !uuidPattern.MatchString(gateway.MerchantID) {
		return nil, errors.New("ZARINPAL_MERCHANT_ID must be a merchant UUID")
	}
	if gateway.Environment != "production" && gateway.Environment != "sandbox" {
		return nil, errors.New("ZARINPAL_ENV must be sandbox or production")
	}
	if gateway.Environment == "sandbox" && os.Getenv("APP_ENV") != "development" {
		return nil, errors.New("sandbox payments require APP_ENV=development")
	}
	origin, e := url.Parse(gateway.PublicOrigin)
	if e != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.Opaque != "" {
		return nil, errors.New("PAYMENT_PUBLIC_ORIGIN must be an exact origin")
	}
	if origin.Scheme != "https" && !(origin.Scheme == "http" && gateway.Environment == "sandbox" && (origin.Hostname() == "localhost" || origin.Hostname() == "127.0.0.1" || origin.Hostname() == "::1")) {
		return nil, errors.New("payment origin must use HTTPS except loopback sandbox")
	}
	return gateway, nil
}
func (z *Zarinpal) Fingerprint() string {
	sum := sha256.Sum256([]byte(z.MerchantID))
	return hex.EncodeToString(sum[:])
}
func (z *Zarinpal) base() string {
	if z.Environment == "sandbox" {
		return "https://sandbox.zarinpal.com"
	}
	return "https://payment.zarinpal.com"
}
func (z *Zarinpal) Redirect(authority string) string {
	if !authorityPattern.MatchString(authority) {
		return ""
	}
	return z.base() + "/pg/StartPay/" + authority
}
func (z *Zarinpal) call(ctx context.Context, method string, body any, target any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return ErrUnavailable
	}
	r, err := http.NewRequestWithContext(ctx, "POST", z.base()+"/pg/v4/payment/"+method+".json", bytes.NewReader(data))
	if err != nil {
		return ErrUnavailable
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	if z.Client != nil {
		client = *z.Client
		client.Timeout = 5 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(r)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return ErrUnavailable
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 65536))
	if decoder.Decode(target) != nil {
		return ErrUnavailable
	}
	return nil
}
func (z *Zarinpal) Request(ctx context.Context, amount int64, orderID string) (string, error) {
	if amount <= 0 {
		return "", ErrInvalid
	}
	var result struct {
		Data struct {
			Code      int    `json:"code"`
			Authority string `json:"authority"`
		} `json:"data"`
		Errors json.RawMessage `json:"errors"`
	}
	err := z.call(ctx, "request", map[string]any{"merchant_id": z.MerchantID, "amount": amount, "currency": "IRR", "description": "Shavandi order payment", "metadata": map[string]string{"order_id": orderID}, "callback_url": z.PublicOrigin + "/api/v1/payments/zarinpal/callback"}, &result)
	if err != nil {
		return "", err
	}
	if result.Data.Code != 100 {
		return "", ErrNotPaid
	}
	expected := byte('A')
	if z.Environment == "sandbox" {
		expected = 'S'
	}
	if !authorityPattern.MatchString(result.Data.Authority) || result.Data.Authority[0] != expected {
		return "", ErrUnavailable
	}
	return result.Data.Authority, nil
}
func (z *Zarinpal) Verify(ctx context.Context, amount int64, authority string) (int64, error) {
	if amount <= 0 || !authorityPattern.MatchString(authority) {
		return 0, ErrInvalid
	}
	var result struct {
		Data struct {
			Code  int         `json:"code"`
			RefID json.Number `json:"ref_id"`
		} `json:"data"`
	}
	err := z.call(ctx, "verify", map[string]any{"merchant_id": z.MerchantID, "amount": amount, "authority": authority}, &result)
	if err != nil {
		return 0, err
	}
	if result.Data.Code != 100 && result.Data.Code != 101 {
		return 0, ErrNotPaid
	}
	ref, err := strconv.ParseInt(result.Data.RefID.String(), 10, 64)
	if err != nil || ref <= 0 {
		return 0, ErrUnavailable
	}
	return ref, nil
}
