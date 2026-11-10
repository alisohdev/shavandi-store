package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type IranPayamakProvider struct {
	APIKey, Pattern, Line, Domain string
	Client                        *http.Client
}

func (p IranPayamakProvider) SendOTP(ctx context.Context, phone, code string) error {
	body, _ := json.Marshal(map[string]any{"code": p.Pattern, "attributes": map[string]string{"code": code, "code2": code, "domain": p.Domain}, "recipient": "0" + strings.TrimPrefix(phone, "+98"), "line_number": p.Line, "number_format": "english"})
	r, e := http.NewRequestWithContext(ctx, "POST", "https://api.iranpayamak.com/ws/v1/sms/pattern", bytes.NewReader(body))
	if e != nil {
		return e
	}
	r.Header.Set("Api-Key", p.APIKey)
	r.Header.Set("Content-Type", "application/json")
	var result struct {
		Status string `json:"status"`
	}
	if e = sendSMS(p.Client, r, &result); e != nil {
		return e
	}
	if result.Status != "success" {
		return ErrSMSUnavailable
	}
	return nil
}

type MeliPayamakProvider struct {
	Username, Password, Pattern string
	Client                      *http.Client
}

func (p MeliPayamakProvider) SendOTP(ctx context.Context, phone, code string) error {
	v := url.Values{"username": {p.Username}, "password": {p.Password}, "bodyId": {p.Pattern}, "text": {code + ";" + code}, "to": {"0" + strings.TrimPrefix(phone, "+98")}}
	r, e := http.NewRequestWithContext(ctx, "POST", "https://rest.payamak-panel.com/api/SendSMS/BaseServiceNumber", strings.NewReader(v.Encode()))
	if e != nil {
		return e
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var result struct {
		Value     string `json:"Value"`
		RetStatus int    `json:"RetStatus"`
	}
	if e = sendSMS(p.Client, r, &result); e != nil {
		return e
	}
	if result.RetStatus != 1 || !regexp.MustCompile(`^[0-9]{16,}$`).MatchString(result.Value) {
		return ErrSMSUnavailable
	}
	return nil
}
func sendSMS(client *http.Client, r *http.Request, target any) error {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, e := client.Do(r)
	if e != nil {
		return ErrSMSUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ErrSMSUnavailable
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(target) != nil {
		return ErrSMSUnavailable
	}
	return nil
}
func SMSFromEnv() (SMS, error) {
	get := os.Getenv
	provider := get("SMS_PROVIDER")
	if provider == "" || provider == "unavailable" {
		return UnavailableSMS{}, nil
	}
	if provider != "iranpayamak" && provider != "melipayamak" {
		return nil, errors.New("unknown SMS_PROVIDER")
	}
	domain := get("OTP_SMS_DOMAIN")
	if !regexp.MustCompile(`^[a-z0-9]+([.-][a-z0-9]+)*$`).MatchString(domain) {
		return nil, errors.New("OTP_SMS_DOMAIN must be a hostname")
	}
	if get("SMS_TEMPLATE_WEBOTP_APPROVED") != "true" {
		return nil, errors.New("approve exact origin-bound SMS template before enabling SMS")
	}
	switch provider {
	case "iranpayamak":
		p := IranPayamakProvider{APIKey: get("IRANPAYAMAK_API_KEY"), Pattern: get("IRANPAYAMAK_PATTERN_CODE"), Line: get("IRANPAYAMAK_SENDER_LINE"), Domain: domain}
		if p.APIKey == "" || p.Pattern == "" || p.Line == "" {
			return nil, errors.New("missing IranPayamak credentials")
		}
		return p, nil
	case "melipayamak":
		password := get("MELIPAYAMAK_API_KEY")
		if password == "" {
			password = get("MELIPAYAMAK_PASSWORD")
		}
		p := MeliPayamakProvider{Username: get("MELIPAYAMAK_USERNAME"), Password: password, Pattern: get("MELIPAYAMAK_PATTERN_ID")}
		if p.Username == "" || p.Password == "" || p.Pattern == "" {
			return nil, errors.New("missing MeliPayamak credentials")
		}
		return p, nil
	}
	return nil, errors.New("unknown SMS_PROVIDER")
}
